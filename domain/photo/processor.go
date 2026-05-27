package photo

import (
	"context"
)

type UploadFile struct {
	FileName    string
	ContentType string
	Size        int64
	Open        func() (ReadCloseSeek, error)
}

type ReadCloseSeek interface {
	Read(p []byte) (n int, err error)
	Close() error
}

type ProcessInput struct {
	SourcePath       string
	DeclaredMimeType string
	OriginalFilename string

	OriginalBucket    string
	OriginalObjectKey string
	OriginalSizeBytes int64

	DerivedBucket        string
	PreviewObjectKey     string
	WatermarkedObjectKey string
}

type ProcessedVariant struct {
	Variant      Variant
	TempFilePath string
	MimeType     string
	SizeBytes    int64
	Width        int
	Height       int

	// Для CUDA-режима: файл уже загружен микросервисом в MinIO.
	Bucket          string
	ObjectKey       string
	AlreadyUploaded bool
}

type ProcessedPhoto struct {
	Original    ProcessedVariant
	Watermarked ProcessedVariant
	Preview     ProcessedVariant
	DayDate     *string
}

type Processor interface {
	Process(ctx context.Context, input ProcessInput) (*ProcessedPhoto, error)
}

type BatchPhotoProcessor interface {
	ProcessBatch(ctx context.Context, inputs []ProcessInput) ([]*ProcessedPhoto, error)
}
