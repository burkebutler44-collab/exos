package handlers

import (
	"net/http"

	"relay/client-backend/internal/http/middleware"
	"relay/client-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) ListMembers(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	members, err := h.svc.ListMembers(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, members)
}

func (h *Handler) UpdateMemberRole(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	targetUserID, ok := paramID(c, "userId")
	if !ok {
		return
	}
	req, ok := bindJSON[updateMemberRoleRequest](c)
	if !ok {
		return
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	member, err := h.svc.UpdateMemberRole(c.Request.Context(), user, organizationID, targetUserID, roleID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, member)
}

func (h *Handler) RemoveMember(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	targetUserID, ok := paramID(c, "userId")
	if !ok {
		return
	}
	if err := h.svc.RemoveMember(c.Request.Context(), user, organizationID, targetUserID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
