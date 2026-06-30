-- +goose Up
create table server_families (
	id uuid primary key default gen_random_uuid(),
	display_name text not null,
	slug text not null unique,
	cpu_manufacturer text not null default '',
	cpu_model text not null,
	core_count integer not null default 0 check (core_count >= 0),
	thread_count integer not null default 0 check (thread_count >= 0),
	base_clock_ghz numeric(5, 2),
	boost_clock_ghz numeric(5, 2),
	generation text not null default '',
	workload_category text not null default 'cpu',
	description text not null default '',
	feature_badges jsonb not null default '[]'::jsonb,
	active boolean not null default true,
	display_order integer not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

alter table servers add column server_family_id uuid references server_families(id) on delete restrict;
alter table servers add column reservation_expires_at timestamptz;

with raw_source as (
	select
		coalesce(nullif(metadata->>'cpu_model', ''), nullif(metadata->>'cpu', ''), nullif(metadata->>'processor', ''), nullif(metadata->>'hardware_profile_name', ''), nullif(metadata->>'sku', ''), 'Dedicated server') as cpu_model,
		case
			when coalesce(nullif(metadata->>'core_count', ''), nullif(metadata->>'cores', ''), nullif(metadata->>'cpu_cores', ''), '') ~ '^[0-9]+$'
			then coalesce(nullif(metadata->>'core_count', ''), nullif(metadata->>'cores', ''), nullif(metadata->>'cpu_cores', ''))::integer
			else 0
		end as core_count,
		case
			when coalesce(nullif(metadata->>'thread_count', ''), nullif(metadata->>'threads', ''), '') ~ '^[0-9]+$'
			then coalesce(nullif(metadata->>'thread_count', ''), nullif(metadata->>'threads', ''))::integer
			when coalesce(nullif(metadata->>'core_count', ''), nullif(metadata->>'cores', ''), nullif(metadata->>'cpu_cores', ''), '') ~ '^[0-9]+$'
			then coalesce(nullif(metadata->>'core_count', ''), nullif(metadata->>'cores', ''), nullif(metadata->>'cpu_cores', ''))::integer * 2
			else 0
		end as thread_count
	from servers
),
source as (
	select cpu_model, core_count, max(thread_count) as thread_count
	from raw_source
	group by cpu_model, core_count
)
insert into server_families (display_name, slug, cpu_manufacturer, cpu_model, core_count, thread_count)
select
	cpu_model,
	trim(both '-' from regexp_replace(lower(cpu_model), '[^a-z0-9]+', '-', 'g')) || '-' || core_count || '-' || substr(md5(cpu_model || ':' || core_count), 1, 8),
	case
		when lower(cpu_model) like '%amd%' or lower(cpu_model) like '%epyc%' or lower(cpu_model) like '%ryzen%' then 'AMD'
		when lower(cpu_model) like '%intel%' or lower(cpu_model) like '%xeon%' then 'Intel'
		when lower(cpu_model) like '%ampere%' or lower(cpu_model) like '%arm%' then 'ARM'
		else ''
	end,
	cpu_model,
	core_count,
	thread_count
from source;

update servers s
set server_family_id = sf.id
from server_families sf
where sf.cpu_model = coalesce(nullif(s.metadata->>'cpu_model', ''), nullif(s.metadata->>'cpu', ''), nullif(s.metadata->>'processor', ''), nullif(s.metadata->>'hardware_profile_name', ''), nullif(s.metadata->>'sku', ''), 'Dedicated server')
	and sf.core_count = case
		when coalesce(nullif(s.metadata->>'core_count', ''), nullif(s.metadata->>'cores', ''), nullif(s.metadata->>'cpu_cores', ''), '') ~ '^[0-9]+$'
		then coalesce(nullif(s.metadata->>'core_count', ''), nullif(s.metadata->>'cores', ''), nullif(s.metadata->>'cpu_cores', ''))::integer
		else 0
	end;

alter table servers alter column server_family_id set not null;
create index servers_server_family_inventory_idx on servers (server_family_id, status, provisionable);

-- +goose Down
drop index if exists servers_server_family_inventory_idx;
alter table servers drop column if exists server_family_id;
alter table servers drop column if exists reservation_expires_at;
drop table if exists server_families;
