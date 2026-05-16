package user

import (
	"context"
	"time"

	"photo-service-back/infra/auth"

	"github.com/google/uuid"
)

type AuthService struct {
	users  UserRepository
	tokens RefreshTokenRepository
	tm     *auth.TokenManager
}

func NewAuthService(users UserRepository, tokens RefreshTokenRepository, tm *auth.TokenManager) *AuthService {
	return &AuthService{users: users, tokens: tokens, tm: tm}
}

func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*User, error) {
	_, err := s.users.GetByLogin(ctx, req.Login)
	if err == nil {
		return nil, ErrLoginAlreadyExists
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	u := &User{
		Login:        req.Login,
		PasswordHash: hash,
		FullName:     req.FullName,
		Role:         RoleParticipant,
		Status:       StatusActive,
	}

	return s.users.Create(ctx, u)
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest, ip, ua *string) (*AuthResponse, error) {
	u, err := s.users.GetByLogin(ctx, req.Login)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := auth.CheckPassword(u.PasswordHash, req.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	if u.Status != StatusActive {
		return nil, ErrInactiveUser
	}

	access, err := s.tm.NewAccessToken(u.ID, string(u.Role), string(u.Status))
	if err != nil {
		return nil, err
	}

	rawRefresh, hashRefresh, expiresAt, err := s.tm.NewRefreshToken()
	if err != nil {
		return nil, err
	}

	err = s.tokens.Create(ctx, &RefreshToken{
		ID:        uuid.New(),
		UserID:    u.ID,
		TokenHash: hashRefresh,
		ExpiresAt: expiresAt,
		IP:        ip,
		UserAgent: ua,
	})
	if err != nil {
		return nil, err
	}

	_ = s.users.TouchLastLogin(ctx, u.ID, time.Now())

	return &AuthResponse{
		AccessToken:  access,
		RefreshToken: rawRefresh,
		User:         *u,
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, raw string, ip, ua *string) (*AuthResponse, error) {
	hash := auth.HashToken(raw)

	rt, err := s.tokens.GetActiveByHash(ctx, hash)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if rt.ExpiresAt.Before(time.Now()) {
		return nil, ErrInvalidToken
	}

	u, err := s.users.GetByID(ctx, rt.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if u.Status != StatusActive {
		return nil, ErrInactiveUser
	}

	if err := s.tokens.RevokeByHash(ctx, hash, time.Now()); err != nil {
		return nil, err
	}

	access, err := s.tm.NewAccessToken(u.ID, string(u.Role), string(u.Status))
	if err != nil {
		return nil, err
	}

	newRaw, newHash, expiresAt, err := s.tm.NewRefreshToken()
	if err != nil {
		return nil, err
	}

	err = s.tokens.Create(ctx, &RefreshToken{
		ID:        uuid.New(),
		UserID:    u.ID,
		TokenHash: newHash,
		ExpiresAt: expiresAt,
		IP:        ip,
		UserAgent: ua,
	})
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		AccessToken:  access,
		RefreshToken: newRaw,
		User:         *u,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, raw string) error {
	hash := auth.HashToken(raw)
	return s.tokens.RevokeByHash(ctx, hash, time.Now())
}
