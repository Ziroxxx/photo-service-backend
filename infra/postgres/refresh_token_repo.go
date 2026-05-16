package postgres

import (
	"context"
	"errors"
	"time"

	"photo-service-back/domain/user"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokenRepo struct {
	db *pgxpool.Pool
}

func NewRefreshTokenRepo(db *pgxpool.Pool) *RefreshTokenRepo {
	return &RefreshTokenRepo{db: db}
}

func (r *RefreshTokenRepo) Create(ctx context.Context, rt *user.RefreshToken) error {
	q := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, q, rt.ID, rt.UserID, rt.TokenHash, rt.ExpiresAt, rt.IP, rt.UserAgent)
	return err
}

func (r *RefreshTokenRepo) GetActiveByHash(ctx context.Context, hash string) (*user.RefreshToken, error) {
	q := `
		SELECT id, user_id, token_hash, expires_at, revoked_at, ip::text, user_agent, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
	`

	var out user.RefreshToken
	err := r.db.QueryRow(ctx, q, hash).Scan(
		&out.ID,
		&out.UserID,
		&out.TokenHash,
		&out.ExpiresAt,
		&out.RevokedAt,
		&out.IP,
		&out.UserAgent,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrInvalidToken
		}
		return nil, err
	}

	return &out, nil
}

func (r *RefreshTokenRepo) RevokeByHash(ctx context.Context, hash string, revokedAt time.Time) error {
	q := `
		UPDATE refresh_tokens
		SET revoked_at = $2
		WHERE token_hash = $1
		  AND revoked_at IS NULL
	`
	_, err := r.db.Exec(ctx, q, hash, revokedAt)
	return err
}
