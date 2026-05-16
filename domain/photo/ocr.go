package photo

import (
	"context"
	"io"
)

type BibRecognitionResult struct {
	Status     BibRecognitionStatus
	Bib        *string
	Confidence *float64
	Error      *string
}

type BibRecognizer interface {
	RecognizeBib(ctx context.Context, photoID string, fileName string, file io.Reader) (*BibRecognitionResult, error)
}
