package handlers

import (
	"encoding/json"
	"net/http"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/http/middleware"
	"relay/client-backend/internal/services"
	"relay/client-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) GetBillingAccount(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	account, err := h.svc.GetBillingAccount(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

func (h *Handler) UpdateBillingAccount(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[updateBillingAccountRequest](c)
	if !ok {
		return
	}
	account, err := h.svc.UpdateBillingAccount(c.Request.Context(), organizationID, store.UpdateBillingAccountParams{
		BillingEmail:               req.BillingEmail,
		PaymentTerms:               domain.PaymentTerms(req.PaymentTerms),
		AutoRechargeEnabled:        req.AutoRechargeEnabled,
		AutoRechargeThresholdCents: req.AutoRechargeThresholdCents,
		AutoRechargeAmountCents:    req.AutoRechargeAmountCents,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

func (h *Handler) CreateOrder(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[createOrderRequest](c)
	if !ok {
		return
	}
	metadata, err := orderMetadata(req)
	if err != nil {
		writeError(c, err)
		return
	}
	order, err := h.svc.CreateOrder(c.Request.Context(), user, store.CreateOrderParams{
		OrganizationID: organizationID,
		OrderType:      domain.OrderType(req.OrderType),
		SubtotalCents:  req.SubtotalCents,
		TaxCents:       req.TaxCents,
		TotalCents:     req.TotalCents,
		Metadata:       metadata,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, order)
}

func (h *Handler) ListOrders(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	orders, err := h.svc.ListOrders(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, orders)
}

func (h *Handler) GetOrder(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	orderID, ok := paramID(c, "orderId")
	if !ok {
		return
	}
	order, err := h.svc.GetOrder(c.Request.Context(), organizationID, orderID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *Handler) ListBillableServices(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	services, err := h.svc.ListBillableServices(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, services)
}

func (h *Handler) GetBillableService(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	serviceID, ok := paramID(c, "serviceId")
	if !ok {
		return
	}
	service, err := h.svc.GetBillableService(c.Request.Context(), organizationID, serviceID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, service)
}

func (h *Handler) UpdateBillableService(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	serviceID, ok := paramID(c, "serviceId")
	if !ok {
		return
	}
	req, ok := bindJSON[updateBillableServiceRequest](c)
	if !ok {
		return
	}
	var service domain.BillableService
	var err error
	switch domain.BillableServiceStatus(req.Status) {
	case domain.BillableCanceled:
		service, err = h.svc.CancelBillableService(c.Request.Context(), user, organizationID, serviceID)
	case domain.BillableSuspended:
		service, err = h.svc.SuspendBillableService(c.Request.Context(), user, organizationID, serviceID)
	case domain.BillableActive:
		service, err = h.svc.ResumeBillableService(c.Request.Context(), user, organizationID, serviceID)
	default:
		writeError(c, services.ErrInvalidInput)
		return
	}
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, service)
}

func (h *Handler) CancelBillableService(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	h.writeServiceAction(c, func(orgID, serviceID uuid.UUID) (domain.BillableService, error) {
		return h.svc.CancelBillableService(c.Request.Context(), user, orgID, serviceID)
	})
}

func (h *Handler) SuspendBillableService(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	h.writeServiceAction(c, func(orgID, serviceID uuid.UUID) (domain.BillableService, error) {
		return h.svc.SuspendBillableService(c.Request.Context(), user, orgID, serviceID)
	})
}

func (h *Handler) ResumeBillableService(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	h.writeServiceAction(c, func(orgID, serviceID uuid.UUID) (domain.BillableService, error) {
		return h.svc.ResumeBillableService(c.Request.Context(), user, orgID, serviceID)
	})
}

func (h *Handler) ListCreditLedger(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	entries, err := h.svc.ListCreditLedger(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, entries)
}

func (h *Handler) CreateCreditCheckout(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[createCreditCheckoutRequest](c)
	if !ok {
		return
	}
	order, err := h.svc.CreateCreditCheckout(c.Request.Context(), user, organizationID, req.AmountCents)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, order)
}

func (h *Handler) ManualCreditAdjustment(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[manualCreditAdjustmentRequest](c)
	if !ok {
		return
	}
	entry, err := h.svc.ManualCreditAdjustment(c.Request.Context(), user, store.ManualAdjustmentParams{
		OrganizationID: organizationID,
		AmountCents:    req.AmountCents,
		Description:    req.Description,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, entry)
}

func (h *Handler) ListUsage(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	usage, err := h.svc.ListUsage(c.Request.Context(), organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, usage)
}

func (h *Handler) ListServiceUsage(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	serviceID, ok := paramID(c, "serviceId")
	if !ok {
		return
	}
	usage, err := h.svc.ListServiceUsage(c.Request.Context(), organizationID, serviceID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, usage)
}

func (h *Handler) GetInvoiceRecord(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	invoiceID, ok := paramID(c, "invoiceId")
	if !ok {
		return
	}
	invoice, lines, err := h.svc.GetInvoiceRecord(c.Request.Context(), organizationID, invoiceID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"invoice": invoice, "line_items": lines})
}

func (h *Handler) GenerateInvoice(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	req, ok := bindJSON[generateInvoiceRequest](c)
	if !ok {
		return
	}
	var serviceID *uuid.UUID
	if req.ServiceID != nil {
		parsed, err := uuid.Parse(*req.ServiceID)
		if err != nil {
			writeError(c, services.ErrInvalidInput)
			return
		}
		serviceID = &parsed
	}
	invoice, lines, err := h.svc.GenerateInvoice(c.Request.Context(), organizationID, serviceID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"invoice": invoice, "line_items": lines})
}

func (h *Handler) FinalizeInvoice(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	invoiceID, ok := paramID(c, "invoiceId")
	if !ok {
		return
	}
	var req finalizeInvoiceRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			writeError(c, services.ErrInvalidInput)
			return
		}
	}
	invoice, err := h.svc.FinalizeInvoice(c.Request.Context(), organizationID, invoiceID, req.StripeInvoiceID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, invoice)
}

func (h *Handler) VoidInvoice(c *gin.Context) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	invoiceID, ok := paramID(c, "invoiceId")
	if !ok {
		return
	}
	invoice, err := h.svc.VoidInvoice(c.Request.Context(), organizationID, invoiceID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, invoice)
}

