-- name: UpsertUser :one
insert into users (auth0_sub, email, name)
values ($1, $2, $3)
on conflict (auth0_sub) do update
set email = excluded.email,
	name = excluded.name,
	updated_at = now()
returning id, auth0_sub, email, name, created_at, updated_at;

-- name: GetUserByAuth0Sub :one
select id, auth0_sub, email, name, created_at, updated_at
from users
where auth0_sub = $1;
