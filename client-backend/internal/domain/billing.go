package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type BillingAccountStatus string
type PaymentTerms string
type OrderStatus string
type OrderType string
type ServiceType string
type BillingMode string
type BillingInterval string
type BillableServiceStatus string
type PriceType string
type BillingUnit string
type UsageType string
type LedgerStatus string
type CreditLedgerType string
type BillingPeriodStatus string
type InvoiceRecordStatus string

const (
	BillingAccountActive    BillingAccountStatus = "active"
	BillingAccountPastDue   BillingAccountStatus = "past_due"
	BillingAccountSuspended BillingAccountStatus = "suspended"
	BillingAccountClosed    BillingAccountStatus = "closed"

	PaymentTermsPrepaid      PaymentTerms = "prepaid"
	PaymentTermsDueOnReceipt PaymentTerms = "due_on_receipt"
	PaymentTermsNet7         PaymentTerms = "net_7"
	PaymentTermsNet15        PaymentTerms = "net_15"
	PaymentTermsNet30        PaymentTerms = "net_30"

	OrderPending  OrderStatus = "pending"
	OrderPaid     OrderStatus = "paid"
	OrderFailed   OrderStatus = "failed"
	OrderCanceled OrderStatus = "canceled"

	OrderServerPurchase OrderType = "server_purchase"
	OrderCreditPurchase OrderType = "credit_purchase"
	OrderUpgrade        OrderType = "upgrade"
	OrderCustom         OrderType = "custom"

	ServiceServer      ServiceType = "server"
	ServiceIPAddress   ServiceType = "ip_address"
	ServiceBandwidth   ServiceType = "bandwidth"
	ServiceBackup      ServiceType = "backup"
	ServiceKaaSCluster ServiceType = "kaas_cluster"
	ServiceSupportPlan ServiceType = "support_plan"
	ServiceCustom      ServiceType = "custom"

	BillingMonthlyPrepaid    BillingMode = "monthly_prepaid"
	BillingMonthlyPostpaid   BillingMode = "monthly_postpaid"
	BillingHourlyPrepaid     BillingMode = "hourly_prepaid"
	BillingHourlyPostpaid    BillingMode = "hourly_postpaid"
	BillingQuarterlyPrepaid  BillingMode = "quarterly_prepaid"
	BillingQuarterlyPostpaid BillingMode = "quarterly_postpaid"
	BillingYearlyPrepaid     BillingMode = "yearly_prepaid"
	BillingYearlyPostpaid    BillingMode = "yearly_postpaid"
	BillingOneTime           BillingMode = "one_time"
	BillingCustomContract    BillingMode = "custom_contract"

	IntervalHourly    BillingInterval = "hourly"
	IntervalMonthly   BillingInterval = "monthly"
	IntervalQuarterly BillingInterval = "quarterly"
	IntervalYearly    BillingInterval = "yearly"

	BillableProvisioning BillableServiceStatus = "provisioning"
	BillableActive       BillableServiceStatus = "active"
	BillableSuspended    BillableServiceStatus = "suspended"
	BillableCanceled     BillableServiceStatus = "canceled"
	BillableTerminated   BillableServiceStatus = "terminated"

	PriceRecurring  PriceType = "recurring"
	PriceUsage      PriceType = "usage"
	PriceSetupFee   PriceType = "setup_fee"
	PriceDiscount   PriceType = "discount"
	PriceCredit     PriceType = "credit"
	PriceAdjustment PriceType = "adjustment"

	UnitMonth   BillingUnit = "month"
	UnitQuarter BillingUnit = "quarter"
	UnitYear    BillingUnit = "year"
	UnitHour    BillingUnit = "hour"
	UnitSecond  BillingUnit = "second"
	UnitGB      BillingUnit = "gb"
	UnitTB      BillingUnit = "tb"
	UnitIP      BillingUnit = "ip"
	UnitEach    BillingUnit = "each"

	UsageServerRuntime UsageType = "server_runtime"
	UsageBandwidthOut  UsageType = "bandwidth_out"
	UsageBandwidthIn   UsageType = "bandwidth_in"
	UsageIPRuntime     UsageType = "ip_runtime"
	UsageBackupStorage UsageType = "backup_storage"
	UsageCustom        UsageType = "custom"

	LedgerPending  LedgerStatus = "pending"
	LedgerInvoiced LedgerStatus = "invoiced"
	LedgerCharged  LedgerStatus = "charged"
	LedgerForgiven LedgerStatus = "forgiven"

	CreditPurchase CreditLedgerType = "credit_purchase"
	UsageDebit     CreditLedgerType = "usage_debit"
	InvoicePayment CreditLedgerType = "invoice_payment"
	ManualCredit   CreditLedgerType = "manual_credit"
	ManualDebit    CreditLedgerType = "manual_debit"
	Refund         CreditLedgerType = "refund"
	Adjustment     CreditLedgerType = "adjustment"

	PeriodOpen     BillingPeriodStatus = "open"
	PeriodInvoiced BillingPeriodStatus = "invoiced"
	PeriodPaid     BillingPeriodStatus = "paid"
	PeriodFailed   BillingPeriodStatus = "failed"
	PeriodVoid     BillingPeriodStatus = "void"
	PeriodSkipped  BillingPeriodStatus = "skipped"

	InvoiceDraft         InvoiceRecordStatus = "draft"
	InvoiceOpen          InvoiceRecordStatus = "open"
	InvoicePaid          InvoiceRecordStatus = "paid"
	InvoiceFailed        InvoiceRecordStatus = "failed"
	InvoiceVoid          InvoiceRecordStatus = "void"
	InvoiceUncollectible InvoiceRecordStatus = "uncollectible"
)

