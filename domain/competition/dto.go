package competition

import (
	"time"

	"github.com/google/uuid"
)

type ListCompetitionsFilter struct {
	Page     int
	PageSize int
}

type CreateCompetitionRequest struct {
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	City        *string    `json:"city,omitempty"`
	Venue       *string    `json:"venue,omitempty"`
	Description *string    `json:"description,omitempty"`
	StartAt     time.Time  `json:"startAt"`
	EndAt       time.Time  `json:"endAt"`
	Timezone    string     `json:"timezone,omitempty"`
	Status      *Status    `json:"status,omitempty"`
	OrganizerID *uuid.UUID `json:"organizerId,omitempty"`
}

type UpdateCompetitionRequest struct {
	Slug        *string    `json:"slug,omitempty"`
	Title       *string    `json:"title,omitempty"`
	Type        *string    `json:"type,omitempty"`
	City        *string    `json:"city,omitempty"`
	Venue       *string    `json:"venue,omitempty"`
	Description *string    `json:"description,omitempty"`
	StartAt     *time.Time `json:"startAt,omitempty"`
	EndAt       *time.Time `json:"endAt,omitempty"`
	Timezone    *string    `json:"timezone,omitempty"`
	Status      *Status    `json:"status,omitempty"`
	OrganizerID *uuid.UUID `json:"organizerId,omitempty"`
	RemoveCover *bool      `json:"removeCover,omitempty"`
}

type CreateStageRequest struct {
	Name      string  `json:"name"`
	SortOrder int     `json:"sortOrder"`
	StageDate *string `json:"stageDate,omitempty"`
	IsActive  *bool   `json:"isActive,omitempty"`
}

type UpdateStageRequest struct {
	Name      *string `json:"name,omitempty"`
	SortOrder *int    `json:"sortOrder,omitempty"`
	StageDate *string `json:"stageDate,omitempty"`
	IsActive  *bool   `json:"isActive,omitempty"`
}
