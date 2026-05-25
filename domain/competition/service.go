package competition

import (
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"time"

	"photo-service-back/domain/user"

	"github.com/google/uuid"
)

type UploadedFile struct {
	Reader           io.Reader
	Size             int64
	ContentType      string
	OriginalFilename string
}

type ObjectStorage interface {
	BucketName() string
	PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error
	RemoveObject(ctx context.Context, objectKey string) error
	ObjectURL(objectKey string) string
}

type Service struct {
	repo    Repository
	storage ObjectStorage
}

func NewService(repo Repository, storage ObjectStorage) *Service {
	return &Service{
		repo:    repo,
		storage: storage,
	}
}

func (s *Service) List(ctx context.Context, filter ListCompetitionsFilter) ([]Competition, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	items, err := s.repo.ListCompetitions(ctx, filter)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}

	stagesMap, err := s.repo.ListStagesByCompetitionIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	for i := range items {
		items[i].Stages = stagesMap[items[i].ID]
		s.enrichCompetition(&items[i])
	}

	return items, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Competition, error) {
	item, err := s.repo.GetCompetitionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	stages, err := s.repo.ListStages(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	item.Stages = stages
	s.enrichCompetition(item)

	return item, nil
}

func (s *Service) Create(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	req CreateCompetitionRequest,
	cover *UploadedFile,
) (*Competition, error) {
	if !canManageCompetitions(actorRole) {
		return nil, ErrForbiddenCompetitionWrite
	}

	if req.EndAt.Before(req.StartAt) {
		return nil, ErrInvalidCompetitionDates
	}

	status := StatusDraft
	if req.Status != nil {
		if err := validateStatus(*req.Status); err != nil {
			return nil, err
		}
		status = *req.Status
	}

	timezone := strings.TrimSpace(req.Timezone)
	if timezone == "" {
		timezone = "Europe/Amsterdam"
	}

	organizerID := actorID
	if actorRole == user.RoleAdmin && req.OrganizerID != nil {
		organizerID = *req.OrganizerID
	}

	c := &Competition{
		Slug:        strings.TrimSpace(req.Slug),
		Title:       strings.TrimSpace(req.Title),
		Type:        strings.TrimSpace(req.Type),
		City:        req.City,
		Venue:       req.Venue,
		Description: req.Description,
		StartAt:     req.StartAt,
		EndAt:       req.EndAt,
		Timezone:    timezone,
		Status:      status,
		OrganizerID: organizerID,
		CreatedBy:   actorID,
		UpdatedBy:   actorID,
	}

	var uploadedObjectKey *string

	if cover != nil {
		objectKey := buildCoverObjectKey(cover.OriginalFilename)

		if err := s.storage.PutObject(ctx, objectKey, cover.Reader, cover.Size, cover.ContentType); err != nil {
			return nil, err
		}

		bucket := s.storage.BucketName()
		c.CoverBucket = &bucket
		c.CoverObjectKey = &objectKey
		uploadedObjectKey = &objectKey
	}

	created, err := s.repo.CreateCompetition(ctx, c)
	if err != nil {
		if uploadedObjectKey != nil {
			_ = s.storage.RemoveObject(ctx, *uploadedObjectKey)
		}
		return nil, err
	}

	created.Stages = []Stage{}
	s.enrichCompetition(created)

	return created, nil
}

func (s *Service) Update(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	id uuid.UUID,
	req UpdateCompetitionRequest,
	cover *UploadedFile,
) (*Competition, error) {
	if !canManageCompetitions(actorRole) {
		return nil, ErrForbiddenCompetitionWrite
	}

	current, err := s.repo.GetCompetitionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	oldObjectKey := current.CoverObjectKey

	if req.Slug != nil {
		current.Slug = strings.TrimSpace(*req.Slug)
	}
	if req.Title != nil {
		current.Title = strings.TrimSpace(*req.Title)
	}
	if req.Type != nil {
		current.Type = strings.TrimSpace(*req.Type)
	}
	if req.City != nil {
		current.City = req.City
	}
	if req.Venue != nil {
		current.Venue = req.Venue
	}
	if req.Description != nil {
		current.Description = req.Description
	}
	if req.StartAt != nil {
		current.StartAt = *req.StartAt
	}
	if req.EndAt != nil {
		current.EndAt = *req.EndAt
	}
	if req.Timezone != nil {
		tz := strings.TrimSpace(*req.Timezone)
		if tz != "" {
			current.Timezone = tz
		}
	}
	if req.Status != nil {
		if err := validateStatus(*req.Status); err != nil {
			return nil, err
		}
		current.Status = *req.Status
	}
	if req.OrganizerID != nil {
		if actorRole == user.RoleOrganizer && *req.OrganizerID != actorID {
			return nil, ErrForbiddenCompetitionWrite
		}
		current.OrganizerID = *req.OrganizerID
	}

	if req.RemoveCover != nil && *req.RemoveCover {
		current.CoverBucket = nil
		current.CoverObjectKey = nil
		current.CoverPhotoID = nil
	}

	var newObjectKey *string

	log.Printf("competition update: cover exists=%v", cover != nil)

	if cover != nil {
		log.Printf(
			"competition update: cover file name=%s size=%d contentType=%s",
			cover.OriginalFilename,
			cover.Size,
			cover.ContentType,
		)

		objectKey := buildCoverObjectKey(cover.OriginalFilename)

		started := time.Now()

		if err := s.storage.PutObject(ctx, objectKey, cover.Reader, cover.Size, cover.ContentType); err != nil {
			return nil, err
		}

		log.Printf("competition update: PutObject took %s", time.Since(started))

		bucket := s.storage.BucketName()
		current.CoverBucket = &bucket
		current.CoverObjectKey = &objectKey
		current.CoverPhotoID = nil
		newObjectKey = &objectKey
	}

	if current.EndAt.Before(current.StartAt) {
		if newObjectKey != nil {
			_ = s.storage.RemoveObject(ctx, *newObjectKey)
		}
		return nil, ErrInvalidCompetitionDates
	}

	if actorRole == user.RoleOrganizer {
		current.OrganizerID = actorID
	}

	current.UpdatedBy = actorID

	started := time.Now()

	updated, err := s.repo.UpdateCompetition(ctx, current)

	log.Printf("competition update: UpdateCompetition took %s", time.Since(started))

	if err != nil {
		if newObjectKey != nil {
			_ = s.storage.RemoveObject(ctx, *newObjectKey)
		}
		return nil, err
	}

	if oldObjectKey != nil && ((req.RemoveCover != nil && *req.RemoveCover) || newObjectKey != nil) {
		_ = s.storage.RemoveObject(ctx, *oldObjectKey)
	}

	stages, err := s.repo.ListStages(ctx, updated.ID)
	if err != nil {
		return nil, err
	}

	updated.Stages = stages
	s.enrichCompetition(updated)

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, actorRole user.Role, id uuid.UUID) error {
	if !canManageCompetitions(actorRole) {
		return ErrForbiddenCompetitionWrite
	}

	current, err := s.repo.GetCompetitionByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteCompetition(ctx, id); err != nil {
		return err
	}

	if current.CoverObjectKey != nil {
		_ = s.storage.RemoveObject(ctx, *current.CoverObjectKey)
	}

	return nil
}

func (s *Service) ListStages(ctx context.Context, competitionID uuid.UUID) ([]Stage, error) {
	if _, err := s.repo.GetCompetitionByID(ctx, competitionID); err != nil {
		return nil, err
	}

	return s.repo.ListStages(ctx, competitionID)
}

func (s *Service) CreateStage(
	ctx context.Context,
	actorRole user.Role,
	competitionID uuid.UUID,
	req CreateStageRequest,
) (*Stage, error) {
	if !canManageCompetitions(actorRole) {
		return nil, ErrForbiddenCompetitionWrite
	}

	comp, err := s.repo.GetCompetitionByID(ctx, competitionID)
	if err != nil {
		return nil, err
	}

	stageDate := strings.TrimSpace(req.StageDate)
	stageEndDate := strings.TrimSpace(req.StageEndDate)

	if err := validateStageDateRange(stageDate, stageEndDate, comp); err != nil {
		return nil, err
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	stage := &Stage{
		CompetitionID: competitionID,
		Name:          strings.TrimSpace(req.Name),
		SortOrder:     req.SortOrder,
		StageDate:     stageDate,
		StageEndDate:  stageEndDate,
		IsActive:      isActive,
	}

	return s.repo.CreateStage(ctx, stage)
}

func (s *Service) UpdateStage(
	ctx context.Context,
	actorRole user.Role,
	competitionID uuid.UUID,
	stageID uuid.UUID,
	req UpdateStageRequest,
) (*Stage, error) {
	if !canManageCompetitions(actorRole) {
		return nil, ErrForbiddenCompetitionWrite
	}

	comp, err := s.repo.GetCompetitionByID(ctx, competitionID)
	if err != nil {
		return nil, err
	}

	stage, err := s.repo.GetStageByID(ctx, competitionID, stageID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		stage.Name = strings.TrimSpace(*req.Name)
	}

	if req.SortOrder != nil {
		stage.SortOrder = *req.SortOrder
	}

	if req.StageDate != nil {
		stage.StageDate = strings.TrimSpace(*req.StageDate)
	}

	if req.StageEndDate != nil {
		stage.StageEndDate = strings.TrimSpace(*req.StageEndDate)
	}

	if err := validateStageDateRange(stage.StageDate, stage.StageEndDate, comp); err != nil {
		return nil, err
	}

	if req.IsActive != nil {
		stage.IsActive = *req.IsActive
	}

	return s.repo.UpdateStage(ctx, stage)
}

func (s *Service) DeleteStage(ctx context.Context, actorRole user.Role, competitionID, stageID uuid.UUID) error {
	if !canManageCompetitions(actorRole) {
		return ErrForbiddenCompetitionWrite
	}

	if _, err := s.repo.GetCompetitionByID(ctx, competitionID); err != nil {
		return err
	}

	return s.repo.DeleteStage(ctx, competitionID, stageID)
}

func (s *Service) enrichCompetition(c *Competition) {
	if c == nil {
		return
	}

	if c.CoverObjectKey != nil {
		url := s.storage.ObjectURL(*c.CoverObjectKey)
		c.CoverURL = &url
	} else {
		c.CoverURL = nil
	}

	if c.Stages == nil {
		c.Stages = []Stage{}
	}
}

func canManageCompetitions(role user.Role) bool {
	return role == user.RoleAdmin || role == user.RoleOrganizer
}

func validateStatus(status Status) error {
	switch status {
	case StatusDraft, StatusPublished, StatusArchived:
		return nil
	default:
		return ErrInvalidCompetitionStatus
	}
}

func validateStageDateRange(stageStartRaw, stageEndRaw string, comp *Competition) error {
	stageStart, err := parseStageDate(stageStartRaw)
	if err != nil {
		return err
	}

	stageEnd, err := parseStageDate(stageEndRaw)
	if err != nil {
		return err
	}

	if stageEnd.Before(stageStart) {
		return ErrInvalidStageDate
	}

	competitionStart := startOfDay(comp.StartAt)
	competitionEnd := endOfDay(comp.EndAt)

	if stageStart.Before(competitionStart) || stageStart.After(competitionEnd) {
		return ErrInvalidStageDate
	}

	if stageEnd.Before(competitionStart) || stageEnd.After(competitionEnd) {
		return ErrInvalidStageDate
	}

	return nil
}

func parseStageDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, ErrInvalidStageDate
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, ErrInvalidStageDate
	}

	return parsed, nil
}

func startOfDay(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func endOfDay(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 23, 59, 59, int(time.Second-time.Nanosecond), value.Location())
}

func buildCoverObjectKey(originalFilename string) string {
	ext := strings.ToLower(filepath.Ext(originalFilename))
	if ext == "" {
		ext = ".bin"
	}

	return fmt.Sprintf("%s%s", uuid.NewString(), ext)
}
