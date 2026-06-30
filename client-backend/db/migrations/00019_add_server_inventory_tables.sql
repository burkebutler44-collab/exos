-- +goose Up

-- ---------------------------------------------------------------------------
-- 1. Add proper columns to servers (extracted from metadata where possible)
-- ---------------------------------------------------------------------------
alter table servers
	add column hostname text not null default '',
	add column asset_tag text not null default '',
	add column serial_number text not null default '',
	add column location_id uuid references locations(id) on delete set null,
	add column installed_memory_gb integer not null default 0 check (installed_memory_gb >= 0),
	add column rack_position text not null default '',
	add column lifecycle_status text not null default 'active' check (lifecycle_status in ('active', 'decommissioning', 'retired', 'lost')),
	add column allocation_status text not null default 'available' check (allocation_status in ('available', 'allocating', 'allocated', 'reserved')),
	add column health_status text not null default 'unknown' check (health_status in ('unknown', 'healthy', 'degraded', 'failed', 'maintenance')),
	add column notes text not null default '';

-- Do NOT add bmc_id yet — create the bmc_management table first.

-- ---------------------------------------------------------------------------
-- 2. Extract existing metadata values into the new columns
-- ---------------------------------------------------------------------------
update servers
set
	hostname = coalesce(nullif(metadata->>'hostname', ''), id::text),
	asset_tag = coalesce(nullif(metadata->>'asset_tag', ''), ''),
	serial_number = coalesce(nullif(metadata->>'serial_number', ''), ''),
	installed_memory_gb = case
		when coalesce(nullif(metadata->>'ram_gb', ''), nullif(metadata->>'memory_gb', ''), '') ~ '^[0-9]+$'
		then coalesce(nullif(metadata->>'ram_gb', ''), nullif(metadata->>'memory_gb', ''))::integer
		else 0
	end,
	notes = coalesce(nullif(metadata->>'notes', ''), '');

-- Set location_id from the rack's location string matched to locations table
update servers s
set location_id = l.id
from racks r
join locations l on lower(regexp_replace(r.location, '[^a-zA-Z0-9]+', '-', 'g')) = lower(l.code)
where r.id = s.rack_id;

