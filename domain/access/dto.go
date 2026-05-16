package access

import (
	"time"

	"github.com/google/uuid"
)

type CreateGrantRequest struct {
	UserID              uuid.UUID  `json:"userId" binding:"required"`
	CanDownloadOriginal bool       `json:"canDownloadOriginal"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty"`
}

type UpdateGrantRequest struct {
	CanDownloadOriginal *bool      `json:"canDownloadOriginal,omitempty"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty"`
	ClearExpiresAt      *bool      `json:"clearExpiresAt,omitempty"`
}
