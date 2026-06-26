-- +goose Up
insert into projects (organization_id, name, slug)
select o.id, 'Default', 'default'
from organizations o
where not exists (
	select 1
	from projects p
	where p.organization_id = o.id and p.slug = 'default'
)
on conflict (organization_id, slug) do nothing;

insert into audit_log (organization_id, actor_user_id, action, entity_type, entity_id, metadata)
select
	p.organization_id,
	o.created_by_user_id,
	'project.created',
	'project',
	p.id,
	jsonb_build_object(
		'slug', p.slug,
		'default_project', true,
		'source', 'migration'
	)
from projects p
join organizations o on o.id = p.organization_id
where p.slug = 'default'
	and not exists (
		select 1
		from audit_log a
		where a.organization_id = p.organization_id
			and a.entity_type = 'project'
			and a.entity_id = p.id
			and a.metadata->>'default_project' = 'true'
	);

-- +goose Down
select 1;
