package postgres

import (
	"context"
	"errors"
	"time"

	"photo-service-back/domain/photo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PhotoRepo struct {
	db *pgxpool.Pool
}

func NewPhotoRepo(db *pgxpool.Pool) *PhotoRepo {
	return &PhotoRepo{db: db}
}

type photoRowScanner interface {
	Scan(dest ...any) error
}

func scanPhoto(s photoRowScanner) (*photo.Photo, error) {
	var p photo.Photo
	err := s.Scan(
		&p.ID,
		&p.CompetitionID,
		&p.StageID,
		&p.AuthorUserID,
		&p.AuthorLogin,
		&p.AuthorFullName,
		&p.OriginalFilename,
		&p.MimeType,
		&p.SizeBytes,
		&p.DayDate,
		&p.Width,
		&p.Height,
		&p.PrimaryBib,
		&p.BibRecognitionStatus,
		&p.BibRecognitionError,
		&p.WatermarkRequired,
		&p.DeletedAt,
		&p.DeletedBy,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanPhotoVersion(s photoRowScanner) (*photo.PhotoVersion, error) {
	var v photo.PhotoVersion
	err := s.Scan(
		&v.ID,
		&v.PhotoID,
		&v.Variant,
		&v.StorageBucket,
		&v.ObjectKey,
		&v.MimeType,
		&v.SizeBytes,
		&v.Width,
		&v.Height,
		&v.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func scanPhotoBib(s photoRowScanner) (*photo.PhotoBib, error) {
	var b photo.PhotoBib
	err := s.Scan(
		&b.ID,
		&b.PhotoID,
		&b.BibValue,
		&b.NormalizedBib,
		&b.Source,
		&b.Confidence,
		&b.CreatedByUserID,
		&b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *PhotoRepo) ListPhotos(ctx context.Context, filter photo.ListPhotosFilter) ([]photo.Photo, error) {
	q := `
		SELECT DISTINCT
			p.id,
			p.competition_id,
			p.stage_id,
			p.author_user_id,
			author.login AS author_login,
			author.full_name AS author_full_name,
			p.original_filename,
			p.mime_type,
			p.size_bytes,
			p.day_date::text,
			p.width,
			p.height,
			p.primary_bib,
			p.bib_recognition_status,
			p.bib_recognition_error,
			p.watermark_required,
			p.deleted_at,
			p.deleted_by,
			p.created_at,
			p.updated_at
		FROM photos p
		LEFT JOIN photo_bibs pb ON pb.photo_id = p.id
		JOIN users author ON author.id = p.author_user_id
		WHERE p.competition_id = $1
		  AND p.deleted_at IS NULL
		  AND ($2::uuid IS NULL OR p.stage_id = $2)
		  AND ($3::text IS NULL OR pb.normalized_bib = $3)
		ORDER BY p.created_at DESC
		LIMIT $4 OFFSET $5
	`

	rows, err := r.db.Query(
		ctx,
		q,
		filter.CompetitionID,
		filter.StageID,
		filter.Bib,
		filter.PageSize,
		(filter.Page-1)*filter.PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]photo.Photo, 0)
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *p)
	}

	return items, rows.Err()
}

func (r *PhotoRepo) GetPhotoByID(ctx context.Context, id uuid.UUID) (*photo.Photo, error) {
	q := `
		SELECT
			p.id,
			p.competition_id,
			p.stage_id,
			p.author_user_id,
			author.login AS author_login,
			author.full_name AS author_full_name,
			p.original_filename,
			p.mime_type,
			p.size_bytes,
			p.day_date::text,
			p.width,
			p.height,
			p.primary_bib,
			p.bib_recognition_status,
			p.bib_recognition_error,
			p.watermark_required,
			p.deleted_at,
			p.deleted_by,
			p.created_at,
			p.updated_at
		FROM photos p
		JOIN users author ON author.id = p.author_user_id
		WHERE p.id = $1
	`

	out, err := scanPhoto(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, photo.ErrPhotoNotFound
		}
		return nil, err
	}

	return out, nil
}

func (r *PhotoRepo) ListPhotoVersionsByPhotoIDs(ctx context.Context, photoIDs []uuid.UUID) (map[uuid.UUID][]photo.PhotoVersion, error) {
	result := make(map[uuid.UUID][]photo.PhotoVersion)
	if len(photoIDs) == 0 {
		return result, nil
	}

	q := `
		SELECT
			id,
			photo_id,
			variant,
			storage_bucket,
			object_key,
			mime_type,
			size_bytes,
			width,
			height,
			created_at
		FROM photo_versions
		WHERE photo_id = ANY($1)
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, q, photoIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		v, err := scanPhotoVersion(rows)
		if err != nil {
			return nil, err
		}
		result[v.PhotoID] = append(result[v.PhotoID], *v)
	}

	return result, rows.Err()
}

func (r *PhotoRepo) GetPhotoVersionByVariant(ctx context.Context, photoID uuid.UUID, variant photo.Variant) (*photo.PhotoVersion, error) {
	q := `
		SELECT
			id,
			photo_id,
			variant,
			storage_bucket,
			object_key,
			mime_type,
			size_bytes,
			width,
			height,
			created_at
		FROM photo_versions
		WHERE photo_id = $1 AND variant = $2
	`

	out, err := scanPhotoVersion(r.db.QueryRow(ctx, q, photoID, variant))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, photo.ErrPhotoVersionNotFound
		}
		return nil, err
	}

	return out, nil
}

func (r *PhotoRepo) ListPhotoBibsByPhotoIDs(ctx context.Context, photoIDs []uuid.UUID) (map[uuid.UUID][]photo.PhotoBib, error) {
	result := make(map[uuid.UUID][]photo.PhotoBib)
	if len(photoIDs) == 0 {
		return result, nil
	}

	q := `
		SELECT
			id,
			photo_id,
			bib_value,
			normalized_bib,
			source,
			confidence,
			created_by_user_id,
			created_at
		FROM photo_bibs
		WHERE photo_id = ANY($1)
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, q, photoIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		b, err := scanPhotoBib(rows)
		if err != nil {
			return nil, err
		}
		result[b.PhotoID] = append(result[b.PhotoID], *b)
	}

	return result, rows.Err()
}

