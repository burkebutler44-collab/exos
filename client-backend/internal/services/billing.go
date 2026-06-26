package services

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/store"

	"github.com/google/uuid"
)

type BillingAccountService struct{ repo Repository }
type OrderService struct{ repo Repository }
type BillableServiceService struct{ repo Repository }
type PricingService struct{ repo Repository }
type UsageLedgerService struct{ repo Repository }
type CreditLedgerService struct{ repo Repository }
type BillingPeriodService struct{ repo Repository }
type InvoiceService struct{ repo Repository }
type StripeWebhookService struct{ repo Repository }
type SuspensionService struct{ repo Repository }

func (s *Services) BillingAccounts() BillingAccountService {
	return BillingAccountService{repo: s.repo}
}
func (s *Services) Orders() OrderService { return OrderService{repo: s.repo} }
func (s *Services) BillableServices() BillableServiceService {
	return BillableServiceService{repo: s.repo}
}
func (s *Services) Pricing() PricingService              { return PricingService{repo: s.repo} }
func (s *Services) UsageLedger() UsageLedgerService      { return UsageLedgerService{repo: s.repo} }
func (s *Services) CreditLedger() CreditLedgerService    { return CreditLedgerService{repo: s.repo} }
func (s *Services) BillingPeriods() BillingPeriodService { return BillingPeriodService{repo: s.repo} }
func (s *Services) Invoices() InvoiceService             { return InvoiceService{repo: s.repo} }
func (s *Services) StripeWebhooks() StripeWebhookService { return StripeWebhookService{repo: s.repo} }
func (s *Services) Suspension() SuspensionService        { return SuspensionService{repo: s.repo} }

func (s *Services) GetBillingAccount(ctx context.Context, organizationID uuid.UUID) (domain.BillingAccount, error) {
	return s.repo.GetBillingAccount(ctx, organizationID)
}

func (s *Services) UpdateBillingAccount(ctx context.Context, organizationID uuid.UUID, params store.UpdateBillingAccountParams) (domain.BillingAccount, error) {
	if strings.TrimSpace(params.BillingEmail) == "" {
		return domain.BillingAccount{}, ErrInvalidInput
	}
	if params.PaymentTerms == "" {
		params.PaymentTerms = domain.PaymentTermsPrepaid
	}
	return s.repo.UpdateBillingAccount(ctx, organizationID, params)
}

func (s *Services) CreateOrder(ctx context.Context, actor domain.User, params store.CreateOrderParams) (domain.Order, error) {
	if params.OrganizationID == uuid.Nil || actor.ID == uuid.Nil || params.TotalCents < 0 {
		return domain.Order{}, ErrInvalidInput
	}
	params.CreatedByUserID = actor.ID
	if params.Metadata == nil {
		params.Metadata = []byte(`{}`)
	}
	order, err := s.repo.CreateOrder(ctx, params)
	if err != nil {
		return domain.Order{}, err
	}
	_ = s.audit(ctx, params.OrganizationID, &actor.ID, "order.created", "order", &order.ID, nil)
	return order, nil
}

func (s *Services) ListOrders(ctx context.Context, organizationID uuid.UUID) ([]domain.Order, error) {
	return s.repo.ListOrders(ctx, organizationID)
}

func (s *Services) GetOrder(ctx context.Context, organizationID, orderID uuid.UUID) (domain.Order, error) {
	return s.repo.GetOrder(ctx, organizationID, orderID)
}

func (s *Services) CreateBillableService(ctx context.Context, params store.CreateBillableServiceParams) (domain.BillableService, error) {
	if params.OrganizationID == uuid.Nil || params.ServiceType == "" || params.BillingMode == "" || len(params.Prices) == 0 {
		return domain.BillableService{}, ErrInvalidInput
	}
	if params.Status == "" {
		params.Status = domain.BillableProvisioning
	}
	if params.BillingInterval == "" {
		params.BillingInterval = domain.IntervalMonthly
	}
	if params.StartedAt.IsZero() {
		params.StartedAt = time.Now().UTC()
	}
	service, err := s.repo.CreateBillableService(ctx, params)
	if err != nil {
		return domain.BillableService{}, err
	}
	_ = s.audit(ctx, params.OrganizationID, nil, "billable_service.created", "billable_service", &service.ID, nil)
	return service, nil
}

func (s *Services) ListBillableServices(ctx context.Context, organizationID uuid.UUID) ([]domain.BillableService, error) {
	return s.repo.ListBillableServices(ctx, organizationID)
}

