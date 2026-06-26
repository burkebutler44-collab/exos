package handlers

import (
	"net/http"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateOrganization(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	req, ok := bindJSON[createOrganizationRequest](c)
	if !ok {
		return
	}
	org, err := h.svc.CreateOrganization(c.Request.Context(), user, req.Name)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, org)
}

func (h *Handler) ListOrganizations(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	orgs, err := h.svc.ListOrganizations(c.Request.Context(), user)
	if err != nil {
		writeError(c, err)
		return
	}
	if orgs == nil {
		orgs = []domain.Organization{}
	}
	c.JSON(http.StatusOK, orgs)
}

func (h *Handler) GetOrganization(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	org, err := h.svc.GetOrganization(c.Request.Context(), user, organizationID)
	if err != nil {
		if middleware.CanViewOrganizationAsPlatformAdmin(c, h.svc) {
			org, adminErr := h.svc.GetOrganizationByID(c.Request.Context(), organizationID)
			if adminErr != nil {
				writeError(c, adminErr)
				return
			}
			c.JSON(http.StatusOK, org)
			return
		}
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *Handler) UpdateOrganization(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[updateOrganizationRequest](c)
	if !ok {
		return
	}
	org, err := h.svc.UpdateOrganization(c.Request.Context(), user, organizationID, req.Name)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *Handler) DeleteOrganization(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteOrganization(c.Request.Context(), user, organizationID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListRoles(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	roles, err := h.svc.ListRoles(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, roles)
}
