package user

import (
	"context"

	"github.com/google/uuid"
)

type UserService struct {
	users UserRepository
}

func NewUserService(users UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) GetMe(ctx context.Context, userID uuid.UUID) (*User, error) {
	return s.users.GetByID(ctx, userID)
}

func (s *UserService) List(ctx context.Context, filter ListUsersFilter) ([]User, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	return s.users.List(ctx, filter)
}

func (s *UserService) UpdateRole(ctx context.Context, id uuid.UUID, role Role) (*User, error) {
	return s.users.UpdateRole(ctx, id, role)
}

func (s *UserService) UpdateStatus(ctx context.Context, id uuid.UUID, status Status) (*User, error) {
	return s.users.UpdateStatus(ctx, id, status)
}