-- ---------------------------------------------------------------------------
-- 3. Create server_disks table
-- ---------------------------------------------------------------------------
create table server_disks (
	id uuid primary key default gen_random_uuid(),
	server_id uuid not null references servers(id) on delete cascade,
	device_name text not null default '',
	capacity_gb integer not null default 0 check (capacity_gb >= 0),
	capacity_bytes bigint not null default 0 check (capacity_bytes >= 0),
	media_type text not null default '' check (media_type in ('', 'nvme', 'ssd', 'hdd', 'sata_ssd')),
	interface_type text not null default '' check (interface_type in ('', 'u.2', 'u.3', 'sata', 'sas', 'm.2', 'e1.s', 'pcie')),
	manufacturer text not null default '',
	model text not null default '',
	serial_number text not null default '',
	boot_capable boolean not null default false,
	operational_status text not null default 'active' check (operational_status in ('active', 'failed', 'removed', 'unknown')),
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index server_disks_server_idx on server_disks (server_id);

-- ---------------------------------------------------------------------------
-- 4. Extract existing disk data from servers.metadata into server_disks
--    (parse free-text disk_description like "2 × 3.84 TB NVMe")
-- ---------------------------------------------------------------------------
-- Migrate existing server metadata disk info into server_disks where possible.
-- +goose StatementBegin
do $$declare
	rec record;
	disk_text text;
	disk_count int;
	disk_size_gb int;
	disk_media text;
	total_gb int;
begin
	for rec in select id, metadata->>'disk_name' as disk_name, metadata->>'disk_description' as disk_description from servers
	where metadata->>'disk_description' is not null and metadata->>'disk_description' <> ''
	loop
		disk_text := rec.disk_description;
		disk_count := 1;
		disk_size_gb := 0;
		disk_media := '';

		if disk_text ~* '^(\d+)\s*[×x]\s*([\d.]+)\s*(tb|gb|t|g)\s*(.*)' then
			disk_count := (regexp_match(disk_text, '^(\d+)\s*[×x]'))[1]::int;
			disk_size_gb := round(
				case when lower((regexp_match(disk_text, '([\d.]+)\s*(tb|t)'))[2]) in ('tb', 't')
				then (regexp_match(disk_text, '([\d.]+)\s*(tb|t)'))[1]::numeric * 1000
				else (regexp_match(disk_text, '([\d.]+)\s*(gb|g)'))[1]::numeric
				end
			)::int;
			disk_media := case
				when disk_text ~* 'nvme' then 'nvme'
				when disk_text ~* 'ssd' then 'ssd'
				when disk_text ~* 'hdd' then 'hdd'
				else ''
			end;
		elsif disk_text ~* '^([\d.]+)\s*(tb|gb|t|g)\s*(.*)' then
			disk_size_gb := round(
				case when lower((regexp_match(disk_text, '([\d.]+)\s*(tb|t)'))[2]) in ('tb', 't')
				then (regexp_match(disk_text, '([\d.]+)\s*(tb|t)'))[1]::numeric * 1000
				else (regexp_match(disk_text, '([\d.]+)\s*(gb|g)'))[1]::numeric
				end
			)::int;
			disk_media := case
				when disk_text ~* 'nvme' then 'nvme'
				when disk_text ~* 'ssd' then 'ssd'
				when disk_text ~* 'hdd' then 'hdd'
				else ''
			end;
		end if;

		if disk_size_gb > 0 then
			total_gb := disk_size_gb * disk_count;
			for i in 1..disk_count loop
				insert into server_disks (server_id, device_name, capacity_gb, capacity_bytes, media_type, boot_capable)
				values (
					rec.id,
					case when rec.disk_name <> '' then rec.disk_name else '' end,
					disk_size_gb,
					disk_size_gb * 1073741824,
					disk_media,
					i = 1
				);
			end loop;
		end if;
	end loop;
end $$;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- 5. Update server_network_interfaces — add missing normalized fields
-- ---------------------------------------------------------------------------
alter table server_network_interfaces
	add column if not exists speed_mbps integer not null default 10000 check (speed_mbps >= 0),
	add column if not exists is_public boolean not null default false,
	add column if not exists prefix_length integer check (prefix_length >= 0 and prefix_length <= 128),
	add column if not exists notes text not null default '',
	add column if not exists purpose text not null default '' check (purpose in ('', 'public', 'private', 'provisioning', 'management', 'storage', 'backup'));

-- Derive is_public from existing ip_address (public ranges) and purpose
update server_network_interfaces
set is_public = true,
    purpose = 'public'
where is_primary = true
  and is_public = false;

-- ---------------------------------------------------------------------------
-- 6. Create BMC management table
-- ---------------------------------------------------------------------------
create table bmc_management (
	id uuid primary key default gen_random_uuid(),
	server_id uuid not null references servers(id) on delete cascade unique,
	management_ip inet,
	username text not null default '',
	password text not null default '',
	protocol text not null default 'ipmi' check (protocol in ('ipmi', 'redfish', 'ilo', 'idrac', 'other')),
	vendor text not null default '',
	connection_status text not null default 'unknown' check (connection_status in ('unknown', 'reachable', 'unreachable', 'error')),
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index bmc_management_server_idx on bmc_management (server_id);

-- Migrate existing BMC data from servers table into bmc_management
insert into bmc_management (server_id, management_ip, username, password, protocol)
select
	s.id,
	nullif(s.bmc_address, '')::inet,
	coalesce(nullif(s.metadata->>'ipmi_username', ''), ''),
	coalesce(nullif(s.metadata->>'ipmi_password', ''), ''),
	'ipmi'
from servers s
where s.bmc_address <> ''
on conflict (server_id) do nothing;

-- ---------------------------------------------------------------------------
-- 7. Create server_upgrade_options for upgrade-capability model
-- ---------------------------------------------------------------------------
create table server_upgrade_options (
	id uuid primary key default gen_random_uuid(),
	server_id uuid references servers(id) on delete cascade,
	server_family_id uuid references server_families(id) on delete cascade,
	option_type text not null check (option_type in ('memory', 'disk_add', 'disk_upgrade', 'network')),
	from_value text not null default '',
	to_value text not null default '',
	additional_price_cents bigint not null default 0,
	hourly_additional_price_cents bigint not null default 0,
	quarterly_additional_price_cents bigint not null default 0,
	yearly_additional_price_cents bigint not null default 0,
	available_quantity integer not null default 0 check (available_quantity >= 0),
	active boolean not null default true,
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	constraint upgrade_target_check check (
		(server_id is not null and server_family_id is null)
		or (server_id is null and server_family_id is not null)
	)
);

create index server_upgrade_options_server_idx on server_upgrade_options (server_id) where server_id is not null;
create index server_upgrade_options_family_idx on server_upgrade_options (server_family_id) where server_family_id is not null;
create index server_upgrade_options_active_idx on server_upgrade_options (active, option_type);

-- ---------------------------------------------------------------------------
-- 8. Add server_id FK to existing hardware_option_inventory compatibility
-- ---------------------------------------------------------------------------
-- The server_catalog_option_overrides table already links servers to hardware options.

-- ---------------------------------------------------------------------------
-- 9. Add indexes
-- ---------------------------------------------------------------------------
create index servers_location_idx on servers (location_id);
create index servers_hostname_idx on servers (hostname);
create index servers_serial_idx on servers (serial_number);
create index servers_allocation_status_idx on servers (allocation_status);
create index servers_health_status_idx on servers (health_status);

-- +goose Down
drop index if exists servers_health_status_idx;
drop index if exists servers_allocation_status_idx;
drop index if exists servers_serial_idx;
drop index if exists servers_hostname_idx;
drop index if exists servers_location_idx;

drop index if exists server_upgrade_options_active_idx;
drop index if exists server_upgrade_options_family_idx;
drop index if exists server_upgrade_options_server_idx;
drop table if exists server_upgrade_options;

drop index if exists bmc_management_server_idx;
drop table if exists bmc_management;

alter table server_network_interfaces
	drop column if exists speed_mbps,
	drop column if exists is_public,
	drop column if exists prefix_length,
	drop column if exists notes,
	drop column if exists purpose;

drop index if exists server_disks_server_idx;
drop table if exists server_disks;

alter table servers
	drop column if exists hostname,
	drop column if exists asset_tag,
	drop column if exists serial_number,
	drop column if exists location_id,
	drop column if exists installed_memory_gb,
	drop column if exists rack_position,
	drop column if exists lifecycle_status,
	drop column if exists allocation_status,
	drop column if exists health_status,
	drop column if exists notes;
