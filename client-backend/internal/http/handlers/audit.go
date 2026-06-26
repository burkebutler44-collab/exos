package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListAuditLog(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	entries, err := h.svc.ListAuditLog(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, entries)
}
