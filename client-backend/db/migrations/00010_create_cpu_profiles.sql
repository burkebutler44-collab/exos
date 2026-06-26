-- +goose Up
create table if not exists cpu_profiles (
	id uuid primary key default gen_random_uuid(),
	name text not null unique,
	vendor text not null default '',
	model text not null,
	socket_count integer not null default 1,
	core_count integer not null,
	thread_count integer not null default 0,
	base_clock_ghz numeric(5,2),
	boost_clock_ghz numeric(5,2),
	architecture text not null default 'x86_64',
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

insert into cpu_profiles (name, vendor, model, socket_count, core_count, thread_count, base_clock_ghz, boost_clock_ghz, architecture, metadata)
values
	('AMD EPYC 9554P', 'AMD', 'EPYC 9554P', 1, 64, 128, 3.10, 3.75, 'x86_64', '{"family":"epyc","generation":"genoa"}'::jsonb),
	('AMD EPYC 9454P', 'AMD', 'EPYC 9454P', 1, 48, 96, 2.75, 3.80, 'x86_64', '{"family":"epyc","generation":"genoa"}'::jsonb),
	('AMD Ryzen 9 9950X', 'AMD', 'Ryzen 9 9950X', 1, 16, 32, 4.30, 5.70, 'x86_64', '{"family":"ryzen","generation":"zen5"}'::jsonb),
	('Intel Xeon Gold 6530', 'Intel', 'Xeon Gold 6530', 2, 64, 128, 2.10, 4.00, 'x86_64', '{"family":"xeon","generation":"emerald-rapids"}'::jsonb)
on conflict (name) do update
set
	vendor = excluded.vendor,
	model = excluded.model,
	socket_count = excluded.socket_count,
	core_count = excluded.core_count,
	thread_count = excluded.thread_count,
	base_clock_ghz = excluded.base_clock_ghz,
	boost_clock_ghz = excluded.boost_clock_ghz,
	architecture = excluded.architecture,
	metadata = cpu_profiles.metadata || excluded.metadata,
	updated_at = now();

-- +goose Down
drop table if exists cpu_profiles;
