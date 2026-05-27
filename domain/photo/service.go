package photo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	accessdomain "photo-service-back/domain/access"
	competitiondomain "photo-service-back/domain/competition"
	"photo-service-back/domain/user"

	"github.com/google/uuid"
)

const maxUploadFiles = 5000

type CompetitionReader interface {
	GetCompetitionByID(ctx context.Context, id uuid.UUID) (*competitiondomain.Competition, error)
	GetStageByID(ctx context.Context, competitionID, stageID uuid.UUID) (*competitiondomain.Stage, error)
}

type AccessChecker interface {
	CanDownloadOriginal(ctx context.Context, actorID uuid.UUID, actorRole user.Role, competitionID uuid.UUID) (bool, error)
}

type Storage interface {
	OriginalBucket() string
	DerivedBucket() string

	BuildOriginalObjectKey(competitionSlug string, photoID uuid.UUID, originalFilename string) string
	BuildWatermarkedObjectKey(competitionSlug string, photoID uuid.UUID) string
	BuildPreviewObjectKey(competitionSlug string, photoID uuid.UUID) string

	PutOriginalFromPath(ctx context.Context, objectKey, filePath, contentType string) error
	PutDerivedFromPath(ctx context.Context, objectKey, filePath, contentType string) error
	RemoveObject(ctx context.Context, bucket, objectKey string) error

	ObjectURL(bucket, objectKey string) string
	OpenObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, string, error)
}

type DownloadFile struct {
	FileName    string
	ContentType string
	Reader      io.ReadCloser
}

type Service struct {
	repo         Repository
	processor    Processor
	storage      Storage
	access       AccessChecker
	competitions CompetitionReader
	recognizer   BibRecognizer
}

func NewService(
	repo Repository,
	processor Processor,
	storage Storage,
	access AccessChecker,
	competitions CompetitionReader,
	recognizer BibRecognizer,
) *Service {
	return &Service{
		repo:         repo,
		processor:    processor,
		storage:      storage,
		access:       access,
		competitions: competitions,
		recognizer:   recognizer,
	}
}

func (s *Service) ListCompetitionPhotos(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	filter ListPhotosFilter,
) (*PhotoListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	if _, err := s.competitions.GetCompetitionByID(ctx, filter.CompetitionID); err != nil {
		return nil, err
	}

	if filter.StageID != nil {
		if _, err := s.competitions.GetStageByID(ctx, filter.CompetitionID, *filter.StageID); err != nil {
			return nil, ErrInvalidStage
		}
	}

	if filter.Bib != nil {
		n := normalizeBib(*filter.Bib)
		filter.Bib = &n
	}

	total, err := s.repo.CountPhotos(ctx, filter)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.ListPhotos(ctx, filter)
	if err != nil {
		return nil, err
	}

	items, err = s.hydratePhotos(ctx, actorID, actorRole, filter.CompetitionID, items)
	if err != nil {
		return nil, err
	}

	return &PhotoListResult{
		Items:    items,
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	}, nil
}

func (s *Service) GetPhotoByID(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	photoID uuid.UUID,
) (*Photo, error) {
	p, err := s.repo.GetPhotoByID(ctx, photoID)
	if err != nil {
		return nil, err
	}

	if p.DeletedAt != nil {
		return nil, ErrPhotoDeleted
	}

	items, err := s.hydratePhotos(ctx, actorID, actorRole, p.CompetitionID, []Photo{*p})
	if err != nil {
		return nil, err
	}

	return &items[0], nil
}

