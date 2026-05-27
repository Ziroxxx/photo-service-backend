package imaging

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"photo-service-back/domain/photo"
)

type AsyncLocalStorage interface {
	OpenObject(ctx context.Context, bucket string, objectKey string) (io.ReadCloser, string, error)

	PutDerivedFromPath(
		ctx context.Context,
		objectKey string,
		filePath string,
		contentType string,
	) error

	RemoveObject(ctx context.Context, bucket string, objectKey string) error
}

type AsyncLocalBatchProcessor struct {
	local   PhotoProcessor
	storage AsyncLocalStorage
}

func NewAsyncLocalBatchProcessor(
	local PhotoProcessor,
	storage AsyncLocalStorage,
) *AsyncLocalBatchProcessor {
	return &AsyncLocalBatchProcessor{
		local:   local,
		storage: storage,
	}
}

func (p *AsyncLocalBatchProcessor) ProcessBatch(
	ctx context.Context,
	inputs []photo.ProcessInput,
) ([]*photo.ProcessedPhoto, error) {
	if len(inputs) == 0 {
		return []*photo.ProcessedPhoto{}, nil
	}

	if p.local == nil {
		return nil, fmt.Errorf("local processor is nil")
	}

	if p.storage == nil {
		return nil, fmt.Errorf("storage is nil")
	}

	results := make([]*photo.ProcessedPhoto, 0, len(inputs))

	for _, input := range inputs {
		result, err := p.processOne(ctx, input)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

func (p *AsyncLocalBatchProcessor) processOne(
	ctx context.Context,
	input photo.ProcessInput,
) (*photo.ProcessedPhoto, error) {
	if strings.TrimSpace(input.OriginalBucket) == "" {
		return nil, fmt.Errorf("original bucket is empty for %s", input.OriginalFilename)
	}

	if strings.TrimSpace(input.OriginalObjectKey) == "" {
		return nil, fmt.Errorf("original object key is empty for %s", input.OriginalFilename)
	}

	if strings.TrimSpace(input.DerivedBucket) == "" {
		return nil, fmt.Errorf("derived bucket is empty for %s", input.OriginalFilename)
	}

	if strings.TrimSpace(input.PreviewObjectKey) == "" {
		return nil, fmt.Errorf("preview object key is empty for %s", input.OriginalFilename)
	}

	if strings.TrimSpace(input.WatermarkedObjectKey) == "" {
		return nil, fmt.Errorf("watermarked object key is empty for %s", input.OriginalFilename)
	}

	tmpOriginalPath, storageContentType, err := p.downloadOriginalToTempFile(ctx, input)
	if err != nil {
		return nil, err
	}
	defer cleanupTempFile(tmpOriginalPath)

	contentType := strings.TrimSpace(input.DeclaredMimeType)
	if contentType == "" {
		contentType = strings.TrimSpace(storageContentType)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	processed, err := p.local.Process(ctx, photo.ProcessInput{
		SourcePath:       tmpOriginalPath,
		OriginalFilename: input.OriginalFilename,
		DeclaredMimeType: contentType,
	})
	if err != nil {
		return nil, err
	}

	if input.OriginalSizeBytes > 0 {
		processed.Original.SizeBytes = input.OriginalSizeBytes
	}

	defer cleanupTempFile(processed.Watermarked.TempFilePath)
	defer cleanupTempFile(processed.Preview.TempFilePath)

	if err := p.storage.PutDerivedFromPath(
		ctx,
		input.WatermarkedObjectKey,
		processed.Watermarked.TempFilePath,
		processed.Watermarked.MimeType,
	); err != nil {
		return nil, err
	}

	if err := p.storage.PutDerivedFromPath(
		ctx,
		input.PreviewObjectKey,
		processed.Preview.TempFilePath,
		processed.Preview.MimeType,
	); err != nil {
		_ = p.storage.RemoveObject(ctx, input.DerivedBucket, input.WatermarkedObjectKey)
		return nil, err
	}

	processed.Original.Bucket = input.OriginalBucket
	processed.Original.ObjectKey = input.OriginalObjectKey
	processed.Original.AlreadyUploaded = true

	processed.Watermarked.Bucket = input.DerivedBucket
	processed.Watermarked.ObjectKey = input.WatermarkedObjectKey
	processed.Watermarked.AlreadyUploaded = true
	processed.Watermarked.TempFilePath = ""

	processed.Preview.Bucket = input.DerivedBucket
	processed.Preview.ObjectKey = input.PreviewObjectKey
	processed.Preview.AlreadyUploaded = true
	processed.Preview.TempFilePath = ""

	return processed, nil
}

func (p *AsyncLocalBatchProcessor) downloadOriginalToTempFile(
	ctx context.Context,
	input photo.ProcessInput,
) (string, string, error) {
	reader, contentType, err := p.storage.OpenObject(
		ctx,
		input.OriginalBucket,
		input.OriginalObjectKey,
	)
	if err != nil {
		return "", "", err
	}
	defer reader.Close()

	ext := strings.ToLower(filepath.Ext(input.OriginalFilename))
	if ext == "" {
		ext = ".img"
	}

	tmpFile, err := os.CreateTemp("", "photo-async-original-*"+ext)
	if err != nil {
		return "", "", err
	}

	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, reader); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", "", err
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", "", err
	}

	return tmpPath, contentType, nil
}

func cleanupTempFile(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}

	_ = os.Remove(path)
}

var _ photo.BatchPhotoProcessor = (*AsyncLocalBatchProcessor)(nil)
