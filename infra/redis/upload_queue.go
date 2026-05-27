package redisinfra

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"photo-service-back/domain/photo"

	"github.com/redis/go-redis/v9"
)

const (
	processingStream = "photo:processing:stream"
	consumerGroup    = "photo-workers"
)

type UploadQueue struct {
	client *redis.Client
}

func NewUploadQueue(client *redis.Client) *UploadQueue {
	return &UploadQueue{client: client}
}

func (q *UploadQueue) EnsureConsumerGroup(ctx context.Context) error {
	err := q.client.XGroupCreateMkStream(ctx, processingStream, consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

func (q *UploadQueue) EnqueueProcessingJob(ctx context.Context, job photo.ProcessingJob) error {
	raw, err := json.Marshal(job)
	if err != nil {
		return err
	}

	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: processingStream,
		Values: map[string]any{
			"job": string(raw),
		},
	}).Err()
}

func (q *UploadQueue) ReadProcessingJobs(
	ctx context.Context,
	consumerName string,
	count int,
	block time.Duration,
) ([]photo.QueuedProcessingJob, error) {
	if count <= 0 {
		count = 100
	}

	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumerName,
		Streams:  []string{processingStream, ">"},
		Count:    int64(count),
		Block:    block,
	}).Result()

	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	out := make([]photo.QueuedProcessingJob, 0)

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			raw, ok := msg.Values["job"].(string)
			if !ok {
				continue
			}

			var job photo.ProcessingJob
			if err := json.Unmarshal([]byte(raw), &job); err != nil {
				continue
			}

			out = append(out, photo.QueuedProcessingJob{
				MessageID: msg.ID,
				Job:       job,
			})
		}
	}

	return out, nil
}

func (q *UploadQueue) AckProcessingJob(ctx context.Context, job photo.QueuedProcessingJob) error {
	return q.client.XAck(ctx, processingStream, consumerGroup, job.MessageID).Err()
}

func statusKey(uploadID string) string {
	return "photo:upload:" + uploadID + ":status"
}

func errorsKey(uploadID string) string {
	return "photo:upload:" + uploadID + ":errors"
}

func (q *UploadQueue) InitUploadStatus(ctx context.Context, uploadID string, ttl time.Duration) error {
	key := statusKey(uploadID)

	pipe := q.client.TxPipeline()
	pipe.HSetNX(ctx, key, "status", "queued")
	pipe.HSetNX(ctx, key, "total", 0)
	pipe.HSetNX(ctx, key, "uploaded", 0)
	pipe.HSetNX(ctx, key, "queued", 0)
	pipe.HSetNX(ctx, key, "processing", 0)
	pipe.HSetNX(ctx, key, "completed", 0)
	pipe.HSetNX(ctx, key, "failed", 0)
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	return err
}

func (q *UploadQueue) AddUploadCounters(
	ctx context.Context,
	uploadID string,
	uploadedDelta, queuedDelta, totalDelta int64,
	ttl time.Duration,
) error {
	key := statusKey(uploadID)

	pipe := q.client.TxPipeline()

	if uploadedDelta != 0 {
		pipe.HIncrBy(ctx, key, "uploaded", uploadedDelta)
	}
	if queuedDelta != 0 {
		pipe.HIncrBy(ctx, key, "queued", queuedDelta)
	}
	if totalDelta != 0 {
		pipe.HIncrBy(ctx, key, "total", totalDelta)
	}

	pipe.HSet(ctx, key, "status", "queued")
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	return err
}

func (q *UploadQueue) MoveQueuedToProcessing(ctx context.Context, uploadID string, delta int64, ttl time.Duration) error {
	key := statusKey(uploadID)

	pipe := q.client.TxPipeline()
	pipe.HIncrBy(ctx, key, "queued", -delta)
	pipe.HIncrBy(ctx, key, "processing", delta)
	pipe.HSet(ctx, key, "status", "processing")
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	return err
}

func (q *UploadQueue) MoveProcessingToCompleted(ctx context.Context, uploadID string, delta int64, ttl time.Duration) error {
	key := statusKey(uploadID)

	pipe := q.client.TxPipeline()
	pipe.HIncrBy(ctx, key, "processing", -delta)
	pipe.HIncrBy(ctx, key, "completed", delta)
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	return err
}

func (q *UploadQueue) MoveProcessingToFailed(ctx context.Context, uploadID string, delta int64, ttl time.Duration) error {
	key := statusKey(uploadID)

	pipe := q.client.TxPipeline()
	pipe.HIncrBy(ctx, key, "processing", -delta)
	pipe.HIncrBy(ctx, key, "failed", delta)
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	return err
}

func (q *UploadQueue) AddUploadError(ctx context.Context, uploadID, fileName, message string, ttl time.Duration) error {
	raw, _ := json.Marshal(map[string]string{
		"fileName": fileName,
		"error":    message,
	})

	pipe := q.client.TxPipeline()
	pipe.RPush(ctx, errorsKey(uploadID), string(raw))
	pipe.Expire(ctx, errorsKey(uploadID), ttl)
	pipe.Expire(ctx, statusKey(uploadID), ttl)

	_, err := pipe.Exec(ctx)
	return err
}

func (q *UploadQueue) GetUploadStatus(ctx context.Context, uploadID string) (*photo.UploadStatusResult, error) {
	values, err := q.client.HGetAll(ctx, statusKey(uploadID)).Result()
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("upload status not found")
	}

	result := &photo.UploadStatusResult{
		UploadID:   uploadID,
		Status:     values["status"],
		Total:      parseInt64(values["total"]),
		Uploaded:   parseInt64(values["uploaded"]),
		Queued:     parseInt64(values["queued"]),
		Processing: parseInt64(values["processing"]),
		Completed:  parseInt64(values["completed"]),
		Failed:     parseInt64(values["failed"]),
	}

	if result.Total > 0 && result.Completed+result.Failed >= result.Total {
		if result.Failed > 0 {
			result.Status = "completed_with_errors"
		} else {
			result.Status = "completed"
		}
	}

	return result, nil
}

func parseInt64(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

var _ photo.UploadQueue = (*UploadQueue)(nil)
