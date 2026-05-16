package user

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin        Role = "admin"
	RoleOrganizer    Role = "organizer"
	RolePhotographer Role = "photographer"
	RoleParticipant  Role = "participant"
)

type Status string

const (
	StatusActive  Status = "active"
	StatusBlocked Status = "blocked"
	StatusPending Status = "pending"
)

type User struct {
	ID           uuid.UUID  `json:"id"`
	Login        string     `json:"login"`
	Email        *string    `json:"email,omitempty"`
	PasswordHash string     `json:"-"`
	FullName     string     `json:"fullName"`
	Phone        *string    `json:"phone,omitempty"`
	Role         Role       `json:"role"`
	Status       Status     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	IP        *string
	UserAgent *string
	CreatedAt time.Time
}
