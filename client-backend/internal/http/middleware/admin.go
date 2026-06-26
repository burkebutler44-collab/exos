package middleware

import (
	"net/http"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/services"

	"github.com/gin-gonic/gin"
)

func RequirePlatformPermission(svc *services.Services, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if err := svc.RequirePlatformPermission(c.Request.Context(), user.ID, permission); err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing platform permission", "permission": permission})
			return
		}
		c.Next()
	}
}

func HasPlatformPermission(c *gin.Context, svc *services.Services, permission string) bool {
	user, ok := CurrentUser(c)
	if !ok {
		return false
	}
	return svc.RequirePlatformPermission(c.Request.Context(), user.ID, permission) == nil
}

func CanViewOrganizationAsPlatformAdmin(c *gin.Context, svc *services.Services) bool {
	if c.Request.Method != http.MethodGet {
		return false
	}
	return HasPlatformPermission(c, svc, domain.PlatformOrganizationsImpersonate)
}
