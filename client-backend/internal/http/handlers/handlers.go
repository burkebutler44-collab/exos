package handlers

import (
	"errors"
	"log"
	"net/http"

	"relay/client-backend/internal/http/middleware"
	"relay/client-backend/internal/services"
	"relay/client-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc                *services.Services
	provisionPublisher ProvisionCommandPublisher
	natsMonitor        NATSConnectionMonitor
}

func New(svc *services.Services, opts ...HandlerOption) *Handler {
	h := &Handler{svc: svc}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func currentUser(c *gin.Context) (uuid.UUID, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return uuid.Nil, false
	}
	return user.ID, true
}

func orgID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("organizationId"))
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return uuid.Nil, false
	}
	return id, true
}

func paramID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return uuid.Nil, false
	}
	return id, true
}

func writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	message := "internal error"
	switch {
	case errors.Is(err, services.ErrInvalidInput) || services.IsInvalidInput(err):
		status, message = http.StatusBadRequest, "invalid input"
	case errors.Is(err, services.ErrForbidden):
		status, message = http.StatusForbidden, "forbidden"
	case errors.Is(err, services.ErrLastOwner):
		status, message = http.StatusConflict, "last owner cannot be removed or downgraded"
	case errors.Is(err, services.ErrBillableResources):
		status, message = http.StatusConflict, "organization has active billable resources"
	case errors.Is(err, services.ErrInvitationExpired):
		status, message = http.StatusConflict, "invitation expired"
	case errors.Is(err, services.ErrInvitationNotPending):
		status, message = http.StatusConflict, "invitation is not pending"
	case errors.Is(err, services.ErrStripeNotConfigured):
		status, message = http.StatusServiceUnavailable, "stripe is not configured"
	case errors.Is(err, services.ErrPaymentMethodRequired):
		status, message = http.StatusConflict, "add a payment method on the billing page before deploying"
	case errors.Is(err, services.ErrStripeRequestFailed):
		status, message = http.StatusPaymentRequired, "the card on file could not be charged"
	case services.IsNotFound(err):
		status, message = http.StatusNotFound, "not found"
	case services.IsConflict(err):
		status, message = http.StatusConflict, "conflict"
	}
	if status == http.StatusInternalServerError {
		log.Printf("handler internal error: path=%s method=%s err=%v", c.Request.URL.Path, c.Request.Method, err)
	}
	c.JSON(status, gin.H{"error": message})
}

func bindJSON[T any](c *gin.Context) (T, bool) {
	var req T
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, services.ErrInvalidInput)
		return req, false
	}
	return req, true
}

func billingParams(req updateBillingRequest) store.UpdateBillingProfileParams {
	return store.UpdateBillingProfileParams{
		BillingEmail: req.BillingEmail,
		CompanyName:  req.CompanyName,
		TaxID:        req.TaxID,
		Line1:        req.Line1,
		Line2:        req.Line2,
		City:         req.City,
		State:        req.State,
		PostalCode:   req.PostalCode,
		Country:      req.Country,
	}
}
