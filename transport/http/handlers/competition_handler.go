package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"photo-service-back/domain/competition"
	"photo-service-back/domain/user"
	"photo-service-back/transport/http/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CompetitionHandler struct {
	svc *competition.Service
}

func NewCompetitionHandler(svc *competition.Service) *CompetitionHandler {
	return &CompetitionHandler{svc: svc}
}

func (h *CompetitionHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	items, err := h.svc.List(c.Request.Context(), competition.ListCompetitionsFilter{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *CompetitionHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	item, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		writeCompetitionError(c, err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *CompetitionHandler) Create(c *gin.Context) {
	currentUser, ok := currentUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	req, cover, err := parseCreateCompetitionRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.svc.Create(c.Request.Context(), currentUser.ID, currentUser.Role, req, cover)
	if err != nil {
		writeCompetitionError(c, err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *CompetitionHandler) Update(c *gin.Context) {
	currentUser, ok := currentUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	req, cover, err := parseUpdateCompetitionRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.svc.Update(c.Request.Context(), currentUser.ID, currentUser.Role, id, req, cover)
	if err != nil {
		writeCompetitionError(c, err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *CompetitionHandler) Delete(c *gin.Context) {
	currentUser, ok := currentUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), currentUser.Role, id); err != nil {
		writeCompetitionError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *CompetitionHandler) ListStages(c *gin.Context) {
	competitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	items, err := h.svc.ListStages(c.Request.Context(), competitionID)
	if err != nil {
		writeCompetitionError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *CompetitionHandler) CreateStage(c *gin.Context) {
	currentUser, ok := currentUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	competitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	var req competition.CreateStageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.svc.CreateStage(c.Request.Context(), currentUser.Role, competitionID, req)
	if err != nil {
		writeCompetitionError(c, err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *CompetitionHandler) UpdateStage(c *gin.Context) {
	currentUser, ok := currentUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	competitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	stageID, err := uuid.Parse(c.Param("stageId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stage id"})
		return
	}

	var req competition.UpdateStageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.svc.UpdateStage(c.Request.Context(), currentUser.Role, competitionID, stageID, req)
	if err != nil {
		writeCompetitionError(c, err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *CompetitionHandler) DeleteStage(c *gin.Context) {
	currentUser, ok := currentUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	competitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	stageID, err := uuid.Parse(c.Param("stageId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stage id"})
		return
	}

	if err := h.svc.DeleteStage(c.Request.Context(), currentUser.Role, competitionID, stageID); err != nil {
		writeCompetitionError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func parseCreateCompetitionRequest(c *gin.Context) (competition.CreateCompetitionRequest, *competition.UploadedFile, error) {
	var req competition.CreateCompetitionRequest

	req.Slug = strings.TrimSpace(c.PostForm("slug"))
	req.Title = strings.TrimSpace(c.PostForm("title"))
	req.Type = strings.TrimSpace(c.PostForm("type"))
	req.City = optionalString(c.PostForm("city"))
	req.Venue = optionalString(c.PostForm("venue"))
	req.Description = optionalString(c.PostForm("description"))
	req.Timezone = strings.TrimSpace(c.PostForm("timezone"))

	if req.Slug == "" || req.Title == "" || req.Type == "" {
		return req, nil, errors.New("slug, title and type are required")
	}

	startAtRaw := strings.TrimSpace(c.PostForm("startAt"))
	endAtRaw := strings.TrimSpace(c.PostForm("endAt"))
	if startAtRaw == "" || endAtRaw == "" {
		return req, nil, errors.New("startAt and endAt are required")
	}

	startAt, err := time.Parse(time.RFC3339, startAtRaw)
	if err != nil {
		return req, nil, errors.New("invalid startAt")
	}
	endAt, err := time.Parse(time.RFC3339, endAtRaw)
	if err != nil {
		return req, nil, errors.New("invalid endAt")
	}
	req.StartAt = startAt
	req.EndAt = endAt

	if statusRaw := strings.TrimSpace(c.PostForm("status")); statusRaw != "" {
		status := competition.Status(statusRaw)
		req.Status = &status
	}

	if organizerRaw := strings.TrimSpace(c.PostForm("organizerId")); organizerRaw != "" {
		id, err := uuid.Parse(organizerRaw)
		if err != nil {
			return req, nil, errors.New("invalid organizerId")
		}
		req.OrganizerID = &id
	}

	cover, err := readUploadedFile(c, "cover")
	if err != nil {
		return req, nil, err
	}

	return req, cover, nil
}

func parseUpdateCompetitionRequest(c *gin.Context) (competition.UpdateCompetitionRequest, *competition.UploadedFile, error) {
	var req competition.UpdateCompetitionRequest

	if raw, ok := c.GetPostForm("slug"); ok {
		v := strings.TrimSpace(raw)
		req.Slug = &v
	}
	if raw, ok := c.GetPostForm("title"); ok {
		v := strings.TrimSpace(raw)
		req.Title = &v
	}
	if raw, ok := c.GetPostForm("type"); ok {
		v := strings.TrimSpace(raw)
		req.Type = &v
	}
	if raw, ok := c.GetPostForm("city"); ok {
		v := raw
		req.City = &v
	}
	if raw, ok := c.GetPostForm("venue"); ok {
		v := raw
		req.Venue = &v
	}
	if raw, ok := c.GetPostForm("description"); ok {
		v := raw
		req.Description = &v
	}
	if raw, ok := c.GetPostForm("timezone"); ok {
		v := strings.TrimSpace(raw)
		req.Timezone = &v
	}
	if raw, ok := c.GetPostForm("status"); ok {
		v := competition.Status(strings.TrimSpace(raw))
		req.Status = &v
	}
	if raw, ok := c.GetPostForm("organizerId"); ok {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return req, nil, errors.New("invalid organizerId")
		}
		req.OrganizerID = &id
	}
	if raw, ok := c.GetPostForm("startAt"); ok {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
		if err != nil {
			return req, nil, errors.New("invalid startAt")
		}
		req.StartAt = &t
	}
	if raw, ok := c.GetPostForm("endAt"); ok {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
		if err != nil {
			return req, nil, errors.New("invalid endAt")
		}
		req.EndAt = &t
	}
	if raw, ok := c.GetPostForm("removeCover"); ok {
		b, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return req, nil, errors.New("invalid removeCover")
		}
		req.RemoveCover = &b
	}

	cover, err := readUploadedFile(c, "cover")
	if err != nil {
		return req, nil, err
	}

	return req, cover, nil
}

func readUploadedFile(c *gin.Context, fieldName string) (*competition.UploadedFile, error) {
	fileHeader, err := c.FormFile(fieldName)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, err
	}

	f, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &competition.UploadedFile{
		Reader:           f,
		Size:             fileHeader.Size,
		ContentType:      contentType,
		OriginalFilename: fileHeader.Filename,
	}, nil
}

func optionalString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func currentUserFromContext(c *gin.Context) (*user.User, bool) {
	val, ok := c.Get(middleware.CurrentUserKey)
	if !ok {
		return nil, false
	}

	u, ok := val.(*user.User)
	if !ok {
		return nil, false
	}

	return u, true
}

func writeCompetitionError(c *gin.Context, err error) {
	switch err {
	case competition.ErrCompetitionNotFound, competition.ErrStageNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

	case competition.ErrForbiddenCompetitionWrite:
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})

	case competition.ErrInvalidCompetitionDates, competition.ErrInvalidCompetitionStatus, competition.ErrInvalidStageDate:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

	case competition.ErrCompetitionSlugAlreadyExists:
		c.JSON(http.StatusConflict, gin.H{
			"error": "slug is already taken",
		})

	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
