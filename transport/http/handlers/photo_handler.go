package handlers

import (
	"archive/zip"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"photo-service-back/domain/photo"
	"photo-service-back/domain/user"
	"photo-service-back/transport/http/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PhotoHandler struct {
	svc      *photo.Service
	asyncSvc *photo.AsyncUploadService
}

func NewPhotoHandler(svc *photo.Service, asyncSvc *photo.AsyncUploadService) *PhotoHandler {
	return &PhotoHandler{
		svc:      svc,
		asyncSvc: asyncSvc,
	}
}

func (h *PhotoHandler) ListCompetitionPhotos(c *gin.Context) {
	currentUser, ok := currentUserFromPhotoContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	competitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	filter := photo.ListPhotosFilter{
		CompetitionID: competitionID,
		Page:          page,
		PageSize:      pageSize,
	}

	if raw := strings.TrimSpace(c.Query("stageId")); raw != "" {
		stageID, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stage id"})
			return
		}
		filter.StageID = &stageID
	}

	if raw := strings.TrimSpace(c.Query("bib")); raw != "" {
		filter.Bib = &raw
	}

	result, err := h.svc.ListCompetitionPhotos(c.Request.Context(), currentUser.ID, currentUser.Role, filter)
	if err != nil {
		writePhotoError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *PhotoHandler) UploadPhotos(c *gin.Context) {
	currentUser, ok := currentUserFromPhotoContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	competitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	var req photo.UploadPhotosRequest
	if raw := strings.TrimSpace(c.PostForm("stageId")); raw != "" {
		stageID, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stage id"})
			return
		}
		req.StageID = &stageID
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid multipart form",
			"details": err.Error(),
		})
		return
	}

	headers := form.File["files"]
	if len(headers) == 0 {
		headers = form.File["file"]
	}
	if len(headers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files provided"})
		return
	}

	files := make([]photo.UploadFile, 0, len(headers))
	for _, hdr := range headers {
		fileHeader := hdr
		files = append(files, photo.UploadFile{
			FileName:    fileHeader.Filename,
			ContentType: fileHeader.Header.Get("Content-Type"),
			Size:        fileHeader.Size,
			Open: func() (photo.ReadCloseSeek, error) {
				return fileHeader.Open()
			},
		})
	}

	if h.asyncSvc != nil {
		uploadID := strings.TrimSpace(c.Query("uploadId"))

		out, err := h.asyncSvc.EnqueueUploads(
			c.Request.Context(),
			currentUser.ID,
			currentUser.Role,
			competitionID,
			req.StageID,
			uploadID,
			files,
		)
		if err != nil {
			writePhotoError(c, err)
			return
		}

		c.JSON(http.StatusAccepted, out)
		return
	}

	out, err := h.svc.UploadPhotos(c.Request.Context(), currentUser.ID, currentUser.Role, competitionID, req, files)
	if err != nil {
		writePhotoError(c, err)
		return
	}

	c.JSON(http.StatusCreated, out)
}

func (h *PhotoHandler) GetPhotoByID(c *gin.Context) {
	currentUser, ok := currentUserFromPhotoContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	photoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid photo id"})
		return
	}

	item, err := h.svc.GetPhotoByID(c.Request.Context(), currentUser.ID, currentUser.Role, photoID)
	if err != nil {
		writePhotoError(c, err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *PhotoHandler) UpdatePhoto(c *gin.Context) {
	currentUser, ok := currentUserFromPhotoContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	photoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid photo id"})
		return
	}

	var req photo.UpdatePhotoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.svc.UpdatePhoto(c.Request.Context(), currentUser.ID, currentUser.Role, photoID, req)
	if err != nil {
		writePhotoError(c, err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *PhotoHandler) DeletePhoto(c *gin.Context) {
	currentUser, ok := currentUserFromPhotoContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	photoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid photo id"})
		return
	}

	if err := h.svc.DeletePhoto(c.Request.Context(), currentUser.ID, currentUser.Role, photoID); err != nil {
		writePhotoError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PhotoHandler) AddBib(c *gin.Context) {
	currentUser, ok := currentUserFromPhotoContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	photoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid photo id"})
		return
	}

	var req photo.AddBibRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := h.svc.AddBib(c.Request.Context(), currentUser.ID, currentUser.Role, photoID, req)
	if err != nil {
		writePhotoError(c, err)
		return
	}

	c.JSON(http.StatusCreated, out)
}

func (h *PhotoHandler) DeleteBib(c *gin.Context) {
	currentUser, ok := currentUserFromPhotoContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	photoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid photo id"})
		return
	}

	bibID, err := uuid.Parse(c.Param("bibId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bib id"})
		return
	}

	if err := h.svc.DeleteBib(c.Request.Context(), currentUser.ID, currentUser.Role, photoID, bibID); err != nil {
		writePhotoError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PhotoHandler) DownloadPhoto(c *gin.Context) {
	currentUser, ok := currentUserFromPhotoContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	photoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid photo id"})
		return
	}

	file, err := h.svc.OpenSingleDownload(c.Request.Context(), currentUser.ID, currentUser.Role, photoID)
	if err != nil {
		writePhotoError(c, err)
		return
	}
	defer file.Reader.Close()

	c.Header("Content-Disposition", `attachment; filename="`+file.FileName+`"`)
	c.Header("Content-Type", file.ContentType)
	c.Status(http.StatusOK)

	_, _ = io.Copy(c.Writer, file.Reader)
}

