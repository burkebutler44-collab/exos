-- +goose Up
create table wireguard_gateways (
	id text primary key,
	name text not null,
	interface_name text not null,
	public_key text not null default '',
	endpoint text not null default '',
	management_cidr cidr not null,
	control_plane_allowed_ips text[] not null default '{}'::text[],
	node_name text not null default '',
	status text not null default 'active' check (status in ('active', 'maintenance', 'disabled')),
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (interface_name, node_name)
);

create table hypervisor_wireguard_peers (
	id uuid primary key default gen_random_uuid(),
	hypervisor_id text not null references hypervisors(id) on delete cascade,
	gateway_id text not null references wireguard_gateways(id) on delete restrict,
	wireguard_public_key text not null unique,
	wireguard_management_ip inet not null,
	allowed_ips text[] not null,
	endpoint text,
	desired_state text not null default 'present' check (desired_state in ('present', 'absent')),
	actual_state text not null default 'pending' check (actual_state in ('pending', 'applied', 'removed', 'failed')),
	last_handshake_at timestamptz,
	last_reconciled_at timestamptz,
	revoked_at timestamptz,
	error_message text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (hypervisor_id),
	unique (gateway_id, wireguard_management_ip)
);

create index hypervisor_wireguard_peers_gateway_state_idx on hypervisor_wireguard_peers (gateway_id, desired_state, actual_state);
create index hypervisor_wireguard_peers_handshake_idx on hypervisor_wireguard_peers (gateway_id, last_handshake_at desc);

create table hypervisor_agent_credentials (
	id uuid primary key default gen_random_uuid(),
	hypervisor_id text not null references hypervisors(id) on delete cascade,
	certificate_serial text not null unique,
	subject text not null,
	status text not null default 'active' check (status in ('active', 'revoked', 'expired')),
	issued_at timestamptz not null default now(),
	expires_at timestamptz not null,
	revoked_at timestamptz,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table hypervisor_conversions (
	id uuid primary key default gen_random_uuid(),
	server_id uuid not null references servers(id) on delete restrict,
	hypervisor_id text not null references hypervisors(id) on delete restrict,
	gateway_id text references wireguard_gateways(id) on delete restrict,
	status text not null default 'pending' check (
		status in (
			'pending',
			'allocating_network',
			'provisioning',
			'waiting_for_agent',
			'ready',
			'failed',
			'rolled_back',
			'decommissioning',
			'decommissioned'
		)
	),
	failure_reason text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	requested_by_user_id uuid references users(id) on delete set null,
	started_at timestamptz not null default now(),
	completed_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create unique index hypervisor_conversions_active_server_uidx
	on hypervisor_conversions (server_id)
	where status not in ('failed', 'rolled_back', 'decommissioned');

create unique index hypervisor_conversions_active_hypervisor_uidx
	on hypervisor_conversions (hypervisor_id)
	where status not in ('failed', 'rolled_back', 'decommissioned');

create index hypervisor_conversions_status_idx
	on hypervisor_conversions (status, created_at desc);

-- +goose Down
drop table if exists hypervisor_conversions;
drop table if exists hypervisor_agent_credentials;
drop table if exists hypervisor_wireguard_peers;
drop table if exists wireguard_gateways;
