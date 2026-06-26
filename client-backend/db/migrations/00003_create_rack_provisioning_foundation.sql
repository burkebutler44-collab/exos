-- +goose Up
create table racks (
	id text primary key,
	name text not null,
	location text not null,
	status text not null check (status in ('online', 'degraded', 'offline', 'maintenance')),
	last_heartbeat_at timestamptz,
	last_seen_at timestamptz,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table rack_agents (
	id uuid primary key default gen_random_uuid(),
	rack_id text not null references racks(id) on delete cascade,
	agent_id text not null,
	version text not null,
	status text not null check (status in ('online', 'degraded', 'offline')),
	last_heartbeat_at timestamptz,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (rack_id, agent_id)
);

create table servers (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid references organizations(id) on delete set null,
	project_id uuid references projects(id) on delete set null,
	rack_id text not null references racks(id),
	status text not null check (status in ('available', 'reserved', 'provisioning_requested', 'provisioning_started', 'pxe_booting', 'installing', 'active', 'failed', 'suspended', 'canceled', 'terminated')),
	bmc_address text not null default '',
	mac_address text not null default '',
	provisionable boolean not null default true,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index servers_org_idx on servers (organization_id);
create index servers_rack_status_idx on servers (rack_id, status);

create table provisioning_jobs (
	id uuid primary key default gen_random_uuid(),
	organization_id uuid not null references organizations(id) on delete cascade,
	project_id uuid references projects(id) on delete set null,
	server_id uuid not null references servers(id),
	rack_id text not null references racks(id),
	requested_by_user_id uuid not null references users(id),
	image_id text not null,
	hostname text not null,
	status text not null check (status in ('pending', 'command_published', 'accepted_by_rack', 'running', 'completed', 'failed', 'expired', 'canceled')),
	failure_reason text,
	correlation_id text not null,
	command_message_id text,
	requested_at timestamptz not null,
	started_at timestamptz,
	completed_at timestamptz,
	expires_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create unique index provisioning_jobs_one_active_per_server
	on provisioning_jobs (server_id)
	where status in ('pending', 'command_published', 'accepted_by_rack', 'running');

create table provisioning_job_events (
	id uuid primary key default gen_random_uuid(),
	provisioning_job_id uuid not null references provisioning_jobs(id) on delete cascade,
	event_type text not null,
	message text not null,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now()
);

create index provisioning_job_events_job_created_idx on provisioning_job_events (provisioning_job_id, created_at);

create table rack_messages (
	id uuid primary key default gen_random_uuid(),
	message_id text not null unique,
	direction text not null check (direction in ('central_to_rack', 'rack_to_central')),
	rack_id text not null,
	server_id uuid,
	job_id uuid,
	message_type text not null,
	status text not null check (status in ('received', 'processed', 'ignored_duplicate', 'failed', 'expired')),
	payload jsonb not null,
	processed_at timestamptz,
	created_at timestamptz not null default now()
);

create index rack_messages_rack_created_idx on rack_messages (rack_id, created_at desc);

insert into role_permissions (role_id, permission)
select r.id, p.permission
from roles r
join (
	values
		('Owner', 'servers.provision'),
		('Admin', 'servers.provision'),
		('Infrastructure', 'servers.provision')
) as p(role_name, permission) on p.role_name = r.name
on conflict (role_id, permission) do nothing;

-- +goose Down
delete from role_permissions where permission = 'servers.provision';
drop table if exists rack_messages;
drop table if exists provisioning_job_events;
drop table if exists provisioning_jobs;
drop table if exists servers;
drop table if exists rack_agents;
drop table if exists racks;
