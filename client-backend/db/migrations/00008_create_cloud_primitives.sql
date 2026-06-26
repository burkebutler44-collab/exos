-- +goose Up
create table clouds (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	name text not null,
	slug text not null,
	location_id uuid references locations(id) on delete set null,
	description text,
	status text not null default 'active' check (status in ('active', 'suspended', 'deleted')),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create unique index clouds_org_slug_active_idx on clouds (organization_id, slug) where deleted_at is null;
create unique index clouds_org_name_active_idx on clouds (organization_id, lower(name)) where deleted_at is null;
create index clouds_org_status_idx on clouds (organization_id, status);

alter table servers
	add column cloud_id uuid references clouds(id) on delete set null,
	add column server_mode text not null default 'bare_metal' check (server_mode in ('bare_metal', 'virtualization_host', 'managed_services_host')),
	add column mode_status text not null default 'ready' check (mode_status in ('ready', 'changing', 'error', 'pending')),
	add column platform_managed boolean not null default false,
	add column reserved_cpu_cores integer,
	add column reserved_memory_mb integer,
	add column reserved_storage_gb integer;

create index servers_cloud_idx on servers (cloud_id);
create index servers_cloud_mode_idx on servers (cloud_id, server_mode);

create table private_networks (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	cloud_id uuid not null references clouds(id) on delete cascade,
	name text not null,
	description text,
	cidr cidr not null,
	gateway_ip inet,
	network_type text not null default 'private' check (network_type in ('private')),
	isolation_type text not null default 'stub' check (isolation_type in ('vlan', 'vxlan', 'evpn_vxlan', 'stub')),
	vlan_id integer,
	vni integer,
	status text not null default 'pending' check (status in ('pending', 'active', 'error', 'deleted')),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create unique index private_networks_cloud_name_active_idx on private_networks (cloud_id, lower(name)) where deleted_at is null;
create index private_networks_cloud_idx on private_networks (cloud_id, status);

create table virtual_machines (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	cloud_id uuid not null references clouds(id) on delete cascade,
	host_server_id uuid references servers(id) on delete set null,
	name text not null,
	hostname text not null default '',
	status text not null default 'pending' check (status in ('draft', 'pending', 'provisioning', 'running', 'stopped', 'error', 'deleted')),
	power_state text not null default 'unknown' check (power_state in ('unknown', 'running', 'stopped')),
	cpu_cores integer not null check (cpu_cores > 0),
	memory_mb integer not null check (memory_mb > 0),
	disk_gb integer not null check (disk_gb > 0),
	image_id text,
	os_image text,
	private_ip inet,
	public_ip_id uuid,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create unique index virtual_machines_cloud_name_active_idx on virtual_machines (cloud_id, lower(name)) where deleted_at is null;
create index virtual_machines_cloud_idx on virtual_machines (cloud_id, status);
create index virtual_machines_host_idx on virtual_machines (host_server_id);

create table managed_services (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	cloud_id uuid not null references clouds(id) on delete cascade,
	host_server_id uuid references servers(id) on delete set null,
	service_type text not null check (service_type in ('postgres', 'redis', 'mysql', 'minio', 'nats', 'rabbitmq')),
	name text not null,
	status text not null default 'pending' check (status in ('draft', 'pending', 'provisioning', 'running', 'stopped', 'error', 'deleted')),
	plan_name text,
	cpu_cores integer not null check (cpu_cores > 0),
	memory_mb integer not null check (memory_mb > 0),
	storage_gb integer not null check (storage_gb > 0),
	version text,
	endpoint_hostname text,
	private_ip inet,
	port integer,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create unique index managed_services_cloud_name_active_idx on managed_services (cloud_id, lower(name)) where deleted_at is null;
create index managed_services_cloud_idx on managed_services (cloud_id, service_type, status);
create index managed_services_host_idx on managed_services (host_server_id);

create table managed_service_backup_policies (
	id uuid primary key default gen_random_uuid(),
	managed_service_id uuid not null unique references managed_services(id) on delete cascade,
	enabled boolean not null default false,
	frequency text not null default 'daily' check (frequency in ('daily', 'weekly')),
	retention_days integer not null default 7 check (retention_days > 0),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table network_attachments (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	cloud_id uuid not null references clouds(id) on delete cascade,
	private_network_id uuid not null references private_networks(id) on delete cascade,
	resource_type text not null check (resource_type in ('server', 'virtual_machine', 'managed_service')),
	resource_id uuid not null,
	private_ip inet,
	mac_address macaddr,
	status text not null default 'attached_stub' check (status in ('pending', 'attached', 'attached_stub', 'error', 'detached')),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (private_network_id, resource_type, resource_id)
);

create unique index network_attachments_ip_active_idx on network_attachments (private_network_id, private_ip) where private_ip is not null and status <> 'detached';
create index network_attachments_cloud_idx on network_attachments (cloud_id, resource_type);

create table resource_actions (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	cloud_id uuid references clouds(id) on delete cascade,
	resource_type text not null,
	resource_id uuid,
	action_type text not null,
	status text not null default 'stubbed' check (status in ('pending', 'stubbed', 'completed', 'failed')),
	message text not null default '',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index resource_actions_cloud_created_idx on resource_actions (cloud_id, created_at desc);
create index resource_actions_org_created_idx on resource_actions (organization_id, created_at desc);

insert into role_permissions (role_id, permission)
select r.id, p.permission
from roles r
join (
	values
		('Owner', 'clouds.view'), ('Owner', 'clouds.manage'),
		('Admin', 'clouds.view'), ('Admin', 'clouds.manage'),
		('Infrastructure', 'clouds.view'), ('Infrastructure', 'clouds.manage'),
		('Read-only', 'clouds.view')
) as p(role_name, permission) on p.role_name = r.name
on conflict (role_id, permission) do nothing;

insert into platform_role_permissions (role_id, permission)
select r.id, p.permission
from platform_roles r
join (
	values
		('SuperAdmin', 'platform.clouds.view'),
		('SuperAdmin', 'platform.clouds.manage'),
		('InfrastructureOps', 'platform.clouds.view'),
		('InfrastructureOps', 'platform.clouds.manage'),
		('SupportOps', 'platform.clouds.view'),
		('ReadOnlyOps', 'platform.clouds.view')
) as p(role_name, permission) on p.role_name = r.name
on conflict (role_id, permission) do nothing;

-- +goose Down
delete from platform_role_permissions where permission in ('platform.clouds.view', 'platform.clouds.manage');
delete from role_permissions where permission in ('clouds.view', 'clouds.manage');
drop table if exists resource_actions;
drop table if exists network_attachments;
drop table if exists managed_service_backup_policies;
drop table if exists managed_services;
drop table if exists virtual_machines;
drop table if exists private_networks;
alter table servers
	drop column if exists reserved_storage_gb,
	drop column if exists reserved_memory_mb,
	drop column if exists reserved_cpu_cores,
	drop column if exists platform_managed,
	drop column if exists mode_status,
	drop column if exists server_mode,
	drop column if exists cloud_id;
drop table if exists clouds;
