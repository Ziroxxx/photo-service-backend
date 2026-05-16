package user

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ListUsersFilter struct {
	Page     int
	PageSize int
	Role     *Role
	Status   *Status
	Query    *string
}

type UserRepository interface {
	Create(ctx context.Context, u *User) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByLogin(ctx context.Context, login string) (*User, error)
	List(ctx context.Context, filter ListUsersFilter) ([]User, error)
	UpdateRole(ctx context.Context, id uuid.UUID, role Role) (*User, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status Status) (*User, error)
	TouchLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *RefreshToken) error
	GetActiveByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeByHash(ctx context.Context, hash string, revokedAt time.Time) error
}
