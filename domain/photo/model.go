package photo

import (
	"time"

	"github.com/google/uuid"
)

type Variant string

const (
	VariantOriginal    Variant = "original"
	VariantPreview     Variant = "preview"
	VariantWatermarked Variant = "watermarked"
	VariantThumb       Variant = "thumb"
)

type BibSource string

const (
	BibSourceManual BibSource = "manual"
	BibSourceOCR    BibSource = "ocr"
)

type BibRecognitionStatus string

const (
	BibRecognitionStatusPending    BibRecognitionStatus = "pending"
	BibRecognitionStatusProcessing BibRecognitionStatus = "processing"
	BibRecognitionStatusCompleted  BibRecognitionStatus = "completed"
	BibRecognitionStatusNotFound   BibRecognitionStatus = "not_found"
	BibRecognitionStatusFailed     BibRecognitionStatus = "failed"
)

type Photo struct {
	ID                   uuid.UUID            `json:"id"`
	CompetitionID        uuid.UUID            `json:"competitionId"`
	StageID              *uuid.UUID           `json:"stageId,omitempty"`
	AuthorUserID         uuid.UUID            `json:"authorUserId"`
	AuthorLogin          string               `json:"authorLogin"`
	AuthorFullName       *string              `json:"authorFullName,omitempty"`
	OriginalFilename     string               `json:"originalFilename"`
	MimeType             string               `json:"mimeType"`
	SizeBytes            int64                `json:"sizeBytes"`
	DayDate              *string              `json:"dayDate,omitempty"`
	Width                *int                 `json:"width,omitempty"`
	Height               *int                 `json:"height,omitempty"`
	PrimaryBib           *string              `json:"primaryBib,omitempty"`
	BibRecognitionStatus BibRecognitionStatus `json:"bibRecognitionStatus"`
	BibRecognitionError  *string              `json:"bibRecognitionError,omitempty"`
	WatermarkRequired    bool                 `json:"watermarkRequired"`
	DeletedAt            *time.Time           `json:"deletedAt,omitempty"`
	DeletedBy            *uuid.UUID           `json:"deletedBy,omitempty"`
	CreatedAt            time.Time            `json:"createdAt"`
	UpdatedAt            time.Time            `json:"updatedAt"`

	PreviewURL          *string        `json:"previewUrl,omitempty"`
	WatermarkedURL      *string        `json:"watermarkedUrl,omitempty"`
	CanDownloadOriginal bool           `json:"canDownloadOriginal"`
	Versions            []PhotoVersion `json:"versions"`
	Bibs                []PhotoBib     `json:"bibs"`
}

type PhotoVersion struct {
	ID            uuid.UUID `json:"id"`
	PhotoID       uuid.UUID `json:"photoId"`
	Variant       Variant   `json:"variant"`
	StorageBucket string    `json:"storageBucket"`
	ObjectKey     string    `json:"objectKey"`
	MimeType      string    `json:"mimeType"`
	SizeBytes     int64     `json:"sizeBytes"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	CreatedAt     time.Time `json:"createdAt"`

	URL *string `json:"url,omitempty"`
}

type PhotoBib struct {
	ID              uuid.UUID  `json:"id"`
	PhotoID         uuid.UUID  `json:"photoId"`
	BibValue        string     `json:"bibValue"`
	NormalizedBib   string     `json:"normalizedBib"`
	Source          BibSource  `json:"source"`
	Confidence      *float64   `json:"confidence,omitempty"`
	CreatedByUserID *uuid.UUID `json:"createdByUserId,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}

type PhotoAccessInfo struct {
	PhotoID       uuid.UUID  `json:"photoId"`
	CompetitionID uuid.UUID  `json:"competitionId"`
	AuthorUserID  uuid.UUID  `json:"authorUserId"`
	OrganizerID   uuid.UUID  `json:"organizerId"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
}

type PhotoListResult struct {
	Items    []Photo `json:"items"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
	Total    int     `json:"total"`
}