func (r *PhotoRepo) ListPhotoBibsByPhotoID(ctx context.Context, photoID uuid.UUID) ([]photo.PhotoBib, error) {
	q := `
		SELECT
			id,
			photo_id,
			bib_value,
			normalized_bib,
			source,
			confidence,
			created_by_user_id,
			created_at
		FROM photo_bibs
		WHERE photo_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, q, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]photo.PhotoBib, 0)
	for rows.Next() {
		b, err := scanPhotoBib(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *b)
	}

	return items, rows.Err()
}

func (r *PhotoRepo) GetPhotoBibByID(ctx context.Context, photoID, bibID uuid.UUID) (*photo.PhotoBib, error) {
	q := `
		SELECT
			id,
			photo_id,
			bib_value,
			normalized_bib,
			source,
			confidence,
			created_by_user_id,
			created_at
		FROM photo_bibs
		WHERE photo_id = $1 AND id = $2
	`

	out, err := scanPhotoBib(r.db.QueryRow(ctx, q, photoID, bibID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, photo.ErrPhotoBibNotFound
		}
		return nil, err
	}

	return out, nil
}

func (r *PhotoRepo) AddPhotoBib(ctx context.Context, bib *photo.PhotoBib) (*photo.PhotoBib, error) {
	q := `
		INSERT INTO photo_bibs (
			id,
			photo_id,
			bib_value,
			normalized_bib,
			source,
			confidence,
			created_by_user_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			photo_id,
			bib_value,
			normalized_bib,
			source,
			confidence,
			created_by_user_id,
			created_at
	`

	if bib.ID == uuid.Nil {
		bib.ID = uuid.New()
	}

	out, err := scanPhotoBib(r.db.QueryRow(
		ctx,
		q,
		bib.ID,
		bib.PhotoID,
		bib.BibValue,
		bib.NormalizedBib,
		bib.Source,
		bib.Confidence,
		bib.CreatedByUserID,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" && pgErr.ConstraintName == "uq_photo_bibs_photo_normalized_bib" {
				return nil, photo.ErrPhotoBibAlreadyExists
			}
		}
		return nil, err
	}

	return out, nil
}

func (r *PhotoRepo) DeletePhotoBib(ctx context.Context, photoID, bibID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM photo_bibs WHERE photo_id = $1 AND id = $2`, photoID, bibID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return photo.ErrPhotoBibNotFound
	}
	return nil
}

