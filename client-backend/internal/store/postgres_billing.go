package store

import (
	"context"
	"encoding/json"
	"time"

	"relay/client-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) GetBillingAccount(ctx context.Context, organizationID uuid.UUID) (domain.BillingAccount, error) {
	return scanBillingAccount(s.pool.QueryRow(ctx, `
		select id, organization_id, stripe_customer_id, billing_email, currency, status, payment_terms,
			credit_balance_cents, auto_recharge_enabled, auto_recharge_threshold_cents, auto_recharge_amount_cents, created_at, updated_at
		from billing_accounts
		where organization_id = $1`, organizationID))
}

func (s *PostgresStore) UpdateBillingAccount(ctx context.Context, organizationID uuid.UUID, params UpdateBillingAccountParams) (domain.BillingAccount, error) {
	return scanBillingAccount(s.pool.QueryRow(ctx, `
		update billing_accounts
		set billing_email = $2,
			payment_terms = $3,
			auto_recharge_enabled = $4,
			auto_recharge_threshold_cents = $5,
			auto_recharge_amount_cents = $6,
			updated_at = now()
		where organization_id = $1
		returning id, organization_id, stripe_customer_id, billing_email, currency, status, payment_terms,
			credit_balance_cents, auto_recharge_enabled, auto_recharge_threshold_cents, auto_recharge_amount_cents, created_at, updated_at`,
		organizationID, params.BillingEmail, params.PaymentTerms, params.AutoRechargeEnabled, params.AutoRechargeThresholdCents, params.AutoRechargeAmountCents))
}

func (s *PostgresStore) SetBillingAccountStripeCustomerID(ctx context.Context, organizationID uuid.UUID, stripeCustomerID string) (domain.BillingAccount, error) {
	return scanBillingAccount(s.pool.QueryRow(ctx, `
		update billing_accounts
		set stripe_customer_id = $2,
			updated_at = now()
		where organization_id = $1
		returning id, organization_id, stripe_customer_id, billing_email, currency, status, payment_terms,
			credit_balance_cents, auto_recharge_enabled, auto_recharge_threshold_cents, auto_recharge_amount_cents, created_at, updated_at`,
		organizationID, stripeCustomerID))
}

func (s *PostgresStore) CreateOrder(ctx context.Context, params CreateOrderParams) (domain.Order, error) {
	if len(params.Metadata) == 0 {
		params.Metadata = []byte(`{}`)
	}
	return scanOrder(s.pool.QueryRow(ctx, `
		insert into orders (organization_id, created_by_user_id, status, order_type, subtotal_cents, tax_cents, total_cents, stripe_checkout_session_id, stripe_payment_intent_id, metadata)
		values ($1, $2, 'pending', $3, $4, $5, $6, $7, $8, $9)
		returning id, organization_id, created_by_user_id, status, order_type, subtotal_cents, tax_cents, total_cents, stripe_checkout_session_id, stripe_payment_intent_id, metadata, created_at, updated_at`,
		params.OrganizationID, params.CreatedByUserID, params.OrderType, params.SubtotalCents, params.TaxCents, params.TotalCents, params.StripeCheckoutSessionID, params.StripePaymentIntentID, params.Metadata))
}

func (s *PostgresStore) SetOrderStripeCheckoutSession(ctx context.Context, organizationID, orderID uuid.UUID, stripeCheckoutSessionID string) (domain.Order, error) {
	return scanOrder(s.pool.QueryRow(ctx, `
		update orders
		set stripe_checkout_session_id = $3,
			updated_at = now()
		where organization_id = $1 and id = $2 and status = 'pending'
		returning id, organization_id, created_by_user_id, status, order_type, subtotal_cents, tax_cents, total_cents, stripe_checkout_session_id, stripe_payment_intent_id, metadata, created_at, updated_at`,
		organizationID, orderID, stripeCheckoutSessionID))
}

