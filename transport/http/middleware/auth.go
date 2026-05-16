package middleware

import (
	"net/http"
	"strings"

	"photo-service-back/domain/user"
	"photo-service-back/infra/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const CurrentUserKey = "currentUser"

type AuthMiddleware struct {
	tm    *auth.TokenManager
	users user.UserRepository
}

func NewAuthMiddleware(tm *auth.TokenManager, users user.UserRepository) *AuthMiddleware {
	return &AuthMiddleware{tm: tm, users: users}
}

func CurrentUser(c *gin.Context) (*user.User, bool) {
	val, ok := c.Get(CurrentUserKey)
	if !ok {
		return nil, false
	}

	u, ok := val.(*user.User)
	if !ok {
		return nil, false
	}

	return u, true
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		raw := strings.TrimPrefix(h, "Bearer ")
		claims, err := m.tm.ParseAccessToken(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
			return
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
			return
		}

		u, err := m.users.GetByID(c.Request.Context(), userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}

		if u.Status != user.StatusActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "user is not active"})
			return
		}

		c.Set(CurrentUserKey, u)
		c.Next()
	}
}

func RequireRoles(roles ...user.Role) gin.HandlerFunc {
	allowed := make(map[user.Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		val, ok := c.Get(CurrentUserKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		u := val.(*user.User)
		if _, ok := allowed[u.Role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.Next()
	}
}