func (s *Service) UploadPhotos(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	competitionID uuid.UUID,
	req UploadPhotosRequest,
	files []UploadFile,
) (*UploadPhotosResult, error) {
	if len(files) == 0 {
		return nil, ErrNoFilesProvided
	}
	if len(files) > maxUploadFiles {
		return nil, ErrTooManyFiles
	}

	comp, err := s.competitions.GetCompetitionByID(ctx, competitionID)
	if err != nil {
		return nil, err
	}

	if !canUploadToCompetition(actorID, actorRole, comp) {
		return nil, ErrForbiddenPhotoUpload
	}

	if req.StageID != nil {
		if _, err := s.competitions.GetStageByID(ctx, competitionID, *req.StageID); err != nil {
			return nil, ErrInvalidStage
		}
	}

	result := &UploadPhotosResult{
		Items:  []UploadPhotoItemResult{},
		Failed: []UploadPhotoItemResult{},
	}

	for _, f := range files {
		created, err := s.processSingleUpload(
			ctx,
			actorID,
			actorRole,
			comp,
			req.StageID,
			f,
		)
		if err != nil {
			result.Failed = append(result.Failed, UploadPhotoItemResult{
				FileName: f.FileName,
				Error:    err.Error(),
			})
			continue
		}

		result.Items = append(result.Items, UploadPhotoItemResult{
			FileName: f.FileName,
			Photo:    created,
		})
	}

	return result, nil
}

func (s *Service) processSingleUpload(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	comp *competitiondomain.Competition,
	stageID *uuid.UUID,
	file UploadFile,
) (*Photo, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	ext := strings.ToLower(filepath.Ext(file.FileName))
	tmpOriginal, err := os.CreateTemp("", "photo-original-*"+ext)
	if err != nil {
		return nil, err
	}

	tmpOriginalPath := tmpOriginal.Name()

	if _, err := io.Copy(tmpOriginal, src); err != nil {
		_ = tmpOriginal.Close()
		_ = os.Remove(tmpOriginalPath)
		return nil, err
	}

	if err := tmpOriginal.Close(); err != nil {
		_ = os.Remove(tmpOriginalPath)
		return nil, err
	}

	defer cleanupTempVariant(tmpOriginalPath)

	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	processed, err := s.processor.Process(ctx, ProcessInput{
		SourcePath:       tmpOriginalPath,
		OriginalFilename: file.FileName,
		DeclaredMimeType: contentType,
	})
	if err != nil {
		return nil, err
	}

	defer cleanupTempVariant(processed.Watermarked.TempFilePath)
	defer cleanupTempVariant(processed.Preview.TempFilePath)

	photoID := uuid.New()

	originalKey := s.storage.BuildOriginalObjectKey(comp.Slug, photoID, file.FileName)
	watermarkedKey := s.storage.BuildWatermarkedObjectKey(comp.Slug, photoID)
	previewKey := s.storage.BuildPreviewObjectKey(comp.Slug, photoID)

	if err := s.storage.PutOriginalFromPath(
		ctx,
		originalKey,
		processed.Original.TempFilePath,
		processed.Original.MimeType,
	); err != nil {
		return nil, err
	}

	if err := s.storage.PutDerivedFromPath(
		ctx,
		watermarkedKey,
		processed.Watermarked.TempFilePath,
		processed.Watermarked.MimeType,
	); err != nil {
		_ = s.storage.RemoveObject(ctx, s.storage.OriginalBucket(), originalKey)
		return nil, err
	}

	if err := s.storage.PutDerivedFromPath(
		ctx,
		previewKey,
		processed.Preview.TempFilePath,
		processed.Preview.MimeType,
	); err != nil {
		_ = s.storage.RemoveObject(ctx, s.storage.OriginalBucket(), originalKey)
		_ = s.storage.RemoveObject(ctx, s.storage.DerivedBucket(), watermarkedKey)
		return nil, err
	}

	width := processed.Original.Width
	height := processed.Original.Height

	p := &Photo{
		ID:                   photoID,
		CompetitionID:        comp.ID,
		StageID:              stageID,
		AuthorUserID:         actorID,
		OriginalFilename:     file.FileName,
		MimeType:             processed.Original.MimeType,
		SizeBytes:            processed.Original.SizeBytes,
		DayDate:              processed.DayDate,
		Width:                &width,
		Height:               &height,
		PrimaryBib:           nil,
		BibRecognitionStatus: BibRecognitionStatusPending,
		BibRecognitionError:  nil,
		WatermarkRequired:    true,
	}

	versions := []PhotoVersion{
		{
			ID:            uuid.New(),
			PhotoID:       photoID,
			Variant:       VariantOriginal,
			StorageBucket: s.storage.OriginalBucket(),
			ObjectKey:     originalKey,
			MimeType:      processed.Original.MimeType,
			SizeBytes:     processed.Original.SizeBytes,
			Width:         processed.Original.Width,
			Height:        processed.Original.Height,
		},
		{
			ID:            uuid.New(),
			PhotoID:       photoID,
			Variant:       VariantWatermarked,
			StorageBucket: s.storage.DerivedBucket(),
			ObjectKey:     watermarkedKey,
			MimeType:      processed.Watermarked.MimeType,
			SizeBytes:     processed.Watermarked.SizeBytes,
			Width:         processed.Watermarked.Width,
			Height:        processed.Watermarked.Height,
		},
		{
			ID:            uuid.New(),
			PhotoID:       photoID,
			Variant:       VariantPreview,
			StorageBucket: s.storage.DerivedBucket(),
			ObjectKey:     previewKey,
			MimeType:      processed.Preview.MimeType,
			SizeBytes:     processed.Preview.SizeBytes,
			Width:         processed.Preview.Width,
			Height:        processed.Preview.Height,
		},
	}

	created, err := s.repo.CreatePhotoWithVersions(ctx, p, versions)
	if err != nil {
		_ = s.storage.RemoveObject(ctx, s.storage.OriginalBucket(), originalKey)
		_ = s.storage.RemoveObject(ctx, s.storage.DerivedBucket(), watermarkedKey)
		_ = s.storage.RemoveObject(ctx, s.storage.DerivedBucket(), previewKey)
		return nil, err
	}

	s.scheduleBibRecognition(photoID, file.FileName, s.storage.OriginalBucket(), originalKey)

	items, err := s.hydratePhotos(ctx, actorID, actorRole, comp.ID, []Photo{*created})
	if err != nil {
		return nil, err
	}

	return &items[0], nil
}