func (h *Handler) HandleStripeWebhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		writeError(c, services.ErrInvalidInput)
		return
	}
	if err := h.svc.HandleStripeWebhook(c.Request.Context(), payload); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (h *Handler) writeServiceAction(c *gin.Context, action func(uuid.UUID, uuid.UUID) (domain.BillableService, error)) {
	organizationID, ok := orgID(c)
	if !ok {
		return
	}
	serviceID, ok := paramID(c, "serviceId")
	if !ok {
		return
	}
	service, err := action(organizationID, serviceID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, service)
}

func orderMetadata(req createOrderRequest) (json.RawMessage, error) {
	if req.ServiceType == "" {
		return []byte(`{}`), nil
	}
	var serviceID *uuid.UUID
	if req.ServiceID != nil {
		parsed, err := uuid.Parse(*req.ServiceID)
		if err != nil {
			return nil, services.ErrInvalidInput
		}
		serviceID = &parsed
	}
	var projectID *uuid.UUID
	if req.ProjectID != nil {
		parsed, err := uuid.Parse(*req.ProjectID)
		if err != nil {
			return nil, services.ErrInvalidInput
		}
		projectID = &parsed
	}
	if req.Quantity == "" {
		req.Quantity = "1"
	}
	if req.Currency == "" {
		req.Currency = "usd"
	}
	metadata, err := json.Marshal(store.PendingServiceMetadata{
		ServiceType:    domain.ServiceType(req.ServiceType),
		ServiceID:      serviceID,
		ProjectID:      projectID,
		BillingMode:    domain.BillingMode(req.BillingMode),
		Description:    req.Description,
		Unit:           domain.BillingUnit(req.Unit),
		UnitPriceCents: req.UnitPriceCents,
		Quantity:       req.Quantity,
		Currency:       req.Currency,
	})
	if err != nil {
		return nil, err
	}
	return metadata, nil
}
