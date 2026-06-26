package handlers

import (
	"net/http"

	"relay/client-backend/internal/http/middleware"
	"relay/client-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) InviteMember(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[inviteMemberRequest](c)
	if !ok {
		return
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	invitation, err := h.svc.InviteMember(c.Request.Context(), user, organizationID, req.Email, roleID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, invitation)
}

func (h *Handler) ListInvitations(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	invitations, err := h.svc.ListInvitations(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, invitations)
}

func (h *Handler) AcceptInvitation(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	invitation, err := h.svc.AcceptInvitation(c.Request.Context(), user, c.Param("token"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, invitation)
}

func (h *Handler) RevokeInvitation(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	invitationID, ok := paramID(c, "invitationId")
	if !ok {
		return
	}
	invitation, err := h.svc.RevokeInvitation(c.Request.Context(), user, organizationID, invitationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, invitation)
}
