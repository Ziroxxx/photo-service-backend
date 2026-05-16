package competition

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	ListCompetitions(ctx context.Context, filter ListCompetitionsFilter) ([]Competition, error)
	GetCompetitionByID(ctx context.Context, id uuid.UUID) (*Competition, error)
	CreateCompetition(ctx context.Context, c *Competition) (*Competition, error)
	UpdateCompetition(ctx context.Context, c *Competition) (*Competition, error)
	DeleteCompetition(ctx context.Context, id uuid.UUID) error

	ListStages(ctx context.Context, competitionID uuid.UUID) ([]Stage, error)
	ListStagesByCompetitionIDs(ctx context.Context, competitionIDs []uuid.UUID) (map[uuid.UUID][]Stage, error)
	GetStageByID(ctx context.Context, competitionID, stageID uuid.UUID) (*Stage, error)
	CreateStage(ctx context.Context, s *Stage) (*Stage, error)
	UpdateStage(ctx context.Context, s *Stage) (*Stage, error)
	DeleteStage(ctx context.Context, competitionID, stageID uuid.UUID) error
}
