package handlers

import (
	"net/http"

	"relay/client-backend/internal/http/middleware"

	"github.com/gin-gonic/gin"
)

type sessionResponse struct {
	User                sessionUserResponse `json:"user"`
	PlatformRoles       []string            `json:"platform_roles"`
	PlatformPermissions []string            `json:"platform_permissions"`
}

type sessionUserResponse struct {
	ID       string `json:"id"`
	Auth0Sub string `json:"auth0_sub"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

func (h *Handler) GetSession(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	platformSession, err := h.svc.GetPlatformSession(c.Request.Context(), user.ID)
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, sessionResponse{
		User: sessionUserResponse{
			ID:       user.ID.String(),
			Auth0Sub: user.Auth0Sub,
			Email:    user.Email,
			Name:     user.Name,
		},
		PlatformRoles:       platformSession.Roles,
		PlatformPermissions: platformSession.Permissions,
	})
}
