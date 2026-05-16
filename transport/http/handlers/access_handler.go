package handlers

import (
	"net/http"

	accessdomain "photo-service-back/domain/access"
	competitiondomain "photo-service-back/domain/competition"
	"photo-service-back/domain/user"
	"photo-service-back/transport/http/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AccessHandler struct {
	svc *accessdomain.Service
}

func NewAccessHandler(svc *accessdomain.Service) *AccessHandler {
	return &AccessHandler{svc: svc}
}

func (h *AccessHandler) GetCompetitionAccess(c *gin.Context) {
	currentUser, ok := currentUserFromAccessContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	competitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	resp, err := h.svc.GetCompetitionAccess(c.Request.Context(), currentUser.ID, currentUser.Role, competitionID)
	if err != nil {
		writeAccessError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AccessHandler) CreateGrant(c *gin.Context) {
	currentUser, ok := currentUserFromAccessContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	competitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	var req accessdomain.CreateGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := h.svc.CreateGrant(c.Request.Context(), currentUser.ID, currentUser.Role, competitionID, req)
	if err != nil {
		writeAccessError(c, err)
		return
	}

	c.JSON(http.StatusCreated, out)
}

func (h *AccessHandler) UpdateGrant(c *gin.Context) {
	currentUser, ok := currentUserFromAccessContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	competitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	grantID, err := uuid.Parse(c.Param("grantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid grant id"})
		return
	}

	var req accessdomain.UpdateGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := h.svc.UpdateGrant(c.Request.Context(), currentUser.ID, currentUser.Role, competitionID, grantID, req)
	if err != nil {
		writeAccessError(c, err)
		return
	}

	c.JSON(http.StatusOK, out)
}

func (h *AccessHandler) DeleteGrant(c *gin.Context) {
	currentUser, ok := currentUserFromAccessContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	competitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid competition id"})
		return
	}

	grantID, err := uuid.Parse(c.Param("grantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid grant id"})
		return
	}

	if err := h.svc.DeleteGrant(c.Request.Context(), currentUser.ID, currentUser.Role, competitionID, grantID); err != nil {
		writeAccessError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func currentUserFromAccessContext(c *gin.Context) (*user.User, bool) {
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

func writeAccessError(c *gin.Context, err error) {
	switch err {
	case accessdomain.ErrGrantNotFound, competitiondomain.ErrCompetitionNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

	case accessdomain.ErrForbidden:
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})

	case accessdomain.ErrGrantAlreadyExists:
		c.JSON(http.StatusConflict, gin.H{"error": "access grant already exists"})

	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
