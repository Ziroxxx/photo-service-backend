package photo

import "github.com/google/uuid"

type ListPhotosFilter struct {
	CompetitionID uuid.UUID
	StageID       *uuid.UUID
	Bib           *string
	Page          int
	PageSize      int
}

type UploadPhotosRequest struct {
	StageID *uuid.UUID `json:"stageId,omitempty"`
}

type UploadPhotoItemResult struct {
	FileName string `json:"fileName"`
	Photo    *Photo `json:"photo,omitempty"`
	Error    string `json:"error,omitempty"`
}

type UploadPhotosResult struct {
	Items  []UploadPhotoItemResult `json:"items"`
	Failed []UploadPhotoItemResult `json:"failed"`
}

type UpdatePhotoRequest struct {
	StageID         *uuid.UUID `json:"stageId,omitempty"`
	ClearStageID    *bool      `json:"clearStageId,omitempty"`
	PrimaryBib      *string    `json:"primaryBib,omitempty"`
	ClearPrimaryBib *bool      `json:"clearPrimaryBib,omitempty"`
}

type AddBibRequest struct {
	BibValue string `json:"bibValue" binding:"required,max=64"`
}

type DownloadPhotosRequest struct {
	PhotoIDs []uuid.UUID `json:"photoIds" binding:"required"`
}
