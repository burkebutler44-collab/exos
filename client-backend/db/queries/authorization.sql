-- name: HasOrganizationPermission :one
select exists (
	select 1
	from organization_memberships m
	join role_permissions rp on rp.role_id = m.role_id
	where m.organization_id = $1
		and m.user_id = $2
		and m.status = 'active'
		and rp.permission = $3
) as has_permission;

-- name: CountActiveOwners :one
select count(*)
from organization_memberships m
join roles r on r.id = m.role_id
where m.organization_id = $1 and m.status = 'active' and r.name = 'Owner';
