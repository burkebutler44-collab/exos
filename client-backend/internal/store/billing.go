package store

import (
	"context"
	"encoding/json"
	"time"

	"relay/client-backend/internal/domain"

	"github.com/google/uuid"
)

type UpdateBillingAccountParams struct {
	BillingEmail               string
	PaymentTerms               domain.PaymentTerms
	AutoRechargeEnabled        bool
	AutoRechargeThresholdCents *int64
	AutoRechargeAmountCents    *int64
}

type CreateOrderParams struct {
	OrganizationID          uuid.UUID
	CreatedByUserID         uuid.UUID
	OrderType               domain.OrderType
	SubtotalCents           int64
	TaxCents                int64
	TotalCents              int64
	StripeCheckoutSessionID *string
	StripePaymentIntentID   *string
	Metadata                json.RawMessage
}

type PendingServiceMetadata struct {
	ServiceType            domain.ServiceType     `json:"service_type"`
	ServiceID              *uuid.UUID             `json:"service_id,omitempty"`
	ProjectID              *uuid.UUID             `json:"project_id,omitempty"`
	BillingMode            domain.BillingMode     `json:"billing_mode"`
	BillingInterval        domain.BillingInterval `json:"billing_interval"`
	Description            string                 `json:"description"`
	Unit                   domain.BillingUnit     `json:"unit"`
	UnitPriceCents         int64                  `json:"unit_price_cents"`
	Quantity               string                 `json:"quantity"`
	Currency               string                 `json:"currency"`
	MonthlyHours           int64                  `json:"monthly_hours,omitempty"`
	FirstPeriodStart       *time.Time             `json:"first_period_start,omitempty"`
	FirstPeriodEnd         *time.Time             `json:"first_period_end,omitempty"`
	FirstPeriodHours       int64                  `json:"first_period_hours,omitempty"`
	FirstPeriodAmountCents int64                  `json:"first_period_amount_cents,omitempty"`
}

type CreateBillableServiceParams struct {
	OrganizationID  uuid.UUID
	ProjectID       *uuid.UUID
	OrderID         *uuid.UUID
	ServiceType     domain.ServiceType
	ServiceID       *uuid.UUID
	BillingMode     domain.BillingMode
	BillingInterval domain.BillingInterval
	Status          domain.BillableServiceStatus
	StartedAt       time.Time
	PeriodStart     *time.Time
	PeriodEnd       *time.Time
	NextInvoiceAt   *time.Time
	Prices          []CreateBillableServicePriceParams
}

type CreateBillableServicePriceParams struct {
	PriceType      domain.PriceType
	Description    string
	Unit           domain.BillingUnit
	UnitPriceCents int64
	Quantity       string
	Currency       string
	EffectiveFrom  time.Time
	EffectiveTo    *time.Time
	Metadata       json.RawMessage
}

type RecordUsageParams struct {
	OrganizationID      uuid.UUID
	BillableServiceID   uuid.UUID
	UsageType           domain.UsageType
	Quantity            string
	Unit                domain.BillingUnit
	UnitPriceCents      int64
	AmountCents         int64
	PeriodStart         time.Time
	PeriodEnd           time.Time
	Status              domain.LedgerStatus
	DebitPrepaidCredits bool
	Description         string
}

type CreateCreditLedgerParams struct {
	OrganizationID uuid.UUID
	Type           domain.CreditLedgerType
	AmountCents    int64
	SourceType     *string
	SourceID       *uuid.UUID
	Description    string
	Metadata       json.RawMessage
}

type GenerateInvoiceParams struct {
	OrganizationID uuid.UUID
	ServiceID      *uuid.UUID
	DueAt          *time.Time
}

type ManualAdjustmentParams struct {
	OrganizationID uuid.UUID
	AmountCents    int64
	Description    string
	Metadata       json.RawMessage
}

type StripeEventParams struct {
	StripeEventID string
	EventType     string
	Payload       json.RawMessage
}

type BillingRepository interface {
	GetBillingAccount(ctx context.Context, organizationID uuid.UUID) (domain.BillingAccount, error)
	UpdateBillingAccount(ctx context.Context, organizationID uuid.UUID, params UpdateBillingAccountParams) (domain.BillingAccount, error)

	CreateOrder(ctx context.Context, params CreateOrderParams) (domain.Order, error)
	SetOrderStripeCheckoutSession(ctx context.Context, organizationID, orderID uuid.UUID, stripeCheckoutSessionID string) (domain.Order, error)
	ListOrders(ctx context.Context, organizationID uuid.UUID) ([]domain.Order, error)
	GetOrder(ctx context.Context, organizationID, orderID uuid.UUID) (domain.Order, error)
	MarkOrderPaidAndActivate(ctx context.Context, organizationID, orderID uuid.UUID, stripePaymentIntentID *string) (domain.Order, error)

	CreateBillableService(ctx context.Context, params CreateBillableServiceParams) (domain.BillableService, error)
	ListBillableServices(ctx context.Context, organizationID uuid.UUID) ([]domain.BillableService, error)
	GetBillableService(ctx context.Context, organizationID, serviceID uuid.UUID) (domain.BillableService, error)
	UpdateBillableServiceStatus(ctx context.Context, organizationID, serviceID uuid.UUID, status domain.BillableServiceStatus, endedAt *time.Time) (domain.BillableService, error)
	ListBillableServicePrices(ctx context.Context, billableServiceID uuid.UUID) ([]domain.BillableServicePrice, error)
	ListDueMonthlyPrepaidServices(ctx context.Context, now time.Time) ([]domain.BillableService, error)

	RecordUsage(ctx context.Context, params RecordUsageParams) (domain.UsageLedgerEntry, error)
	ListUsage(ctx context.Context, organizationID uuid.UUID) ([]domain.UsageLedgerEntry, error)
	ListServiceUsage(ctx context.Context, organizationID, serviceID uuid.UUID) ([]domain.UsageLedgerEntry, error)

	CreateCreditLedgerEntry(ctx context.Context, params CreateCreditLedgerParams) (domain.CreditLedgerEntry, error)
	ListCreditLedger(ctx context.Context, organizationID uuid.UUID) ([]domain.CreditLedgerEntry, error)

	GenerateInvoice(ctx context.Context, params GenerateInvoiceParams) (domain.InvoiceRecord, []domain.InvoiceLineItem, error)
	CreateDueMonthlyPrepaidInvoice(ctx context.Context, serviceID uuid.UUID, now time.Time) (domain.InvoiceRecord, []domain.InvoiceLineItem, error)
	ListInvoiceRecords(ctx context.Context, organizationID uuid.UUID) ([]domain.InvoiceRecord, error)
	GetInvoiceRecord(ctx context.Context, organizationID, invoiceID uuid.UUID) (domain.InvoiceRecord, error)
	ListInvoiceLineItems(ctx context.Context, organizationID, invoiceID uuid.UUID) ([]domain.InvoiceLineItem, error)
	FinalizeInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID, stripeInvoiceID *string) (domain.InvoiceRecord, error)
	VoidInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID) (domain.InvoiceRecord, error)

	RecordStripeEvent(ctx context.Context, params StripeEventParams) (bool, error)
	MarkStripeEventProcessed(ctx context.Context, stripeEventID string) error
}
