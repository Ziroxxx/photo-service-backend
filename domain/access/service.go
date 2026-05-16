package access

import (
	"context"
	"time"

	competitiondomain "photo-service-back/domain/competition"
	"photo-service-back/domain/user"

	"github.com/google/uuid"
)

type CompetitionReader interface {
	GetCompetitionByID(ctx context.Context, id uuid.UUID) (*competitiondomain.Competition, error)
}

type Service struct {
	repo         Repository
	competitions CompetitionReader
}

func NewService(repo Repository, competitions CompetitionReader) *Service {
	return &Service{
		repo:         repo,
		competitions: competitions,
	}
}

func (s *Service) GetCompetitionAccess(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	competitionID uuid.UUID,
) (*CompetitionAccessResponse, error) {
	comp, err := s.competitions.GetCompetitionByID(ctx, competitionID)
	if err != nil {
		return nil, err
	}

	canManage := canManageCompetitionAccess(actorID, actorRole, comp)

	self, err := s.buildEffectiveAccess(ctx, actorID, actorRole, comp)
	if err != nil {
		return nil, err
	}

	resp := &CompetitionAccessResponse{
		Self:  *self,
		Items: []Grant{},
	}

	if canManage {
		items, err := s.repo.ListByCompetitionID(ctx, competitionID)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		for i := range items {
			items[i].IsActive = isGrantActive(items[i], now)
		}
		resp.Items = items
	}

	return resp, nil
}

func (s *Service) CreateGrant(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	competitionID uuid.UUID,
	req CreateGrantRequest,
) (*Grant, error) {
	comp, err := s.competitions.GetCompetitionByID(ctx, competitionID)
	if err != nil {
		return nil, err
	}

	if !canManageCompetitionAccess(actorID, actorRole, comp) {
		return nil, ErrForbidden
	}

	g := &Grant{
		CompetitionID:       competitionID,
		UserID:              req.UserID,
		CanViewPhotos:       true,
		CanDownloadOriginal: req.CanDownloadOriginal,
		GrantedByUserID:     actorID,
		ExpiresAt:           req.ExpiresAt,
	}

	out, err := s.repo.Create(ctx, g)
	if err != nil {
		return nil, err
	}

	out.IsActive = isGrantActive(*out, time.Now())
	return out, nil
}

func (s *Service) UpdateGrant(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	competitionID, grantID uuid.UUID,
	req UpdateGrantRequest,
) (*Grant, error) {
	comp, err := s.competitions.GetCompetitionByID(ctx, competitionID)
	if err != nil {
		return nil, err
	}

	if !canManageCompetitionAccess(actorID, actorRole, comp) {
		return nil, ErrForbidden
	}

	current, err := s.repo.GetByID(ctx, competitionID, grantID)
	if err != nil {
		return nil, err
	}

	if req.CanDownloadOriginal != nil {
		current.CanDownloadOriginal = *req.CanDownloadOriginal
	}
	if req.ExpiresAt != nil {
		current.ExpiresAt = req.ExpiresAt
	}
	if req.ClearExpiresAt != nil && *req.ClearExpiresAt {
		current.ExpiresAt = nil
	}

	current.CanViewPhotos = true

	out, err := s.repo.Update(ctx, current)
	if err != nil {
		return nil, err
	}

	out.IsActive = isGrantActive(*out, time.Now())
	return out, nil
}

func (s *Service) DeleteGrant(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	competitionID, grantID uuid.UUID,
) error {
	comp, err := s.competitions.GetCompetitionByID(ctx, competitionID)
	if err != nil {
		return err
	}

	if !canManageCompetitionAccess(actorID, actorRole, comp) {
		return ErrForbidden
	}

	return s.repo.Revoke(ctx, competitionID, grantID)
}

func (s *Service) CanDownloadOriginal(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	competitionID uuid.UUID,
) (bool, error) {
	comp, err := s.competitions.GetCompetitionByID(ctx, competitionID)
	if err != nil {
		return false, err
	}

	self, err := s.buildEffectiveAccess(ctx, actorID, actorRole, comp)
	if err != nil {
		return false, err
	}

	return self.CanDownloadOriginal, nil
}

func (s *Service) buildEffectiveAccess(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	comp *competitiondomain.Competition,
) (*EffectiveAccess, error) {
	if actorRole == user.RoleAdmin {
		return &EffectiveAccess{
			CompetitionID:       comp.ID,
			UserID:              actorID,
			CanViewPhotos:       true,
			CanDownloadOriginal: true,
			CanManageAccess:     true,
			Source:              "admin",
		}, nil
	}

	if actorRole == user.RoleOrganizer && comp.OrganizerID == actorID {
		return &EffectiveAccess{
			CompetitionID:       comp.ID,
			UserID:              actorID,
			CanViewPhotos:       true,
			CanDownloadOriginal: true,
			CanManageAccess:     true,
			Source:              "organizer",
		}, nil
	}

	g, err := s.repo.GetByCompetitionAndUser(ctx, comp.ID, actorID)
	if err != nil {
		return nil, err
	}

	if g != nil && isGrantActive(*g, time.Now()) {
		grantID := g.ID
		return &EffectiveAccess{
			CompetitionID:       comp.ID,
			UserID:              actorID,
			CanViewPhotos:       true,
			CanDownloadOriginal: g.CanDownloadOriginal,
			CanManageAccess:     false,
			Source:              "grant",
			GrantID:             &grantID,
			ExpiresAt:           g.ExpiresAt,
		}, nil
	}

	return &EffectiveAccess{
		CompetitionID:       comp.ID,
		UserID:              actorID,
		CanViewPhotos:       true,
		CanDownloadOriginal: false,
		CanManageAccess:     false,
		Source:              "default",
	}, nil
}

func canManageCompetitionAccess(
	actorID uuid.UUID,
	actorRole user.Role,
	comp *competitiondomain.Competition,
) bool {
	if actorRole == user.RoleAdmin {
		return true
	}
	return actorRole == user.RoleOrganizer && comp.OrganizerID == actorID
}

func isGrantActive(g Grant, now time.Time) bool {
	if g.RevokedAt != nil {
		return false
	}
	if g.ExpiresAt != nil && g.ExpiresAt.Before(now) {
		return false
	}
	return true
}
