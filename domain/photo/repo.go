package photo

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	ListPhotos(ctx context.Context, filter ListPhotosFilter) ([]Photo, error)
	GetPhotoByID(ctx context.Context, id uuid.UUID) (*Photo, error)

	ListPhotoVersionsByPhotoIDs(ctx context.Context, photoIDs []uuid.UUID) (map[uuid.UUID][]PhotoVersion, error)
	GetPhotoVersionByVariant(ctx context.Context, photoID uuid.UUID, variant Variant) (*PhotoVersion, error)

	ListPhotoBibsByPhotoIDs(ctx context.Context, photoIDs []uuid.UUID) (map[uuid.UUID][]PhotoBib, error)
	ListPhotoBibsByPhotoID(ctx context.Context, photoID uuid.UUID) ([]PhotoBib, error)
	GetPhotoBibByID(ctx context.Context, photoID, bibID uuid.UUID) (*PhotoBib, error)
	AddPhotoBib(ctx context.Context, bib *PhotoBib) (*PhotoBib, error)
	UpdateBibRecognitionStatus(ctx context.Context, photoID uuid.UUID, status BibRecognitionStatus, recognitionError *string) error
	DeletePhotoBib(ctx context.Context, photoID, bibID uuid.UUID) error

	CreatePhotoWithVersions(ctx context.Context, p *Photo, versions []PhotoVersion) (*Photo, error)
	UpdatePhoto(ctx context.Context, p *Photo) (*Photo, error)
	SetPrimaryBib(ctx context.Context, photoID uuid.UUID, primaryBib *string) error

	SoftDeletePhoto(ctx context.Context, photoID uuid.UUID, deletedBy uuid.UUID, deletedAt time.Time) error

	GetPhotoAccessInfo(ctx context.Context, photoID uuid.UUID) (*PhotoAccessInfo, error)
	CountPhotos(ctx context.Context, filter ListPhotosFilter) (int, error)
}
