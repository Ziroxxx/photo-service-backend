package photo

import (
	"context"
	"io"
)

type RecognizedBib struct {
	Bib        string   `json:"bib"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type BibRecognitionResult struct {
	Status BibRecognitionStatus `json:"status"`

	// Старый формат: один номер.
	Bib        *string  `json:"bib,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`

	// Новый формат: несколько номеров.
	Bibs []RecognizedBib `json:"bibs,omitempty"`

	Error *string `json:"error,omitempty"`
}

type BibRecognizer interface {
	RecognizeBib(ctx context.Context, photoID string, fileName string, file io.Reader) (*BibRecognitionResult, error)
}