func (s *Service) UpdatePhoto(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	photoID uuid.UUID,
	req UpdatePhotoRequest,
) (*Photo, error) {
	info, err := s.repo.GetPhotoAccessInfo(ctx, photoID)
	if err != nil {
		return nil, err
	}
	if info.DeletedAt != nil {
		return nil, ErrPhotoDeleted
	}
	if !canManagePhoto(actorID, actorRole, info) {
		return nil, ErrForbiddenPhotoWrite
	}

	p, err := s.repo.GetPhotoByID(ctx, photoID)
	if err != nil {
		return nil, err
	}

	if req.ClearStageID != nil && *req.ClearStageID {
		p.StageID = nil
	}
	if req.StageID != nil {
		if _, err := s.competitions.GetStageByID(ctx, p.CompetitionID, *req.StageID); err != nil {
			return nil, ErrInvalidStage
		}
		p.StageID = req.StageID
	}

	if req.ClearPrimaryBib != nil && *req.ClearPrimaryBib {
		p.PrimaryBib = nil
	}
	if req.PrimaryBib != nil {
		v := strings.TrimSpace(*req.PrimaryBib)
		if v == "" {
			p.PrimaryBib = nil
		} else {
			p.PrimaryBib = &v
		}
	}

	updated, err := s.repo.UpdatePhoto(ctx, p)
	if err != nil {
		return nil, err
	}

	items, err := s.hydratePhotos(ctx, actorID, actorRole, updated.CompetitionID, []Photo{*updated})
	if err != nil {
		return nil, err
	}

	return &items[0], nil
}

func (s *Service) DeletePhoto(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	photoID uuid.UUID,
) error {
	info, err := s.repo.GetPhotoAccessInfo(ctx, photoID)
	if err != nil {
		return err
	}
	if info.DeletedAt != nil {
		return ErrPhotoDeleted
	}
	if !canManagePhoto(actorID, actorRole, info) {
		return ErrForbiddenPhotoWrite
	}

	return s.repo.SoftDeletePhoto(ctx, photoID, actorID, time.Now())
}