func (s *Services) GetBillableService(ctx context.Context, organizationID, serviceID uuid.UUID) (domain.BillableService, error) {
	return s.repo.GetBillableService(ctx, organizationID, serviceID)
}

func (s *Services) ListBillableServicePrices(ctx context.Context, billableServiceID uuid.UUID) ([]domain.BillableServicePrice, error) {
	return s.repo.ListBillableServicePrices(ctx, billableServiceID)
}

func (s *Services) CancelBillableService(ctx context.Context, actor domain.User, organizationID, serviceID uuid.UUID) (domain.BillableService, error) {
	now := time.Now().UTC()
	service, err := s.repo.UpdateBillableServiceStatus(ctx, organizationID, serviceID, domain.BillableCanceled, &now)
	if err != nil {
		return domain.BillableService{}, err
	}
	_ = s.audit(ctx, organizationID, &actor.ID, "billable_service.canceled", "billable_service", &service.ID, nil)
	return service, nil
}

func (s *Services) SuspendBillableService(ctx context.Context, actor domain.User, organizationID, serviceID uuid.UUID) (domain.BillableService, error) {
	service, err := s.repo.UpdateBillableServiceStatus(ctx, organizationID, serviceID, domain.BillableSuspended, nil)
	if err != nil {
		return domain.BillableService{}, err
	}
	_ = s.audit(ctx, organizationID, &actor.ID, "billable_service.suspended", "billable_service", &service.ID, nil)
	return service, nil
}

func (s *Services) ResumeBillableService(ctx context.Context, actor domain.User, organizationID, serviceID uuid.UUID) (domain.BillableService, error) {
	service, err := s.repo.UpdateBillableServiceStatus(ctx, organizationID, serviceID, domain.BillableActive, nil)
	if err != nil {
		return domain.BillableService{}, err
	}
	_ = s.audit(ctx, organizationID, &actor.ID, "billable_service.resumed", "billable_service", &service.ID, nil)
	return service, nil
}

func (s *Services) ListUsage(ctx context.Context, organizationID uuid.UUID) ([]domain.UsageLedgerEntry, error) {
	return s.repo.ListUsage(ctx, organizationID)
}

func (s *Services) ListServiceUsage(ctx context.Context, organizationID, serviceID uuid.UUID) ([]domain.UsageLedgerEntry, error) {
	return s.repo.ListServiceUsage(ctx, organizationID, serviceID)
}

func (s *Services) RecordHourlyUsage(ctx context.Context, organizationID, serviceID uuid.UUID, periodStart, periodEnd time.Time, unitPriceCents int64) (domain.UsageLedgerEntry, error) {
	hours := math.Ceil(periodEnd.Sub(periodStart).Hours())
	if hours < 1 {
		hours = 1
	}
	amount := int64(hours) * unitPriceCents
	service, err := s.repo.GetBillableService(ctx, organizationID, serviceID)
	if err != nil {
		return domain.UsageLedgerEntry{}, err
	}
	debitPrepaid := service.BillingMode == domain.BillingHourlyPrepaid
	entry, err := s.repo.RecordUsage(ctx, store.RecordUsageParams{
		OrganizationID:      organizationID,
		BillableServiceID:   serviceID,
		UsageType:           domain.UsageServerRuntime,
		Quantity:            formatQuantity(hours),
		Unit:                domain.UnitHour,
		UnitPriceCents:      unitPriceCents,
		AmountCents:         amount,
		PeriodStart:         periodStart,
		PeriodEnd:           periodEnd,
		Status:              domain.LedgerCharged,
		DebitPrepaidCredits: debitPrepaid,
		Description:         "Hourly server runtime",
	})
	if err != nil {
		return domain.UsageLedgerEntry{}, err
	}
	if debitPrepaid {
		account, err := s.repo.GetBillingAccount(ctx, organizationID)
		if err != nil {
			return domain.UsageLedgerEntry{}, err
		}
		if account.CreditBalanceCents < 0 {
			_, _ = s.repo.UpdateBillableServiceStatus(ctx, organizationID, serviceID, domain.BillableSuspended, nil)
			return entry, ErrInsufficientBalance
		}
	}
	return entry, nil
}

func (s *Services) ListCreditLedger(ctx context.Context, organizationID uuid.UUID) ([]domain.CreditLedgerEntry, error) {
	return s.repo.ListCreditLedger(ctx, organizationID)
}

