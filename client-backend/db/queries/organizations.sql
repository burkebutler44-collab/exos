-- name: ListOrganizationsForUser :many
select o.id, o.name, o.slug, o.created_by_user_id, o.created_at, o.updated_at
from organizations o
join organization_memberships m on m.organization_id = o.id
where m.user_id = $1 and m.status = 'active'
order by o.created_at desc;

-- name: GetOrganizationForUser :one
select o.id, o.name, o.slug, o.created_by_user_id, o.created_at, o.updated_at
from organizations o
join organization_memberships m on m.organization_id = o.id
where o.id = $1 and m.user_id = $2 and m.status = 'active';

-- name: CreateOrganization :one
insert into organizations (name, slug, created_by_user_id)
values ($1, $2, $3)
returning id, name, slug, created_by_user_id, created_at, updated_at;

-- name: UpdateOrganization :one
update organizations
set name = $2, slug = $3, updated_at = now()
where id = $1
returning id, name, slug, created_by_user_id, created_at, updated_at;
