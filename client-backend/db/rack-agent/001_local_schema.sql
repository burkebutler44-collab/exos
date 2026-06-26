create table if not exists local_jobs (
	id text primary key,
	central_job_id text not null,
	command_message_id text not null unique,
	rack_id text not null,
	server_id text not null,
	status text not null,
	last_step text not null,
	failure_reason text,
	started_at text,
	completed_at text,
	created_at text not null,
	updated_at text not null
);

create index if not exists local_jobs_central_job_idx on local_jobs (central_job_id);

create table if not exists local_processed_messages (
	id text primary key,
	message_id text not null unique,
	message_type text not null,
	processed_at text not null,
	result_status text not null,
	payload_hash text not null,
	created_at text not null
);
