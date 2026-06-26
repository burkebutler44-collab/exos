-- +goose Up
create table platform_roles (
	id uuid primary key default gen_random_uuid(),
	name text not null unique,
	description text not null default '',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table platform_role_permissions (
	id uuid primary key default gen_random_uuid(),
	role_id uuid not null references platform_roles(id) on delete cascade,
	permission text not null,
	created_at timestamptz not null default now(),
	unique (role_id, permission)
);

create table platform_user_roles (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	role_id uuid not null references platform_roles(id) on delete cascade,
	assigned_by_user_id uuid references users(id) on delete set null,
	created_at timestamptz not null default now(),
	unique (user_id, role_id)
);

create table admin_audit_log (
	id uuid primary key default gen_random_uuid(),
	actor_user_id uuid references users(id) on delete set null,
	action text not null,
	target_type text not null,
	target_id text not null,
	organization_id uuid,
	server_id uuid,
	reason text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now()
);

create index admin_audit_log_created_idx on admin_audit_log (created_at desc);
create index admin_audit_log_actor_idx on admin_audit_log (actor_user_id, created_at desc);

create table os_images (
	id uuid primary key default gen_random_uuid(),
	name text not null,
	slug text not null unique,
	version text not null,
	family text not null,
	architecture text not null,
	enabled boolean not null default true,
	is_default boolean not null default false,
	tinkerbell_template_ref text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

insert into platform_roles (name, description)
values
	('SuperAdmin', 'Full platform access'),
	('InfrastructureOps', 'Server, rack, and provisioning operations'),
	('BillingOps', 'Billing, orders, invoices, and credits operations'),
	('SupportOps', 'Read support context and limited organization support actions'),
	('ReadOnlyOps', 'Read-only platform operations')
on conflict (name) do nothing;

insert into platform_role_permissions (role_id, permission)
select r.id, p.permission
from platform_roles r
join (
	values
		('SuperAdmin', 'platform.users.view'), ('SuperAdmin', 'platform.users.manage'), ('SuperAdmin', 'platform.organizations.view'), ('SuperAdmin', 'platform.organizations.manage'), ('SuperAdmin', 'platform.organizations.suspend'), ('SuperAdmin', 'platform.organizations.impersonate'), ('SuperAdmin', 'platform.billing.view'), ('SuperAdmin', 'platform.billing.manage'), ('SuperAdmin', 'platform.billing.adjust'), ('SuperAdmin', 'platform.orders.view'), ('SuperAdmin', 'platform.invoices.view'), ('SuperAdmin', 'platform.credits.manage'), ('SuperAdmin', 'platform.servers.view'), ('SuperAdmin', 'platform.servers.create'), ('SuperAdmin', 'platform.servers.update'), ('SuperAdmin', 'platform.servers.assign'), ('SuperAdmin', 'platform.servers.retire'), ('SuperAdmin', 'platform.servers.power.manage'), ('SuperAdmin', 'platform.servers.provision'), ('SuperAdmin', 'platform.racks.view'), ('SuperAdmin', 'platform.racks.manage'), ('SuperAdmin', 'platform.provisioning.view'), ('SuperAdmin', 'platform.provisioning.manage'), ('SuperAdmin', 'platform.audit_log.view'), ('SuperAdmin', 'platform.settings.manage'),
		('InfrastructureOps', 'platform.servers.view'), ('InfrastructureOps', 'platform.servers.create'), ('InfrastructureOps', 'platform.servers.update'), ('InfrastructureOps', 'platform.servers.assign'), ('InfrastructureOps', 'platform.servers.retire'), ('InfrastructureOps', 'platform.servers.power.manage'), ('InfrastructureOps', 'platform.servers.provision'), ('InfrastructureOps', 'platform.racks.view'), ('InfrastructureOps', 'platform.racks.manage'), ('InfrastructureOps', 'platform.provisioning.view'), ('InfrastructureOps', 'platform.provisioning.manage'), ('InfrastructureOps', 'platform.audit_log.view'),
		('BillingOps', 'platform.organizations.view'), ('BillingOps', 'platform.billing.view'), ('BillingOps', 'platform.billing.manage'), ('BillingOps', 'platform.billing.adjust'), ('BillingOps', 'platform.orders.view'), ('BillingOps', 'platform.invoices.view'), ('BillingOps', 'platform.credits.manage'), ('BillingOps', 'platform.audit_log.view'),
		('SupportOps', 'platform.users.view'), ('SupportOps', 'platform.organizations.view'), ('SupportOps', 'platform.billing.view'), ('SupportOps', 'platform.orders.view'), ('SupportOps', 'platform.invoices.view'), ('SupportOps', 'platform.servers.view'), ('SupportOps', 'platform.racks.view'), ('SupportOps', 'platform.provisioning.view'), ('SupportOps', 'platform.audit_log.view'),
		('ReadOnlyOps', 'platform.users.view'), ('ReadOnlyOps', 'platform.organizations.view'), ('ReadOnlyOps', 'platform.billing.view'), ('ReadOnlyOps', 'platform.orders.view'), ('ReadOnlyOps', 'platform.invoices.view'), ('ReadOnlyOps', 'platform.servers.view'), ('ReadOnlyOps', 'platform.racks.view'), ('ReadOnlyOps', 'platform.provisioning.view'), ('ReadOnlyOps', 'platform.audit_log.view')
) as p(role_name, permission) on p.role_name = r.name
on conflict (role_id, permission) do nothing;

-- +goose Down
drop table if exists os_images;
drop table if exists admin_audit_log;
drop table if exists platform_user_roles;
drop table if exists platform_role_permissions;
drop table if exists platform_roles;