type BillingAccount struct {
	ID                         uuid.UUID            `json:"id"`
	OrganizationID             uuid.UUID            `json:"organization_id"`
	StripeCustomerID           *string              `json:"stripe_customer_id,omitempty"`
	BillingEmail               string               `json:"billing_email"`
	Currency                   string               `json:"currency"`
	Status                     BillingAccountStatus `json:"status"`
	PaymentTerms               PaymentTerms         `json:"payment_terms"`
	CreditBalanceCents         int64                `json:"credit_balance_cents"`
	AutoRechargeEnabled        bool                 `json:"auto_recharge_enabled"`
	AutoRechargeThresholdCents *int64               `json:"auto_recharge_threshold_cents,omitempty"`
	AutoRechargeAmountCents    *int64               `json:"auto_recharge_amount_cents,omitempty"`
	CreatedAt                  time.Time            `json:"created_at"`
	UpdatedAt                  time.Time            `json:"updated_at"`
}

type Order struct {
	ID                      uuid.UUID       `json:"id"`
	OrganizationID          uuid.UUID       `json:"organization_id"`
	CreatedByUserID         uuid.UUID       `json:"created_by_user_id"`
	Status                  OrderStatus     `json:"status"`
	OrderType               OrderType       `json:"order_type"`
	SubtotalCents           int64           `json:"subtotal_cents"`
	TaxCents                int64           `json:"tax_cents"`
	TotalCents              int64           `json:"total_cents"`
	StripeCheckoutSessionID *string         `json:"stripe_checkout_session_id,omitempty"`
	StripePaymentIntentID   *string         `json:"stripe_payment_intent_id,omitempty"`
	Metadata                json.RawMessage `json:"metadata"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
}

type BillableService struct {
	ID                 uuid.UUID             `json:"id"`
	OrganizationID     uuid.UUID             `json:"organization_id"`
	ProjectID          *uuid.UUID            `json:"project_id,omitempty"`
	OrderID            *uuid.UUID            `json:"order_id,omitempty"`
	ServiceType        ServiceType           `json:"service_type"`
	ServiceID          *uuid.UUID            `json:"service_id,omitempty"`
	BillingMode        BillingMode           `json:"billing_mode"`
	BillingInterval    BillingInterval       `json:"billing_interval"`
	Status             BillableServiceStatus `json:"status"`
	BillingStartedAt   time.Time             `json:"billing_started_at"`
	BillingEndedAt     *time.Time            `json:"billing_ended_at,omitempty"`
	CurrentPeriodStart *time.Time            `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time            `json:"current_period_end,omitempty"`
	NextInvoiceAt      *time.Time            `json:"next_invoice_at,omitempty"`
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
}

