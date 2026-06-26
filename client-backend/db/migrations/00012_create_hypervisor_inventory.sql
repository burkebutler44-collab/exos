-- +goose Up
create table hypervisors (
	id text primary key,
	server_id uuid references servers(id) on delete set null,
	hostname text not null default '',
	status text not null default 'online' check (status in ('online', 'degraded', 'offline', 'maintenance')),
	vcpus_total integer not null default 0,
	vcpus_active integer not null default 0,
	memory_total_bytes bigint not null default 0,
	memory_active_bytes bigint not null default 0,
	disk_total_bytes bigint not null default 0,
	disk_available_bytes bigint not null default 0,
	wireguard_interface text not null default '',
	control_plane_address text not null default '',
	last_reported_at timestamptz,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index hypervisors_server_idx on hypervisors (server_id);
create index hypervisors_status_idx on hypervisors (status, last_reported_at desc);

create table hypervisor_vms (
	id text not null,
	hypervisor_id text not null references hypervisors(id) on delete cascade,
	name text not null,
	status text not null default 'unknown' check (status in ('running', 'stopped', 'paused', 'unknown')),
	vcpus integer not null default 0,
	memory_bytes bigint not null default 0,
	disk_bytes bigint not null default 0,
	mac_addresses text[] not null default '{}'::text[],
	ip_addresses text[] not null default '{}'::text[],
	metadata jsonb not null default '{}'::jsonb,
	last_reported_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (hypervisor_id, id)
);

create unique index hypervisor_vms_name_idx on hypervisor_vms (hypervisor_id, lower(name));
create index hypervisor_vms_status_idx on hypervisor_vms (hypervisor_id, status);

create table hypervisor_events (
	id uuid primary key default gen_random_uuid(),
	hypervisor_id text not null references hypervisors(id) on delete cascade,
	event_type text not null,
	message text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now()
);

create index hypervisor_events_hypervisor_created_idx on hypervisor_events (hypervisor_id, created_at desc);

insert into platform_role_permissions (role_id, permission)
select r.id, p.permission
from platform_roles r
join (
	values
		('SuperAdmin', 'platform.hypervisors.view'),
		('SuperAdmin', 'platform.hypervisors.manage'),
		('InfrastructureOps', 'platform.hypervisors.view'),
		('InfrastructureOps', 'platform.hypervisors.manage'),
		('SupportOps', 'platform.hypervisors.view'),
		('ReadOnlyOps', 'platform.hypervisors.view')
) as p(role_name, permission) on p.role_name = r.name
on conflict (role_id, permission) do nothing;

-- +goose Down
delete from platform_role_permissions where permission in ('platform.hypervisors.view', 'platform.hypervisors.manage');
drop table if exists hypervisor_events;
drop table if exists hypervisor_vms;
drop table if exists hypervisors;