func (s *Service) AddBib(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	photoID uuid.UUID,
	req AddBibRequest,
) (*PhotoBib, error) {
	info, err := s.repo.GetPhotoAccessInfo(ctx, photoID)
	if err != nil {
		return nil, err
	}
	if info.DeletedAt != nil {
		return nil, ErrPhotoDeleted
	}
	if !canManagePhoto(actorID, actorRole, info) {
		return nil, ErrForbiddenPhotoWrite
	}

	rawBib := strings.TrimSpace(req.BibValue)
	norm := normalizeBib(rawBib)
	if rawBib == "" || norm == "" {
		return nil, ErrInvalidBib
	}

	bib := &PhotoBib{
		PhotoID:         photoID,
		BibValue:        rawBib,
		NormalizedBib:   norm,
		Source:          BibSourceManual,
		CreatedByUserID: &actorID,
	}

	out, err := s.repo.AddPhotoBib(ctx, bib)
	if err != nil {
		return nil, err
	}

	p, err := s.repo.GetPhotoByID(ctx, photoID)
	if err != nil {
		return nil, err
	}
	if p.PrimaryBib == nil || strings.TrimSpace(*p.PrimaryBib) == "" {
		if err := s.repo.SetPrimaryBib(ctx, photoID, &out.BibValue); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func (s *Service) DeleteBib(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	photoID, bibID uuid.UUID,
) error {
	info, err := s.repo.GetPhotoAccessInfo(ctx, photoID)
	if err != nil {
		return err
	}
	if info.DeletedAt != nil {
		return ErrPhotoDeleted
	}
	if !canManagePhoto(actorID, actorRole, info) {
		return ErrForbiddenPhotoWrite
	}

	p, err := s.repo.GetPhotoByID(ctx, photoID)
	if err != nil {
		return err
	}

	bib, err := s.repo.GetPhotoBibByID(ctx, photoID, bibID)
	if err != nil {
		return err
	}

	if err := s.repo.DeletePhotoBib(ctx, photoID, bibID); err != nil {
		return err
	}

	if p.PrimaryBib != nil && *p.PrimaryBib == bib.BibValue {
		remaining, err := s.repo.ListPhotoBibsByPhotoID(ctx, photoID)
		if err != nil {
			return err
		}

		if len(remaining) == 0 {
			if err := s.repo.SetPrimaryBib(ctx, photoID, nil); err != nil {
				return err
			}
		} else {
			next := remaining[0].BibValue
			if err := s.repo.SetPrimaryBib(ctx, photoID, &next); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Service) OpenSingleDownload(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	photoID uuid.UUID,
) (*DownloadFile, error) {
	ref, err := s.resolveDownloadRef(ctx, actorID, actorRole, photoID)
	if err != nil {
		return nil, err
	}

	reader, contentType, err := s.storage.OpenObject(ctx, ref.Bucket, ref.ObjectKey)
	if err != nil {
		return nil, err
	}

	return &DownloadFile{
		FileName:    ref.FileName,
		ContentType: contentType,
		Reader:      reader,
	}, nil
}

func (s *Service) OpenBatchDownloads(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	photoIDs []uuid.UUID,
) ([]DownloadFile, error) {
	if len(photoIDs) == 0 {
		return nil, ErrEmptyPhotoIDs
	}

	files := make([]DownloadFile, 0, len(photoIDs))
	for _, photoID := range photoIDs {
		ref, err := s.resolveDownloadRef(ctx, actorID, actorRole, photoID)
		if err != nil {
			for i := range files {
				_ = files[i].Reader.Close()
			}
			return nil, err
		}

		reader, contentType, err := s.storage.OpenObject(ctx, ref.Bucket, ref.ObjectKey)
		if err != nil {
			for i := range files {
				_ = files[i].Reader.Close()
			}
			return nil, err
		}

		files = append(files, DownloadFile{
			FileName:    ref.FileName,
			ContentType: contentType,
			Reader:      reader,
		})
	}

	return files, nil
}

type resolvedDownloadRef struct {
	Bucket    string
	ObjectKey string
	FileName  string
}

func (s *Service) resolveDownloadRef(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	photoID uuid.UUID,
) (*resolvedDownloadRef, error) {
	info, err := s.repo.GetPhotoAccessInfo(ctx, photoID)
	if err != nil {
		return nil, err
	}
	if info.DeletedAt != nil {
		return nil, ErrPhotoDeleted
	}

	p, err := s.repo.GetPhotoByID(ctx, photoID)
	if err != nil {
		return nil, err
	}

	canOriginal, err := s.access.CanDownloadOriginal(ctx, actorID, actorRole, info.CompetitionID)
	if err != nil {
		return nil, err
	}
	if actorID == info.AuthorUserID {
		canOriginal = true
	}

	variant := VariantWatermarked
	fileName := buildWatermarkedDownloadName(p.OriginalFilename)

	if canOriginal {
		variant = VariantOriginal
		fileName = p.OriginalFilename
	}

	version, err := s.repo.GetPhotoVersionByVariant(ctx, photoID, variant)
	if err != nil {
		return nil, err
	}

	return &resolvedDownloadRef{
		Bucket:    version.StorageBucket,
		ObjectKey: version.ObjectKey,
		FileName:  fileName,
	}, nil
}

func (s *Service) scheduleBibRecognition(photoID uuid.UUID, fileName, bucket, objectKey string) {
	if s.recognizer == nil {
		return
	}

	go s.processBibRecognition(photoID, fileName, bucket, objectKey)
}

func (s *Service) processBibRecognition(photoID uuid.UUID, fileName, bucket, objectKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusProcessing, nil)

	reader, _, err := s.storage.OpenObject(ctx, bucket, objectKey)
	if err != nil {
		msg := err.Error()
		_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusFailed, &msg)
		return
	}
	defer reader.Close()

	result, err := s.recognizer.RecognizeBib(ctx, photoID.String(), fileName, reader)
	if err != nil {
		msg := err.Error()
		_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusFailed, &msg)
		return
	}
	if result == nil {
		msg := "empty recognizer response"
		_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusFailed, &msg)
		return
	}

	switch result.Status {
	case BibRecognitionStatusCompleted:
		if len(result.Bibs) == 0 {
			_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusNotFound, nil)
			return
		}

		var primaryBib *string
		addedOrExistingCount := 0

		for _, recognizedBib := range result.Bibs {
			bibValue := strings.TrimSpace(recognizedBib.Bib)
			if bibValue == "" {
				continue
			}

			normalized := normalizeBib(bibValue)
			if normalized == "" {
				continue
			}

			_, err := s.repo.AddPhotoBib(ctx, &PhotoBib{
				PhotoID:       photoID,
				BibValue:      bibValue,
				NormalizedBib: normalized,
				Source:        BibSourceOCR,
				Confidence:    recognizedBib.Confidence,
			})
			if err != nil && !errors.Is(err, ErrPhotoBibAlreadyExists) {
				msg := err.Error()
				_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusFailed, &msg)
				return
			}

			addedOrExistingCount++

			if primaryBib == nil {
				value := bibValue
				primaryBib = &value
			}
		}

		if addedOrExistingCount == 0 || primaryBib == nil {
			_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusNotFound, nil)
			return
		}

		_ = s.repo.SetPrimaryBib(ctx, photoID, primaryBib)
		_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusCompleted, nil)

	case BibRecognitionStatusNotFound:
		_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusNotFound, nil)

	case BibRecognitionStatusFailed:
		msg := "ocr failed"
		if result.Error != nil && strings.TrimSpace(*result.Error) != "" {
			msg = strings.TrimSpace(*result.Error)
		}
		_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusFailed, &msg)

	default:
		msg := "unknown recognizer status"
		_ = s.repo.UpdateBibRecognitionStatus(ctx, photoID, BibRecognitionStatusFailed, &msg)
	}
}

