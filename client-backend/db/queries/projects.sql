-- name: CreateProject :one
insert into projects (organization_id, name, slug)
values ($1, $2, $3)
returning id, organization_id, name, slug, created_at, updated_at;

-- name: ListProjects :many
select id, organization_id, name, slug, created_at, updated_at
from projects
where organization_id = $1
order by name;

-- name: GetProject :one
select id, organization_id, name, slug, created_at, updated_at
from projects
where organization_id = $1 and id = $2;
