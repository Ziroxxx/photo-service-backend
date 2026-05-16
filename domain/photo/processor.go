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
	OriginalFilename string
	DeclaredMimeType string
}

type ProcessedVariant struct {
	Variant      Variant
	TempFilePath string
	MimeType     string
	SizeBytes    int64
	Width        int
	Height       int
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