func (r *PhotoRepo) CreatePhotoWithVersions(ctx context.Context, p *photo.Photo, versions []photo.PhotoVersion) (*photo.Photo, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.BibRecognitionStatus == "" {
		p.BibRecognitionStatus = photo.BibRecognitionStatusPending
	}

	insertPhoto := `
		WITH inserted AS (
			INSERT INTO photos (
				id,
				competition_id,
				stage_id,
				author_user_id,
				original_filename,
				mime_type,
				size_bytes,
				day_date,
				width,
				height,
				primary_bib,
				bib_recognition_status,
				bib_recognition_error,
				watermark_required
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::date, $9, $10, $11, $12, $13, $14)
			RETURNING
				id,
				competition_id,
				stage_id,
				author_user_id,
				original_filename,
				mime_type,
				size_bytes,
				day_date::text AS day_date,
				width,
				height,
				primary_bib,
				bib_recognition_status,
				bib_recognition_error,
				watermark_required,
				deleted_at,
				deleted_by,
				created_at,
				updated_at
		)
		SELECT
			i.id,
			i.competition_id,
			i.stage_id,
			i.author_user_id,
			author.login AS author_login,
			author.full_name AS author_full_name,
			i.original_filename,
			i.mime_type,
			i.size_bytes,
			i.day_date,
			i.width,
			i.height,
			i.primary_bib,
			i.bib_recognition_status,
			i.bib_recognition_error,
			i.watermark_required,
			i.deleted_at,
			i.deleted_by,
			i.created_at,
			i.updated_at
		FROM inserted i
		JOIN users author ON author.id = i.author_user_id
	`

	created, err := scanPhoto(tx.QueryRow(
		ctx,
		insertPhoto,
		p.ID,
		p.CompetitionID,
		p.StageID,
		p.AuthorUserID,
		p.OriginalFilename,
		p.MimeType,
		p.SizeBytes,
		p.DayDate,
		p.Width,
		p.Height,
		p.PrimaryBib,
		p.BibRecognitionStatus,
		p.BibRecognitionError,
		p.WatermarkRequired,
	))
	if err != nil {
		return nil, err
	}

	insertVersion := `
		INSERT INTO photo_versions (
			id,
			photo_id,
			variant,
			storage_bucket,
			object_key,
			mime_type,
			size_bytes,
			width,
			height
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	for _, v := range versions {
		if v.ID == uuid.Nil {
			v.ID = uuid.New()
		}
		if _, err := tx.Exec(
			ctx,
			insertVersion,
			v.ID,
			created.ID,
			v.Variant,
			v.StorageBucket,
			v.ObjectKey,
			v.MimeType,
			v.SizeBytes,
			v.Width,
			v.Height,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return created, nil
}

func (r *PhotoRepo) UpdatePhoto(ctx context.Context, p *photo.Photo) (*photo.Photo, error) {
	q := `
		WITH updated AS (
			UPDATE photos
			SET
				stage_id = $2,
				primary_bib = $3,
				updated_at = now()
			WHERE id = $1
			RETURNING
				id,
				competition_id,
				stage_id,
				author_user_id,
				original_filename,
				mime_type,
				size_bytes,
				day_date::text AS day_date,
				width,
				height,
				primary_bib,
				bib_recognition_status,
				bib_recognition_error,
				watermark_required,
				deleted_at,
				deleted_by,
				created_at,
				updated_at
		)
		SELECT
			u.id,
			u.competition_id,
			u.stage_id,
			u.author_user_id,
			author.login AS author_login,
			author.full_name AS author_full_name,
			u.original_filename,
			u.mime_type,
			u.size_bytes,
			u.day_date,
			u.width,
			u.height,
			u.primary_bib,
			u.bib_recognition_status,
			u.bib_recognition_error,
			u.watermark_required,
			u.deleted_at,
			u.deleted_by,
			u.created_at,
			u.updated_at
		FROM updated u
		JOIN users author ON author.id = u.author_user_id
	`

	out, err := scanPhoto(r.db.QueryRow(ctx, q, p.ID, p.StageID, p.PrimaryBib))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, photo.ErrPhotoNotFound
		}
		return nil, err
	}
	return out, nil
}

func (r *PhotoRepo) SetPrimaryBib(ctx context.Context, photoID uuid.UUID, primaryBib *string) error {
	tag, err := r.db.Exec(ctx, `UPDATE photos SET primary_bib = $2, updated_at = now() WHERE id = $1`, photoID, primaryBib)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return photo.ErrPhotoNotFound
	}
	return nil
}

func (r *PhotoRepo) UpdateBibRecognitionStatus(
	ctx context.Context,
	photoID uuid.UUID,
	status photo.BibRecognitionStatus,
	recognitionError *string,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE photos SET bib_recognition_status = $2, updated_at = now() , bib_recognition_error = $3 WHERE id = $1`,
		photoID,
		status,
		recognitionError,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return photo.ErrPhotoNotFound
	}
	return nil
}

