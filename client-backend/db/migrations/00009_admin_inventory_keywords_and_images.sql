-- +goose Up
alter table locations
	add column if not exists keyword text;

update locations
set keyword = upper(code)
where keyword is null or trim(keyword) = '';

update locations l
set keyword = case
		when lower(l.code) = 'ny' or not exists (select 1 from locations existing where lower(existing.code) = 'ny' and existing.id <> l.id) then 'NY'
		else coalesce(nullif(l.keyword, ''), upper(l.code))
	end,
	code = case
		when lower(l.code) = 'ny' or not exists (select 1 from locations existing where lower(existing.code) = 'ny' and existing.id <> l.id) then 'ny'
		else l.code
	end
where lower(l.code) in ('new-jersey', 'nj', 'ny') or lower(l.name) in ('new jersey', 'ny', 'new-york');

insert into locations (code, keyword, name, city, region, country)
values ('ny', 'NY', 'New Jersey', '', 'NJ', 'US')
on conflict (code) do update
set
	keyword = excluded.keyword,
	name = excluded.name,
	region = excluded.region,
	country = excluded.country,
	updated_at = now();

create unique index if not exists locations_keyword_unique_idx
	on locations (keyword)
	where keyword is not null and trim(keyword) <> '';

insert into os_images (name, slug, version, family, architecture, enabled, is_default, tinkerbell_template_ref, metadata)
values (
	'Ubuntu 24.04 LTS',
	'ubuntu-24',
	'24.04',
	'ubuntu',
	'x86_64',
	true,
	true,
	'ubuntu-24',
	jsonb_build_object(
		'artifact_name', 'noble-server-cloudimg-arm64',
		'artifact_file', 'noble-server-cloudimg-arm64.raw.gz',
		'display_name', 'Ubuntu 24.04 Noble'
	)
)
on conflict (slug) do update
set
	name = excluded.name,
	version = excluded.version,
	family = excluded.family,
	architecture = excluded.architecture,
	enabled = excluded.enabled,
	tinkerbell_template_ref = excluded.tinkerbell_template_ref,
	metadata = os_images.metadata || excluded.metadata,
	updated_at = now();

-- +goose Down
delete from os_images where slug = 'ubuntu-24';
drop index if exists locations_keyword_unique_idx;
alter table locations drop column if exists keyword;