func (s *Service) hydratePhotos(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	competitionID uuid.UUID,
	photos []Photo,
) ([]Photo, error) {
	if len(photos) == 0 {
		return photos, nil
	}

	ids := make([]uuid.UUID, 0, len(photos))
	for i := range photos {
		ids = append(ids, photos[i].ID)
	}

	versionsMap, err := s.repo.ListPhotoVersionsByPhotoIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	bibsMap, err := s.repo.ListPhotoBibsByPhotoIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	canOriginalForCompetition, err := s.access.CanDownloadOriginal(ctx, actorID, actorRole, competitionID)
	if err != nil {
		return nil, err
	}

	for i := range photos {
		photos[i].Versions = versionsMap[photos[i].ID]
		if photos[i].Versions == nil {
			photos[i].Versions = []PhotoVersion{}
		}
		photos[i].Bibs = bibsMap[photos[i].ID]
		if photos[i].Bibs == nil {
			photos[i].Bibs = []PhotoBib{}
		}

		photos[i].CanDownloadOriginal = canOriginalForCompetition || photos[i].AuthorUserID == actorID
		s.enrichPhotoURLs(&photos[i])
	}

	return photos, nil
}

func (s *Service) enrichPhotoURLs(p *Photo) {
	p.PreviewURL = nil
	p.WatermarkedURL = nil

	for i := range p.Versions {
		if p.Versions[i].StorageBucket == s.storage.OriginalBucket() {
			p.Versions[i].URL = nil
			continue
		}

		url := s.storage.ObjectURL(p.Versions[i].StorageBucket, p.Versions[i].ObjectKey)
		p.Versions[i].URL = &url

		switch p.Versions[i].Variant {
		case VariantPreview:
			p.PreviewURL = &url
		case VariantWatermarked:
			p.WatermarkedURL = &url
		}
	}
}

