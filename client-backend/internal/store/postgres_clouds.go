package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) ListClouds(ctx context.Context, organizationID uuid.UUID) ([]Cloud, error) {
	rows, err := s.pool.Query(ctx, cloudSelectSQL()+`
		where c.organization_id = $1 and c.deleted_at is null
		group by c.id, l.name
		order by c.created_at desc`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClouds(rows)
}

func (s *PostgresStore) CreateCloud(ctx context.Context, params CreateCloudParams) (Cloud, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Cloud{}, err
	}
	defer tx.Rollback(ctx)

	var cloudID uuid.UUID
	if err := tx.QueryRow(ctx, `
		insert into clouds (organization_id, name, slug, location_id, description)
		values ($1, $2, $3, $4, $5)
		returning id`, params.OrganizationID, params.Name, params.Slug, params.LocationID, params.Description).Scan(&cloudID); err != nil {
		return Cloud{}, mapConstraint(err)
	}

	if params.CreateDefaultNetwork {
		cidr := params.DefaultCIDR
		if cidr == "" {
			cidr = "10.80.0.0/16"
		}
		if _, err := tx.Exec(ctx, `
			insert into private_networks (organization_id, cloud_id, name, cidr, isolation_type, status)
			values ($1, $2, 'Default private network', $3::cidr, 'stub', 'pending')`,
			params.OrganizationID, cloudID, cidr); err != nil {
			return Cloud{}, mapConstraint(err)
		}
	}

	if _, err := tx.Exec(ctx, `
		insert into resource_actions (organization_id, cloud_id, resource_type, resource_id, action_type, status, message)
		values ($1, $2, 'cloud', $2, 'create_cloud', 'stubbed', 'Cloud record created. Infrastructure provisioning is not active in this milestone.')`,
		params.OrganizationID, cloudID); err != nil {
		return Cloud{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Cloud{}, err
	}
	return s.GetCloud(ctx, params.OrganizationID, cloudID)
}

func (s *PostgresStore) GetCloud(ctx context.Context, organizationID, cloudID uuid.UUID) (Cloud, error) {
	row := s.pool.QueryRow(ctx, cloudSelectSQL()+`
		where c.organization_id = $1 and c.id = $2 and c.deleted_at is null
		group by c.id, l.name`, organizationID, cloudID)
	cloud, err := scanCloud(row)
	return cloud, mapNoRows(err)
}

func (s *PostgresStore) UpdateCloud(ctx context.Context, organizationID, cloudID uuid.UUID, params UpdateCloudParams) (Cloud, error) {
	_, err := s.pool.Exec(ctx, `
		update clouds
		set name = $3, slug = $4, description = $5, updated_at = now()
		where organization_id = $1 and id = $2 and deleted_at is null`,
		organizationID, cloudID, params.Name, params.Slug, params.Description)
	if err != nil {
		return Cloud{}, mapConstraint(err)
	}
	return s.GetCloud(ctx, organizationID, cloudID)
}

func (s *PostgresStore) DeleteCloud(ctx context.Context, organizationID, cloudID uuid.UUID) error {
	var activeResources int64
	if err := s.pool.QueryRow(ctx, `
		select
			(select count(*) from servers where cloud_id = $2) +
			(select count(*) from virtual_machines where cloud_id = $2 and deleted_at is null and status <> 'deleted') +
			(select count(*) from managed_services where cloud_id = $2 and deleted_at is null and status <> 'deleted') +
			(select count(*) from private_networks where cloud_id = $2 and deleted_at is null and status <> 'deleted')
		from clouds
		where organization_id = $1 and id = $2 and deleted_at is null`,
		organizationID, cloudID).Scan(&activeResources); err != nil {
		return mapNoRows(err)
	}
	if activeResources > 0 {
		return ErrConflict
	}
	tag, err := s.pool.Exec(ctx, `
		update clouds set status = 'deleted', deleted_at = now(), updated_at = now()
		where organization_id = $1 and id = $2 and deleted_at is null`, organizationID, cloudID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, _ = s.AddResourceAction(ctx, organizationID, &cloudID, "cloud", &cloudID, "delete_cloud", "stubbed", "Cloud soft-deleted. No infrastructure cleanup was run.")
	return nil
}

func (s *PostgresStore) GetCloudOverview(ctx context.Context, organizationID, cloudID uuid.UUID) (CloudOverview, error) {
	cloud, err := s.GetCloud(ctx, organizationID, cloudID)
	if err != nil {
		return CloudOverview{}, err
	}
	var overview CloudOverview
	overview.Cloud = cloud
	if err := s.pool.QueryRow(ctx, `
		select
			count(*) filter (where s.cloud_id = $2),
			count(*) filter (where s.cloud_id = $2 and s.server_mode = 'virtualization_host'),
			count(*) filter (where s.cloud_id = $2 and s.server_mode = 'managed_services_host'),
			(select count(*) from virtual_machines where organization_id = $1 and cloud_id = $2 and deleted_at is null),
			(select count(*) from managed_services where organization_id = $1 and cloud_id = $2 and deleted_at is null),
			(select count(*) from private_networks where organization_id = $1 and cloud_id = $2 and deleted_at is null)
		from servers s
		where s.organization_id = $1 or s.cloud_id = $2`,
		organizationID, cloudID).Scan(&overview.ServerCount, &overview.VirtualizationHostCount, &overview.ManagedServicesHostCount, &overview.VMCount, &overview.ManagedServiceCount, &overview.PrivateNetworkCount); err != nil {
		return CloudOverview{}, err
	}
	overview.Capacity, _ = s.GetCloudCapacity(ctx, organizationID, cloudID)
	overview.RecentActions, _ = s.ListResourceActions(ctx, organizationID, &cloudID)
	if overview.ServerCount == 0 {
		overview.Warnings = append(overview.Warnings, "No servers assigned")
	}
	if overview.PrivateNetworkCount == 0 {
		overview.Warnings = append(overview.Warnings, "No private network")
	}
	if overview.VirtualizationHostCount == 0 {
		overview.Warnings = append(overview.Warnings, "No virtualization host")
	}
	if overview.ManagedServicesHostCount == 0 {
		overview.Warnings = append(overview.Warnings, "No managed services host")
	}
	return overview, nil
}

func (s *PostgresStore) ListCloudServers(ctx context.Context, organizationID, cloudID uuid.UUID) ([]CloudServer, error) {
	rows, err := s.pool.Query(ctx, cloudServerSelectSQL()+`
		where s.organization_id = $1 and s.cloud_id = $2
		group by s.id, r.location
		order by hostname`, organizationID, cloudID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCloudServers(rows)
}

func (s *PostgresStore) ListOrganizationServers(ctx context.Context, organizationID uuid.UUID) ([]FleetServer, error) {
	rows, err := s.pool.Query(ctx, `
		select
			s.id,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as hostname,
			nullif(coalesce(s.asset_tag, s.metadata->>'asset_tag', s.metadata->>'label'), '') as inventory_label,
			s.status,
			coalesce(l.name, r.location) as location_name,
			s.project_id,
			p.name as project_name,
			s.cloud_id,
			c.name as cloud_name,
			s.server_mode,
			s.mode_status,
			s.reserved_cpu_cores,
			s.reserved_memory_mb,
			s.reserved_storage_gb,
			nullif(coalesce(s.metadata->>'hardware_profile_name', s.metadata->>'sku'), '') as hardware_profile_name,
			nullif(sf.display_name, '') as server_family_name,
			nullif(coalesce(sf.cpu_model, s.metadata->>'cpu_model', s.metadata->>'cpu', s.metadata->>'processor'), '') as cpu_model,
			nullif(coalesce(s.metadata->>'disk_description', s.metadata->>'storage', s.metadata->>'disk'), '') as disk_description,
			nullif(coalesce(s.metadata->>'network_capacity', s.metadata->>'network_speed', s.metadata->>'nic_speed'), '') as network_capacity,
			nullif(s.metadata->>'gpu', '') as gpu,
			coalesce(nullif(s.metadata->>'public_ip', ''), primary_network.public_ip) as public_ip,
			coalesce(nullif(s.metadata->>'private_ip', ''), primary_network.private_ip) as private_ip,
			count(distinct vm.id) filter (where vm.deleted_at is null) as vm_count,
			count(distinct ms.id) filter (where ms.deleted_at is null) as managed_service_count,
			price.monthly_cost_cents,
			s.created_at,
			s.updated_at
		from servers s
		join racks r on r.id = s.rack_id
		left join server_families sf on sf.id = s.server_family_id
		left join locations l on l.id = r.location_id
		left join projects p on p.id = s.project_id
		left join clouds c on c.id = s.cloud_id and c.deleted_at is null
		left join virtual_machines vm on vm.host_server_id = s.id
		left join managed_services ms on ms.host_server_id = s.id
		left join lateral (
			select
				max(sni.ip_address::text) filter (where sni.is_public = true or sni.purpose = 'public') as public_ip,
				max(sni.ip_address::text) filter (where sni.is_public = false and sni.purpose in ('private', '')) as private_ip
			from server_network_interfaces sni
			where sni.server_id = s.id
		) primary_network on true
		left join lateral (
			select sum((bsp.unit_price_cents * bsp.quantity)::bigint) as monthly_cost_cents
			from billable_services bs
			join billable_service_prices bsp on bsp.billable_service_id = bs.id
			where bs.organization_id = s.organization_id
				and bs.service_type = 'server'
				and bs.service_id = s.id
				and bs.status in ('provisioning', 'active', 'suspended')
				and bsp.price_type = 'recurring'
				and bsp.unit = 'month'
				and bsp.effective_to is null
		) price on true
		where s.organization_id = $1
		group by s.id, r.location, l.name, p.name, c.name, sf.display_name, sf.cpu_model,
			primary_network.public_ip, primary_network.private_ip, price.monthly_cost_cents
		order by s.updated_at desc`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []FleetServer{}
	for rows.Next() {
		var item FleetServer
		if err := rows.Scan(
			&item.ID, &item.Hostname, &item.InventoryLabel, &item.Status, &item.LocationName,
			&item.ProjectID, &item.ProjectName, &item.CloudID, &item.CloudName,
			&item.ServerMode, &item.ModeStatus, &item.ReservedCPUCores,
			&item.ReservedMemoryMB, &item.ReservedStorageGB, &item.HardwareProfileName,
			&item.ServerFamilyName, &item.CPUModel, &item.DiskDescription,
			&item.NetworkCapacity, &item.GPU, &item.PublicIP, &item.PrivateIP,
			&item.VMCount, &item.ServiceCount, &item.MonthlyCostCents,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) AssignServerToCloud(ctx context.Context, organizationID, cloudID, serverID uuid.UUID) (CloudServer, error) {
	tag, err := s.pool.Exec(ctx, `
		update servers
		set cloud_id = $2, updated_at = now()
		where id = $3 and organization_id = $1 and (cloud_id is null or cloud_id = $2)`,
		organizationID, cloudID, serverID)
	if err != nil {
		return CloudServer{}, err
	}
	if tag.RowsAffected() == 0 {
		return CloudServer{}, ErrNotFound
	}
	_, _ = s.AddResourceAction(ctx, organizationID, &cloudID, "server", &serverID, "assign_server", "stubbed", "Server assigned to cloud. No infrastructure changes were run.")
	return s.getCloudServer(ctx, organizationID, cloudID, serverID)
}

func (s *PostgresStore) UnassignServerFromCloud(ctx context.Context, organizationID, cloudID, serverID uuid.UUID) (CloudServer, error) {
	var dependents int64
	if err := s.pool.QueryRow(ctx, `
		select
			(select count(*) from virtual_machines where host_server_id = $3 and deleted_at is null and status <> 'deleted') +
			(select count(*) from managed_services where host_server_id = $3 and deleted_at is null and status <> 'deleted')`,
		organizationID, cloudID, serverID).Scan(&dependents); err != nil {
		return CloudServer{}, err
	}
	if dependents > 0 {
		return CloudServer{}, ErrConflict
	}
	server, err := s.getCloudServer(ctx, organizationID, cloudID, serverID)
	if err != nil {
		return CloudServer{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		update servers set cloud_id = null, updated_at = now()
		where id = $3 and organization_id = $1 and cloud_id = $2`, organizationID, cloudID, serverID)
	if err != nil {
		return CloudServer{}, err
	}
	if tag.RowsAffected() == 0 {
		return CloudServer{}, ErrNotFound
	}
	_, _ = s.AddResourceAction(ctx, organizationID, &cloudID, "server", &serverID, "unassign_server", "stubbed", "Server removed from cloud. No infrastructure changes were run.")
	server.CloudID = nil
	return server, nil
}

func (s *PostgresStore) ChangeServerMode(ctx context.Context, organizationID, serverID uuid.UUID, mode string) (CloudServer, error) {
	tag, err := s.pool.Exec(ctx, `
		update servers
		set server_mode = $3, mode_status = 'pending', platform_managed = ($3 <> 'bare_metal'), updated_at = now()
		where organization_id = $1 and id = $2`,
		organizationID, serverID, mode)
	if err != nil {
		return CloudServer{}, mapConstraint(err)
	}
	if tag.RowsAffected() == 0 {
		return CloudServer{}, ErrNotFound
	}
	var cloudID *uuid.UUID
	_ = s.pool.QueryRow(ctx, `select cloud_id from servers where id = $1`, serverID).Scan(&cloudID)
	_, _ = s.AddResourceAction(ctx, organizationID, cloudID, "server", &serverID, "change_server_mode", "stubbed", "Server mode change recorded. Future releases may reinstall this server; this build does not reprovision it.")
	return s.getServerByID(ctx, organizationID, serverID)
}

func (s *PostgresStore) ListPrivateNetworks(ctx context.Context, organizationID, cloudID uuid.UUID) ([]PrivateNetwork, error) {
	rows, err := s.pool.Query(ctx, privateNetworkSelectSQL()+`
		where pn.organization_id = $1 and pn.cloud_id = $2 and pn.deleted_at is null
		group by pn.id
		order by pn.created_at desc`, organizationID, cloudID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrivateNetworks(rows)
}

func (s *PostgresStore) CreatePrivateNetwork(ctx context.Context, params CreatePrivateNetworkParams) (PrivateNetwork, error) {
	var overlaps bool
	if err := s.pool.QueryRow(ctx, `
		select exists (
			select 1 from private_networks
			where organization_id = $1 and cloud_id = $2 and deleted_at is null and cidr && $3::cidr
		)`, params.OrganizationID, params.CloudID, params.CIDR).Scan(&overlaps); err != nil {
		return PrivateNetwork{}, mapConstraint(err)
	}
	if overlaps {
		return PrivateNetwork{}, ErrConflict
	}
	var id uuid.UUID
	if err := s.pool.QueryRow(ctx, `
		insert into private_networks (organization_id, cloud_id, name, description, cidr, gateway_ip, isolation_type, status)
		values ($1, $2, $3, $4, $5::cidr, $6::inet, 'stub', 'pending')
		returning id`, params.OrganizationID, params.CloudID, params.Name, params.Description, params.CIDR, params.GatewayIP).Scan(&id); err != nil {
		return PrivateNetwork{}, mapConstraint(err)
	}
	_, _ = s.AddResourceAction(ctx, params.OrganizationID, &params.CloudID, "private_network", &id, "create_private_network", "stubbed", "Private network record created. No switch or overlay configuration was run.")
	return s.GetPrivateNetwork(ctx, params.OrganizationID, params.CloudID, id)
}

func (s *PostgresStore) GetPrivateNetwork(ctx context.Context, organizationID, cloudID, networkID uuid.UUID) (PrivateNetwork, error) {
	row := s.pool.QueryRow(ctx, privateNetworkSelectSQL()+`
		where pn.organization_id = $1 and pn.cloud_id = $2 and pn.id = $3 and pn.deleted_at is null
		group by pn.id`, organizationID, cloudID, networkID)
	network, err := scanPrivateNetwork(row)
	return network, mapNoRows(err)
}

func (s *PostgresStore) DeletePrivateNetwork(ctx context.Context, organizationID, cloudID, networkID uuid.UUID) error {
	var attachments int64
	if err := s.pool.QueryRow(ctx, `
		select count(*) from network_attachments
		where organization_id = $1 and cloud_id = $2 and private_network_id = $3 and status <> 'detached'`,
		organizationID, cloudID, networkID).Scan(&attachments); err != nil {
		return err
	}
	if attachments > 0 {
		return ErrConflict
	}
	tag, err := s.pool.Exec(ctx, `
		update private_networks set status = 'deleted', deleted_at = now(), updated_at = now()
		where organization_id = $1 and cloud_id = $2 and id = $3 and deleted_at is null`,
		organizationID, cloudID, networkID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, _ = s.AddResourceAction(ctx, organizationID, &cloudID, "private_network", &networkID, "delete_private_network", "stubbed", "Private network soft-deleted. No infrastructure cleanup was run.")
	return nil
}

func (s *PostgresStore) ListNetworkAttachments(ctx context.Context, organizationID, cloudID, networkID uuid.UUID) ([]NetworkAttachment, error) {
	rows, err := s.pool.Query(ctx, networkAttachmentSelectSQL()+`
		where na.organization_id = $1 and na.cloud_id = $2 and na.private_network_id = $3
		order by na.created_at desc`, organizationID, cloudID, networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNetworkAttachments(rows)
}

func (s *PostgresStore) CreateNetworkAttachment(ctx context.Context, params CreateNetworkAttachmentParams) (NetworkAttachment, error) {
	var networkCIDR string
	if err := s.pool.QueryRow(ctx, `
		select cidr::text from private_networks
		where organization_id = $1 and cloud_id = $2 and id = $3 and deleted_at is null`,
		params.OrganizationID, params.CloudID, params.PrivateNetworkID).Scan(&networkCIDR); err != nil {
		return NetworkAttachment{}, mapNoRows(err)
	}
	if params.PrivateIP != nil {
		var contained bool
		if err := s.pool.QueryRow(ctx, `select $1::inet << $2::cidr`, *params.PrivateIP, networkCIDR).Scan(&contained); err != nil {
			return NetworkAttachment{}, mapConstraint(err)
		}
		if !contained {
			return NetworkAttachment{}, ErrInvalidInput
		}
	}
	var id uuid.UUID
	if err := s.pool.QueryRow(ctx, `
		insert into network_attachments (organization_id, cloud_id, private_network_id, resource_type, resource_id, private_ip, mac_address, status)
		values ($1, $2, $3, $4, $5, $6::inet, $7::macaddr, 'attached_stub')
		on conflict (private_network_id, resource_type, resource_id)
		do update set private_ip = excluded.private_ip, mac_address = excluded.mac_address, status = 'attached_stub', updated_at = now()
		returning id`,
		params.OrganizationID, params.CloudID, params.PrivateNetworkID, params.ResourceType, params.ResourceID, params.PrivateIP, params.MACAddress).Scan(&id); err != nil {
		return NetworkAttachment{}, mapConstraint(err)
	}
	_, _ = s.AddResourceAction(ctx, params.OrganizationID, &params.CloudID, params.ResourceType, &params.ResourceID, "attach_network", "stubbed", "Network attachment recorded. No networking changes were applied.")
	return s.getNetworkAttachment(ctx, params.OrganizationID, params.CloudID, params.PrivateNetworkID, id)
}

func (s *PostgresStore) DetachNetworkAttachment(ctx context.Context, organizationID, cloudID, networkID, attachmentID uuid.UUID) error {
	var resourceType string
	var resourceID uuid.UUID
	if err := s.pool.QueryRow(ctx, `
		update network_attachments
		set status = 'detached', updated_at = now()
		where organization_id = $1 and cloud_id = $2 and private_network_id = $3 and id = $4 and status <> 'detached'
		returning resource_type, resource_id`,
		organizationID, cloudID, networkID, attachmentID).Scan(&resourceType, &resourceID); err != nil {
		return mapNoRows(err)
	}
	_, _ = s.AddResourceAction(ctx, organizationID, &cloudID, resourceType, &resourceID, "detach_network", "stubbed", "Network attachment detached in control plane. No switch or fabric cleanup was run.")
	return nil
}

func (s *PostgresStore) ListVirtualMachines(ctx context.Context, organizationID, cloudID uuid.UUID) ([]VirtualMachine, error) {
	rows, err := s.pool.Query(ctx, virtualMachineSelectSQL()+`
		where vm.organization_id = $1 and vm.cloud_id = $2 and vm.deleted_at is null
		order by vm.created_at desc`, organizationID, cloudID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVirtualMachines(rows)
}

func (s *PostgresStore) CreateVirtualMachine(ctx context.Context, params CreateVirtualMachineParams) (VirtualMachine, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return VirtualMachine{}, err
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	if err := tx.QueryRow(ctx, `
		insert into virtual_machines (organization_id, cloud_id, host_server_id, name, hostname, status, power_state, cpu_cores, memory_mb, disk_gb, image_id, os_image, private_ip)
		values ($1, $2, $3, $4, $5, 'pending', 'unknown', $6, $7, $8, $9, $10, $11::inet)
		returning id`,
		params.OrganizationID, params.CloudID, params.HostServerID, params.Name, params.Hostname, params.CPUCores, params.MemoryMB, params.DiskGB, params.ImageID, params.OSImage, params.PrivateIP).Scan(&id); err != nil {
		return VirtualMachine{}, mapConstraint(err)
	}
	if params.PrivateNetworkID != nil {
		if _, err := tx.Exec(ctx, `
			insert into network_attachments (organization_id, cloud_id, private_network_id, resource_type, resource_id, private_ip, status)
			values ($1, $2, $3, 'virtual_machine', $4, $5::inet, 'attached_stub')`,
			params.OrganizationID, params.CloudID, *params.PrivateNetworkID, id, params.PrivateIP); err != nil {
			return VirtualMachine{}, mapConstraint(err)
		}
	}
	if _, err := tx.Exec(ctx, `
		insert into resource_actions (organization_id, cloud_id, resource_type, resource_id, action_type, status, message)
		values ($1, $2, 'virtual_machine', $3, 'create_vm', 'stubbed', 'VM record created. VM provisioning is not active yet.')`,
		params.OrganizationID, params.CloudID, id); err != nil {
		return VirtualMachine{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return VirtualMachine{}, err
	}
	return s.GetVirtualMachine(ctx, params.OrganizationID, params.CloudID, id)
}

func (s *PostgresStore) GetVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID) (VirtualMachine, error) {
	row := s.pool.QueryRow(ctx, virtualMachineSelectSQL()+`
		where vm.organization_id = $1 and vm.cloud_id = $2 and vm.id = $3 and vm.deleted_at is null`, organizationID, cloudID, vmID)
	vm, err := scanVirtualMachine(row)
	return vm, mapNoRows(err)
}

func (s *PostgresStore) UpdateVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID, params UpdateVirtualMachineParams) (VirtualMachine, error) {
	tag, err := s.pool.Exec(ctx, `
		update virtual_machines
		set host_server_id = $4,
			name = $5,
			hostname = $6,
			cpu_cores = $7,
			memory_mb = $8,
			disk_gb = $9,
			image_id = $10,
			os_image = $11,
			private_ip = $12::inet,
			updated_at = now()
		where organization_id = $1 and cloud_id = $2 and id = $3 and deleted_at is null`,
		organizationID, cloudID, vmID, params.HostServerID, params.Name, params.Hostname, params.CPUCores, params.MemoryMB, params.DiskGB, params.ImageID, params.OSImage, params.PrivateIP)
	if err != nil {
		return VirtualMachine{}, mapConstraint(err)
	}
	if tag.RowsAffected() == 0 {
		return VirtualMachine{}, ErrNotFound
	}
	_, _ = s.AddResourceAction(ctx, organizationID, &cloudID, "virtual_machine", &vmID, "update_vm", "stubbed", "VM record updated. No hypervisor changes were run.")
	return s.GetVirtualMachine(ctx, organizationID, cloudID, vmID)
}

func (s *PostgresStore) DeleteVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		update virtual_machines set status = 'deleted', deleted_at = now(), updated_at = now()
		where organization_id = $1 and cloud_id = $2 and id = $3 and deleted_at is null`, organizationID, cloudID, vmID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, _ = s.AddResourceAction(ctx, organizationID, &cloudID, "virtual_machine", &vmID, "delete_vm", "stubbed", "VM deletion recorded. No hypervisor action was run.")
	return nil
}

func (s *PostgresStore) PowerVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID, action string) (VirtualMachine, error) {
	status := "pending"
	power := "unknown"
	switch action {
	case "start":
		status, power = "pending", "running"
	case "stop":
		status, power = "stopped", "stopped"
	case "reboot":
		status, power = "pending", "running"
	default:
		return VirtualMachine{}, ErrInvalidInput
	}
	tag, err := s.pool.Exec(ctx, `
		update virtual_machines set status = $4, power_state = $5, updated_at = now()
		where organization_id = $1 and cloud_id = $2 and id = $3 and deleted_at is null`,
		organizationID, cloudID, vmID, status, power)
	if err != nil {
		return VirtualMachine{}, err
	}
	if tag.RowsAffected() == 0 {
		return VirtualMachine{}, ErrNotFound
	}
	_, _ = s.AddResourceAction(ctx, organizationID, &cloudID, "virtual_machine", &vmID, fmt.Sprintf("%s_vm", action), "stubbed", "VM power action recorded. No hypervisor action was run.")
	return s.GetVirtualMachine(ctx, organizationID, cloudID, vmID)
}

func (s *PostgresStore) ListManagedServices(ctx context.Context, organizationID, cloudID uuid.UUID) ([]ManagedService, error) {
	rows, err := s.pool.Query(ctx, managedServiceSelectSQL()+`
		where ms.organization_id = $1 and ms.cloud_id = $2 and ms.deleted_at is null
		order by ms.created_at desc`, organizationID, cloudID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanManagedServices(rows)
}

func (s *PostgresStore) CreateManagedService(ctx context.Context, params CreateManagedServiceParams) (ManagedService, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ManagedService{}, err
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	if err := tx.QueryRow(ctx, `
		insert into managed_services (organization_id, cloud_id, host_server_id, service_type, name, status, cpu_cores, memory_mb, storage_gb, version, endpoint_hostname, private_ip, port)
		values ($1, $2, $3, $4, $5, 'pending', $6, $7, $8, $9, $10, $11::inet, 5432)
		returning id`,
		params.OrganizationID, params.CloudID, params.HostServerID, params.ServiceType, params.Name, params.CPUCores, params.MemoryMB, params.StorageGB, params.Version, params.EndpointHostname, params.PrivateIP).Scan(&id); err != nil {
		return ManagedService{}, mapConstraint(err)
	}
	retention := params.BackupRetentionDays
	if retention <= 0 {
		retention = 7
	}
	if _, err := tx.Exec(ctx, `
		insert into managed_service_backup_policies (managed_service_id, enabled, frequency, retention_days)
		values ($1, $2, 'daily', $3)`, id, params.BackupEnabled, retention); err != nil {
		return ManagedService{}, err
	}
	if params.PrivateNetworkID != nil {
		if _, err := tx.Exec(ctx, `
			insert into network_attachments (organization_id, cloud_id, private_network_id, resource_type, resource_id, private_ip, status)
			values ($1, $2, $3, 'managed_service', $4, $5::inet, 'attached_stub')`,
			params.OrganizationID, params.CloudID, *params.PrivateNetworkID, id, params.PrivateIP); err != nil {
			return ManagedService{}, mapConstraint(err)
		}
	}
	if _, err := tx.Exec(ctx, `
		insert into resource_actions (organization_id, cloud_id, resource_type, resource_id, action_type, status, message)
		values ($1, $2, 'managed_service', $3, 'create_managed_service', 'stubbed', 'Managed Postgres record created. Database provisioning is not active yet.')`,
		params.OrganizationID, params.CloudID, id); err != nil {
		return ManagedService{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ManagedService{}, err
	}
	return s.GetManagedService(ctx, params.OrganizationID, params.CloudID, id)
}

func (s *PostgresStore) GetManagedService(ctx context.Context, organizationID, cloudID, serviceID uuid.UUID) (ManagedService, error) {
	row := s.pool.QueryRow(ctx, managedServiceSelectSQL()+`
		where ms.organization_id = $1 and ms.cloud_id = $2 and ms.id = $3 and ms.deleted_at is null`, organizationID, cloudID, serviceID)
	service, err := scanManagedService(row)
	return service, mapNoRows(err)
}

func (s *PostgresStore) DeleteManagedService(ctx context.Context, organizationID, cloudID, serviceID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		update managed_services set status = 'deleted', deleted_at = now(), updated_at = now()
		where organization_id = $1 and cloud_id = $2 and id = $3 and deleted_at is null`, organizationID, cloudID, serviceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, _ = s.AddResourceAction(ctx, organizationID, &cloudID, "managed_service", &serviceID, "delete_managed_service", "stubbed", "Managed service deletion recorded. No database action was run.")
	return nil
}

func (s *PostgresStore) ActOnManagedService(ctx context.Context, organizationID, cloudID, serviceID uuid.UUID, action string) (ManagedService, error) {
	status := "pending"
	if action == "stop" {
		status = "stopped"
	}
	if action != "start" && action != "stop" && action != "restart" {
		return ManagedService{}, ErrInvalidInput
	}
	tag, err := s.pool.Exec(ctx, `
		update managed_services set status = $4, updated_at = now()
		where organization_id = $1 and cloud_id = $2 and id = $3 and deleted_at is null`,
		organizationID, cloudID, serviceID, status)
	if err != nil {
		return ManagedService{}, err
	}
	if tag.RowsAffected() == 0 {
		return ManagedService{}, ErrNotFound
	}
	_, _ = s.AddResourceAction(ctx, organizationID, &cloudID, "managed_service", &serviceID, fmt.Sprintf("%s_managed_service", action), "stubbed", "Managed service action recorded. No database process action was run.")
	return s.GetManagedService(ctx, organizationID, cloudID, serviceID)
}

func (s *PostgresStore) GetCloudCapacity(ctx context.Context, organizationID, cloudID uuid.UUID) (CloudCapacity, error) {
	var c CloudCapacity
	if err := s.pool.QueryRow(ctx, `
		select
			sum(reserved_cpu_cores)::bigint,
			sum(reserved_memory_mb)::bigint,
			sum(reserved_storage_gb)::bigint,
			coalesce((select sum(cpu_cores) from virtual_machines where organization_id = $1 and cloud_id = $2 and deleted_at is null), 0)::bigint,
			coalesce((select sum(memory_mb) from virtual_machines where organization_id = $1 and cloud_id = $2 and deleted_at is null), 0)::bigint,
			coalesce((select sum(disk_gb) from virtual_machines where organization_id = $1 and cloud_id = $2 and deleted_at is null), 0)::bigint,
			coalesce((select sum(cpu_cores) from managed_services where organization_id = $1 and cloud_id = $2 and deleted_at is null), 0)::bigint,
			coalesce((select sum(memory_mb) from managed_services where organization_id = $1 and cloud_id = $2 and deleted_at is null), 0)::bigint,
			coalesce((select sum(storage_gb) from managed_services where organization_id = $1 and cloud_id = $2 and deleted_at is null), 0)::bigint
		from servers
		where organization_id = $1 and cloud_id = $2`,
		organizationID, cloudID).Scan(&c.TotalCPUCores, &c.TotalMemoryMB, &c.TotalStorageGB, &c.AllocatedVMCPUCores, &c.AllocatedVMMemoryMB, &c.AllocatedVMDiskGB, &c.AllocatedServiceCPUCores, &c.AllocatedServiceMemoryMB, &c.AllocatedServiceStorageGB); err != nil {
		return CloudCapacity{}, err
	}
	c.EstimateAvailable = c.TotalCPUCores != nil || c.TotalMemoryMB != nil || c.TotalStorageGB != nil
	allocatedCPU := c.AllocatedVMCPUCores + c.AllocatedServiceCPUCores
	allocatedMemory := c.AllocatedVMMemoryMB + c.AllocatedServiceMemoryMB
	allocatedStorage := c.AllocatedVMDiskGB + c.AllocatedServiceStorageGB
	if c.TotalCPUCores != nil {
		value := *c.TotalCPUCores - allocatedCPU
		c.RemainingCPUCores = &value
	}
	if c.TotalMemoryMB != nil {
		value := *c.TotalMemoryMB - allocatedMemory
		c.RemainingMemoryMB = &value
	}
	if c.TotalStorageGB != nil {
		value := *c.TotalStorageGB - allocatedStorage
		c.RemainingStorageGB = &value
	}
	return c, nil
}

func (s *PostgresStore) ListPlacementOptions(ctx context.Context, organizationID, cloudID uuid.UUID, resourceType string) ([]PlacementOption, error) {
	mode := "virtualization_host"
	if resourceType == "postgres" {
		mode = "managed_services_host"
	}
	rows, err := s.pool.Query(ctx, `
		select
			s.id,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as hostname,
			s.server_mode,
			s.mode_status,
			r.location,
			(s.reserved_cpu_cores::bigint - coalesce((select sum(cpu_cores) from virtual_machines where host_server_id = s.id and deleted_at is null), 0)::bigint - coalesce((select sum(cpu_cores) from managed_services where host_server_id = s.id and deleted_at is null), 0)::bigint) as remaining_cpu,
			(s.reserved_memory_mb::bigint - coalesce((select sum(memory_mb) from virtual_machines where host_server_id = s.id and deleted_at is null), 0)::bigint - coalesce((select sum(memory_mb) from managed_services where host_server_id = s.id and deleted_at is null), 0)::bigint) as remaining_memory,
			(s.reserved_storage_gb::bigint - coalesce((select sum(disk_gb) from virtual_machines where host_server_id = s.id and deleted_at is null), 0)::bigint - coalesce((select sum(storage_gb) from managed_services where host_server_id = s.id and deleted_at is null), 0)::bigint) as remaining_storage
		from servers s
		join racks r on r.id = s.rack_id
		where s.organization_id = $1 and s.cloud_id = $2 and s.server_mode = $3
		order by hostname`, organizationID, cloudID, mode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PlacementOption{}
	for rows.Next() {
		var item PlacementOption
		if err := rows.Scan(&item.ServerID, &item.Hostname, &item.ServerMode, &item.ModeStatus, &item.LocationName, &item.RemainingCPUCores, &item.RemainingMemoryMB, &item.RemainingStorageGB); err != nil {
			return nil, err
		}
		if item.ModeStatus != "ready" {
			item.Warnings = append(item.Warnings, "Host mode is not ready yet")
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListResourceActions(ctx context.Context, organizationID uuid.UUID, cloudID *uuid.UUID) ([]ResourceAction, error) {
	args := []any{organizationID}
	where := "where organization_id = $1"
	if cloudID != nil {
		args = append(args, *cloudID)
		where += " and cloud_id = $2"
	}
	rows, err := s.pool.Query(ctx, `
		select id, organization_id, cloud_id, resource_type, resource_id, action_type, status, message, created_at, updated_at
		from resource_actions `+where+`
		order by created_at desc
		limit 100`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanResourceActions(rows)
}

func (s *PostgresStore) AddResourceAction(ctx context.Context, organizationID uuid.UUID, cloudID *uuid.UUID, resourceType string, resourceID *uuid.UUID, actionType, status, message string) (ResourceAction, error) {
	row := s.pool.QueryRow(ctx, `
		insert into resource_actions (organization_id, cloud_id, resource_type, resource_id, action_type, status, message)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id, organization_id, cloud_id, resource_type, resource_id, action_type, status, message, created_at, updated_at`,
		organizationID, cloudID, resourceType, resourceID, actionType, status, message)
	action, err := scanResourceAction(row)
	return action, err
}

func (s *PostgresStore) ListAdminCloudResources(ctx context.Context) ([]Cloud, []VirtualMachine, []ManagedService, []PrivateNetwork, []ResourceAction, error) {
	cloudRows, err := s.pool.Query(ctx, cloudSelectSQL()+`
		where c.deleted_at is null
		group by c.id, l.name
		order by c.created_at desc`)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	clouds, err := scanClouds(cloudRows)
	cloudRows.Close()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	vmRows, err := s.pool.Query(ctx, virtualMachineSelectSQL()+` where vm.deleted_at is null order by vm.created_at desc`)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	vms, err := scanVirtualMachines(vmRows)
	vmRows.Close()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	serviceRows, err := s.pool.Query(ctx, managedServiceSelectSQL()+` where ms.deleted_at is null order by ms.created_at desc`)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	services, err := scanManagedServices(serviceRows)
	serviceRows.Close()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	networkRows, err := s.pool.Query(ctx, privateNetworkSelectSQL()+`
		where pn.deleted_at is null
		group by pn.id
		order by pn.created_at desc`)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	networks, err := scanPrivateNetworks(networkRows)
	networkRows.Close()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	actionRows, err := s.pool.Query(ctx, `
		select id, organization_id, cloud_id, resource_type, resource_id, action_type, status, message, created_at, updated_at
		from resource_actions
		order by created_at desc
		limit 100`)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	defer actionRows.Close()
	actions, err := scanResourceActions(actionRows)
	return clouds, vms, services, networks, actions, err
}

func (s *PostgresStore) getCloudServer(ctx context.Context, organizationID, cloudID, serverID uuid.UUID) (CloudServer, error) {
	row := s.pool.QueryRow(ctx, cloudServerSelectSQL()+`
		where s.organization_id = $1 and s.cloud_id = $2 and s.id = $3
		group by s.id, r.location`, organizationID, cloudID, serverID)
	server, err := scanCloudServer(row)
	return server, mapNoRows(err)
}

func (s *PostgresStore) getServerByID(ctx context.Context, organizationID, serverID uuid.UUID) (CloudServer, error) {
	row := s.pool.QueryRow(ctx, cloudServerSelectSQL()+`
		where s.organization_id = $1 and s.id = $2
		group by s.id, r.location`, organizationID, serverID)
	server, err := scanCloudServer(row)
	return server, mapNoRows(err)
}

func (s *PostgresStore) getNetworkAttachment(ctx context.Context, organizationID, cloudID, networkID, attachmentID uuid.UUID) (NetworkAttachment, error) {
	row := s.pool.QueryRow(ctx, networkAttachmentSelectSQL()+`
		where na.organization_id = $1 and na.cloud_id = $2 and na.private_network_id = $3 and na.id = $4`,
		organizationID, cloudID, networkID, attachmentID)
	attachment, err := scanNetworkAttachment(row)
	return attachment, mapNoRows(err)
}

func cloudSelectSQL() string {
	return `
		select
			c.id, c.organization_id, c.name, c.slug, c.location_id, l.name as location_name, c.description, c.status,
			count(distinct s.id) as server_count,
			count(distinct vm.id) filter (where vm.deleted_at is null) as vm_count,
			count(distinct ms.id) filter (where ms.deleted_at is null) as managed_service_count,
			count(distinct pn.id) filter (where pn.deleted_at is null) as private_network_count,
			c.created_at, c.updated_at, c.deleted_at
		from clouds c
		left join locations l on l.id = c.location_id
		left join servers s on s.cloud_id = c.id
		left join virtual_machines vm on vm.cloud_id = c.id
		left join managed_services ms on ms.cloud_id = c.id
		left join private_networks pn on pn.cloud_id = c.id`
}

func cloudServerSelectSQL() string {
	return `
		select
			s.id,
			s.cloud_id,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as hostname,
			s.status,
			r.location,
			s.server_mode,
			s.mode_status,
			s.platform_managed,
			s.reserved_cpu_cores,
			s.reserved_memory_mb,
			s.reserved_storage_gb,
			s.reserved_cpu_cores,
			s.reserved_memory_mb,
			s.reserved_storage_gb,
			count(distinct vm.id) filter (where vm.deleted_at is null) as vm_count,
			count(distinct ms.id) filter (where ms.deleted_at is null) as managed_service_count,
			s.updated_at
		from servers s
		join racks r on r.id = s.rack_id
		left join virtual_machines vm on vm.host_server_id = s.id
		left join managed_services ms on ms.host_server_id = s.id`
}

func privateNetworkSelectSQL() string {
	return `
		select
			pn.id, pn.organization_id, pn.cloud_id, pn.name, pn.description, pn.cidr::text, pn.gateway_ip::text,
			pn.network_type, pn.isolation_type, pn.vlan_id, pn.vni, pn.status,
			count(distinct na.id) filter (where na.status <> 'detached') as attachment_count,
			pn.created_at, pn.updated_at, pn.deleted_at
		from private_networks pn
		left join network_attachments na on na.private_network_id = pn.id`
}

func networkAttachmentSelectSQL() string {
	return `
		select
			na.id, na.organization_id, na.cloud_id, na.private_network_id, na.resource_type, na.resource_id,
			coalesce(nullif(s.metadata->>'hostname', ''), vm.name, ms.name, na.resource_id::text) as resource_name,
			na.private_ip::text, na.mac_address::text, na.status, na.created_at, na.updated_at
		from network_attachments na
		left join servers s on na.resource_type = 'server' and s.id = na.resource_id
		left join virtual_machines vm on na.resource_type = 'virtual_machine' and vm.id = na.resource_id
		left join managed_services ms on na.resource_type = 'managed_service' and ms.id = na.resource_id`
}

func virtualMachineSelectSQL() string {
	return `
		select
			vm.id, vm.organization_id, vm.cloud_id, vm.host_server_id,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as host_server_name,
			vm.name, vm.hostname, vm.status, vm.power_state, vm.cpu_cores, vm.memory_mb, vm.disk_gb,
			vm.image_id, vm.os_image, vm.private_ip::text, vm.created_at, vm.updated_at, vm.deleted_at
		from virtual_machines vm
		left join servers s on s.id = vm.host_server_id`
}

func managedServiceSelectSQL() string {
	return `
		select
			ms.id, ms.organization_id, ms.cloud_id, ms.host_server_id,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as host_server_name,
			ms.service_type, ms.name, ms.status, ms.plan_name, ms.cpu_cores, ms.memory_mb, ms.storage_gb,
			ms.version, ms.endpoint_hostname, ms.private_ip::text, ms.port,
			coalesce(bp.enabled, false) as backup_enabled,
			coalesce(bp.retention_days, 0) as backup_retention_days,
			ms.created_at, ms.updated_at, ms.deleted_at
		from managed_services ms
		left join servers s on s.id = ms.host_server_id
		left join managed_service_backup_policies bp on bp.managed_service_id = ms.id`
}

func scanCloud(row pgx.Row) (Cloud, error) {
	var c Cloud
	err := row.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.Slug, &c.LocationID, &c.LocationName, &c.Description, &c.Status, &c.ServerCount, &c.VMCount, &c.ServiceCount, &c.NetworkCount, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt)
	return c, err
}

func scanClouds(rows pgx.Rows) ([]Cloud, error) {
	items := []Cloud{}
	for rows.Next() {
		item, err := scanCloud(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanCloudServer(row pgx.Row) (CloudServer, error) {
	var s CloudServer
	err := row.Scan(&s.ID, &s.CloudID, &s.Hostname, &s.Status, &s.LocationName, &s.ServerMode, &s.ModeStatus, &s.PlatformManaged, &s.ReservedCPUCores, &s.ReservedMemoryMB, &s.ReservedStorageGB, &s.TotalCPUCores, &s.TotalMemoryMB, &s.TotalStorageGB, &s.VMCount, &s.ServiceCount, &s.UpdatedAt)
	return s, err
}

func scanCloudServers(rows pgx.Rows) ([]CloudServer, error) {
	items := []CloudServer{}
	for rows.Next() {
		item, err := scanCloudServer(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanPrivateNetwork(row pgx.Row) (PrivateNetwork, error) {
	var n PrivateNetwork
	err := row.Scan(&n.ID, &n.OrganizationID, &n.CloudID, &n.Name, &n.Description, &n.CIDR, &n.GatewayIP, &n.NetworkType, &n.IsolationType, &n.VLANID, &n.VNI, &n.Status, &n.AttachmentCount, &n.CreatedAt, &n.UpdatedAt, &n.DeletedAt)
	return n, err
}

func scanPrivateNetworks(rows pgx.Rows) ([]PrivateNetwork, error) {
	items := []PrivateNetwork{}
	for rows.Next() {
		item, err := scanPrivateNetwork(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanNetworkAttachment(row pgx.Row) (NetworkAttachment, error) {
	var a NetworkAttachment
	err := row.Scan(&a.ID, &a.OrganizationID, &a.CloudID, &a.PrivateNetworkID, &a.ResourceType, &a.ResourceID, &a.ResourceName, &a.PrivateIP, &a.MACAddress, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func scanNetworkAttachments(rows pgx.Rows) ([]NetworkAttachment, error) {
	items := []NetworkAttachment{}
	for rows.Next() {
		item, err := scanNetworkAttachment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanVirtualMachine(row pgx.Row) (VirtualMachine, error) {
	var vm VirtualMachine
	err := row.Scan(&vm.ID, &vm.OrganizationID, &vm.CloudID, &vm.HostServerID, &vm.HostServerName, &vm.Name, &vm.Hostname, &vm.Status, &vm.PowerState, &vm.CPUCores, &vm.MemoryMB, &vm.DiskGB, &vm.ImageID, &vm.OSImage, &vm.PrivateIP, &vm.CreatedAt, &vm.UpdatedAt, &vm.DeletedAt)
	return vm, err
}

func scanVirtualMachines(rows pgx.Rows) ([]VirtualMachine, error) {
	items := []VirtualMachine{}
	for rows.Next() {
		item, err := scanVirtualMachine(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanManagedService(row pgx.Row) (ManagedService, error) {
	var ms ManagedService
	err := row.Scan(&ms.ID, &ms.OrganizationID, &ms.CloudID, &ms.HostServerID, &ms.HostServerName, &ms.ServiceType, &ms.Name, &ms.Status, &ms.PlanName, &ms.CPUCores, &ms.MemoryMB, &ms.StorageGB, &ms.Version, &ms.EndpointHostname, &ms.PrivateIP, &ms.Port, &ms.BackupEnabled, &ms.BackupRetentionDays, &ms.CreatedAt, &ms.UpdatedAt, &ms.DeletedAt)
	return ms, err
}

func scanManagedServices(rows pgx.Rows) ([]ManagedService, error) {
	items := []ManagedService{}
	for rows.Next() {
		item, err := scanManagedService(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanResourceAction(row pgx.Row) (ResourceAction, error) {
	var a ResourceAction
	err := row.Scan(&a.ID, &a.OrganizationID, &a.CloudID, &a.ResourceType, &a.ResourceID, &a.ActionType, &a.Status, &a.Message, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func scanResourceActions(rows pgx.Rows) ([]ResourceAction, error) {
	items := []ResourceAction{}
	for rows.Next() {
		item, err := scanResourceAction(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
