-- +goose Up
with owner_role as (
	select id
	from roles
	where organization_id is null and name = 'Owner'
	limit 1
),
users_without_org as (
	select u.id, u.email
	from users u
	where not exists (
		select 1
		from organization_memberships m
		where m.user_id = u.id and m.status = 'active'
	)
),
created_organizations as (
	insert into organizations (name, slug, created_by_user_id)
	select
		'Default',
		'default-' || left(replace(u.id::text, '-', ''), 8),
		u.id
	from users_without_org u
	on conflict (slug) do nothing
	returning id, name, slug, created_by_user_id
)
insert into organization_memberships (organization_id, user_id, role_id, status, joined_at)
select o.id, o.created_by_user_id, r.id, 'active', now()
from created_organizations o
cross join owner_role r
on conflict (organization_id, user_id) do nothing;

insert into billing_profiles (organization_id, billing_email, company_name)
select o.id, u.email, o.name
from organizations o
join users u on u.id = o.created_by_user_id
where o.slug = 'default-' || left(replace(u.id::text, '-', ''), 8)
on conflict (organization_id) do nothing;

insert into billing_accounts (organization_id, billing_email, currency, status, payment_terms)
select o.id, u.email, 'usd', 'active', 'prepaid'
from organizations o
join users u on u.id = o.created_by_user_id
where o.slug = 'default-' || left(replace(u.id::text, '-', ''), 8)
on conflict (organization_id) do nothing;

insert into audit_log (organization_id, actor_user_id, action, entity_type, entity_id, metadata)
select
	o.id,
	o.created_by_user_id,
	'organization.created',
	'organization',
	o.id,
	jsonb_build_object(
		'slug', o.slug,
		'default_organization', true,
		'source', 'migration'
	)
from organizations o
join users u on u.id = o.created_by_user_id
where o.slug = 'default-' || left(replace(u.id::text, '-', ''), 8)
	and not exists (
		select 1
		from audit_log a
		where a.organization_id = o.id
			and a.action = 'organization.created'
			and a.metadata->>'default_organization' = 'true'
	);

-- +goose Down
select 1;
