-- +goose Up
create extension if not exists pgcrypto;

create table users (
	id uuid primary key default gen_random_uuid(),
	auth0_sub text not null unique,
	email text not null,
	name text not null default '',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table organizations (
	id uuid primary key default gen_random_uuid(),
	name text not null,
	slug text not null unique,
	created_by_user_id uuid not null references users(id),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table roles (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid references organizations(id) on delete cascade,
	name text not null,
	is_system_role boolean not null default false,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (organization_id, name)
);

create unique index roles_system_name_unique on roles (name) where organization_id is null;

create table role_permissions (
	id uuid primary key default gen_random_uuid(),
	role_id uuid not null references roles(id) on delete cascade,
	permission text not null,
	created_at timestamptz not null default now(),
	unique (role_id, permission)
);

create table organization_memberships (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	user_id uuid not null references users(id) on delete cascade,
	role_id uuid not null references roles(id),
	status text not null check (status in ('active', 'invited', 'suspended', 'removed')),
	joined_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (organization_id, user_id)
);

create table invitations (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	email text not null,
	role_id uuid not null references roles(id),
	invited_by_user_id uuid not null references users(id),
	token text not null unique,
	status text not null check (status in ('pending', 'accepted', 'expired', 'revoked')),
	expires_at timestamptz not null,
	accepted_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index invitations_org_status_idx on invitations (organization_id, status);

create table projects (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	name text not null,
	slug text not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (organization_id, slug)
);

create table billing_profiles (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null unique references organizations(id) on delete cascade,
	stripe_customer_id text,
	billing_email text not null,
	company_name text not null,
	tax_id text,
	line1 text,
	line2 text,
	city text,
	state text,
	postal_code text,
	country text,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table payment_methods (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	stripe_payment_method_id text not null unique,
	brand text not null,
	last4 text not null,
	exp_month integer not null,
	exp_year integer not null,
	is_default boolean not null default false,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create unique index payment_methods_one_default_per_org on payment_methods (organization_id) where is_default;

create table invoices (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	stripe_invoice_id text not null unique,
	status text not null,
	amount_due bigint not null default 0,
	amount_paid bigint not null default 0,
	period_start timestamptz not null,
	period_end timestamptz not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table audit_log (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	actor_user_id uuid references users(id) on delete set null,
	action text not null,
	entity_type text not null,
	entity_id uuid,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now()
);

create index audit_log_org_created_idx on audit_log (organization_id, created_at desc);

insert into roles (name, is_system_role)
values ('Owner', true), ('Admin', true), ('Infrastructure', true), ('Billing', true), ('Read-only', true);

insert into role_permissions (role_id, permission)
select r.id, p.permission
from roles r
join (
	values
		('Owner', 'organizations.view'), ('Owner', 'organizations.update'), ('Owner', 'organizations.delete'),
		('Owner', 'servers.view'), ('Owner', 'servers.create'), ('Owner', 'servers.power.manage'), ('Owner', 'servers.reinstall'), ('Owner', 'servers.delete'),
		('Owner', 'networks.view'), ('Owner', 'networks.manage'),
		('Owner', 'ssh_keys.view'), ('Owner', 'ssh_keys.manage'),
		('Owner', 'billing.view'), ('Owner', 'billing.manage'), ('Owner', 'invoices.view'),
		('Owner', 'members.view'), ('Owner', 'members.invite'), ('Owner', 'members.remove'), ('Owner', 'members.roles.manage'),
		('Owner', 'api_keys.view'), ('Owner', 'api_keys.create'), ('Owner', 'api_keys.revoke'),
		('Owner', 'projects.view'), ('Owner', 'projects.create'), ('Owner', 'projects.update'), ('Owner', 'projects.delete'),
		('Owner', 'audit_log.view'),
		('Admin', 'organizations.view'), ('Admin', 'organizations.update'),
		('Admin', 'servers.view'), ('Admin', 'servers.create'), ('Admin', 'servers.power.manage'), ('Admin', 'servers.reinstall'), ('Admin', 'servers.delete'),
		('Admin', 'networks.view'), ('Admin', 'networks.manage'),
		('Admin', 'ssh_keys.view'), ('Admin', 'ssh_keys.manage'),
		('Admin', 'billing.view'), ('Admin', 'invoices.view'),
		('Admin', 'members.view'), ('Admin', 'members.invite'), ('Admin', 'members.remove'), ('Admin', 'members.roles.manage'),
		('Admin', 'api_keys.view'), ('Admin', 'api_keys.create'), ('Admin', 'api_keys.revoke'),
		('Admin', 'projects.view'), ('Admin', 'projects.create'), ('Admin', 'projects.update'), ('Admin', 'projects.delete'),
		('Admin', 'audit_log.view'),
		('Infrastructure', 'organizations.view'),
		('Infrastructure', 'servers.view'), ('Infrastructure', 'servers.create'), ('Infrastructure', 'servers.power.manage'), ('Infrastructure', 'servers.reinstall'), ('Infrastructure', 'servers.delete'),
		('Infrastructure', 'networks.view'), ('Infrastructure', 'networks.manage'),
		('Infrastructure', 'ssh_keys.view'), ('Infrastructure', 'ssh_keys.manage'),
		('Infrastructure', 'api_keys.view'), ('Infrastructure', 'api_keys.create'), ('Infrastructure', 'api_keys.revoke'),
		('Infrastructure', 'projects.view'),
		('Billing', 'organizations.view'), ('Billing', 'billing.view'), ('Billing', 'billing.manage'), ('Billing', 'invoices.view'),
		('Read-only', 'organizations.view'), ('Read-only', 'servers.view'), ('Read-only', 'networks.view'), ('Read-only', 'ssh_keys.view'),
		('Read-only', 'billing.view'), ('Read-only', 'invoices.view'), ('Read-only', 'members.view'),
		('Read-only', 'api_keys.view'), ('Read-only', 'projects.view'), ('Read-only', 'audit_log.view')
) as p(role_name, permission) on p.role_name = r.name;

-- +goose Down
drop table if exists audit_log;
drop table if exists invoices;
drop table if exists payment_methods;
drop table if exists billing_profiles;
drop table if exists projects;
drop table if exists invitations;
drop table if exists organization_memberships;
drop table if exists role_permissions;
drop table if exists roles;
drop table if exists organizations;
drop table if exists users;
drop extension if exists pgcrypto;
