package postgres

import (
	"context"
	"errors"

	"photo-service-back/domain/competition"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CompetitionRepo struct {
	db *pgxpool.Pool
}

func NewCompetitionRepo(db *pgxpool.Pool) *CompetitionRepo {
	return &CompetitionRepo{db: db}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCompetition(s rowScanner) (*competition.Competition, error) {
	var c competition.Competition
	err := s.Scan(
		&c.ID,
		&c.Slug,
		&c.Title,
		&c.Type,
		&c.City,
		&c.Venue,
		&c.Description,
		&c.StartAt,
		&c.EndAt,
		&c.Timezone,
		&c.Status,
		&c.OrganizerID,
		&c.OrganizerLogin,
		&c.OrganizerFullName,
		&c.CoverPhotoID,
		&c.CoverBucket,
		&c.CoverObjectKey,
		&c.CreatedBy,
		&c.UpdatedBy,
		&c.CreatedAt,
		&c.UpdatedAt,
		&c.PhotosCount,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func scanStage(s rowScanner) (*competition.Stage, error) {
	var st competition.Stage
	err := s.Scan(
		&st.ID,
		&st.CompetitionID,
		&st.Name,
		&st.SortOrder,
		&st.StageDate,
		&st.IsActive,
		&st.CreatedAt,
		&st.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (r *CompetitionRepo) ListStagesByCompetitionIDs(ctx context.Context, competitionIDs []uuid.UUID) (map[uuid.UUID][]competition.Stage, error) {
	result := make(map[uuid.UUID][]competition.Stage)
	if len(competitionIDs) == 0 {
		return result, nil
	}

	q := `
		SELECT
			id, competition_id, name, sort_order, stage_date::text,
			is_active, created_at, updated_at
		FROM competition_stages
		WHERE competition_id = ANY($1)
		ORDER BY competition_id, sort_order ASC, created_at ASC
	`

	rows, err := r.db.Query(ctx, q, competitionIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var st competition.Stage
		if err := rows.Scan(
			&st.ID,
			&st.CompetitionID,
			&st.Name,
			&st.SortOrder,
			&st.StageDate,
			&st.IsActive,
			&st.CreatedAt,
			&st.UpdatedAt,
		); err != nil {
			return nil, err
		}

		result[st.CompetitionID] = append(result[st.CompetitionID], st)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *CompetitionRepo) ListCompetitions(ctx context.Context, filter competition.ListCompetitionsFilter) ([]competition.Competition, error) {
	q := `
		SELECT
			c.id,
			c.slug,
			c.title,
			c.type,
			c.city,
			c.venue,
			c.description,
			c.start_at,
			c.end_at,
			c.timezone,
			c.status,
			c.organizer_id,
			organizer.login AS organizer_login,
			organizer.full_name AS organizer_full_name,
			c.cover_photo_id,
			c.cover_bucket,
			c.cover_object_key,
			c.created_by,
			c.updated_by,
			c.created_at,
			c.updated_at,
			(
				SELECT COUNT(*)
				FROM photos p
				WHERE p.competition_id = c.id
				  AND p.deleted_at IS NULL
			) AS photos_count
		FROM competitions c
		JOIN users organizer ON organizer.id = c.organizer_id
		ORDER BY c.start_at DESC, c.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, q, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]competition.Competition, 0)
	for rows.Next() {
		c, err := scanCompetition(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *c)
	}

	return items, rows.Err()
}

func (r *CompetitionRepo) GetCompetitionByID(ctx context.Context, id uuid.UUID) (*competition.Competition, error) {
	q := `
		SELECT
			c.id,
			c.slug,
			c.title,
			c.type,
			c.city,
			c.venue,
			c.description,
			c.start_at,
			c.end_at,
			c.timezone,
			c.status,
			c.organizer_id,
			organizer.login AS organizer_login,
			organizer.full_name AS organizer_full_name,
			c.cover_photo_id,
			c.cover_bucket,
			c.cover_object_key,
			c.created_by,
			c.updated_by,
			c.created_at,
			c.updated_at,
			(
				SELECT COUNT(*)
				FROM photos p
				WHERE p.competition_id = c.id
				  AND p.deleted_at IS NULL
			) AS photos_count
		FROM competitions c
		JOIN users organizer ON organizer.id = c.organizer_id
		WHERE c.id = $1
	`

	c, err := scanCompetition(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, competition.ErrCompetitionNotFound
		}
		return nil, err
	}

	return c, nil
}

func (r *CompetitionRepo) CreateCompetition(ctx context.Context, c *competition.Competition) (*competition.Competition, error) {
	q := `
		WITH inserted AS (
			INSERT INTO competitions (
				slug, title, type, city, venue, description,
				start_at, end_at, timezone, status, organizer_id,
				cover_photo_id, cover_bucket, cover_object_key,
				created_by, updated_by
			)
			VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11,
				$12, $13, $14, $15, $16
			)
			RETURNING
				id, slug, title, type, city, venue, description,
				start_at, end_at, timezone, status, organizer_id,
				cover_photo_id, cover_bucket, cover_object_key,
				created_by, updated_by, created_at, updated_at
		)
		SELECT
			i.id,
			i.slug,
			i.title,
			i.type,
			i.city,
			i.venue,
			i.description,
			i.start_at,
			i.end_at,
			i.timezone,
			i.status,
			i.organizer_id,
			organizer.login AS organizer_login,
			organizer.full_name AS organizer_full_name,
			i.cover_photo_id,
			i.cover_bucket,
			i.cover_object_key,
			i.created_by,
			i.updated_by,
			i.created_at,
			i.updated_at,
			0 AS photos_count
		FROM inserted i
		JOIN users organizer ON organizer.id = i.organizer_id
	`

	out, err := scanCompetition(r.db.QueryRow(
		ctx,
		q,
		c.Slug,
		c.Title,
		c.Type,
		c.City,
		c.Venue,
		c.Description,
		c.StartAt,
		c.EndAt,
		c.Timezone,
		c.Status,
		c.OrganizerID,
		c.CoverPhotoID,
		c.CoverBucket,
		c.CoverObjectKey,
		c.CreatedBy,
		c.UpdatedBy,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" && pgErr.ConstraintName == "competitions_slug_key" {
				return nil, competition.ErrCompetitionSlugAlreadyExists
			}
		}
		return nil, err
	}

	return out, nil
}

func (r *CompetitionRepo) UpdateCompetition(ctx context.Context, c *competition.Competition) (*competition.Competition, error) {
	q := `
		WITH updated AS (
			UPDATE competitions
			SET
				slug = $2,
				title = $3,
				type = $4,
				city = $5,
				venue = $6,
				description = $7,
				start_at = $8,
				end_at = $9,
				timezone = $10,
				status = $11,
				organizer_id = $12,
				cover_photo_id = $13,
				cover_bucket = $14,
				cover_object_key = $15,
				updated_by = $16
			WHERE id = $1
			RETURNING
				id, slug, title, type, city, venue, description,
				start_at, end_at, timezone, status, organizer_id,
				cover_photo_id, cover_bucket, cover_object_key,
				created_by, updated_by, created_at, updated_at
		)
		SELECT
			u.id,
			u.slug,
			u.title,
			u.type,
			u.city,
			u.venue,
			u.description,
			u.start_at,
			u.end_at,
			u.timezone,
			u.status,
			u.organizer_id,
			organizer.login AS organizer_login,
			organizer.full_name AS organizer_full_name,
			u.cover_photo_id,
			u.cover_bucket,
			u.cover_object_key,
			u.created_by,
			u.updated_by,
			u.created_at,
			u.updated_at,
			(
				SELECT COUNT(*)
				FROM photos p
				WHERE p.competition_id = u.id
				  AND p.deleted_at IS NULL
			) AS photos_count
		FROM updated u
		JOIN users organizer ON organizer.id = u.organizer_id
	`

	out, err := scanCompetition(r.db.QueryRow(
		ctx,
		q,
		c.ID,
		c.Slug,
		c.Title,
		c.Type,
		c.City,
		c.Venue,
		c.Description,
		c.StartAt,
		c.EndAt,
		c.Timezone,
		c.Status,
		c.OrganizerID,
		c.CoverPhotoID,
		c.CoverBucket,
		c.CoverObjectKey,
		c.UpdatedBy,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, competition.ErrCompetitionNotFound
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" && pgErr.ConstraintName == "competitions_slug_key" {
				return nil, competition.ErrCompetitionSlugAlreadyExists
			}
		}

		return nil, err
	}

	return out, nil
}

func (r *CompetitionRepo) DeleteCompetition(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM competitions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return competition.ErrCompetitionNotFound
	}
	return nil
}

func (r *CompetitionRepo) ListStages(ctx context.Context, competitionID uuid.UUID) ([]competition.Stage, error) {
	q := `
		SELECT
			id, competition_id, name, sort_order, stage_date::text,
			is_active, created_at, updated_at
		FROM competition_stages
		WHERE competition_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`

	rows, err := r.db.Query(ctx, q, competitionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]competition.Stage, 0)
	for rows.Next() {
		var st competition.Stage
		if err := rows.Scan(
			&st.ID,
			&st.CompetitionID,
			&st.Name,
			&st.SortOrder,
			&st.StageDate,
			&st.IsActive,
			&st.CreatedAt,
			&st.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, st)
	}

	return items, rows.Err()
}

func (r *CompetitionRepo) GetStageByID(ctx context.Context, competitionID, stageID uuid.UUID) (*competition.Stage, error) {
	q := `
		SELECT
			id, competition_id, name, sort_order, stage_date::text,
			is_active, created_at, updated_at
		FROM competition_stages
		WHERE competition_id = $1 AND id = $2
	`

	st, err := scanStage(r.db.QueryRow(ctx, q, competitionID, stageID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, competition.ErrStageNotFound
		}
		return nil, err
	}

	return st, nil
}

func (r *CompetitionRepo) CreateStage(ctx context.Context, s *competition.Stage) (*competition.Stage, error) {
	q := `
		INSERT INTO competition_stages (
			competition_id, name, sort_order, stage_date, is_active
		)
		VALUES ($1, $2, $3, $4::date, $5)
		RETURNING
			id, competition_id, name, sort_order, stage_date::text,
			is_active, created_at, updated_at
	`

	return scanStage(r.db.QueryRow(
		ctx,
		q,
		s.CompetitionID,
		s.Name,
		s.SortOrder,
		s.StageDate,
		s.IsActive,
	))
}

func (r *CompetitionRepo) UpdateStage(ctx context.Context, s *competition.Stage) (*competition.Stage, error) {
	q := `
		UPDATE competition_stages
		SET
			name = $3,
			sort_order = $4,
			stage_date = $5::date,
			is_active = $6
		WHERE competition_id = $1 AND id = $2
		RETURNING
			id, competition_id, name, sort_order, stage_date::text,
			is_active, created_at, updated_at
	`

	out, err := scanStage(r.db.QueryRow(
		ctx,
		q,
		s.CompetitionID,
		s.ID,
		s.Name,
		s.SortOrder,
		s.StageDate,
		s.IsActive,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, competition.ErrStageNotFound
		}
		return nil, err
	}

	return out, nil
}

func (r *CompetitionRepo) DeleteStage(ctx context.Context, competitionID, stageID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM competition_stages WHERE competition_id = $1 AND id = $2`, competitionID, stageID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return competition.ErrStageNotFound
	}
	return nil
}
