package photo

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ProcessingJob struct {
	UploadID string

	PhotoID uuid.UUID

	CompetitionID uuid.UUID
	StageID       *uuid.UUID
	AuthorUserID  uuid.UUID

	OriginalFilename string
	MimeType         string
	SizeBytes        int64

	OriginalBucket    string
	OriginalObjectKey string

	DerivedBucket        string
	PreviewObjectKey     string
	WatermarkedObjectKey string
}

type UploadQueue interface {
	EnqueueProcessingJob(ctx context.Context, job ProcessingJob) error
	ReadProcessingJobs(ctx context.Context, consumerName string, count int, block time.Duration) ([]QueuedProcessingJob, error)
	AckProcessingJob(ctx context.Context, job QueuedProcessingJob) error

	InitUploadStatus(ctx context.Context, uploadID string, ttl time.Duration) error
	AddUploadCounters(ctx context.Context, uploadID string, uploadedDelta, queuedDelta, totalDelta int64, ttl time.Duration) error
	MoveQueuedToProcessing(ctx context.Context, uploadID string, delta int64, ttl time.Duration) error
	MoveProcessingToCompleted(ctx context.Context, uploadID string, delta int64, ttl time.Duration) error
	MoveProcessingToFailed(ctx context.Context, uploadID string, delta int64, ttl time.Duration) error
	AddUploadError(ctx context.Context, uploadID, fileName, message string, ttl time.Duration) error
	GetUploadStatus(ctx context.Context, uploadID string) (*UploadStatusResult, error)
}

type QueuedProcessingJob struct {
	MessageID string
	Job       ProcessingJob
}
