package handlers

import (
	"net/http"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetBillingProfile(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	profile, err := h.svc.GetBillingProfile(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *Handler) UpdateBillingProfile(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[updateBillingRequest](c)
	if !ok {
		return
	}
	profile, err := h.svc.UpdateBillingProfile(c.Request.Context(), user, organizationID, billingParams(req))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *Handler) ListPaymentMethods(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	methods, err := h.svc.ListPaymentMethods(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, methods)
}

func (h *Handler) CreatePaymentMethodSetupIntent(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	intent, err := h.svc.CreatePaymentMethodSetupIntent(c.Request.Context(), user, organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, intent)
}

func (h *Handler) ConfirmPaymentMethodSetup(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[confirmPaymentMethodSetupRequest](c)
	if !ok {
		return
	}
	method, err := h.svc.ConfirmPaymentMethodSetup(c.Request.Context(), user, organizationID, req.SetupIntentID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, method)
}

func (h *Handler) CreatePaymentMethod(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[createPaymentMethodRequest](c)
	if !ok {
		return
	}
	method, err := h.svc.CreatePaymentMethod(c.Request.Context(), user, domain.PaymentMethod{
		OrganizationID:        organizationID,
		StripePaymentMethodID: req.StripePaymentMethodID,
		Brand:                 req.Brand,
		Last4:                 req.Last4,
		ExpMonth:              req.ExpMonth,
		ExpYear:               req.ExpYear,
		IsDefault:             req.IsDefault,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, method)
}

func (h *Handler) DeletePaymentMethod(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	paymentMethodID, ok := paramID(c, "paymentMethodId")
	if !ok {
		return
	}
	if err := h.svc.DeletePaymentMethod(c.Request.Context(), user, organizationID, paymentMethodID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListInvoices(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	invoices, err := h.svc.ListInvoiceRecords(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, invoices)
}
