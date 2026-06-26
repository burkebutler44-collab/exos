-- +goose Up
insert into os_images (name, slug, version, family, architecture, enabled, is_default, tinkerbell_template_ref, metadata)
values (
	'Exos Hypervisor Ubuntu 24.04',
	'exos-hypervisor-ubuntu-24',
	'24.04',
	'exos-hypervisor',
	'x86_64',
	true,
	false,
	'exos-hypervisor-ubuntu-24',
	jsonb_build_object(
		'artifact_name', 'exos-hypervisor-ubuntu-24',
		'artifact_file', 'exos-hypervisor-ubuntu-24.raw.gz',
		'display_name', 'Exos Hypervisor Ubuntu 24.04',
		'usage', 'hypervisor'
	)
)
on conflict (slug) do update
set
	name = excluded.name,
	version = excluded.version,
	family = excluded.family,
	architecture = excluded.architecture,
	enabled = excluded.enabled,
	is_default = excluded.is_default,
	tinkerbell_template_ref = excluded.tinkerbell_template_ref,
	metadata = os_images.metadata || excluded.metadata,
	updated_at = now();

-- +goose Down
delete from os_images where slug = 'exos-hypervisor-ubuntu-24';