func (s *PostgresStore) ListOrders(ctx context.Context, organizationID uuid.UUID) ([]domain.Order, error) {
	rows, err := s.pool.Query(ctx, `
		select id, organization_id, created_by_user_id, status, order_type, subtotal_cents, tax_cents, total_cents, stripe_checkout_session_id, stripe_payment_intent_id, metadata, created_at, updated_at
		from orders
		where organization_id = $1
		order by created_at desc`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []domain.Order
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *PostgresStore) GetOrder(ctx context.Context, organizationID, orderID uuid.UUID) (domain.Order, error) {
	return scanOrder(s.pool.QueryRow(ctx, `
		select id, organization_id, created_by_user_id, status, order_type, subtotal_cents, tax_cents, total_cents, stripe_checkout_session_id, stripe_payment_intent_id, metadata, created_at, updated_at
		from orders
		where organization_id = $1 and id = $2`, organizationID, orderID))
}

func (s *PostgresStore) MarkOrderPaidAndActivate(ctx context.Context, organizationID, orderID uuid.UUID, stripePaymentIntentID *string) (domain.Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	defer tx.Rollback(ctx)

	order, err := scanOrder(tx.QueryRow(ctx, `
		update orders
		set status = 'paid', stripe_payment_intent_id = coalesce($3, stripe_payment_intent_id), updated_at = now()
		where organization_id = $1 and id = $2
		returning id, organization_id, created_by_user_id, status, order_type, subtotal_cents, tax_cents, total_cents, stripe_checkout_session_id, stripe_payment_intent_id, metadata, created_at, updated_at`,
		organizationID, orderID, stripePaymentIntentID))
	if err != nil {
		return domain.Order{}, err
	}

	switch order.OrderType {
	case domain.OrderCreditPurchase:
		if _, err := createCreditLedgerEntryTx(ctx, tx, CreateCreditLedgerParams{
			OrganizationID: organizationID,
			Type:           domain.CreditPurchase,
			AmountCents:    order.TotalCents,
			SourceType:     strPtr("order"),
			SourceID:       &order.ID,
			Description:    "Credit purchase",
		}); err != nil {
			return domain.Order{}, err
		}
	case domain.OrderServerPurchase:
		var pending PendingServiceMetadata
		if err := json.Unmarshal(order.Metadata, &pending); err == nil && pending.ServiceType != "" {
			pending.BillingInterval = normalizeBillingInterval(pending.BillingInterval)
			if pending.BillingMode == "" {
				pending.BillingMode = prepaidBillingModeForInterval(pending.BillingInterval)
			}
			if pending.Unit == "" {
				pending.Unit = billingUnitForInterval(pending.BillingInterval)
			}
			priceType := domain.PriceRecurring
			if pending.BillingInterval == domain.IntervalHourly {
				priceType = domain.PriceUsage
			}
			now := time.Now().UTC()
			periodStart := now
			if pending.FirstPeriodStart != nil && !pending.FirstPeriodStart.IsZero() {
				periodStart = pending.FirstPeriodStart.UTC()
			}
			periodEnd := nextMonthlyBillingAnchor(now)
			if pending.FirstPeriodEnd != nil && !pending.FirstPeriodEnd.IsZero() {
				periodEnd = pending.FirstPeriodEnd.UTC()
			}
			service, err := createBillableServiceTx(ctx, tx, CreateBillableServiceParams{
				OrganizationID:  organizationID,
				ProjectID:       pending.ProjectID,
				OrderID:         &order.ID,
				ServiceType:     pending.ServiceType,
				ServiceID:       pending.ServiceID,
				BillingMode:     pending.BillingMode,
				BillingInterval: pending.BillingInterval,
				Status:          domain.BillableProvisioning,
				StartedAt:       now,
				PeriodStart:     &periodStart,
				PeriodEnd:       &periodEnd,
				NextInvoiceAt:   &periodEnd,
				Prices: []CreateBillableServicePriceParams{{
					PriceType:      priceType,
					Description:    pending.Description,
					Unit:           pending.Unit,
					UnitPriceCents: pending.UnitPriceCents,
					Quantity:       pending.Quantity,
					Currency:       pending.Currency,
					EffectiveFrom:  now,
				}},
			})
			if err != nil {
				return domain.Order{}, err
			}
			if _, err := tx.Exec(ctx, `
				insert into billing_periods (organization_id, billable_service_id, period_start, period_end, status)
				values ($1, $2, $3, $4, 'paid')`, organizationID, service.ID, periodStart, periodEnd); err != nil {
				return domain.Order{}, err
			}
		}
	}

	return order, tx.Commit(ctx)
}

func (s *PostgresStore) CreateBillableService(ctx context.Context, params CreateBillableServiceParams) (domain.BillableService, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.BillableService{}, err
	}
	defer tx.Rollback(ctx)
	service, err := createBillableServiceTx(ctx, tx, params)
	if err != nil {
		return domain.BillableService{}, err
	}
	return service, tx.Commit(ctx)
}

