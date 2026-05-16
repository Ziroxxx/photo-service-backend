package postgres

import (
	"context"
	"errors"

	"photo-service-back/domain/access"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccessRepo struct {
	db *pgxpool.Pool
}

func NewAccessRepo(db *pgxpool.Pool) *AccessRepo {
	return &AccessRepo{db: db}
}

func scanGrant(row pgx.Row) (*access.Grant, error) {
	var out access.Grant

	err := row.Scan(
		&out.ID,
		&out.CompetitionID,
		&out.UserID,
		&out.CanViewPhotos,
		&out.CanDownloadOriginal,
		&out.GrantedByUserID,
		&out.ExpiresAt,
		&out.RevokedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
		&out.UserLogin,
		&out.UserFullName,
	)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func (r *AccessRepo) ListByCompetitionID(ctx context.Context, competitionID uuid.UUID) ([]access.Grant, error) {
	q := `
		SELECT
			g.id,
			g.competition_id,
			g.user_id,
			g.can_view_photos,
			g.can_download_original,
			g.granted_by_user_id,
			g.expires_at,
			g.revoked_at,
			g.created_at,
			g.updated_at,
			u.login,
			u.full_name
		FROM competition_access_grants g
		JOIN users u ON u.id = g.user_id
		WHERE g.competition_id = $1
		ORDER BY g.created_at DESC
	`

	rows, err := r.db.Query(ctx, q, competitionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]access.Grant, 0)
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *g)
	}

	return items, rows.Err()
}

func (r *AccessRepo) GetByID(ctx context.Context, competitionID, grantID uuid.UUID) (*access.Grant, error) {
	q := `
		SELECT
			g.id,
			g.competition_id,
			g.user_id,
			g.can_view_photos,
			g.can_download_original,
			g.granted_by_user_id,
			g.expires_at,
			g.revoked_at,
			g.created_at,
			g.updated_at,
			u.login,
			u.full_name
		FROM competition_access_grants g
		JOIN users u ON u.id = g.user_id
		WHERE g.competition_id = $1 AND g.id = $2
	`

	out, err := scanGrant(r.db.QueryRow(ctx, q, competitionID, grantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, access.ErrGrantNotFound
		}
		return nil, err
	}

	return out, nil
}

func (r *AccessRepo) GetByCompetitionAndUser(ctx context.Context, competitionID, userID uuid.UUID) (*access.Grant, error) {
	q := `
		SELECT
			g.id,
			g.competition_id,
			g.user_id,
			g.can_view_photos,
			g.can_download_original,
			g.granted_by_user_id,
			g.expires_at,
			g.revoked_at,
			g.created_at,
			g.updated_at,
			u.login,
			u.full_name
		FROM competition_access_grants g
		JOIN users u ON u.id = g.user_id
		WHERE g.competition_id = $1 AND g.user_id = $2
	`

	out, err := scanGrant(r.db.QueryRow(ctx, q, competitionID, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return out, nil
}

func (r *AccessRepo) Create(ctx context.Context, g *access.Grant) (*access.Grant, error) {
	q := `
		WITH inserted AS (
			INSERT INTO competition_access_grants (
				competition_id,
				user_id,
				can_view_photos,
				can_download_original,
				granted_by_user_id,
				expires_at
			)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING
				id,
				competition_id,
				user_id,
				can_view_photos,
				can_download_original,
				granted_by_user_id,
				expires_at,
				revoked_at,
				created_at,
				updated_at
		)
		SELECT
			i.id,
			i.competition_id,
			i.user_id,
			i.can_view_photos,
			i.can_download_original,
			i.granted_by_user_id,
			i.expires_at,
			i.revoked_at,
			i.created_at,
			i.updated_at,
			u.login,
			u.full_name
		FROM inserted i
		JOIN users u ON u.id = i.user_id
	`

	out, err := scanGrant(r.db.QueryRow(
		ctx,
		q,
		g.CompetitionID,
		g.UserID,
		g.CanViewPhotos,
		g.CanDownloadOriginal,
		g.GrantedByUserID,
		g.ExpiresAt,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" && pgErr.ConstraintName == "uq_competition_access_grants" {
				return nil, access.ErrGrantAlreadyExists
			}
		}
		return nil, err
	}

	return out, nil
}

func (r *AccessRepo) Update(ctx context.Context, g *access.Grant) (*access.Grant, error) {
	q := `
		WITH updated AS (
			UPDATE competition_access_grants
			SET
				can_view_photos = $3,
				can_download_original = $4,
				expires_at = $5
			WHERE competition_id = $1 AND id = $2
			RETURNING
				id,
				competition_id,
				user_id,
				can_view_photos,
				can_download_original,
				granted_by_user_id,
				expires_at,
				revoked_at,
				created_at,
				updated_at
		)
		SELECT
			u2.id,
			u2.competition_id,
			u2.user_id,
			u2.can_view_photos,
			u2.can_download_original,
			u2.granted_by_user_id,
			u2.expires_at,
			u2.revoked_at,
			u2.created_at,
			u2.updated_at,
			u.login,
			u.full_name
		FROM updated u2
		JOIN users u ON u.id = u2.user_id
	`

	out, err := scanGrant(r.db.QueryRow(
		ctx,
		q,
		g.CompetitionID,
		g.ID,
		g.CanViewPhotos,
		g.CanDownloadOriginal,
		g.ExpiresAt,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, access.ErrGrantNotFound
		}
		return nil, err
	}

	return out, nil
}

func (r *AccessRepo) Revoke(ctx context.Context, competitionID, grantID uuid.UUID) error {
	q := `
		DELETE FROM competition_access_grants
		WHERE competition_id = $1 AND id = $2
	`

	tag, err := r.db.Exec(ctx, q, competitionID, grantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return access.ErrGrantNotFound
	}

	return nil
}
