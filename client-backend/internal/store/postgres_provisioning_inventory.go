package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

func (s *PostgresStore) GetProvisioningServerInventory(ctx context.Context, organizationID, serverID uuid.UUID) (ProvisioningServerInventory, error) {
	var item ProvisioningServerInventory
	var metadata []byte
	row := s.pool.QueryRow(ctx, `
		select
			s.id,
			s.organization_id,
			s.project_id,
			s.rack_id,
			lower(coalesce(nullif(r.metadata->>'code', ''), nullif(r.location, ''), s.rack_id)) as rack_location,
			s.status,
			s.bmc_address,
			coalesce(nullif(ni.mac_address::text, ''), s.mac_address) as mac_address,
			ni.ip_address::text,
			ni.gateway::text,
			ni.subnet_mask,
			coalesce(nullif(s.metadata->>'disk_name', ''), '/dev/nvme0n1') as disk_name,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as hostname,
			s.metadata
		from servers s
		join racks r on r.id = s.rack_id
		left join lateral (
			select mac_address, ip_address, gateway, subnet_mask
			from server_network_interfaces
			where server_id = s.id
			order by is_primary desc, created_at asc
			limit 1
		) ni on true
		where s.id = $1 and s.organization_id = $2`,
		serverID, organizationID,
	)
	if err := row.Scan(&item.ID, &item.OrganizationID, &item.ProjectID, &item.RackID, &item.RackLocation, &item.Status, &item.BMCAddress, &item.MACAddress, &item.IPAddress, &item.Gateway, &item.SubnetMask, &item.DiskName, &item.Hostname, &metadata); err != nil {
		return ProvisioningServerInventory{}, mapNoRows(err)
	}
	item.Metadata = map[string]any{}
	_ = json.Unmarshal(metadata, &item.Metadata)
	return item, nil
}
