package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/store"

	"github.com/google/uuid"
)

func TestCreatingOrganizationCreatesBillingAccount(t *testing.T) {
	repo := newBillingFakeRepo()
	svc := New(repo)
	user := domain.User{ID: uuid.New(), Email: "founder@example.com"}

	org, err := svc.CreateOrganization(context.Background(), user, "Exos Labs")
	if err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}
	account, err := svc.GetBillingAccount(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("GetBillingAccount returned error: %v", err)
	}
	if account.OrganizationID != org.ID || account.PaymentTerms != domain.PaymentTermsPrepaid || account.Status != domain.BillingAccountActive {
		t.Fatalf("billing account = %+v, want active prepaid account for org", account)
	}
}

func TestMonthlyPrepaidServerPaymentCreatesServiceAndLockedPrice(t *testing.T) {
	repo := newBillingFakeRepo()
	svc := New(repo)
	orgID := uuid.New()
	user := domain.User{ID: uuid.New(), Email: "buyer@example.com"}
	repo.accounts[orgID] = domain.BillingAccount{ID: uuid.New(), OrganizationID: orgID, Status: domain.BillingAccountActive, PaymentTerms: domain.PaymentTermsPrepaid}

	order := createServerOrder(t, svc, user, orgID)
	if order.Status != domain.OrderPending {
		t.Fatalf("order status = %s, want pending", order.Status)
	}

	payload := stripePayload("evt_paid_1", "checkout.session.completed", orgID, order.ID)
	if err := svc.HandleStripeWebhook(context.Background(), payload); err != nil {
		t.Fatalf("HandleStripeWebhook returned error: %v", err)
	}

	paid, _ := svc.GetOrder(context.Background(), orgID, order.ID)
	if paid.Status != domain.OrderPaid {
		t.Fatalf("order status = %s, want paid", paid.Status)
	}
	services, _ := svc.ListBillableServices(context.Background(), orgID)
	if len(services) != 1 {
		t.Fatalf("services len = %d, want 1", len(services))
	}
	prices, _ := svc.ListBillableServicePrices(context.Background(), services[0].ID)
	if len(prices) != 1 || prices[0].UnitPriceCents != 55000 || prices[0].Description == "" {
		t.Fatalf("prices = %+v, want one locked-in price", prices)
	}
}

func TestStripeWebhookIsIdempotent(t *testing.T) {
	repo := newBillingFakeRepo()
	svc := New(repo)
	orgID := uuid.New()
	user := domain.User{ID: uuid.New(), Email: "buyer@example.com"}
	repo.accounts[orgID] = domain.BillingAccount{ID: uuid.New(), OrganizationID: orgID}
	order := createServerOrder(t, svc, user, orgID)
	payload := stripePayload("evt_duplicate", "checkout.session.completed", orgID, order.ID)

	if err := svc.HandleStripeWebhook(context.Background(), payload); err != nil {
		t.Fatalf("first webhook returned error: %v", err)
	}
	if err := svc.HandleStripeWebhook(context.Background(), payload); err != nil {
		t.Fatalf("second webhook returned error: %v", err)
	}
	services, _ := svc.ListBillableServices(context.Background(), orgID)
	if len(services) != 1 {
		t.Fatalf("services len = %d, want exactly one after duplicate webhook", len(services))
	}
}