func (s *Services) CreateCreditCheckout(ctx context.Context, actor domain.User, organizationID uuid.UUID, amountCents int64) (domain.Order, error) {
	if amountCents <= 0 {
		return domain.Order{}, ErrInvalidInput
	}
	return s.CreateOrder(ctx, actor, store.CreateOrderParams{
		OrganizationID: organizationID,
		OrderType:      domain.OrderCreditPurchase,
		SubtotalCents:  amountCents,
		TotalCents:     amountCents,
		Metadata:       []byte(`{"checkout":"stripe_credit_purchase"}`),
	})
}

func (s *Services) ManualCreditAdjustment(ctx context.Context, actor domain.User, params store.ManualAdjustmentParams) (domain.CreditLedgerEntry, error) {
	if params.OrganizationID == uuid.Nil || params.AmountCents == 0 || strings.TrimSpace(params.Description) == "" {
		return domain.CreditLedgerEntry{}, ErrInvalidInput
	}
	entryType := domain.ManualCredit
	if params.AmountCents < 0 {
		entryType = domain.ManualDebit
	}
	entry, err := s.repo.CreateCreditLedgerEntry(ctx, store.CreateCreditLedgerParams{
		OrganizationID: params.OrganizationID,
		Type:           entryType,
		AmountCents:    params.AmountCents,
		Description:    params.Description,
		Metadata:       params.Metadata,
	})
	if err != nil {
		return domain.CreditLedgerEntry{}, err
	}
	_ = s.audit(ctx, params.OrganizationID, &actor.ID, "billing.manual_adjustment", "credit_ledger", &entry.ID, nil)
	return entry, nil
}

func (s *Services) GenerateInvoice(ctx context.Context, organizationID uuid.UUID, serviceID *uuid.UUID) (domain.InvoiceRecord, []domain.InvoiceLineItem, error) {
	return s.repo.GenerateInvoice(ctx, store.GenerateInvoiceParams{OrganizationID: organizationID, ServiceID: serviceID})
}

func (s *Services) ListInvoiceRecords(ctx context.Context, organizationID uuid.UUID) ([]domain.InvoiceRecord, error) {
	return s.repo.ListInvoiceRecords(ctx, organizationID)
}

func (s *Services) GetInvoiceRecord(ctx context.Context, organizationID, invoiceID uuid.UUID) (domain.InvoiceRecord, []domain.InvoiceLineItem, error) {
	invoice, err := s.repo.GetInvoiceRecord(ctx, organizationID, invoiceID)
	if err != nil {
		return domain.InvoiceRecord{}, nil, err
	}
	lines, err := s.repo.ListInvoiceLineItems(ctx, organizationID, invoiceID)
	return invoice, lines, err
}

func (s *Services) FinalizeInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID, stripeInvoiceID *string) (domain.InvoiceRecord, error) {
	return s.repo.FinalizeInvoice(ctx, organizationID, invoiceID, stripeInvoiceID)
}

func (s *Services) VoidInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID) (domain.InvoiceRecord, error) {
	return s.repo.VoidInvoice(ctx, organizationID, invoiceID)
}

type StripeWebhookPayload struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID            string            `json:"id"`
			PaymentIntent string            `json:"payment_intent"`
			Metadata      map[string]string `json:"metadata"`
		} `json:"object"`
	} `json:"data"`
}

func (s *Services) HandleStripeWebhook(ctx context.Context, payload []byte) error {
	var event StripeWebhookPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		return ErrInvalidInput
	}
	if event.ID == "" || event.Type == "" {
		return ErrInvalidInput
	}
	created, err := s.repo.RecordStripeEvent(ctx, store.StripeEventParams{
		StripeEventID: event.ID,
		EventType:     event.Type,
		Payload:       payload,
	})
	if err != nil {
		return err
	}
	if !created {
		return nil
	}
	defer s.repo.MarkStripeEventProcessed(ctx, event.ID)

	switch event.Type {
	case "checkout.session.completed", "payment_intent.succeeded":
		orderID, orderErr := uuid.Parse(event.Data.Object.Metadata["order_id"])
		orgID, orgErr := uuid.Parse(event.Data.Object.Metadata["organization_id"])
		if orderErr != nil || orgErr != nil {
			return nil
		}
		paymentIntent := event.Data.Object.PaymentIntent
		if paymentIntent == "" {
			paymentIntent = event.Data.Object.ID
		}
		_, err := s.repo.MarkOrderPaidAndActivate(ctx, orgID, orderID, &paymentIntent)
		return err
	}
	return nil
}

func formatQuantity(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 6, 64), "0"), ".")
}
