-- +goose Up
create table locations (
	id uuid primary key default gen_random_uuid(),
	code text not null unique,
	name text not null,
	city text not null default '',
	region text not null default '',
	country text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

insert into locations (code, name)
select distinct
	lower(regexp_replace(r.location, '[^a-zA-Z0-9]+', '-', 'g')) as code,
	r.location as name
from racks r
where trim(r.location) <> ''
on conflict (code) do nothing;

alter table racks
	add column location_id uuid references locations(id) on delete set null;

update racks r
set location_id = l.id
from locations l
where l.name = r.location and r.location <> '';

create table network_switches (
	id uuid primary key default gen_random_uuid(),
	label text not null,
	ip_address inet not null,
	location_id uuid not null references locations(id),
	rack_id text references racks(id) on delete set null,
	management_ip inet,
	vendor text not null default '',
	model text not null default '',
	serial_number text not null default '',
	port_count integer not null default 0 check (port_count >= 0),
	default_port_speed text not null default '',
	status text not null default 'active' check (status in ('active', 'maintenance', 'retired')),
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (location_id, label),
	unique (ip_address)
);

create index network_switches_location_idx on network_switches (location_id);

create table edge_routers (
	id uuid primary key default gen_random_uuid(),
	label text not null,
	ip_address inet not null,
	location_id uuid not null references locations(id),
	management_ip inet,
	vendor text not null default '',
	model text not null default '',
	serial_number text not null default '',
	asn integer,
	upstream_isps jsonb not null default '[]'::jsonb,
	port_count integer not null default 0 check (port_count >= 0),
	port_speed text not null default '',
	status text not null default 'active' check (status in ('active', 'maintenance', 'retired')),
	notes text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (location_id, label),
	unique (ip_address)
);

create index edge_routers_location_idx on edge_routers (location_id);

create table server_network_interfaces (
	id uuid primary key default gen_random_uuid(),
	server_id uuid not null references servers(id) on delete cascade,
	switch_id uuid references network_switches(id) on delete set null,
	label text not null default '',
	mac_address macaddr not null,
	ip_address inet,
	gateway inet,
	subnet_mask text,
	switch_port text not null default '',
	vlan_id integer,
	is_primary boolean not null default false,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (server_id, mac_address)
);

create index server_network_interfaces_server_idx on server_network_interfaces (server_id);
create index server_network_interfaces_switch_idx on server_network_interfaces (switch_id);
create index server_network_interfaces_mac_idx on server_network_interfaces (mac_address);

insert into platform_role_permissions (role_id, permission)
select r.id, p.permission
from platform_roles r
join (
	values
		('SuperAdmin', 'platform.network.view'),
		('SuperAdmin', 'platform.network.manage'),
		('InfrastructureOps', 'platform.network.view'),
		('InfrastructureOps', 'platform.network.manage'),
		('SupportOps', 'platform.network.view'),
		('ReadOnlyOps', 'platform.network.view')
) as p(role_name, permission) on p.role_name = r.name
on conflict (role_id, permission) do nothing;

-- +goose Down
delete from platform_role_permissions where permission in ('platform.network.view', 'platform.network.manage');
drop table if exists server_network_interfaces;
drop table if exists edge_routers;
drop table if exists network_switches;
alter table racks drop column if exists location_id;
drop table if exists locations;
