package services

import (
	"context"

	"relay/client-backend/internal/domain"
)

type StripeBillingService interface {
	EnsureCustomer(ctx context.Context, account domain.BillingAccount) (string, error)
	CreateCreditCheckoutSession(ctx context.Context, account domain.BillingAccount, amountCents int64, idempotencyKey string) (string, error)
	CreateServerCheckoutSession(ctx context.Context, order domain.Order, idempotencyKey string) (string, error)
	CreateInvoice(ctx context.Context, invoice domain.InvoiceRecord, idempotencyKey string) (string, error)
	CreateInvoiceItem(ctx context.Context, invoice domain.InvoiceRecord, line domain.InvoiceLineItem, idempotencyKey string) error
	FinalizeInvoice(ctx context.Context, stripeInvoiceID string, idempotencyKey string) error
	VoidInvoice(ctx context.Context, stripeInvoiceID string, idempotencyKey string) error
	RetrieveInvoice(ctx context.Context, stripeInvoiceID string) (StripeInvoiceStatus, error)
	CreateCustomerPortalSession(ctx context.Context, account domain.BillingAccount, returnURL string, idempotencyKey string) (string, error)
}

type StripeInvoiceStatus struct {
	StripeInvoiceID string
	Status          domain.InvoiceRecordStatus
	AmountPaidCents int64
}

type NoopStripeBillingService struct{}

func (NoopStripeBillingService) EnsureCustomer(ctx context.Context, account domain.BillingAccount) (string, error) {
	// TODO: Create or retrieve a Stripe Customer and persist stripe_customer_id locally.
	return "", nil
}

func (NoopStripeBillingService) CreateCreditCheckoutSession(ctx context.Context, account domain.BillingAccount, amountCents int64, idempotencyKey string) (string, error) {
	// TODO: Create Stripe Checkout Session for prepaid credit purchase using idempotencyKey.
	return "", nil
}

func (NoopStripeBillingService) CreateServerCheckoutSession(ctx context.Context, order domain.Order, idempotencyKey string) (string, error) {
	// TODO: Create Stripe Checkout Session or PaymentIntent for initial server purchase.
	return "", nil
}

func (NoopStripeBillingService) CreateInvoice(ctx context.Context, invoice domain.InvoiceRecord, idempotencyKey string) (string, error) {
	// TODO: Create Stripe invoice for the local invoice_record.
	return "", nil
}

func (NoopStripeBillingService) CreateInvoiceItem(ctx context.Context, invoice domain.InvoiceRecord, line domain.InvoiceLineItem, idempotencyKey string) error {
	// TODO: Create Stripe invoice item from immutable local invoice_line_item.
	return nil
}

func (NoopStripeBillingService) FinalizeInvoice(ctx context.Context, stripeInvoiceID string, idempotencyKey string) error {
	// TODO: Finalize Stripe invoice and let Stripe collect/send according to payment terms.
	return nil
}

func (NoopStripeBillingService) VoidInvoice(ctx context.Context, stripeInvoiceID string, idempotencyKey string) error {
	// TODO: Void the Stripe invoice when the local draft/open invoice is voided.
	return nil
}

func (NoopStripeBillingService) RetrieveInvoice(ctx context.Context, stripeInvoiceID string) (StripeInvoiceStatus, error) {
	// TODO: Retrieve Stripe invoice status during reconciliation jobs.
	return StripeInvoiceStatus{StripeInvoiceID: stripeInvoiceID}, nil
}

func (NoopStripeBillingService) CreateCustomerPortalSession(ctx context.Context, account domain.BillingAccount, returnURL string, idempotencyKey string) (string, error) {
	// TODO: Add customer portal once account management is ready.
	return "", nil
}
