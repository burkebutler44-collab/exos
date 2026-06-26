package handlers

import (
	"net/http"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateProject(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[createProjectRequest](c)
	if !ok {
		return
	}
	project, err := h.svc.CreateProject(c.Request.Context(), user, organizationID, req.Name)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, project)
}

func (h *Handler) ListProjects(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	projects, err := h.svc.ListProjects(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	if projects == nil {
		projects = []domain.Project{}
	}
	c.JSON(http.StatusOK, projects)
}

func (h *Handler) GetProject(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	projectID, ok := paramID(c, "projectId")
	if !ok {
		return
	}
	project, err := h.svc.GetProject(c.Request.Context(), organizationID, projectID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (h *Handler) UpdateProject(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	projectID, ok := paramID(c, "projectId")
	if !ok {
		return
	}
	req, ok := bindJSON[updateProjectRequest](c)
	if !ok {
		return
	}
	project, err := h.svc.UpdateProject(c.Request.Context(), user, organizationID, projectID, req.Name)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (h *Handler) DeleteProject(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	projectID, ok := paramID(c, "projectId")
	if !ok {
		return
	}
	if err := h.svc.DeleteProject(c.Request.Context(), user, organizationID, projectID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
