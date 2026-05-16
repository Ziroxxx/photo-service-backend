package competition

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
)

type Competition struct {
	ID                uuid.UUID  `json:"id"`
	Slug              string     `json:"slug"`
	Title             string     `json:"title"`
	Type              string     `json:"type"`
	City              *string    `json:"city,omitempty"`
	Venue             *string    `json:"venue,omitempty"`
	Description       *string    `json:"description,omitempty"`
	StartAt           time.Time  `json:"startAt"`
	EndAt             time.Time  `json:"endAt"`
	Timezone          string     `json:"timezone"`
	Status            Status     `json:"status"`
	OrganizerID       uuid.UUID  `json:"organizerId"`
	OrganizerLogin    string     `json:"organizerLogin"`
	OrganizerFullName *string    `json:"organizerFullName,omitempty"`
	CoverPhotoID      *uuid.UUID `json:"coverPhotoId,omitempty"`
	CoverURL          *string    `json:"coverUrl,omitempty"`
	CoverBucket       *string    `json:"-"`
	CoverObjectKey    *string    `json:"-"`
	CreatedBy         uuid.UUID  `json:"createdBy"`
	UpdatedBy         uuid.UUID  `json:"updatedBy"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	PhotosCount       int        `json:"photosCount"`

	Stages []Stage `json:"stages"`
}

type Stage struct {
	ID            uuid.UUID `json:"id"`
	CompetitionID uuid.UUID `json:"competitionId"`
	Name          string    `json:"name"`
	SortOrder     int       `json:"sortOrder"`
	StageDate     *string   `json:"stageDate,omitempty"`
	IsActive      bool      `json:"isActive"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
