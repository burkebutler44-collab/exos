-- +goose Up
alter table wireguard_gateways
	add column if not exists tunnel_address cidr;

insert into wireguard_gateways (
	id,
	name,
	interface_name,
	public_key,
	endpoint,
	tunnel_address,
	management_cidr,
	control_plane_allowed_ips,
	status,
	metadata
)
values (
	'vpn-1',
	'VPN 1',
	'wg0',
	'',
	'vpn-1.exos.tech:51820',
	'172.200.0.1/32',
	'172.200.1.0/24',
	array['172.200.0.1/32'],
	'active',
	jsonb_build_object(
		'source', 'seed',
		'description', 'Default public WireGuard gateway. Public key must be filled in before provisioning hypervisors.'
	)
)
on conflict (id) do update
set
	endpoint = excluded.endpoint,
	tunnel_address = coalesce(wireguard_gateways.tunnel_address, excluded.tunnel_address),
	management_cidr = excluded.management_cidr,
	control_plane_allowed_ips = excluded.control_plane_allowed_ips,
	status = excluded.status,
	metadata = wireguard_gateways.metadata || excluded.metadata,
	updated_at = now();

-- +goose Down
delete from wireguard_gateways
where id = 'vpn-1'
	and metadata->>'source' = 'seed'
	and public_key = '';

alter table wireguard_gateways
	drop column if exists tunnel_address;
