package photo

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

type AsyncProcessingWorker struct {
	queue     UploadQueue
	processor BatchPhotoProcessor
	repo      Repository

	batchSize     int
	flushInterval time.Duration
	statusTTL     time.Duration
}

func NewAsyncProcessingWorker(
	queue UploadQueue,
	processor BatchPhotoProcessor,
	repo Repository,
	batchSize int,
	flushInterval time.Duration,
	statusTTL time.Duration,
) *AsyncProcessingWorker {
	if batchSize <= 0 {
		batchSize = 500
	}
	if flushInterval <= 0 {
		flushInterval = 2 * time.Second
	}
	if statusTTL <= 0 {
		statusTTL = 24 * time.Hour
	}

	return &AsyncProcessingWorker{
		queue:         queue,
		processor:     processor,
		repo:          repo,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		statusTTL:     statusTTL,
	}
}

func (w *AsyncProcessingWorker) Run(ctx context.Context, consumerName string) {
	log.Printf("photo async processing worker started: %s", consumerName)

	for {
		select {
		case <-ctx.Done():
			log.Printf("photo async processing worker stopped: %s", consumerName)
			return
		default:
		}

		jobs, err := w.queue.ReadProcessingJobs(ctx, consumerName, w.batchSize, w.flushInterval)
		if err != nil {
			log.Printf("read processing jobs failed: %v", err)
			time.Sleep(time.Second)
			continue
		}

		if len(jobs) == 0 {
			continue
		}

		w.processJobs(ctx, jobs)
	}
}

func (w *AsyncProcessingWorker) processJobs(ctx context.Context, queued []QueuedProcessingJob) {
	inputs := make([]ProcessInput, 0, len(queued))

	for _, item := range queued {
		job := item.Job

		_ = w.queue.MoveQueuedToProcessing(ctx, job.UploadID, 1, w.statusTTL)

		inputs = append(inputs, ProcessInput{
			OriginalFilename: job.OriginalFilename,
			DeclaredMimeType: job.MimeType,

			OriginalBucket:    job.OriginalBucket,
			OriginalObjectKey: job.OriginalObjectKey,
			OriginalSizeBytes: job.SizeBytes,

			DerivedBucket:        job.DerivedBucket,
			PreviewObjectKey:     job.PreviewObjectKey,
			WatermarkedObjectKey: job.WatermarkedObjectKey,
		})
	}

	processedItems, err := w.processor.ProcessBatch(ctx, inputs)
	if err != nil {
		log.Printf("process async batch failed: %v", err)

		for _, item := range queued {
			job := item.Job
			_ = w.queue.MoveProcessingToFailed(ctx, job.UploadID, 1, w.statusTTL)
			_ = w.queue.AddUploadError(ctx, job.UploadID, job.OriginalFilename, err.Error(), w.statusTTL)
			_ = w.queue.AckProcessingJob(ctx, item)
		}

		return
	}

	if len(processedItems) != len(queued) {
		msg := "processed items count mismatch"

		for _, item := range queued {
			job := item.Job
			_ = w.queue.MoveProcessingToFailed(ctx, job.UploadID, 1, w.statusTTL)
			_ = w.queue.AddUploadError(ctx, job.UploadID, job.OriginalFilename, msg, w.statusTTL)
			_ = w.queue.AckProcessingJob(ctx, item)
		}

		return
	}

	for i, item := range queued {
		job := item.Job
		processed := processedItems[i]

		if err := w.createPhotoFromProcessed(ctx, job, processed); err != nil {
			_ = w.queue.MoveProcessingToFailed(ctx, job.UploadID, 1, w.statusTTL)
			_ = w.queue.AddUploadError(ctx, job.UploadID, job.OriginalFilename, err.Error(), w.statusTTL)
			_ = w.queue.AckProcessingJob(ctx, item)
			continue
		}

		_ = w.queue.MoveProcessingToCompleted(ctx, job.UploadID, 1, w.statusTTL)
		_ = w.queue.AckProcessingJob(ctx, item)
	}
}

func (w *AsyncProcessingWorker) createPhotoFromProcessed(
	ctx context.Context,
	job ProcessingJob,
	processed *ProcessedPhoto,
) error {
	width := processed.Original.Width
	height := processed.Original.Height

	p := &Photo{
		ID:                   job.PhotoID,
		CompetitionID:        job.CompetitionID,
		StageID:              job.StageID,
		AuthorUserID:         job.AuthorUserID,
		OriginalFilename:     job.OriginalFilename,
		MimeType:             processed.Original.MimeType,
		SizeBytes:            processed.Original.SizeBytes,
		DayDate:              processed.DayDate,
		Width:                &width,
		Height:               &height,
		PrimaryBib:           nil,
		BibRecognitionStatus: BibRecognitionStatusPending,
		BibRecognitionError:  nil,
		WatermarkRequired:    true,
	}

	versions := []PhotoVersion{
		{
			ID:            uuid.New(),
			PhotoID:       job.PhotoID,
			Variant:       VariantOriginal,
			StorageBucket: job.OriginalBucket,
			ObjectKey:     job.OriginalObjectKey,
			MimeType:      processed.Original.MimeType,
			SizeBytes:     processed.Original.SizeBytes,
			Width:         processed.Original.Width,
			Height:        processed.Original.Height,
		},
		{
			ID:            uuid.New(),
			PhotoID:       job.PhotoID,
			Variant:       VariantWatermarked,
			StorageBucket: job.DerivedBucket,
			ObjectKey:     job.WatermarkedObjectKey,
			MimeType:      processed.Watermarked.MimeType,
			SizeBytes:     processed.Watermarked.SizeBytes,
			Width:         processed.Watermarked.Width,
			Height:        processed.Watermarked.Height,
		},
		{
			ID:            uuid.New(),
			PhotoID:       job.PhotoID,
			Variant:       VariantPreview,
			StorageBucket: job.DerivedBucket,
			ObjectKey:     job.PreviewObjectKey,
			MimeType:      processed.Preview.MimeType,
			SizeBytes:     processed.Preview.SizeBytes,
			Width:         processed.Preview.Width,
			Height:        processed.Preview.Height,
		},
	}

	_, err := w.repo.CreatePhotoWithVersions(ctx, p, versions)
	return err
}