func canUploadToCompetition(actorID uuid.UUID, actorRole user.Role, comp *competitiondomain.Competition) bool {
	if actorRole == user.RoleAdmin {
		return true
	}
	if actorRole == user.RoleOrganizer && comp.OrganizerID == actorID {
		return true
	}
	return actorRole == user.RolePhotographer && comp.Status == competitiondomain.StatusPublished
}

func canManagePhoto(actorID uuid.UUID, actorRole user.Role, info *PhotoAccessInfo) bool {
	if actorRole == user.RoleAdmin {
		return true
	}
	if info.AuthorUserID == actorID {
		return true
	}
	return actorRole == user.RoleOrganizer && info.OrganizerID == actorID
}

func normalizeBib(v string) string {
	v = strings.TrimSpace(strings.ToUpper(v))
	v = strings.ReplaceAll(v, " ", "")
	v = strings.ReplaceAll(v, "-", "")
	return v
}

func buildWatermarkedDownloadName(original string) string {
	ext := filepath.Ext(original)
	base := strings.TrimSuffix(original, ext)
	if ext == "" {
		ext = ".jpg"
	}
	return fmt.Sprintf("%s_watermarked%s", base, ext)
}

func cleanupTempVariant(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

var _ AccessChecker = (*accessdomain.Service)(nil)

type preparedUpload struct {
	File UploadFile

	PhotoID uuid.UUID

	TempOriginalPath string

	OriginalObjectKey    string
	WatermarkedObjectKey string
	PreviewObjectKey     string
}

func (s *Service) prepareUploadForProcessing(
	ctx context.Context,
	comp *competitiondomain.Competition,
	file UploadFile,
) (preparedUpload, ProcessInput, error) {
	src, err := file.Open()
	if err != nil {
		return preparedUpload{}, ProcessInput{}, err
	}
	defer src.Close()

	ext := strings.ToLower(filepath.Ext(file.FileName))
	tmpOriginal, err := os.CreateTemp("", "photo-original-*"+ext)
	if err != nil {
		return preparedUpload{}, ProcessInput{}, err
	}

	tmpPath := tmpOriginal.Name()

	if _, err := io.Copy(tmpOriginal, src); err != nil {
		_ = tmpOriginal.Close()
		_ = os.Remove(tmpPath)
		return preparedUpload{}, ProcessInput{}, err
	}

	if err := tmpOriginal.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return preparedUpload{}, ProcessInput{}, err
	}

	photoID := uuid.New()

	originalKey := s.storage.BuildOriginalObjectKey(comp.Slug, photoID, file.FileName)
	watermarkedKey := s.storage.BuildWatermarkedObjectKey(comp.Slug, photoID)
	previewKey := s.storage.BuildPreviewObjectKey(comp.Slug, photoID)

	contentType := file.ContentType
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	if err := s.storage.PutOriginalFromPath(ctx, originalKey, tmpPath, contentType); err != nil {
		_ = os.Remove(tmpPath)
		return preparedUpload{}, ProcessInput{}, err
	}

	prep := preparedUpload{
		File:                 file,
		PhotoID:              photoID,
		TempOriginalPath:     tmpPath,
		OriginalObjectKey:    originalKey,
		WatermarkedObjectKey: watermarkedKey,
		PreviewObjectKey:     previewKey,
	}

	input := ProcessInput{
		SourcePath:       tmpPath,
		OriginalFilename: file.FileName,
		DeclaredMimeType: contentType,

		OriginalBucket:    s.storage.OriginalBucket(),
		OriginalObjectKey: originalKey,

		DerivedBucket:        s.storage.DerivedBucket(),
		PreviewObjectKey:     previewKey,
		WatermarkedObjectKey: watermarkedKey,
	}

	return prep, input, nil
}

