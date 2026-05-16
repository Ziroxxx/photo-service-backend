package access

import (
	"time"

	"github.com/google/uuid"
)

type Grant struct {
	ID                  uuid.UUID  `json:"id"`
	CompetitionID       uuid.UUID  `json:"competitionId"`
	UserID              uuid.UUID  `json:"userId"`
	CanViewPhotos       bool       `json:"canViewPhotos"`
	CanDownloadOriginal bool       `json:"canDownloadOriginal"`
	GrantedByUserID     uuid.UUID  `json:"grantedByUserId"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty"`
	RevokedAt           *time.Time `json:"revokedAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	UserLogin           string     `json:"userLogin,omitempty"`
	UserFullName        string     `json:"userFullName,omitempty"`
	IsActive            bool       `json:"isActive"`
}

type EffectiveAccess struct {
	CompetitionID       uuid.UUID  `json:"competitionId"`
	UserID              uuid.UUID  `json:"userId"`
	CanViewPhotos       bool       `json:"canViewPhotos"`
	CanDownloadOriginal bool       `json:"canDownloadOriginal"`
	CanManageAccess     bool       `json:"canManageAccess"`
	Source              string     `json:"source"`
	GrantID             *uuid.UUID `json:"grantId,omitempty"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty"`
}

type CompetitionAccessResponse struct {
	Self  EffectiveAccess `json:"self"`
	Items []Grant         `json:"items"`
}
