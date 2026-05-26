package imaging

import (
	"context"
	"fmt"
	"os"
	"time"

	"photo-service-back/domain/photo"
)

type PresignedPhotoStorage interface {
	PresignedGetObject(ctx context.Context, bucket string, objectKey string, ttl time.Duration) (string, error)
	PresignedPutObject(ctx context.Context, bucket string, objectKey string, ttl time.Duration) (string, error)
}

type CudaBatchProcessor struct {
	client  *CudaClient
	storage PresignedPhotoStorage

	previewMaxWidth  int
	previewMaxHeight int
	jpegQuality      int

	watermarkMaxRatio float64
	watermarkOpacity  float64
	padding           int

	presignedTTL time.Duration
}

func NewCudaBatchProcessor(
	client *CudaClient,
	storage PresignedPhotoStorage,
	previewMaxWidth int,
	previewMaxHeight int,
	jpegQuality int,
	watermarkMaxRatio float64,
	watermarkOpacity float64,
	padding int,
	presignedTTL time.Duration,
) *CudaBatchProcessor {
	if previewMaxWidth <= 0 {
		previewMaxWidth = 1600
	}
	if previewMaxHeight <= 0 {
		previewMaxHeight = 1600
	}
	if jpegQuality <= 0 || jpegQuality > 100 {
		jpegQuality = 85
	}
	if watermarkMaxRatio <= 0 || watermarkMaxRatio >= 1 {
		watermarkMaxRatio = 0.22
	}
	if watermarkOpacity <= 0 || watermarkOpacity > 1 {
		watermarkOpacity = 0.35
	}
	if padding < 0 {
		padding = 20
	}
	if presignedTTL <= 0 {
		presignedTTL = 10 * time.Minute
	}

	return &CudaBatchProcessor{
		client:            client,
		storage:           storage,
		previewMaxWidth:   previewMaxWidth,
		previewMaxHeight:  previewMaxHeight,
		jpegQuality:       jpegQuality,
		watermarkMaxRatio: watermarkMaxRatio,
		watermarkOpacity:  watermarkOpacity,
		padding:           padding,
		presignedTTL:      presignedTTL,
	}
}

func (p *CudaBatchProcessor) ProcessBatch(
	ctx context.Context,
	inputs []photo.ProcessInput,
) ([]*photo.ProcessedPhoto, error) {
	if len(inputs) == 0 {
		return []*photo.ProcessedPhoto{}, nil
	}
	if p.client == nil {
		return nil, fmt.Errorf("cuda client is nil")
	}
	if p.storage == nil {
		return nil, fmt.Errorf("cuda storage is nil")
	}

	items := make([]CudaBatchItem, 0, len(inputs))

	for _, input := range inputs {
		if input.OriginalBucket == "" {
			return nil, fmt.Errorf("original bucket is empty for %s", input.OriginalFilename)
		}
		if input.OriginalObjectKey == "" {
			return nil, fmt.Errorf("original object key is empty for %s", input.OriginalFilename)
		}
		if input.DerivedBucket == "" {
			return nil, fmt.Errorf("derived bucket is empty for %s", input.OriginalFilename)
		}
		if input.PreviewObjectKey == "" {
			return nil, fmt.Errorf("preview object key is empty for %s", input.OriginalFilename)
		}
		if input.WatermarkedObjectKey == "" {
			return nil, fmt.Errorf("watermarked object key is empty for %s", input.OriginalFilename)
		}

		sourceURL, err := p.storage.PresignedGetObject(
			ctx,
			input.OriginalBucket,
			input.OriginalObjectKey,
			p.presignedTTL,
		)
		if err != nil {
			return nil, fmt.Errorf("create original presigned get url: %w", err)
		}

		previewURL, err := p.storage.PresignedPutObject(
			ctx,
			input.DerivedBucket,
			input.PreviewObjectKey,
			p.presignedTTL,
		)
		if err != nil {
			return nil, fmt.Errorf("create preview presigned put url: %w", err)
		}

		watermarkedURL, err := p.storage.PresignedPutObject(
			ctx,
			input.DerivedBucket,
			input.WatermarkedObjectKey,
			p.presignedTTL,
		)
		if err != nil {
			return nil, fmt.Errorf("create watermarked presigned put url: %w", err)
		}

		items = append(items, CudaBatchItem{
			PhotoID:           input.OriginalObjectKey,
			SourceGetURL:      sourceURL,
			PreviewPutURL:     previewURL,
			WatermarkedPutURL: watermarkedURL,
		})
	}

	response, err := p.client.ProcessBatch(ctx, CudaBatchRequest{
		PreviewMaxWidth:   p.previewMaxWidth,
		PreviewMaxHeight:  p.previewMaxHeight,
		JPEGQuality:       p.jpegQuality,
		WatermarkMaxRatio: p.watermarkMaxRatio,
		WatermarkOpacity:  p.watermarkOpacity,
		Padding:           p.padding,
		Items:             items,
	})
	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("empty cuda response")
	}

	if response.FailedCount > 0 {
		for _, item := range response.Items {
			if item.Status != "completed" {
				return nil, fmt.Errorf("cuda failed for %s: %s", item.PhotoID, item.Error)
			}
		}

		return nil, fmt.Errorf("cuda batch failed")
	}

	resultsByPhotoID := make(map[string]CudaBatchItemResult, len(response.Items))
	for _, item := range response.Items {
		resultsByPhotoID[item.PhotoID] = item
	}

	results := make([]*photo.ProcessedPhoto, 0, len(inputs))

	for _, input := range inputs {
		item, ok := resultsByPhotoID[input.OriginalObjectKey]
		if !ok {
			return nil, fmt.Errorf("cuda response item not found for %s", input.OriginalObjectKey)
		}

		if item.Status != "completed" {
			return nil, fmt.Errorf("cuda failed for %s: %s", item.PhotoID, item.Error)
		}

		originalSize := int64(0)
		if input.SourcePath != "" {
			if stat, err := os.Stat(input.SourcePath); err == nil {
				originalSize = stat.Size()
			}
		}

		originalMime := input.DeclaredMimeType
		if originalMime == "" {
			originalMime = "application/octet-stream"
		}

		results = append(results, &photo.ProcessedPhoto{
			Original: photo.ProcessedVariant{
				Variant:         photo.VariantOriginal,
				TempFilePath:    input.SourcePath,
				MimeType:        originalMime,
				SizeBytes:       originalSize,
				Width:           item.Watermarked.Width,
				Height:          item.Watermarked.Height,
				Bucket:          input.OriginalBucket,
				ObjectKey:       input.OriginalObjectKey,
				AlreadyUploaded: true,
			},
			Watermarked: photo.ProcessedVariant{
				Variant:         photo.VariantWatermarked,
				MimeType:        item.Watermarked.MimeType,
				SizeBytes:       item.Watermarked.SizeBytes,
				Width:           item.Watermarked.Width,
				Height:          item.Watermarked.Height,
				Bucket:          input.DerivedBucket,
				ObjectKey:       input.WatermarkedObjectKey,
				AlreadyUploaded: true,
			},
			Preview: photo.ProcessedVariant{
				Variant:         photo.VariantPreview,
				MimeType:        item.Preview.MimeType,
				SizeBytes:       item.Preview.SizeBytes,
				Width:           item.Preview.Width,
				Height:          item.Preview.Height,
				Bucket:          input.DerivedBucket,
				ObjectKey:       input.PreviewObjectKey,
				AlreadyUploaded: true,
			},
		})
	}

	return results, nil
}

var _ photo.BatchPhotoProcessor = (*CudaBatchProcessor)(nil)
