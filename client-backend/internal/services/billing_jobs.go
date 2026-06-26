package services

import (
	"context"
	"time"
)

type BillingPeriodProcessor struct {
	services *Services
}

type HourlyUsageProcessor struct {
	services *Services
}

type InvoiceStatusReconciler struct {
	services *Services
	stripe   StripeBillingService
}

type PastDueSuspensionProcessor struct {
	services *Services
}

func NewBillingPeriodProcessor(services *Services) BillingPeriodProcessor {
	return BillingPeriodProcessor{services: services}
}

func NewHourlyUsageProcessor(services *Services) HourlyUsageProcessor {
	return HourlyUsageProcessor{services: services}
}

func NewInvoiceStatusReconciler(services *Services, stripe StripeBillingService) InvoiceStatusReconciler {
	return InvoiceStatusReconciler{services: services, stripe: stripe}
}

func NewPastDueSuspensionProcessor(services *Services) PastDueSuspensionProcessor {
	return PastDueSuspensionProcessor{services: services}
}

func (p BillingPeriodProcessor) RunOnce(ctx context.Context, now time.Time) error {
	// TODO: Find active services where next_invoice_at/current_period_end <= now,
	// create invoice_records and invoice_line_items, then push invoice items to Stripe.
	return nil
}

func (p HourlyUsageProcessor) RunOnce(ctx context.Context, now time.Time) error {
	// TODO: Calculate runtime windows for hourly services, write usage_ledger rows,
	// debit prepaid credits, and suspend services when balance policy requires it.
	return nil
}

func (p InvoiceStatusReconciler) RunOnce(ctx context.Context, now time.Time) error {
	// TODO: Retrieve open Stripe invoices, mirror paid/failed/void status locally,
	// and update billing account status from authoritative Stripe payment state.
	return nil
}

func (p PastDueSuspensionProcessor) RunOnce(ctx context.Context, now time.Time) error {
	// TODO: Apply grace periods to past_due accounts and suspend active services
	// without deleting billing history.
	return nil
}