func (s *PostgresStore) ListBillableServices(ctx context.Context, organizationID uuid.UUID) ([]domain.BillableService, error) {
	rows, err := s.pool.Query(ctx, `
		select id, organization_id, project_id, order_id, service_type, service_id, billing_mode, billing_interval, status, billing_started_at, billing_ended_at, current_period_start, current_period_end, next_invoice_at, created_at, updated_at
		from billable_services
		where organization_id = $1
		order by created_at desc`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var services []domain.BillableService
	for rows.Next() {
		service, err := scanBillableService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, rows.Err()
}

func (s *PostgresStore) GetBillableService(ctx context.Context, organizationID, serviceID uuid.UUID) (domain.BillableService, error) {
	return scanBillableService(s.pool.QueryRow(ctx, `
		select id, organization_id, project_id, order_id, service_type, service_id, billing_mode, billing_interval, status, billing_started_at, billing_ended_at, current_period_start, current_period_end, next_invoice_at, created_at, updated_at
		from billable_services
		where organization_id = $1 and id = $2`, organizationID, serviceID))
}

func (s *PostgresStore) UpdateBillableServiceStatus(ctx context.Context, organizationID, serviceID uuid.UUID, status domain.BillableServiceStatus, endedAt *time.Time) (domain.BillableService, error) {
	return scanBillableService(s.pool.QueryRow(ctx, `
		update billable_services
		set status = $3, billing_ended_at = coalesce($4, billing_ended_at), updated_at = now()
		where organization_id = $1 and id = $2
		returning id, organization_id, project_id, order_id, service_type, service_id, billing_mode, billing_interval, status, billing_started_at, billing_ended_at, current_period_start, current_period_end, next_invoice_at, created_at, updated_at`,
		organizationID, serviceID, status, endedAt))
}

func (s *PostgresStore) ListBillableServicePrices(ctx context.Context, billableServiceID uuid.UUID) ([]domain.BillableServicePrice, error) {
	rows, err := s.pool.Query(ctx, `
		select id, billable_service_id, price_type, description, unit, unit_price_cents, quantity::text, currency, effective_from, effective_to, metadata, created_at
		from billable_service_prices
		where billable_service_id = $1
		order by created_at`, billableServiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prices []domain.BillableServicePrice
	for rows.Next() {
		price, err := scanBillableServicePrice(rows)
		if err != nil {
			return nil, err
		}
		prices = append(prices, price)
	}
	return prices, rows.Err()
}

func (s *PostgresStore) ListDueMonthlyPrepaidServices(ctx context.Context, now time.Time) ([]domain.BillableService, error) {
	rows, err := s.pool.Query(ctx, `
		select id, organization_id, project_id, order_id, service_type, service_id, billing_mode, billing_interval, status, billing_started_at, billing_ended_at, current_period_start, current_period_end, next_invoice_at, created_at, updated_at
		from billable_services
		where billing_mode in ('monthly_prepaid', 'quarterly_prepaid', 'yearly_prepaid')
			and status in ('active', 'provisioning')
			and next_invoice_at is not null
			and next_invoice_at <= $1
		order by next_invoice_at, created_at`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var services []domain.BillableService
	for rows.Next() {
		service, err := scanBillableService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, rows.Err()
}

func (s *PostgresStore) RecordUsage(ctx context.Context, params RecordUsageParams) (domain.UsageLedgerEntry, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.UsageLedgerEntry{}, err
	}
	defer tx.Rollback(ctx)
	entry, err := createUsageTx(ctx, tx, params)
	if err != nil {
		return domain.UsageLedgerEntry{}, err
	}
	if params.DebitPrepaidCredits {
		if _, err := createCreditLedgerEntryTx(ctx, tx, CreateCreditLedgerParams{
			OrganizationID: params.OrganizationID,
			Type:           domain.UsageDebit,
			AmountCents:    -params.AmountCents,
			SourceType:     strPtr("usage_ledger"),
			SourceID:       &entry.ID,
			Description:    params.Description,
		}); err != nil {
			return domain.UsageLedgerEntry{}, err
		}
	}
	return entry, tx.Commit(ctx)
}

func (s *PostgresStore) ListUsage(ctx context.Context, organizationID uuid.UUID) ([]domain.UsageLedgerEntry, error) {
	rows, err := s.pool.Query(ctx, usageSelect()+` where organization_id = $1 order by created_at desc`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsageRows(rows)
}

func (s *PostgresStore) ListServiceUsage(ctx context.Context, organizationID, serviceID uuid.UUID) ([]domain.UsageLedgerEntry, error) {
	rows, err := s.pool.Query(ctx, usageSelect()+` where organization_id = $1 and billable_service_id = $2 order by created_at desc`, organizationID, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsageRows(rows)
}

func (s *PostgresStore) CreateCreditLedgerEntry(ctx context.Context, params CreateCreditLedgerParams) (domain.CreditLedgerEntry, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.CreditLedgerEntry{}, err
	}
	defer tx.Rollback(ctx)
	entry, err := createCreditLedgerEntryTx(ctx, tx, params)
	if err != nil {
		return domain.CreditLedgerEntry{}, err
	}
	return entry, tx.Commit(ctx)
}

func (s *PostgresStore) ListCreditLedger(ctx context.Context, organizationID uuid.UUID) ([]domain.CreditLedgerEntry, error) {
	rows, err := s.pool.Query(ctx, `
		select id, organization_id, billing_account_id, type, amount_cents, balance_after_cents, source_type, source_id, description, metadata, created_at
		from credit_ledger
		where organization_id = $1
		order by created_at desc`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []domain.CreditLedgerEntry
	for rows.Next() {
		entry, err := scanCreditLedgerEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *PostgresStore) GenerateInvoice(ctx context.Context, params GenerateInvoiceParams) (domain.InvoiceRecord, []domain.InvoiceLineItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.InvoiceRecord{}, nil, err
	}
	defer tx.Rollback(ctx)

	filter := `where bs.organization_id = $1 and bs.status = 'active' and bsp.effective_to is null and bsp.price_type in ('recurring', 'usage', 'setup_fee', 'discount', 'credit', 'adjustment')`
	args := []any{params.OrganizationID}
	if params.ServiceID != nil {
		filter += ` and bs.id = $2`
		args = append(args, *params.ServiceID)
	}
	rows, err := tx.Query(ctx, `
		select bs.id, bsp.description, bsp.quantity::text, bsp.unit, bsp.unit_price_cents,
			round(bsp.quantity * bsp.unit_price_cents)::bigint,
			bs.current_period_start, bs.current_period_end, bsp.metadata
		from billable_services bs
		join billable_service_prices bsp on bsp.billable_service_id = bs.id
		`+filter, args...)
	if err != nil {
		return domain.InvoiceRecord{}, nil, err
	}
	defer rows.Close()

	type pendingLine struct {
		serviceID      uuid.UUID
		description    string
		quantity       string
		unit           domain.BillingUnit
		unitPriceCents int64
		amountCents    int64
		periodStart    *time.Time
		periodEnd      *time.Time
		metadata       json.RawMessage
	}
	var pending []pendingLine
	var total int64
	for rows.Next() {
		var line pendingLine
		if err := rows.Scan(&line.serviceID, &line.description, &line.quantity, &line.unit, &line.unitPriceCents, &line.amountCents, &line.periodStart, &line.periodEnd, &line.metadata); err != nil {
			return domain.InvoiceRecord{}, nil, err
		}
		pending = append(pending, line)
		total += line.amountCents
	}
	if err := rows.Err(); err != nil {
		return domain.InvoiceRecord{}, nil, err
	}

	invoice, err := scanInvoiceRecord(tx.QueryRow(ctx, `
		insert into invoice_records (organization_id, status, subtotal_cents, tax_cents, total_cents, due_at)
		values ($1, 'draft', $2, 0, $2, $3)
		returning id, organization_id, stripe_invoice_id, invoice_number, status, subtotal_cents, tax_cents, total_cents, amount_paid_cents, due_at, finalized_at, paid_at, created_at, updated_at`,
		params.OrganizationID, total, params.DueAt))
	if err != nil {
		return domain.InvoiceRecord{}, nil, err
	}

	var lines []domain.InvoiceLineItem
	for _, pendingLine := range pending {
		line, err := scanInvoiceLineItem(tx.QueryRow(ctx, `
			insert into invoice_line_items (invoice_record_id, organization_id, billable_service_id, description, quantity, unit, unit_price_cents, amount_cents, period_start, period_end, metadata)
			values ($1, $2, $3, $4, $5::numeric, $6, $7, $8, $9, $10, $11)
			returning id, invoice_record_id, organization_id, billable_service_id, description, quantity::text, unit, unit_price_cents, amount_cents, period_start, period_end, metadata, created_at`,
			invoice.ID, params.OrganizationID, pendingLine.serviceID, pendingLine.description, pendingLine.quantity, pendingLine.unit, pendingLine.unitPriceCents, pendingLine.amountCents, pendingLine.periodStart, pendingLine.periodEnd, pendingLine.metadata))
		if err != nil {
			return domain.InvoiceRecord{}, nil, err
		}
		lines = append(lines, line)
	}

	return invoice, lines, tx.Commit(ctx)
}

func (s *PostgresStore) CreateDueMonthlyPrepaidInvoice(ctx context.Context, serviceID uuid.UUID, now time.Time) (domain.InvoiceRecord, []domain.InvoiceLineItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.InvoiceRecord{}, nil, err
	}
	defer tx.Rollback(ctx)

	service, err := scanBillableService(tx.QueryRow(ctx, `
		select id, organization_id, project_id, order_id, service_type, service_id, billing_mode, billing_interval, status, billing_started_at, billing_ended_at, current_period_start, current_period_end, next_invoice_at, created_at, updated_at
		from billable_services
		where id = $1
			and billing_mode in ('monthly_prepaid', 'quarterly_prepaid', 'yearly_prepaid')
			and status in ('active', 'provisioning')
			and next_invoice_at is not null
			and next_invoice_at <= $2
		for update`, serviceID, now))
	if err != nil {
		return domain.InvoiceRecord{}, nil, err
	}

	price, err := scanBillableServicePrice(tx.QueryRow(ctx, `
		select id, billable_service_id, price_type, description, unit, unit_price_cents, quantity::text, currency, effective_from, effective_to, metadata, created_at
		from billable_service_prices
		where billable_service_id = $1
			and effective_to is null
			and price_type = 'recurring'
		order by effective_from desc
		limit 1`, service.ID))
	if err != nil {
		return domain.InvoiceRecord{}, nil, err
	}

	periodStart := now.UTC()
	if service.CurrentPeriodEnd != nil && !service.CurrentPeriodEnd.IsZero() {
		periodStart = service.CurrentPeriodEnd.UTC()
	} else if service.NextInvoiceAt != nil && !service.NextInvoiceAt.IsZero() {
		periodStart = service.NextInvoiceAt.UTC()
	}
	interval := normalizeBillingInterval(service.BillingInterval)
	periodEnd := addBillingInterval(periodStart, interval)
	amount := price.UnitPriceCents

	invoice, err := scanInvoiceRecord(tx.QueryRow(ctx, `
		insert into invoice_records (organization_id, status, subtotal_cents, tax_cents, total_cents, due_at)
		values ($1, 'draft', $2, 0, $2, $3)
		returning id, organization_id, stripe_invoice_id, invoice_number, status, subtotal_cents, tax_cents, total_cents, amount_paid_cents, due_at, finalized_at, paid_at, created_at, updated_at`,
		service.OrganizationID, amount, now))
	if err != nil {
		return domain.InvoiceRecord{}, nil, err
	}

	line, err := scanInvoiceLineItem(tx.QueryRow(ctx, `
		insert into invoice_line_items (invoice_record_id, organization_id, billable_service_id, description, quantity, unit, unit_price_cents, amount_cents, period_start, period_end, metadata)
		values ($1, $2, $3, $4, 1, $5, $6, $6, $7, $8, $9)
		returning id, invoice_record_id, organization_id, billable_service_id, description, quantity::text, unit, unit_price_cents, amount_cents, period_start, period_end, metadata, created_at`,
		invoice.ID, service.OrganizationID, service.ID, price.Description, price.Unit, amount, periodStart, periodEnd, price.Metadata))
	if err != nil {
		return domain.InvoiceRecord{}, nil, err
	}

	if _, err := tx.Exec(ctx, `
		insert into billing_periods (organization_id, billable_service_id, period_start, period_end, status, invoice_record_id)
		values ($1, $2, $3, $4, 'invoiced', $5)`, service.OrganizationID, service.ID, periodStart, periodEnd, invoice.ID); err != nil {
		return domain.InvoiceRecord{}, nil, err
	}

	if _, err := tx.Exec(ctx, `
		update billable_services
		set current_period_start = $2,
			current_period_end = $3,
			next_invoice_at = $3,
			updated_at = now()
		where id = $1`, service.ID, periodStart, periodEnd); err != nil {
		return domain.InvoiceRecord{}, nil, err
	}

	return invoice, []domain.InvoiceLineItem{line}, tx.Commit(ctx)
}

func (s *PostgresStore) ListInvoiceRecords(ctx context.Context, organizationID uuid.UUID) ([]domain.InvoiceRecord, error) {
	rows, err := s.pool.Query(ctx, invoiceRecordSelect()+` where organization_id = $1 order by created_at desc`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invoices []domain.InvoiceRecord
	for rows.Next() {
		invoice, err := scanInvoiceRecord(rows)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, invoice)
	}
	return invoices, rows.Err()
}

func (s *PostgresStore) GetInvoiceRecord(ctx context.Context, organizationID, invoiceID uuid.UUID) (domain.InvoiceRecord, error) {
	return scanInvoiceRecord(s.pool.QueryRow(ctx, invoiceRecordSelect()+` where organization_id = $1 and id = $2`, organizationID, invoiceID))
}

func (s *PostgresStore) ListInvoiceLineItems(ctx context.Context, organizationID, invoiceID uuid.UUID) ([]domain.InvoiceLineItem, error) {
	rows, err := s.pool.Query(ctx, `
		select id, invoice_record_id, organization_id, billable_service_id, description, quantity::text, unit, unit_price_cents, amount_cents, period_start, period_end, metadata, created_at
		from invoice_line_items
		where organization_id = $1 and invoice_record_id = $2
		order by created_at`, organizationID, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []domain.InvoiceLineItem
	for rows.Next() {
		line, err := scanInvoiceLineItem(rows)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func (s *PostgresStore) FinalizeInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID, stripeInvoiceID *string) (domain.InvoiceRecord, error) {
	return scanInvoiceRecord(s.pool.QueryRow(ctx, `
		update invoice_records
		set status = 'open', stripe_invoice_id = coalesce($3, stripe_invoice_id), finalized_at = now(), updated_at = now()
		where organization_id = $1 and id = $2 and status = 'draft'
		returning id, organization_id, stripe_invoice_id, invoice_number, status, subtotal_cents, tax_cents, total_cents, amount_paid_cents, due_at, finalized_at, paid_at, created_at, updated_at`,
		organizationID, invoiceID, stripeInvoiceID))
}

func (s *PostgresStore) VoidInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID) (domain.InvoiceRecord, error) {
	return scanInvoiceRecord(s.pool.QueryRow(ctx, `
		update invoice_records
		set status = 'void', updated_at = now()
		where organization_id = $1 and id = $2 and status in ('draft', 'open')
		returning id, organization_id, stripe_invoice_id, invoice_number, status, subtotal_cents, tax_cents, total_cents, amount_paid_cents, due_at, finalized_at, paid_at, created_at, updated_at`,
		organizationID, invoiceID))
}

func (s *PostgresStore) RecordStripeEvent(ctx context.Context, params StripeEventParams) (bool, error) {
	if len(params.Payload) == 0 {
		params.Payload = []byte(`{}`)
	}
	tag, err := s.pool.Exec(ctx, `
		insert into stripe_events (stripe_event_id, event_type, payload)
		values ($1, $2, $3)
		on conflict (stripe_event_id) do nothing`, params.StripeEventID, params.EventType, params.Payload)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *PostgresStore) MarkStripeEventProcessed(ctx context.Context, stripeEventID string) error {
	_, err := s.pool.Exec(ctx, `update stripe_events set processed_at = now() where stripe_event_id = $1`, stripeEventID)
	return err
}

func createBillableServiceTx(ctx context.Context, tx pgx.Tx, params CreateBillableServiceParams) (domain.BillableService, error) {
	params.BillingInterval = normalizeBillingInterval(params.BillingInterval)
	service, err := scanBillableService(tx.QueryRow(ctx, `
		insert into billable_services (organization_id, project_id, order_id, service_type, service_id, billing_mode, billing_interval, status, billing_started_at, current_period_start, current_period_end, next_invoice_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		returning id, organization_id, project_id, order_id, service_type, service_id, billing_mode, billing_interval, status, billing_started_at, billing_ended_at, current_period_start, current_period_end, next_invoice_at, created_at, updated_at`,
		params.OrganizationID, params.ProjectID, params.OrderID, params.ServiceType, params.ServiceID, params.BillingMode, params.BillingInterval, params.Status, params.StartedAt, params.PeriodStart, params.PeriodEnd, params.NextInvoiceAt))
	if err != nil {
		return domain.BillableService{}, err
	}
	for _, price := range params.Prices {
		if len(price.Metadata) == 0 {
			price.Metadata = []byte(`{}`)
		}
		if price.Quantity == "" {
			price.Quantity = "1"
		}
		if price.Currency == "" {
			price.Currency = "usd"
		}
		_, err := tx.Exec(ctx, `
			insert into billable_service_prices (billable_service_id, price_type, description, unit, unit_price_cents, quantity, currency, effective_from, effective_to, metadata)
			values ($1, $2, $3, $4, $5, $6::numeric, $7, $8, $9, $10)`,
			service.ID, price.PriceType, price.Description, price.Unit, price.UnitPriceCents, price.Quantity, price.Currency, price.EffectiveFrom, price.EffectiveTo, price.Metadata)
		if err != nil {
			return domain.BillableService{}, err
		}
	}
	return service, nil
}

func createUsageTx(ctx context.Context, tx pgx.Tx, params RecordUsageParams) (domain.UsageLedgerEntry, error) {
	return scanUsageEntry(tx.QueryRow(ctx, `
		insert into usage_ledger (organization_id, billable_service_id, usage_type, quantity, unit, unit_price_cents, amount_cents, period_start, period_end, status)
		values ($1, $2, $3, $4::numeric, $5, $6, $7, $8, $9, $10)
		returning id, organization_id, billable_service_id, usage_type, quantity::text, unit, unit_price_cents, amount_cents, period_start, period_end, status, created_at`,
		params.OrganizationID, params.BillableServiceID, params.UsageType, params.Quantity, params.Unit, params.UnitPriceCents, params.AmountCents, params.PeriodStart, params.PeriodEnd, params.Status))
}

func createCreditLedgerEntryTx(ctx context.Context, tx pgx.Tx, params CreateCreditLedgerParams) (domain.CreditLedgerEntry, error) {
	if len(params.Metadata) == 0 {
		params.Metadata = []byte(`{}`)
	}
	var account domain.BillingAccount
	if err := tx.QueryRow(ctx, `
		select id, organization_id, stripe_customer_id, billing_email, currency, status, payment_terms,
			credit_balance_cents, auto_recharge_enabled, auto_recharge_threshold_cents, auto_recharge_amount_cents, created_at, updated_at
		from billing_accounts
		where organization_id = $1
		for update`, params.OrganizationID).Scan(
		&account.ID, &account.OrganizationID, &account.StripeCustomerID, &account.BillingEmail, &account.Currency, &account.Status, &account.PaymentTerms,
		&account.CreditBalanceCents, &account.AutoRechargeEnabled, &account.AutoRechargeThresholdCents, &account.AutoRechargeAmountCents, &account.CreatedAt, &account.UpdatedAt,
	); err != nil {
		return domain.CreditLedgerEntry{}, err
	}
	nextBalance := account.CreditBalanceCents + params.AmountCents
	if _, err := tx.Exec(ctx, `
		update billing_accounts
		set credit_balance_cents = $2,
			status = case when $2 < 0 then 'past_due' else status end,
			updated_at = now()
		where id = $1`, account.ID, nextBalance); err != nil {
		return domain.CreditLedgerEntry{}, err
	}
	return scanCreditLedgerEntry(tx.QueryRow(ctx, `
		insert into credit_ledger (organization_id, billing_account_id, type, amount_cents, balance_after_cents, source_type, source_id, description, metadata)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		returning id, organization_id, billing_account_id, type, amount_cents, balance_after_cents, source_type, source_id, description, metadata, created_at`,
		params.OrganizationID, account.ID, params.Type, params.AmountCents, nextBalance, params.SourceType, params.SourceID, params.Description, params.Metadata))
}

func scanBillingAccount(row pgx.Row) (domain.BillingAccount, error) {
	var account domain.BillingAccount
	err := row.Scan(&account.ID, &account.OrganizationID, &account.StripeCustomerID, &account.BillingEmail, &account.Currency, &account.Status, &account.PaymentTerms, &account.CreditBalanceCents, &account.AutoRechargeEnabled, &account.AutoRechargeThresholdCents, &account.AutoRechargeAmountCents, &account.CreatedAt, &account.UpdatedAt)
	return account, mapNoRows(err)
}

func scanOrder(row pgx.Row) (domain.Order, error) {
	var order domain.Order
	err := row.Scan(&order.ID, &order.OrganizationID, &order.CreatedByUserID, &order.Status, &order.OrderType, &order.SubtotalCents, &order.TaxCents, &order.TotalCents, &order.StripeCheckoutSessionID, &order.StripePaymentIntentID, &order.Metadata, &order.CreatedAt, &order.UpdatedAt)
	return order, mapNoRows(err)
}

func scanBillableService(row pgx.Row) (domain.BillableService, error) {
	var service domain.BillableService
	err := row.Scan(&service.ID, &service.OrganizationID, &service.ProjectID, &service.OrderID, &service.ServiceType, &service.ServiceID, &service.BillingMode, &service.BillingInterval, &service.Status, &service.BillingStartedAt, &service.BillingEndedAt, &service.CurrentPeriodStart, &service.CurrentPeriodEnd, &service.NextInvoiceAt, &service.CreatedAt, &service.UpdatedAt)
	return service, mapNoRows(err)
}

func scanBillableServicePrice(row pgx.Row) (domain.BillableServicePrice, error) {
	var price domain.BillableServicePrice
	err := row.Scan(&price.ID, &price.BillableServiceID, &price.PriceType, &price.Description, &price.Unit, &price.UnitPriceCents, &price.Quantity, &price.Currency, &price.EffectiveFrom, &price.EffectiveTo, &price.Metadata, &price.CreatedAt)
	return price, mapNoRows(err)
}

func scanUsageEntry(row pgx.Row) (domain.UsageLedgerEntry, error) {
	var entry domain.UsageLedgerEntry
	err := row.Scan(&entry.ID, &entry.OrganizationID, &entry.BillableServiceID, &entry.UsageType, &entry.Quantity, &entry.Unit, &entry.UnitPriceCents, &entry.AmountCents, &entry.PeriodStart, &entry.PeriodEnd, &entry.Status, &entry.CreatedAt)
	return entry, mapNoRows(err)
}

func scanUsageRows(rows pgx.Rows) ([]domain.UsageLedgerEntry, error) {
	var entries []domain.UsageLedgerEntry
	for rows.Next() {
		entry, err := scanUsageEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func scanCreditLedgerEntry(row pgx.Row) (domain.CreditLedgerEntry, error) {
	var entry domain.CreditLedgerEntry
	err := row.Scan(&entry.ID, &entry.OrganizationID, &entry.BillingAccountID, &entry.Type, &entry.AmountCents, &entry.BalanceAfterCents, &entry.SourceType, &entry.SourceID, &entry.Description, &entry.Metadata, &entry.CreatedAt)
	return entry, mapNoRows(err)
}

func scanInvoiceRecord(row pgx.Row) (domain.InvoiceRecord, error) {
	var invoice domain.InvoiceRecord
	err := row.Scan(&invoice.ID, &invoice.OrganizationID, &invoice.StripeInvoiceID, &invoice.InvoiceNumber, &invoice.Status, &invoice.SubtotalCents, &invoice.TaxCents, &invoice.TotalCents, &invoice.AmountPaidCents, &invoice.DueAt, &invoice.FinalizedAt, &invoice.PaidAt, &invoice.CreatedAt, &invoice.UpdatedAt)
	return invoice, mapNoRows(err)
}

func scanInvoiceLineItem(row pgx.Row) (domain.InvoiceLineItem, error) {
	var line domain.InvoiceLineItem
	err := row.Scan(&line.ID, &line.InvoiceRecordID, &line.OrganizationID, &line.BillableServiceID, &line.Description, &line.Quantity, &line.Unit, &line.UnitPriceCents, &line.AmountCents, &line.PeriodStart, &line.PeriodEnd, &line.Metadata, &line.CreatedAt)
	return line, mapNoRows(err)
}

func usageSelect() string {
	return `select id, organization_id, billable_service_id, usage_type, quantity::text, unit, unit_price_cents, amount_cents, period_start, period_end, status, created_at from usage_ledger`
}

func invoiceRecordSelect() string {
	return `select id, organization_id, stripe_invoice_id, invoice_number, status, subtotal_cents, tax_cents, total_cents, amount_paid_cents, due_at, finalized_at, paid_at, created_at, updated_at from invoice_records`
}

func strPtr(value string) *string {
	return &value
}
