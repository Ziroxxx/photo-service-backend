package handlers

import (
	"net/http"

	"photo-service-back/domain/user"
	"photo-service-back/transport/http/middleware"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authSvc *user.AuthService
	userSvc *user.UserService
}

func NewAuthHandler(authSvc *user.AuthService, userSvc *user.UserService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, userSvc: userSvc}
}

func clientMeta(c *gin.Context) (*string, *string) {
	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")
	return &ip, &ua
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req user.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.authSvc.Register(c.Request.Context(), req)
	if err != nil {
		if err == user.ErrLoginAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "login is already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user": u})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req user.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ip, ua := clientMeta(c)
	resp, err := h.authSvc.Login(c.Request.Context(), req, ip, ua)
	if err != nil {
		switch err {
		case user.ErrInvalidCredentials:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case user.ErrInactiveUser:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req user.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ip, ua := clientMeta(c)
	resp, err := h.authSvc.Refresh(c.Request.Context(), req.RefreshToken, ip, ua)
	if err != nil {
		switch err {
		case user.ErrInvalidToken:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case user.ErrInactiveUser:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req user.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authSvc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) Me(c *gin.Context) {
	val, _ := c.Get(middleware.CurrentUserKey)
	u := val.(*user.User)
	c.JSON(http.StatusOK, u)
}
