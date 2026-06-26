-- +goose Up
create table billing_accounts (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null unique references organizations(id) on delete cascade,
	stripe_customer_id text,
	billing_email text not null,
	currency text not null default 'usd',
	status text not null default 'active' check (status in ('active', 'past_due', 'suspended', 'closed')),
	payment_terms text not null default 'prepaid' check (payment_terms in ('prepaid', 'due_on_receipt', 'net_7', 'net_15', 'net_30')),
	credit_balance_cents bigint not null default 0,
	auto_recharge_enabled boolean not null default false,
	auto_recharge_threshold_cents bigint,
	auto_recharge_amount_cents bigint,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

insert into billing_accounts (organization_id, billing_email, currency, status, payment_terms)
select o.id, coalesce(nullif(u.email, ''), 'billing@example.invalid'), 'usd', 'active', 'prepaid'
from organizations o
join users u on u.id = o.created_by_user_id
on conflict (organization_id) do nothing;

create table orders (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	created_by_user_id uuid not null references users(id),
	status text not null check (status in ('pending', 'paid', 'failed', 'canceled')),
	order_type text not null check (order_type in ('server_purchase', 'credit_purchase', 'upgrade', 'custom')),
	subtotal_cents bigint not null default 0,
	tax_cents bigint not null default 0,
	total_cents bigint not null default 0,
	stripe_checkout_session_id text,
	stripe_payment_intent_id text,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index orders_org_created_idx on orders (organization_id, created_at desc);

create table billable_services (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	project_id uuid references projects(id) on delete set null,
	order_id uuid references orders(id) on delete set null,
	service_type text not null check (service_type in ('server', 'ip_address', 'bandwidth', 'backup', 'kaas_cluster', 'support_plan', 'custom')),
	service_id uuid,
	billing_mode text not null check (billing_mode in ('monthly_prepaid', 'monthly_postpaid', 'hourly_prepaid', 'hourly_postpaid', 'one_time', 'custom_contract')),
	status text not null check (status in ('provisioning', 'active', 'suspended', 'canceled', 'terminated')),
	billing_started_at timestamptz not null,
	billing_ended_at timestamptz,
	current_period_start timestamptz,
	current_period_end timestamptz,
	next_invoice_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index billable_services_org_status_idx on billable_services (organization_id, status);
create index billable_services_org_service_idx on billable_services (organization_id, service_type, service_id);
create index billable_services_next_invoice_idx on billable_services (next_invoice_at) where status = 'active';

create table billable_service_prices (
	id uuid primary key default gen_random_uuid(),
	billable_service_id uuid not null references billable_services(id) on delete cascade,
	price_type text not null check (price_type in ('recurring', 'usage', 'setup_fee', 'discount', 'credit', 'adjustment')),
	description text not null,
	unit text not null check (unit in ('month', 'hour', 'second', 'gb', 'tb', 'ip', 'each')),
	unit_price_cents bigint not null,
	quantity numeric(20, 6) not null default 1,
	currency text not null default 'usd',
	effective_from timestamptz not null,
	effective_to timestamptz,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now()
);

create index billable_service_prices_service_idx on billable_service_prices (billable_service_id, effective_from desc);

create table usage_ledger (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	billable_service_id uuid not null references billable_services(id) on delete cascade,
	usage_type text not null check (usage_type in ('server_runtime', 'bandwidth_out', 'bandwidth_in', 'ip_runtime', 'backup_storage', 'custom')),
	quantity numeric(20, 6) not null,
	unit text not null check (unit in ('month', 'hour', 'second', 'gb', 'tb', 'ip', 'each')),
	unit_price_cents bigint not null,
	amount_cents bigint not null,
	period_start timestamptz not null,
	period_end timestamptz not null,
	status text not null check (status in ('pending', 'invoiced', 'charged', 'forgiven')),
	created_at timestamptz not null default now()
);

create index usage_ledger_org_created_idx on usage_ledger (organization_id, created_at desc);
create index usage_ledger_service_period_idx on usage_ledger (billable_service_id, period_start, period_end);

create table credit_ledger (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	billing_account_id uuid not null references billing_accounts(id) on delete cascade,
	type text not null check (type in ('credit_purchase', 'usage_debit', 'invoice_payment', 'manual_credit', 'manual_debit', 'refund', 'adjustment')),
	amount_cents bigint not null,
	balance_after_cents bigint not null,
	source_type text,
	source_id uuid,
	description text not null,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now()
);

create index credit_ledger_org_created_idx on credit_ledger (organization_id, created_at desc);

create table billing_periods (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	billable_service_id uuid not null references billable_services(id) on delete cascade,
	period_start timestamptz not null,
	period_end timestamptz not null,
	status text not null check (status in ('open', 'invoiced', 'paid', 'failed', 'void', 'skipped')),
	invoice_record_id uuid,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index billing_periods_service_period_idx on billing_periods (billable_service_id, period_start, period_end);

create table invoice_records (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	stripe_invoice_id text unique,
	invoice_number text,
	status text not null check (status in ('draft', 'open', 'paid', 'failed', 'void', 'uncollectible')),
	subtotal_cents bigint not null default 0,
	tax_cents bigint not null default 0,
	total_cents bigint not null default 0,
	amount_paid_cents bigint not null default 0,
	due_at timestamptz,
	finalized_at timestamptz,
	paid_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

alter table billing_periods
	add constraint billing_periods_invoice_record_fk
	foreign key (invoice_record_id) references invoice_records(id) on delete set null;

create index invoice_records_org_created_idx on invoice_records (organization_id, created_at desc);

create table invoice_line_items (
	id uuid primary key default gen_random_uuid(),
	invoice_record_id uuid not null references invoice_records(id) on delete cascade,
	organization_id uuid not null references organizations(id) on delete cascade,
	billable_service_id uuid references billable_services(id) on delete set null,
	description text not null,
	quantity numeric(20, 6) not null,
	unit text not null check (unit in ('month', 'hour', 'second', 'gb', 'tb', 'ip', 'each')),
	unit_price_cents bigint not null,
	amount_cents bigint not null,
	period_start timestamptz,
	period_end timestamptz,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now()
);

create index invoice_line_items_invoice_idx on invoice_line_items (invoice_record_id);

create table stripe_events (
	id uuid primary key default gen_random_uuid(),
	stripe_event_id text not null unique,
	event_type text not null,
	processed_at timestamptz,
	payload jsonb not null,
	created_at timestamptz not null default now()
);

insert into role_permissions (role_id, permission)
select r.id, p.permission
from roles r
join (
	values
		('Owner', 'payment_methods.manage'), ('Owner', 'credits.view'), ('Owner', 'credits.manage'), ('Owner', 'services.view'), ('Owner', 'services.manage'),
		('Admin', 'credits.view'), ('Admin', 'services.view'), ('Admin', 'services.manage'),
		('Infrastructure', 'services.view'), ('Infrastructure', 'services.manage'),
		('Billing', 'payment_methods.manage'), ('Billing', 'credits.view'), ('Billing', 'credits.manage'), ('Billing', 'services.view'),
		('Read-only', 'credits.view'), ('Read-only', 'services.view')
) as p(role_name, permission) on p.role_name = r.name
on conflict (role_id, permission) do nothing;

-- +goose Down
delete from role_permissions
where permission in ('payment_methods.manage', 'credits.view', 'credits.manage', 'services.view', 'services.manage');

drop table if exists stripe_events;
drop table if exists invoice_line_items;
alter table if exists billing_periods drop constraint if exists billing_periods_invoice_record_fk;
drop table if exists invoice_records;
drop table if exists billing_periods;
drop table if exists credit_ledger;
drop table if exists usage_ledger;
drop table if exists billable_service_prices;
drop table if exists billable_services;
drop table if exists orders;
drop table if exists billing_accounts;
