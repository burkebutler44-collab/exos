-- name: GetBillingProfile :one
select id, organization_id, stripe_customer_id, billing_email, company_name, tax_id, line1, line2, city, state, postal_code, country, created_at, updated_at
from billing_profiles
where organization_id = $1;

-- name: ListInvoices :many
select id, organization_id, stripe_invoice_id, status, amount_due, amount_paid, period_start, period_end, created_at, updated_at
from invoices
where organization_id = $1
order by period_start desc;