func TestGenerateInvoiceCreatesRecordAndImmutableLines(t *testing.T) {
	repo := newBillingFakeRepo()
	svc := New(repo)
	orgID := uuid.New()
	now := time.Now().UTC()
	service, err := svc.CreateBillableService(context.Background(), store.CreateBillableServiceParams{
		OrganizationID: orgID,
		ServiceType:    domain.ServiceServer,
		BillingMode:    domain.BillingMonthlyPrepaid,
		Status:         domain.BillableActive,
		StartedAt:      now,
		PeriodStart:    &now,
		PeriodEnd:      ptrTime(now.AddDate(0, 1, 0)),
		Prices: []store.CreateBillableServicePriceParams{{
			PriceType:      domain.PriceRecurring,
			Description:    "Ryzen 9950X monthly rental",
			Unit:           domain.UnitMonth,
			UnitPriceCents: 55000,
			Quantity:       "1",
			Currency:       "usd",
			EffectiveFrom:  now,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBillableService returned error: %v", err)
	}

	invoice, lines, err := svc.GenerateInvoice(context.Background(), orgID, &service.ID)
	if err != nil {
		t.Fatalf("GenerateInvoice returned error: %v", err)
	}
	if invoice.Status != domain.InvoiceDraft || len(lines) != 1 || lines[0].AmountCents != 55000 {
		t.Fatalf("invoice=%+v lines=%+v, want draft invoice with one line", invoice, lines)
	}
	if _, err := svc.FinalizeInvoice(context.Background(), orgID, invoice.ID, nil); err != nil {
		t.Fatalf("FinalizeInvoice returned error: %v", err)
	}
	if err := repo.mutateFinalizedLine(invoice.ID); !errors.Is(err, ErrFinalizedInvoice) {
		t.Fatalf("mutateFinalizedLine error = %v, want ErrFinalizedInvoice", err)
	}
}

func TestHourlyUsageDebitsCreditsAndSuspendsWhenInsufficient(t *testing.T) {
	repo := newBillingFakeRepo()
	svc := New(repo)
	orgID := uuid.New()
	accountID := uuid.New()
	repo.accounts[orgID] = domain.BillingAccount{ID: accountID, OrganizationID: orgID}
	_, _ = repo.CreateCreditLedgerEntry(context.Background(), store.CreateCreditLedgerParams{
		OrganizationID: orgID,
		Type:           domain.ManualCredit,
		AmountCents:    150,
		Description:    "Opening test balance",
	})
	now := time.Now().UTC()
	service, err := svc.CreateBillableService(context.Background(), store.CreateBillableServiceParams{
		OrganizationID: orgID,
		ServiceType:    domain.ServiceServer,
		BillingMode:    domain.BillingHourlyPrepaid,
		Status:         domain.BillableActive,
		StartedAt:      now,
		Prices: []store.CreateBillableServicePriceParams{{
			PriceType:      domain.PriceUsage,
			Description:    "Hourly runtime",
			Unit:           domain.UnitHour,
			UnitPriceCents: 125,
			Quantity:       "1",
			Currency:       "usd",
			EffectiveFrom:  now,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBillableService returned error: %v", err)
	}

	entry, err := svc.RecordHourlyUsage(context.Background(), orgID, service.ID, now, now.Add(35*time.Minute), 125)
	if err != nil {
		t.Fatalf("RecordHourlyUsage first debit returned error: %v", err)
	}
	if entry.AmountCents != 125 || repo.accounts[orgID].CreditBalanceCents != 25 {
		t.Fatalf("amount=%d balance=%d, want 125 and 25", entry.AmountCents, repo.accounts[orgID].CreditBalanceCents)
	}
	_, err = svc.RecordHourlyUsage(context.Background(), orgID, service.ID, now.Add(time.Hour), now.Add(2*time.Hour), 125)
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("RecordHourlyUsage second debit error = %v, want ErrInsufficientBalance", err)
	}
	updated, _ := svc.GetBillableService(context.Background(), orgID, service.ID)
	if updated.Status != domain.BillableSuspended {
		t.Fatalf("service status = %s, want suspended", updated.Status)
	}
	if repo.ledgerBalance(orgID) != repo.accounts[orgID].CreditBalanceCents {
		t.Fatal("credit balance is inconsistent with credit_ledger")
	}
}

func TestCreditPurchaseAndManualAdjustmentCreateLedgerEntries(t *testing.T) {
	repo := newBillingFakeRepo()
	svc := New(repo)
	orgID := uuid.New()
	user := domain.User{ID: uuid.New(), Email: "buyer@example.com"}
	repo.accounts[orgID] = domain.BillingAccount{ID: uuid.New(), OrganizationID: orgID}

	order, err := svc.CreateCreditCheckout(context.Background(), user, orgID, 10000)
	if err != nil {
		t.Fatalf("CreateCreditCheckout returned error: %v", err)
	}
	if err := svc.HandleStripeWebhook(context.Background(), stripePayload("evt_credit", "checkout.session.completed", orgID, order.ID)); err != nil {
		t.Fatalf("credit webhook returned error: %v", err)
	}
	if repo.accounts[orgID].CreditBalanceCents != 10000 {
		t.Fatalf("balance = %d, want 10000", repo.accounts[orgID].CreditBalanceCents)
	}
	if _, err := svc.ManualCreditAdjustment(context.Background(), user, store.ManualAdjustmentParams{OrganizationID: orgID, AmountCents: -2500, Description: "Manual debit"}); err != nil {
		t.Fatalf("ManualCreditAdjustment returned error: %v", err)
	}
	if repo.accounts[orgID].CreditBalanceCents != 7500 {
		t.Fatalf("balance = %d, want 7500", repo.accounts[orgID].CreditBalanceCents)
	}
}

func TestAccessRequiresMatchingOrganization(t *testing.T) {
	repo := newBillingFakeRepo()
	svc := New(repo)
	orgA := uuid.New()
	orgB := uuid.New()
	now := time.Now().UTC()
	service, err := svc.CreateBillableService(context.Background(), store.CreateBillableServiceParams{
		OrganizationID: orgA,
		ServiceType:    domain.ServiceServer,
		BillingMode:    domain.BillingMonthlyPrepaid,
		Status:         domain.BillableActive,
		StartedAt:      now,
		Prices: []store.CreateBillableServicePriceParams{{
			PriceType: domain.PriceRecurring, Description: "Monthly", Unit: domain.UnitMonth, UnitPriceCents: 55000, Quantity: "1", Currency: "usd", EffectiveFrom: now,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBillableService returned error: %v", err)
	}
	if _, err := svc.GetBillableService(context.Background(), orgB, service.ID); !IsNotFound(err) {
		t.Fatalf("GetBillableService with wrong org error = %v, want not found", err)
	}
	if ok, _ := svc.ResourceBelongsToOrganization(context.Background(), "billable_services", service.ID, orgB); ok {
		t.Fatal("resource matched the wrong organization")
	}
}

func TestCancelingServiceDoesNotDeleteBillingHistory(t *testing.T) {
	repo := newBillingFakeRepo()
	svc := New(repo)
	orgID := uuid.New()
	user := domain.User{ID: uuid.New()}
	now := time.Now().UTC()
	service, _ := svc.CreateBillableService(context.Background(), store.CreateBillableServiceParams{
		OrganizationID: orgID,
		ServiceType:    domain.ServiceServer,
		BillingMode:    domain.BillingMonthlyPrepaid,
		Status:         domain.BillableActive,
		StartedAt:      now,
		Prices: []store.CreateBillableServicePriceParams{{
			PriceType: domain.PriceRecurring, Description: "Monthly", Unit: domain.UnitMonth, UnitPriceCents: 55000, Quantity: "1", Currency: "usd", EffectiveFrom: now,
		}},
	})
	_, _, _ = svc.GenerateInvoice(context.Background(), orgID, &service.ID)
	if _, err := svc.CancelBillableService(context.Background(), user, orgID, service.ID); err != nil {
		t.Fatalf("CancelBillableService returned error: %v", err)
	}
	if len(repo.invoices) != 1 || len(repo.prices[service.ID]) != 1 {
		t.Fatal("canceling service deleted billing history")
	}
}

type billingFakeRepo struct {
	store.Repository
	organizations map[uuid.UUID]domain.Organization
	accounts      map[uuid.UUID]domain.BillingAccount
	orders        map[uuid.UUID]domain.Order
	services      map[uuid.UUID]domain.BillableService
	prices        map[uuid.UUID][]domain.BillableServicePrice
	usage         []domain.UsageLedgerEntry
	credits       []domain.CreditLedgerEntry
	invoices      map[uuid.UUID]domain.InvoiceRecord
	lines         map[uuid.UUID][]domain.InvoiceLineItem
	stripeEvents  map[string]bool
}

func newBillingFakeRepo() *billingFakeRepo {
	return &billingFakeRepo{
		organizations: map[uuid.UUID]domain.Organization{},
		accounts:      map[uuid.UUID]domain.BillingAccount{},
		orders:        map[uuid.UUID]domain.Order{},
		services:      map[uuid.UUID]domain.BillableService{},
		prices:        map[uuid.UUID][]domain.BillableServicePrice{},
		invoices:      map[uuid.UUID]domain.InvoiceRecord{},
		lines:         map[uuid.UUID][]domain.InvoiceLineItem{},
		stripeEvents:  map[string]bool{},
	}
}

func (r *billingFakeRepo) CreateOrganization(ctx context.Context, params store.CreateOrganizationParams) (domain.Organization, error) {
	org := domain.Organization{ID: uuid.New(), Name: params.Name, Slug: params.Slug, CreatedByUserID: params.CreatedByUserID}
	r.organizations[org.ID] = org
	r.accounts[org.ID] = domain.BillingAccount{ID: uuid.New(), OrganizationID: org.ID, BillingEmail: params.BillingEmail, Currency: "usd", Status: domain.BillingAccountActive, PaymentTerms: domain.PaymentTermsPrepaid}
	return org, nil
}

func (r *billingFakeRepo) GetBillingAccount(ctx context.Context, organizationID uuid.UUID) (domain.BillingAccount, error) {
	account, ok := r.accounts[organizationID]
	if !ok {
		return domain.BillingAccount{}, store.ErrNotFound
	}
	return account, nil
}

func (r *billingFakeRepo) UpdateBillingAccount(ctx context.Context, organizationID uuid.UUID, params store.UpdateBillingAccountParams) (domain.BillingAccount, error) {
	account, err := r.GetBillingAccount(ctx, organizationID)
	if err != nil {
		return domain.BillingAccount{}, err
	}
	account.BillingEmail = params.BillingEmail
	account.PaymentTerms = params.PaymentTerms
	account.AutoRechargeEnabled = params.AutoRechargeEnabled
	account.AutoRechargeThresholdCents = params.AutoRechargeThresholdCents
	account.AutoRechargeAmountCents = params.AutoRechargeAmountCents
	r.accounts[organizationID] = account
	return account, nil
}

func (r *billingFakeRepo) CreateOrder(ctx context.Context, params store.CreateOrderParams) (domain.Order, error) {
	order := domain.Order{ID: uuid.New(), OrganizationID: params.OrganizationID, CreatedByUserID: params.CreatedByUserID, Status: domain.OrderPending, OrderType: params.OrderType, SubtotalCents: params.SubtotalCents, TaxCents: params.TaxCents, TotalCents: params.TotalCents, Metadata: params.Metadata}
	r.orders[order.ID] = order
	return order, nil
}

func (r *billingFakeRepo) ListOrders(ctx context.Context, organizationID uuid.UUID) ([]domain.Order, error) {
	var orders []domain.Order
	for _, order := range r.orders {
		if order.OrganizationID == organizationID {
			orders = append(orders, order)
		}
	}
	return orders, nil
}

func (r *billingFakeRepo) GetOrder(ctx context.Context, organizationID, orderID uuid.UUID) (domain.Order, error) {
	order, ok := r.orders[orderID]
	if !ok || order.OrganizationID != organizationID {
		return domain.Order{}, store.ErrNotFound
	}
	return order, nil
}

func (r *billingFakeRepo) MarkOrderPaidAndActivate(ctx context.Context, organizationID, orderID uuid.UUID, stripePaymentIntentID *string) (domain.Order, error) {
	order, err := r.GetOrder(ctx, organizationID, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	order.Status = domain.OrderPaid
	order.StripePaymentIntentID = stripePaymentIntentID
	r.orders[orderID] = order
	if order.OrderType == domain.OrderCreditPurchase {
		_, err := r.CreateCreditLedgerEntry(ctx, store.CreateCreditLedgerParams{OrganizationID: organizationID, Type: domain.CreditPurchase, AmountCents: order.TotalCents, SourceType: strPtr("order"), SourceID: &order.ID, Description: "Credit purchase"})
		return order, err
	}
	if order.OrderType == domain.OrderServerPurchase {
		var pending store.PendingServiceMetadata
		_ = json.Unmarshal(order.Metadata, &pending)
		now := time.Now().UTC()
		end := now.AddDate(0, 1, 0)
		_, err := r.CreateBillableService(ctx, store.CreateBillableServiceParams{OrganizationID: organizationID, ProjectID: pending.ProjectID, OrderID: &order.ID, ServiceType: pending.ServiceType, ServiceID: pending.ServiceID, BillingMode: pending.BillingMode, Status: domain.BillableProvisioning, StartedAt: now, PeriodStart: &now, PeriodEnd: &end, NextInvoiceAt: &end, Prices: []store.CreateBillableServicePriceParams{{PriceType: domain.PriceRecurring, Description: pending.Description, Unit: pending.Unit, UnitPriceCents: pending.UnitPriceCents, Quantity: pending.Quantity, Currency: pending.Currency, EffectiveFrom: now}}})
		return order, err
	}
	return order, nil
}

func (r *billingFakeRepo) CreateBillableService(ctx context.Context, params store.CreateBillableServiceParams) (domain.BillableService, error) {
	service := domain.BillableService{ID: uuid.New(), OrganizationID: params.OrganizationID, ProjectID: params.ProjectID, OrderID: params.OrderID, ServiceType: params.ServiceType, ServiceID: params.ServiceID, BillingMode: params.BillingMode, Status: params.Status, BillingStartedAt: params.StartedAt, CurrentPeriodStart: params.PeriodStart, CurrentPeriodEnd: params.PeriodEnd, NextInvoiceAt: params.NextInvoiceAt}
	r.services[service.ID] = service
	for _, price := range params.Prices {
		r.prices[service.ID] = append(r.prices[service.ID], domain.BillableServicePrice{ID: uuid.New(), BillableServiceID: service.ID, PriceType: price.PriceType, Description: price.Description, Unit: price.Unit, UnitPriceCents: price.UnitPriceCents, Quantity: defaultQuantity(price.Quantity), Currency: price.Currency, EffectiveFrom: price.EffectiveFrom, Metadata: price.Metadata})
	}
	return service, nil
}

func (r *billingFakeRepo) ListBillableServices(ctx context.Context, organizationID uuid.UUID) ([]domain.BillableService, error) {
	var services []domain.BillableService
	for _, service := range r.services {
		if service.OrganizationID == organizationID {
			services = append(services, service)
		}
	}
	return services, nil
}

func (r *billingFakeRepo) GetBillableService(ctx context.Context, organizationID, serviceID uuid.UUID) (domain.BillableService, error) {
	service, ok := r.services[serviceID]
	if !ok || service.OrganizationID != organizationID {
		return domain.BillableService{}, store.ErrNotFound
	}
	return service, nil
}

func (r *billingFakeRepo) UpdateBillableServiceStatus(ctx context.Context, organizationID, serviceID uuid.UUID, status domain.BillableServiceStatus, endedAt *time.Time) (domain.BillableService, error) {
	service, err := r.GetBillableService(ctx, organizationID, serviceID)
	if err != nil {
		return domain.BillableService{}, err
	}
	service.Status = status
	service.BillingEndedAt = endedAt
	r.services[serviceID] = service
	return service, nil
}

func (r *billingFakeRepo) ListBillableServicePrices(ctx context.Context, billableServiceID uuid.UUID) ([]domain.BillableServicePrice, error) {
	return r.prices[billableServiceID], nil
}

func (r *billingFakeRepo) RecordUsage(ctx context.Context, params store.RecordUsageParams) (domain.UsageLedgerEntry, error) {
	entry := domain.UsageLedgerEntry{ID: uuid.New(), OrganizationID: params.OrganizationID, BillableServiceID: params.BillableServiceID, UsageType: params.UsageType, Quantity: params.Quantity, Unit: params.Unit, UnitPriceCents: params.UnitPriceCents, AmountCents: params.AmountCents, PeriodStart: params.PeriodStart, PeriodEnd: params.PeriodEnd, Status: params.Status}
	r.usage = append(r.usage, entry)
	if params.DebitPrepaidCredits {
		_, err := r.CreateCreditLedgerEntry(ctx, store.CreateCreditLedgerParams{OrganizationID: params.OrganizationID, Type: domain.UsageDebit, AmountCents: -params.AmountCents, SourceType: strPtr("usage_ledger"), SourceID: &entry.ID, Description: params.Description})
		return entry, err
	}
	return entry, nil
}

func (r *billingFakeRepo) ListUsage(ctx context.Context, organizationID uuid.UUID) ([]domain.UsageLedgerEntry, error) {
	var entries []domain.UsageLedgerEntry
	for _, entry := range r.usage {
		if entry.OrganizationID == organizationID {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func (r *billingFakeRepo) ListServiceUsage(ctx context.Context, organizationID, serviceID uuid.UUID) ([]domain.UsageLedgerEntry, error) {
	var entries []domain.UsageLedgerEntry
	for _, entry := range r.usage {
		if entry.OrganizationID == organizationID && entry.BillableServiceID == serviceID {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func (r *billingFakeRepo) CreateCreditLedgerEntry(ctx context.Context, params store.CreateCreditLedgerParams) (domain.CreditLedgerEntry, error) {
	account := r.accounts[params.OrganizationID]
	account.CreditBalanceCents += params.AmountCents
	r.accounts[params.OrganizationID] = account
	entry := domain.CreditLedgerEntry{ID: uuid.New(), OrganizationID: params.OrganizationID, BillingAccountID: account.ID, Type: params.Type, AmountCents: params.AmountCents, BalanceAfterCents: account.CreditBalanceCents, SourceType: params.SourceType, SourceID: params.SourceID, Description: params.Description, Metadata: params.Metadata}
	r.credits = append(r.credits, entry)
	return entry, nil
}

func (r *billingFakeRepo) ListCreditLedger(ctx context.Context, organizationID uuid.UUID) ([]domain.CreditLedgerEntry, error) {
	var entries []domain.CreditLedgerEntry
	for _, entry := range r.credits {
		if entry.OrganizationID == organizationID {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func (r *billingFakeRepo) GenerateInvoice(ctx context.Context, params store.GenerateInvoiceParams) (domain.InvoiceRecord, []domain.InvoiceLineItem, error) {
	var lines []domain.InvoiceLineItem
	var total int64
	for _, service := range r.services {
		if service.OrganizationID != params.OrganizationID || (params.ServiceID != nil && service.ID != *params.ServiceID) {
			continue
		}
		for _, price := range r.prices[service.ID] {
			amount := price.UnitPriceCents
			total += amount
			lines = append(lines, domain.InvoiceLineItem{ID: uuid.New(), OrganizationID: params.OrganizationID, BillableServiceID: &service.ID, Description: price.Description, Quantity: price.Quantity, Unit: price.Unit, UnitPriceCents: price.UnitPriceCents, AmountCents: amount, PeriodStart: service.CurrentPeriodStart, PeriodEnd: service.CurrentPeriodEnd})
		}
	}
	invoice := domain.InvoiceRecord{ID: uuid.New(), OrganizationID: params.OrganizationID, Status: domain.InvoiceDraft, SubtotalCents: total, TotalCents: total}
	r.invoices[invoice.ID] = invoice
	for i := range lines {
		lines[i].InvoiceRecordID = invoice.ID
		r.lines[invoice.ID] = append(r.lines[invoice.ID], lines[i])
	}
	return invoice, lines, nil
}

func (r *billingFakeRepo) ListInvoiceRecords(ctx context.Context, organizationID uuid.UUID) ([]domain.InvoiceRecord, error) {
	var invoices []domain.InvoiceRecord
	for _, invoice := range r.invoices {
		if invoice.OrganizationID == organizationID {
			invoices = append(invoices, invoice)
		}
	}
	return invoices, nil
}

func (r *billingFakeRepo) GetInvoiceRecord(ctx context.Context, organizationID, invoiceID uuid.UUID) (domain.InvoiceRecord, error) {
	invoice, ok := r.invoices[invoiceID]
	if !ok || invoice.OrganizationID != organizationID {
		return domain.InvoiceRecord{}, store.ErrNotFound
	}
	return invoice, nil
}

func (r *billingFakeRepo) ListInvoiceLineItems(ctx context.Context, organizationID, invoiceID uuid.UUID) ([]domain.InvoiceLineItem, error) {
	return r.lines[invoiceID], nil
}

func (r *billingFakeRepo) FinalizeInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID, stripeInvoiceID *string) (domain.InvoiceRecord, error) {
	invoice, err := r.GetInvoiceRecord(ctx, organizationID, invoiceID)
	if err != nil {
		return domain.InvoiceRecord{}, err
	}
	invoice.Status = domain.InvoiceOpen
	now := time.Now().UTC()
	invoice.FinalizedAt = &now
	invoice.StripeInvoiceID = stripeInvoiceID
	r.invoices[invoiceID] = invoice
	return invoice, nil
}

func (r *billingFakeRepo) VoidInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID) (domain.InvoiceRecord, error) {
	invoice, err := r.GetInvoiceRecord(ctx, organizationID, invoiceID)
	if err != nil {
		return domain.InvoiceRecord{}, err
	}
	invoice.Status = domain.InvoiceVoid
	r.invoices[invoiceID] = invoice
	return invoice, nil
}

func (r *billingFakeRepo) RecordStripeEvent(ctx context.Context, params store.StripeEventParams) (bool, error) {
	if r.stripeEvents[params.StripeEventID] {
		return false, nil
	}
	r.stripeEvents[params.StripeEventID] = true
	return true, nil
}

func (r *billingFakeRepo) MarkStripeEventProcessed(ctx context.Context, stripeEventID string) error {
	return nil
}

func (r *billingFakeRepo) ResourceBelongsToOrganization(ctx context.Context, resourceType string, resourceID, organizationID uuid.UUID) (bool, error) {
	if resourceType == "billable_services" {
		service, ok := r.services[resourceID]
		return ok && service.OrganizationID == organizationID, nil
	}
	return false, nil
}

func (r *billingFakeRepo) AddAuditLog(ctx context.Context, organizationID uuid.UUID, actorUserID *uuid.UUID, action, entityType string, entityID *uuid.UUID, metadata []byte) error {
	return nil
}

func (r *billingFakeRepo) mutateFinalizedLine(invoiceID uuid.UUID) error {
	if r.invoices[invoiceID].Status != domain.InvoiceDraft {
		return ErrFinalizedInvoice
	}
	return nil
}

func (r *billingFakeRepo) ledgerBalance(organizationID uuid.UUID) int64 {
	var balance int64
	for _, entry := range r.credits {
		if entry.OrganizationID == organizationID {
			balance += entry.AmountCents
		}
	}
	return balance
}

func createServerOrder(t *testing.T, svc *Services, user domain.User, orgID uuid.UUID) domain.Order {
	t.Helper()
	metadata, _ := json.Marshal(store.PendingServiceMetadata{ServiceType: domain.ServiceServer, BillingMode: domain.BillingMonthlyPrepaid, Description: "Ryzen 9950X / 192GB / 2x3.84TB NVMe monthly rental", Unit: domain.UnitMonth, UnitPriceCents: 55000, Quantity: "1", Currency: "usd"})
	order, err := svc.CreateOrder(context.Background(), user, store.CreateOrderParams{OrganizationID: orgID, OrderType: domain.OrderServerPurchase, SubtotalCents: 55000, TotalCents: 55000, Metadata: metadata})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	return order
}

func stripePayload(eventID, eventType string, orgID, orderID uuid.UUID) []byte {
	payload, _ := json.Marshal(map[string]any{
		"id":   eventID,
		"type": eventType,
		"data": map[string]any{"object": map[string]any{
			"id":             "pi_test",
			"payment_intent": "pi_test",
			"metadata": map[string]string{
				"organization_id": orgID.String(),
				"order_id":        orderID.String(),
			},
		}},
	})
	return payload
}

func ptrTime(value time.Time) *time.Time { return &value }

func defaultQuantity(value string) string {
	if value == "" {
		return "1"
	}
	return value
}

func strPtr(value string) *string {
	return &value
}
