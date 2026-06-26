-- +goose Up
create table hypervisor_commands (
	id uuid primary key default gen_random_uuid(),
	command_id text not null unique,
	hypervisor_id text not null references hypervisors(id) on delete cascade,
	command_type text not null,
	status text not null default 'pending' check (status in ('pending', 'sent', 'succeeded', 'failed', 'expired')),
	payload jsonb not null default '{}'::jsonb,
	result jsonb not null default '{}'::jsonb,
	error_message text not null default '',
	created_at timestamptz not null default now(),
	sent_at timestamptz,
	completed_at timestamptz,
	updated_at timestamptz not null default now()
);

create index hypervisor_commands_hypervisor_status_idx on hypervisor_commands (hypervisor_id, status, created_at desc);
create index hypervisor_commands_created_idx on hypervisor_commands (created_at desc);

-- +goose Down
drop table if exists hypervisor_commands;
