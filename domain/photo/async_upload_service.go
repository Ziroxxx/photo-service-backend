package photo

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	competitiondomain "photo-service-back/domain/competition"
	"photo-service-back/domain/user"

	"github.com/google/uuid"
)

type AsyncUploadService struct {
	storage      Storage
	competitions CompetitionReader
	queue        UploadQueue

	chunkMaxFiles int
	statusTTL     time.Duration
}

func NewAsyncUploadService(
	storage Storage,
	competitions CompetitionReader,
	queue UploadQueue,
	chunkMaxFiles int,
	statusTTL time.Duration,
) *AsyncUploadService {
	if chunkMaxFiles <= 0 {
		chunkMaxFiles = 200
	}
	if statusTTL <= 0 {
		statusTTL = 24 * time.Hour
	}

	return &AsyncUploadService{
		storage:       storage,
		competitions:  competitions,
		queue:         queue,
		chunkMaxFiles: chunkMaxFiles,
		statusTTL:     statusTTL,
	}
}

func (s *AsyncUploadService) EnqueueUploads(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	competitionID uuid.UUID,
	stageID *uuid.UUID,
	uploadID string,
	files []UploadFile,
) (*AsyncUploadPhotosResult, error) {
	if len(files) == 0 {
		return nil, ErrNoFilesProvided
	}
	if len(files) > s.chunkMaxFiles {
		return nil, ErrTooManyFiles
	}

	comp, err := s.competitions.GetCompetitionByID(ctx, competitionID)
	if err != nil {
		return nil, err
	}

	if !canUploadToCompetition(actorID, actorRole, comp) {
		return nil, ErrForbiddenPhotoUpload
	}

	if stageID != nil {
		if _, err := s.competitions.GetStageByID(ctx, competitionID, *stageID); err != nil {
			return nil, ErrInvalidStage
		}
	}

	if strings.TrimSpace(uploadID) == "" {
		uploadID = uuid.NewString()
	}

	if err := s.queue.InitUploadStatus(ctx, uploadID, s.statusTTL); err != nil {
		return nil, err
	}

	result := &AsyncUploadPhotosResult{
		UploadID: uploadID,
		Accepted: 0,
		Failed:   []UploadPhotoItemResult{},
		Status:   "queued",
	}

	for _, file := range files {
		if err := s.enqueueOne(ctx, comp, actorID, stageID, uploadID, file); err != nil {
			result.Failed = append(result.Failed, UploadPhotoItemResult{
				FileName: file.FileName,
				Error:    err.Error(),
			})
			_ = s.queue.AddUploadError(ctx, uploadID, file.FileName, err.Error(), s.statusTTL)
			continue
		}

		result.Accepted++
	}

	if result.Accepted > 0 {
		if err := s.queue.AddUploadCounters(
			ctx,
			uploadID,
			int64(result.Accepted),
			int64(result.Accepted),
			int64(result.Accepted),
			s.statusTTL,
		); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (s *AsyncUploadService) enqueueOne(
	ctx context.Context,
	comp *competitiondomain.Competition,
	actorID uuid.UUID,
	stageID *uuid.UUID,
	uploadID string,
	file UploadFile,
) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	ext := strings.ToLower(filepath.Ext(file.FileName))
	tmpOriginal, err := os.CreateTemp("", "photo-original-*"+ext)
	if err != nil {
		return err
	}

	tmpPath := tmpOriginal.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpOriginal, src); err != nil {
		_ = tmpOriginal.Close()
		return err
	}

	if err := tmpOriginal.Close(); err != nil {
		return err
	}

	photoID := uuid.New()

	originalKey := s.storage.BuildOriginalObjectKey(comp.Slug, photoID, file.FileName)
	watermarkedKey := s.storage.BuildWatermarkedObjectKey(comp.Slug, photoID)
	previewKey := s.storage.BuildPreviewObjectKey(comp.Slug, photoID)

	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := s.storage.PutOriginalFromPath(ctx, originalKey, tmpPath, contentType); err != nil {
		return err
	}

	sizeBytes := file.Size
	if sizeBytes <= 0 {
		if stat, err := os.Stat(tmpPath); err == nil {
			sizeBytes = stat.Size()
		}
	}

	job := ProcessingJob{
		UploadID: uploadID,

		PhotoID: photoID,

		CompetitionID: comp.ID,
		StageID:       stageID,
		AuthorUserID:  actorID,

		OriginalFilename: file.FileName,
		MimeType:         contentType,
		SizeBytes:        sizeBytes,

		OriginalBucket:    s.storage.OriginalBucket(),
		OriginalObjectKey: originalKey,

		DerivedBucket:        s.storage.DerivedBucket(),
		PreviewObjectKey:     previewKey,
		WatermarkedObjectKey: watermarkedKey,
	}

	if err := s.queue.EnqueueProcessingJob(ctx, job); err != nil {
		_ = s.storage.RemoveObject(ctx, s.storage.OriginalBucket(), originalKey)
		return err
	}

	return nil
}

func (s *AsyncUploadService) GetStatus(ctx context.Context, uploadID string) (*UploadStatusResult, error) {
	return s.queue.GetUploadStatus(ctx, uploadID)
}
