-- +goose Up
create table hardware_option_inventory (
	id uuid primary key default gen_random_uuid(),
	option_type text not null check (option_type in ('memory', 'storage', 'network', 'gpu', 'custom')),
	label text not null,
	description text not null default '',
	unit text not null default 'each',
	value_text text not null default '',
	value_gb integer,
	price_delta_cents bigint not null default 0,
	hourly_price_delta_cents bigint not null default 0,
	quarterly_price_delta_cents bigint not null default 0,
	yearly_price_delta_cents bigint not null default 0,
	currency text not null default 'usd',
	quantity_available integer not null default 0 check (quantity_available >= 0),
	fulfillment_mode text not null default 'available' check (fulfillment_mode in ('available', 'requires_install', 'special_order', 'manual')),
	estimated_ready_min_hours integer not null default 0 check (estimated_ready_min_hours >= 0),
	estimated_ready_max_hours integer not null default 0 check (estimated_ready_max_hours >= 0),
	location_id uuid references locations(id) on delete set null,
	hardware_profile_name text not null default '',
	cpu_model text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	active boolean not null default true,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index hardware_option_inventory_active_idx on hardware_option_inventory (active, option_type);
create index hardware_option_inventory_location_idx on hardware_option_inventory (location_id);
create index hardware_option_inventory_profile_idx on hardware_option_inventory (lower(hardware_profile_name), lower(cpu_model));

create table server_catalog_option_overrides (
	id uuid primary key default gen_random_uuid(),
	server_id uuid not null references servers(id) on delete cascade,
	hardware_option_id uuid not null references hardware_option_inventory(id) on delete cascade,
	compatible boolean not null default true,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	unique (server_id, hardware_option_id)
);

create index server_catalog_option_overrides_server_idx on server_catalog_option_overrides (server_id, compatible);

-- +goose Down
drop table if exists server_catalog_option_overrides;
drop table if exists hardware_option_inventory;