func (s *Service) finalizeProcessedUpload(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole user.Role,
	comp *competitiondomain.Competition,
	stageID *uuid.UUID,
	prep preparedUpload,
	processed *ProcessedPhoto,
) (*Photo, error) {
	if processed == nil {
		return nil, fmt.Errorf("processed photo is nil")
	}

	defer cleanupTempVariant(processed.Watermarked.TempFilePath)
	defer cleanupTempVariant(processed.Preview.TempFilePath)

	if !processed.Watermarked.AlreadyUploaded {
		if err := s.storage.PutDerivedFromPath(
			ctx,
			prep.WatermarkedObjectKey,
			processed.Watermarked.TempFilePath,
			processed.Watermarked.MimeType,
		); err != nil {
			return nil, err
		}
	}

	if !processed.Preview.AlreadyUploaded {
		if err := s.storage.PutDerivedFromPath(
			ctx,
			prep.PreviewObjectKey,
			processed.Preview.TempFilePath,
			processed.Preview.MimeType,
		); err != nil {
			return nil, err
		}
	}

	width := processed.Original.Width
	height := processed.Original.Height

	p := &Photo{
		ID:                   prep.PhotoID,
		CompetitionID:        comp.ID,
		StageID:              stageID,
		AuthorUserID:         actorID,
		OriginalFilename:     prep.File.FileName,
		MimeType:             processed.Original.MimeType,
		SizeBytes:            processed.Original.SizeBytes,
		DayDate:              processed.DayDate,
		Width:                &width,
		Height:               &height,
		PrimaryBib:           nil,
		BibRecognitionStatus: BibRecognitionStatusPending,
		BibRecognitionError:  nil,
		WatermarkRequired:    true,
	}

	versions := []PhotoVersion{
		{
			ID:            uuid.New(),
			PhotoID:       prep.PhotoID,
			Variant:       VariantOriginal,
			StorageBucket: s.storage.OriginalBucket(),
			ObjectKey:     prep.OriginalObjectKey,
			MimeType:      processed.Original.MimeType,
			SizeBytes:     processed.Original.SizeBytes,
			Width:         processed.Original.Width,
			Height:        processed.Original.Height,
		},
		{
			ID:            uuid.New(),
			PhotoID:       prep.PhotoID,
			Variant:       VariantWatermarked,
			StorageBucket: s.storage.DerivedBucket(),
			ObjectKey:     prep.WatermarkedObjectKey,
			MimeType:      processed.Watermarked.MimeType,
			SizeBytes:     processed.Watermarked.SizeBytes,
			Width:         processed.Watermarked.Width,
			Height:        processed.Watermarked.Height,
		},
		{
			ID:            uuid.New(),
			PhotoID:       prep.PhotoID,
			Variant:       VariantPreview,
			StorageBucket: s.storage.DerivedBucket(),
			ObjectKey:     prep.PreviewObjectKey,
			MimeType:      processed.Preview.MimeType,
			SizeBytes:     processed.Preview.SizeBytes,
			Width:         processed.Preview.Width,
			Height:        processed.Preview.Height,
		},
	}

	created, err := s.repo.CreatePhotoWithVersions(ctx, p, versions)
	if err != nil {
		return nil, err
	}

	s.scheduleBibRecognition(prep.PhotoID, prep.File.FileName, s.storage.OriginalBucket(), prep.OriginalObjectKey)

	items, err := s.hydratePhotos(ctx, actorID, actorRole, comp.ID, []Photo{*created})
	if err != nil {
		return nil, err
	}

	return &items[0], nil
}
