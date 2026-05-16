package access

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	ListByCompetitionID(ctx context.Context, competitionID uuid.UUID) ([]Grant, error)
	GetByID(ctx context.Context, competitionID, grantID uuid.UUID) (*Grant, error)
	GetByCompetitionAndUser(ctx context.Context, competitionID, userID uuid.UUID) (*Grant, error)
	Create(ctx context.Context, g *Grant) (*Grant, error)
	Update(ctx context.Context, g *Grant) (*Grant, error)
	Revoke(ctx context.Context, competitionID, grantID uuid.UUID) error
}