type BillableServicePrice struct {
	ID                uuid.UUID       `json:"id"`
	BillableServiceID uuid.UUID       `json:"billable_service_id"`
	PriceType         PriceType       `json:"price_type"`
	Description       string          `json:"description"`
	Unit              BillingUnit     `json:"unit"`
	UnitPriceCents    int64           `json:"unit_price_cents"`
	Quantity          string          `json:"quantity"`
	Currency          string          `json:"currency"`
	EffectiveFrom     time.Time       `json:"effective_from"`
	EffectiveTo       *time.Time      `json:"effective_to,omitempty"`
	Metadata          json.RawMessage `json:"metadata"`
	CreatedAt         time.Time       `json:"created_at"`
}

type UsageLedgerEntry struct {
	ID                uuid.UUID    `json:"id"`
	OrganizationID    uuid.UUID    `json:"organization_id"`
	BillableServiceID uuid.UUID    `json:"billable_service_id"`
	UsageType         UsageType    `json:"usage_type"`
	Quantity          string       `json:"quantity"`
	Unit              BillingUnit  `json:"unit"`
	UnitPriceCents    int64        `json:"unit_price_cents"`
	AmountCents       int64        `json:"amount_cents"`
	PeriodStart       time.Time    `json:"period_start"`
	PeriodEnd         time.Time    `json:"period_end"`
	Status            LedgerStatus `json:"status"`
	CreatedAt         time.Time    `json:"created_at"`
}

type CreditLedgerEntry struct {
	ID                uuid.UUID        `json:"id"`
	OrganizationID    uuid.UUID        `json:"organization_id"`
	BillingAccountID  uuid.UUID        `json:"billing_account_id"`
	Type              CreditLedgerType `json:"type"`
	AmountCents       int64            `json:"amount_cents"`
	BalanceAfterCents int64            `json:"balance_after_cents"`
	SourceType        *string          `json:"source_type,omitempty"`
	SourceID          *uuid.UUID       `json:"source_id,omitempty"`
	Description       string           `json:"description"`
	Metadata          json.RawMessage  `json:"metadata"`
	CreatedAt         time.Time        `json:"created_at"`
}

type BillingPeriod struct {
	ID                uuid.UUID           `json:"id"`
	OrganizationID    uuid.UUID           `json:"organization_id"`
	BillableServiceID uuid.UUID           `json:"billable_service_id"`
	PeriodStart       time.Time           `json:"period_start"`
	PeriodEnd         time.Time           `json:"period_end"`
	Status            BillingPeriodStatus `json:"status"`
	InvoiceRecordID   *uuid.UUID          `json:"invoice_record_id,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

type InvoiceRecord struct {
	ID              uuid.UUID           `json:"id"`
	OrganizationID  uuid.UUID           `json:"organization_id"`
	StripeInvoiceID *string             `json:"stripe_invoice_id,omitempty"`
	InvoiceNumber   *string             `json:"invoice_number,omitempty"`
	Status          InvoiceRecordStatus `json:"status"`
	SubtotalCents   int64               `json:"subtotal_cents"`
	TaxCents        int64               `json:"tax_cents"`
	TotalCents      int64               `json:"total_cents"`
	AmountPaidCents int64               `json:"amount_paid_cents"`
	DueAt           *time.Time          `json:"due_at,omitempty"`
	FinalizedAt     *time.Time          `json:"finalized_at,omitempty"`
	PaidAt          *time.Time          `json:"paid_at,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

type InvoiceLineItem struct {
	ID                uuid.UUID       `json:"id"`
	InvoiceRecordID   uuid.UUID       `json:"invoice_record_id"`
	OrganizationID    uuid.UUID       `json:"organization_id"`
	BillableServiceID *uuid.UUID      `json:"billable_service_id,omitempty"`
	Description       string          `json:"description"`
	Quantity          string          `json:"quantity"`
	Unit              BillingUnit     `json:"unit"`
	UnitPriceCents    int64           `json:"unit_price_cents"`
	AmountCents       int64           `json:"amount_cents"`
	PeriodStart       *time.Time      `json:"period_start,omitempty"`
	PeriodEnd         *time.Time      `json:"period_end,omitempty"`
	Metadata          json.RawMessage `json:"metadata"`
	CreatedAt         time.Time       `json:"created_at"`
}

type StripeEvent struct {
	ID            uuid.UUID       `json:"id"`
	StripeEventID string          `json:"stripe_event_id"`
	EventType     string          `json:"event_type"`
	ProcessedAt   *time.Time      `json:"processed_at,omitempty"`
	Payload       json.RawMessage `json:"payload"`
	CreatedAt     time.Time       `json:"created_at"`
}
