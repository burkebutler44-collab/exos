package handlers

import (
	"net/http"
	"strings"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/http/middleware"
	"relay/client-backend/internal/services"
	"relay/client-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) ListServerCatalog(c *gin.Context) {
	catalog, err := h.svc.ListServerCatalog(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, catalog)
}

func (h *Handler) AllocateServer(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[allocateServerRequest](c)
	if !ok {
		return
	}
	serverFamilyID, err := uuid.Parse(req.ServerFamilyID)
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	var projectID *uuid.UUID
	if req.ProjectID != nil && *req.ProjectID != "" {
		parsed, err := uuid.Parse(*req.ProjectID)
		if err != nil {
			writeError(c, services.ErrInvalidInput)
			return
		}
		projectID = &parsed
	}
	hardwareOptionIDs := make([]uuid.UUID, 0, len(req.HardwareOptionIDs))
	for _, raw := range req.HardwareOptionIDs {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeError(c, services.ErrInvalidInput)
			return
		}
		hardwareOptionIDs = append(hardwareOptionIDs, parsed)
	}
	result, err := h.svc.AllocateServer(c.Request.Context(), user, store.AllocateServerParams{
		OrganizationID:    organizationID,
		ProjectID:         projectID,
		ServerFamilyID:    serverFamilyID,
		ConfigurationID:   strings.TrimSpace(req.ConfigurationID),
		BillingInterval:   domain.BillingInterval(req.BillingInterval),
		HardwareOptionIDs: hardwareOptionIDs,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}