func (r *PhotoRepo) SoftDeletePhoto(ctx context.Context, photoID uuid.UUID, deletedBy uuid.UUID, deletedAt time.Time) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE photos SET deleted_at = $2, updated_at = now(), deleted_by = $3 WHERE id = $1 AND deleted_at IS NULL`,
		photoID,
		deletedAt,
		deletedBy,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return photo.ErrPhotoNotFound
	}
	return nil
}

func (r *PhotoRepo) GetPhotoAccessInfo(ctx context.Context, photoID uuid.UUID) (*photo.PhotoAccessInfo, error) {
	q := `
		SELECT
			p.id,
			p.competition_id,
			p.author_user_id,
			c.organizer_id,
			p.deleted_at
		FROM photos p
		JOIN competitions c ON c.id = p.competition_id
		WHERE p.id = $1
	`

	var out photo.PhotoAccessInfo
	err := r.db.QueryRow(ctx, q, photoID).Scan(
		&out.PhotoID,
		&out.CompetitionID,
		&out.AuthorUserID,
		&out.OrganizerID,
		&out.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, photo.ErrPhotoNotFound
		}
		return nil, err
	}

	return &out, nil
}

func (r *PhotoRepo) CountPhotos(ctx context.Context, filter photo.ListPhotosFilter) (int, error) {
	q := `
		SELECT COUNT(DISTINCT p.id)
		FROM photos p
		LEFT JOIN photo_bibs pb ON pb.photo_id = p.id
		WHERE p.competition_id = $1
		  AND p.deleted_at IS NULL
		  AND ($2::uuid IS NULL OR p.stage_id = $2)
		  AND ($3::text IS NULL OR pb.normalized_bib = $3)
	`

	var total int
	err := r.db.QueryRow(
		ctx,
		q,
		filter.CompetitionID,
		filter.StageID,
		filter.Bib,
	).Scan(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
}