func (h *PhotoHandler) DownloadPhotos(c *gin.Context) {
	currentUser, ok := currentUserFromPhotoContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req photo.DownloadPhotosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	files, err := h.svc.OpenBatchDownloads(c.Request.Context(), currentUser.ID, currentUser.Role, req.PhotoIDs)
	if err != nil {
		writePhotoError(c, err)
		return
	}

	if len(files) == 1 {
		defer files[0].Reader.Close()
		c.Header("Content-Disposition", `attachment; filename="`+files[0].FileName+`"`)
		c.Header("Content-Type", files[0].ContentType)
		c.Status(http.StatusOK)
		_, _ = io.Copy(c.Writer, files[0].Reader)
		return
	}

	defer func() {
		for i := range files {
			_ = files[i].Reader.Close()
		}
	}()

	c.Header("Content-Disposition", `attachment; filename="photos-`+time.Now().Format("20060102-150405")+`.zip"`)
	c.Header("Content-Type", "application/zip")
	c.Status(http.StatusOK)

	zw := zip.NewWriter(c.Writer)
	defer zw.Close()

	usedNames := map[string]int{}
	for i := range files {
		name := uniqueZipName(files[i].FileName, usedNames)
		w, err := zw.Create(name)
		if err != nil {
			return
		}
		_, _ = io.Copy(w, files[i].Reader)
		_ = files[i].Reader.Close()
	}

	_ = zw.Close()
}

func currentUserFromPhotoContext(c *gin.Context) (*user.User, bool) {
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

func uniqueZipName(name string, used map[string]int) string {
	if _, exists := used[name]; !exists {
		used[name] = 1
		return name
	}
	used[name]++
	ext := ""
	base := name
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		base = name[:idx]
		ext = name[idx:]
	}
	return base + "-" + strconv.Itoa(used[name]) + ext
}

func writePhotoError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, photo.ErrPhotoNotFound),
		errors.Is(err, photo.ErrPhotoVersionNotFound),
		errors.Is(err, photo.ErrPhotoBibNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

	case errors.Is(err, photo.ErrPhotoDeleted):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

	case errors.Is(err, photo.ErrForbiddenPhotoRead),
		errors.Is(err, photo.ErrForbiddenPhotoWrite),
		errors.Is(err, photo.ErrForbiddenPhotoUpload):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})

	case errors.Is(err, photo.ErrInvalidStage),
		errors.Is(err, photo.ErrInvalidImage),
		errors.Is(err, photo.ErrInvalidBib),
		errors.Is(err, photo.ErrEmptyPhotoIDs),
		errors.Is(err, photo.ErrTooManyFiles),
		errors.Is(err, photo.ErrNoFilesProvided):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

	case errors.Is(err, photo.ErrPhotoBibAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})

	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

func (h *PhotoHandler) GetUploadStatus(c *gin.Context) {
	if h.asyncSvc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "async upload is disabled"})
		return
	}

	uploadID := strings.TrimSpace(c.Param("uploadId"))
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload id"})
		return
	}

	out, err := h.asyncSvc.GetStatus(c.Request.Context(), uploadID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, out)
}
