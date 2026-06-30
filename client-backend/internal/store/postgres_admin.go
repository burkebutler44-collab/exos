package store

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonSlugChars.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "rack"
	}
	return value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (s *PostgresStore) HasPlatformPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		select exists (
			select 1
			from platform_user_roles pur
			join platform_role_permissions prp on prp.role_id = pur.role_id
			where pur.user_id = $1 and prp.permission = $2
		)`, userID, permission).Scan(&ok)
	return ok, err
}

func (s *PostgresStore) GetPlatformSession(ctx context.Context, userID uuid.UUID) (PlatformSession, error) {
	roleRows, err := s.pool.Query(ctx, `
		select distinct r.name
		from platform_user_roles pur
		join platform_roles r on r.id = pur.role_id
		where pur.user_id = $1
		order by r.name`, userID)
	if err != nil {
		return PlatformSession{}, err
	}
	defer roleRows.Close()

	session := PlatformSession{
		Roles:       []string{},
		Permissions: []string{},
	}
	for roleRows.Next() {
		var role string
		if err := roleRows.Scan(&role); err != nil {
			return PlatformSession{}, err
		}
		session.Roles = append(session.Roles, role)
	}
	if err := roleRows.Err(); err != nil {
		return PlatformSession{}, err
	}

	permissionRows, err := s.pool.Query(ctx, `
		select distinct prp.permission
		from platform_user_roles pur
		join platform_role_permissions prp on prp.role_id = pur.role_id
		where pur.user_id = $1
		order by prp.permission`, userID)
	if err != nil {
		return PlatformSession{}, err
	}
	defer permissionRows.Close()

	for permissionRows.Next() {
		var permission string
		if err := permissionRows.Scan(&permission); err != nil {
			return PlatformSession{}, err
		}
		session.Permissions = append(session.Permissions, permission)
	}
	if err := permissionRows.Err(); err != nil {
		return PlatformSession{}, err
	}

	return session, nil
}

func (s *PostgresStore) AddAdminAuditLog(ctx context.Context, actorUserID uuid.UUID, action, targetType, targetID, reason string, metadata []byte) error {
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	_, err := s.pool.Exec(ctx, `
		insert into admin_audit_log (actor_user_id, action, target_type, target_id, reason, metadata)
		values ($1, $2, $3, $4, $5, $6)`, actorUserID, action, targetType, targetID, reason, metadata)
	return err
}

func (s *PostgresStore) ListAdminUsers(ctx context.Context) ([]AdminUserListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			u.id,
			u.email,
			u.name,
			u.auth0_sub,
			coalesce(array_remove(array_agg(distinct pr.name), null), '{}'::text[]) as platform_roles,
			count(distinct m.organization_id) filter (where m.status = 'active') as organization_count,
			'active' as status,
			u.created_at,
			null::timestamptz as last_login_at
		from users u
		left join organization_memberships m on m.user_id = u.id
		left join platform_user_roles pur on pur.user_id = u.id
		left join platform_roles pr on pr.id = pur.role_id
		group by u.id
		order by u.created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminUserListItem{}
	for rows.Next() {
		var item AdminUserListItem
		if err := rows.Scan(&item.ID, &item.Email, &item.Name, &item.AuthProviderSubject, &item.PlatformRoles, &item.OrganizationCount, &item.Status, &item.CreatedAt, &item.LastLoginAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminOrganizations(ctx context.Context) ([]AdminOrganizationListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			o.id,
			o.name,
			o.slug,
			case when coalesce(ba.status, 'active') = 'suspended' then 'suspended' else 'active' end as status,
			coalesce(ba.status, 'active') as billing_status,
			coalesce(ba.billing_email, bp.billing_email, '') as billing_email,
			count(distinct m.user_id) filter (where m.status = 'active') as member_count,
			count(distinct s.id) filter (where s.status = 'active') as active_server_count,
			o.created_at
		from organizations o
		left join billing_accounts ba on ba.organization_id = o.id
		left join billing_profiles bp on bp.organization_id = o.id
		left join organization_memberships m on m.organization_id = o.id
		left join servers s on s.organization_id = o.id
		group by o.id, ba.status, ba.billing_email, bp.billing_email
		order by o.created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminOrganizationListItem{}
	for rows.Next() {
		var item AdminOrganizationListItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Status, &item.BillingStatus, &item.BillingEmail, &item.MemberCount, &item.ActiveServerCount, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminUserOrganizations(ctx context.Context, userID uuid.UUID) ([]AdminOrganizationListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			o.id,
			o.name,
			o.slug,
			case when coalesce(ba.status, 'active') = 'suspended' then 'suspended' else 'active' end as status,
			coalesce(ba.status, 'active') as billing_status,
			coalesce(ba.billing_email, bp.billing_email, '') as billing_email,
			count(distinct m2.user_id) filter (where m2.status = 'active') as member_count,
			count(distinct s.id) filter (where s.status = 'active') as active_server_count,
			o.created_at
		from organization_memberships m
		join organizations o on o.id = m.organization_id
		left join billing_accounts ba on ba.organization_id = o.id
		left join billing_profiles bp on bp.organization_id = o.id
		left join organization_memberships m2 on m2.organization_id = o.id
		left join servers s on s.organization_id = o.id
		where m.user_id = $1 and m.status = 'active'
		group by o.id, ba.status, ba.billing_email, bp.billing_email
		order by o.created_at desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminOrganizationListItem{}
	for rows.Next() {
		var item AdminOrganizationListItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Status, &item.BillingStatus, &item.BillingEmail, &item.MemberCount, &item.ActiveServerCount, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminBillingAccounts(ctx context.Context) ([]AdminBillingAccountListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			ba.id,
			o.name,
			ba.billing_email,
			ba.status,
			ba.payment_terms,
			ba.credit_balance_cents,
			ba.stripe_customer_id
		from billing_accounts ba
		join organizations o on o.id = ba.organization_id
		order by ba.updated_at desc, ba.created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminBillingAccountListItem{}
	for rows.Next() {
		var item AdminBillingAccountListItem
		if err := rows.Scan(&item.ID, &item.OrganizationName, &item.BillingEmail, &item.Status, &item.PaymentTerms, &item.CreditBalanceCents, &item.StripeCustomerID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminServers(ctx context.Context) ([]AdminServerListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			s.id,
			s.hostname,
			s.asset_tag,
			s.serial_number,
			s.status,
			r.id as rack_id,
			r.name as rack_name,
			s.rack_position,
			s.location_id,
			coalesce(l.name, r.location) as location_name,
			s.server_family_id,
			sf.display_name as server_family_name,
			s.installed_memory_gb,
			o.name as organization_name,
			p.name as project_name,
			s.lifecycle_status,
			s.allocation_status,
			s.health_status,
			s.provisionable,
			s.notes,
			s.created_at,
			s.updated_at
		from servers s
		join racks r on r.id = s.rack_id
		left join locations l on l.id = s.location_id
		left join server_families sf on sf.id = s.server_family_id
		left join organizations o on o.id = s.organization_id
		left join projects p on p.id = s.project_id
		order by s.updated_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminServerListItem{}
	for rows.Next() {
		var item AdminServerListItem
		if err := rows.Scan(
			&item.ID,
			&item.Hostname,
			&item.AssetTag,
			&item.SerialNumber,
			&item.Status,
			&item.RackID,
			&item.RackName,
			&item.RackPosition,
			&item.LocationID,
			&item.LocationName,
			&item.ServerFamilyID,
			&item.ServerFamilyName,
			&item.InstalledMemoryGB,
			&item.OrganizationName,
			&item.ProjectName,
			&item.LifecycleStatus,
			&item.AllocationStatus,
			&item.HealthStatus,
			&item.Provisionable,
			&item.Notes,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) CreateAdminServer(ctx context.Context, params CreateAdminServerParams) (AdminServerListItem, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AdminServerListItem{}, err
	}
	defer tx.Rollback(ctx)

	hostname := params.Hostname
	if hostname == "" {
		hostname = params.Hostname
	}

	var locationName, locationKeyword string
	if err := tx.QueryRow(ctx, `
		select name, coalesce(nullif(keyword, ''), upper(code))
		from locations
		where id = $1`, params.LocationID).Scan(&locationName, &locationKeyword); err != nil {
		return AdminServerListItem{}, err
	}
	rackID := params.RackID
	if rackID == "" {
		rackID = normalizeSlug(locationKeyword)
	}
	rackName := rackID
	if locationKeyword != "" {
		rackName = locationKeyword + " Rack"
	}

	_, err = tx.Exec(ctx, `
		insert into racks (id, name, location, location_id, status)
		values ($1, $2, $3, $4, 'offline')
		on conflict (id) do update
		set
			location_id = excluded.location_id,
			location = excluded.location,
			updated_at = now()`,
		rackID, rackName, locationName, params.LocationID)
	if err != nil {
		return AdminServerListItem{}, err
	}

	var serverID uuid.UUID
	if err := tx.QueryRow(ctx, `
		insert into servers (rack_id, server_family_id, location_id, status, hostname, asset_tag, serial_number,
			installed_memory_gb, rack_position, provisionable, notes, bmc_address, mac_address, metadata)
		values ($1, $2, $3, 'available', $4, $5, $6, $7, $8, $9, $10, '', '', '{}'::jsonb)
		returning id`,
		rackID, params.ServerFamilyID, params.LocationID,
		hostname, params.AssetTag, params.SerialNumber,
		params.InstalledMemoryGB, params.RackPosition, params.Provisionable, params.Notes).Scan(&serverID); err != nil {
		return AdminServerListItem{}, err
	}

	// Insert network interfaces
	for _, nic := range params.NetworkInterfaces {
		_, err = tx.Exec(ctx, `
			insert into server_network_interfaces
				(server_id, label, mac_address, speed_mbps, is_public, ip_address, gateway, prefix_length, vlan_id,
				 switch_id, switch_port, purpose, notes)
			values
				($1, $2, $3::macaddr, $4, $5, nullif($6, '')::inet, nullif($7, '')::inet, $8, $9,
				 $10, $11, $12, $13)`,
			serverID, nic.Label, nic.MACAddress, nic.SpeedMbps, nic.IsPublic,
			stringValue(nic.IPAddress), stringValue(nic.Gateway), nic.PrefixLength, nic.VLANID,
			nic.SwitchID, nic.SwitchPort, nic.Purpose, nic.Notes)
		if err != nil {
			return AdminServerListItem{}, err
		}
	}

	// Insert disks
	for _, disk := range params.Disks {
		_, err = tx.Exec(ctx, `
			insert into server_disks
				(server_id, device_name, capacity_gb, media_type, interface_type, manufacturer, model, serial_number, boot_capable)
			values
				($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			serverID, disk.DeviceName, disk.CapacityGB, disk.MediaType, disk.InterfaceType,
			disk.Manufacturer, disk.Model, disk.SerialNumber, disk.BootCapable)
		if err != nil {
			return AdminServerListItem{}, err
		}
	}

	// Insert BMC
	if params.BMC != nil {
		_, err = tx.Exec(ctx, `
			insert into bmc_management
				(server_id, management_ip, username, password, protocol, vendor)
			values
				($1, nullif($2, '')::inet, $3, $4, $5, $6)`,
			serverID, params.BMC.ManagementIP, params.BMC.Username, params.BMC.Password,
			params.BMC.Protocol, params.BMC.Vendor)
		if err != nil {
			return AdminServerListItem{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return AdminServerListItem{}, err
	}

	return s.GetAdminServerListItem(ctx, serverID)
}

func (s *PostgresStore) GetAdminServerListItem(ctx context.Context, serverID uuid.UUID) (AdminServerListItem, error) {
	var item AdminServerListItem
	err := s.pool.QueryRow(ctx, `
		select
			s.id,
			s.hostname,
			s.asset_tag,
			s.serial_number,
			s.status,
			r.id as rack_id,
			r.name as rack_name,
			s.rack_position,
			s.location_id,
			coalesce(l.name, r.location) as location_name,
			s.server_family_id,
			sf.display_name as server_family_name,
			s.installed_memory_gb,
			o.name as organization_name,
			p.name as project_name,
			s.lifecycle_status,
			s.allocation_status,
			s.health_status,
			s.provisionable,
			s.notes,
			s.created_at,
			s.updated_at
		from servers s
		join racks r on r.id = s.rack_id
		left join locations l on l.id = s.location_id
		left join server_families sf on sf.id = s.server_family_id
		left join organizations o on o.id = s.organization_id
		left join projects p on p.id = s.project_id
		where s.id = $1`, serverID).Scan(
		&item.ID,
		&item.Hostname,
		&item.AssetTag,
		&item.SerialNumber,
		&item.Status,
		&item.RackID,
		&item.RackName,
		&item.RackPosition,
		&item.LocationID,
		&item.LocationName,
		&item.ServerFamilyID,
		&item.ServerFamilyName,
		&item.InstalledMemoryGB,
		&item.OrganizationName,
		&item.ProjectName,
		&item.LifecycleStatus,
		&item.AllocationStatus,
		&item.HealthStatus,
		&item.Provisionable,
		&item.Notes,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return AdminServerListItem{}, err
	}
	return item, nil
}

func (s *PostgresStore) ListAdminRacks(ctx context.Context) ([]AdminRackListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			r.id,
			r.name,
			coalesce(l.name, r.location) as location,
			coalesce(nullif(l.keyword, ''), upper(nullif(l.code, '')), upper(r.location)) as location_code,
			r.status,
			r.last_heartbeat_at,
			coalesce(ra.version, '') as agent_version,
			count(distinct s.id) filter (where s.status = 'available') as available_servers,
			count(distinct s.id) filter (where s.status = 'active') as active_servers,
			count(distinct pj.id) filter (where pj.status = 'failed') as failed_jobs
		from racks r
		left join lateral (
			select version
			from rack_agents
			where rack_id = r.id
			order by last_heartbeat_at desc nulls last, updated_at desc
			limit 1
		) ra on true
		left join locations l on l.id = r.location_id
		left join servers s on s.rack_id = r.id
		left join provisioning_jobs pj on pj.rack_id = r.id
		group by r.id, l.name, l.keyword, l.code, ra.version
		order by r.location, r.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminRackListItem{}
	for rows.Next() {
		var item AdminRackListItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Location, &item.LocationCode, &item.Status, &item.LastHeartbeatAt, &item.AgentVersion, &item.AvailableServers, &item.ActiveServers, &item.FailedJobs); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminLocations(ctx context.Context) ([]AdminLocationListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select id, code, coalesce(nullif(keyword, ''), upper(code)) as keyword, name, city, region, country
		from locations
		order by name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminLocationListItem{}
	for rows.Next() {
		var item AdminLocationListItem
		if err := rows.Scan(&item.ID, &item.Code, &item.Keyword, &item.Name, &item.City, &item.Region, &item.Country); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminCPUProfiles(ctx context.Context) ([]AdminCPUProfileListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			id,
			name,
			vendor,
			model,
			socket_count,
			core_count,
			thread_count,
			base_clock_ghz::float8,
			boost_clock_ghz::float8,
			architecture,
			metadata
		from cpu_profiles
		order by vendor, model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminCPUProfileListItem{}
	for rows.Next() {
		var item AdminCPUProfileListItem
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Vendor, &item.Model, &item.SocketCount, &item.CoreCount, &item.ThreadCount, &item.BaseClockGHz, &item.BoostClockGHz, &item.Architecture, &metadata); err != nil {
			return nil, err
		}
		item.Metadata = map[string]any{}
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminServerFamilies(ctx context.Context) ([]AdminServerFamilyListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			id,
			display_name,
			slug,
			cpu_manufacturer,
			cpu_model,
			core_count,
			thread_count,
			base_clock_ghz::float8,
			boost_clock_ghz::float8,
			workload_category,
			active,
			display_order
		from server_families
		order by display_order, display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminServerFamilyListItem{}
	for rows.Next() {
		var item AdminServerFamilyListItem
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Slug, &item.CPUManufacturer, &item.CPUModel, &item.CoreCount, &item.ThreadCount, &item.BaseClockGHz, &item.BoostClockGHz, &item.WorkloadCategory, &item.Active, &item.DisplayOrder); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminOSImages(ctx context.Context) ([]AdminOSImageListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			id,
			name,
			slug,
			version,
			family,
			architecture,
			enabled,
			is_default,
			tinkerbell_template_ref,
			coalesce(metadata->>'artifact_name', slug) as artifact_name,
			coalesce(metadata->>'artifact_file', '') as artifact_file,
			metadata
		from os_images
		order by is_default desc, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminOSImageListItem{}
	for rows.Next() {
		var item AdminOSImageListItem
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Version, &item.Family, &item.Architecture, &item.Enabled, &item.IsDefault, &item.TinkerbellTemplateRef, &item.ArtifactName, &item.ArtifactFile, &metadata); err != nil {
			return nil, err
		}
		item.Metadata = map[string]any{}
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminSwitches(ctx context.Context) ([]AdminSwitchListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			sw.id,
			sw.label,
			sw.ip_address::text,
			sw.management_ip::text,
			sw.location_id,
			l.name as location_name,
			sw.rack_id,
			r.name as rack_name,
			sw.vendor,
			sw.model,
			sw.serial_number,
			sw.port_count,
			sw.default_port_speed,
			sw.status,
			sw.updated_at
		from network_switches sw
		join locations l on l.id = sw.location_id
		left join racks r on r.id = sw.rack_id
		order by l.name, sw.label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminSwitchListItem{}
	for rows.Next() {
		var item AdminSwitchListItem
		if err := rows.Scan(&item.ID, &item.Label, &item.IPAddress, &item.ManagementIP, &item.LocationID, &item.LocationName, &item.RackID, &item.RackName, &item.Vendor, &item.Model, &item.SerialNumber, &item.PortCount, &item.DefaultPortSpeed, &item.Status, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminEdgeRouters(ctx context.Context) ([]AdminEdgeRouterListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			er.id,
			er.label,
			er.ip_address::text,
			er.management_ip::text,
			er.location_id,
			l.name as location_name,
			er.vendor,
			er.model,
			er.serial_number,
			coalesce(er.asn, 0) as asn,
			er.upstream_isps,
			er.port_count,
			er.port_speed,
			er.status,
			er.notes,
			er.updated_at
		from edge_routers er
		join locations l on l.id = er.location_id
		order by l.name, er.label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminEdgeRouterListItem{}
	for rows.Next() {
		var item AdminEdgeRouterListItem
		var upstream []byte
		if err := rows.Scan(&item.ID, &item.Label, &item.IPAddress, &item.ManagementIP, &item.LocationID, &item.LocationName, &item.Vendor, &item.Model, &item.SerialNumber, &item.ASN, &upstream, &item.PortCount, &item.PortSpeed, &item.Status, &item.Notes, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.UpstreamISPs = []string{}
		_ = json.Unmarshal(upstream, &item.UpstreamISPs)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminServerNetworkInterfaces(ctx context.Context) ([]AdminServerNetworkInterfaceListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			ni.id,
			ni.server_id,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as server_name,
			ni.switch_id,
			sw.label as switch_label,
			coalesce(l.name, r.location) as location_name,
			ni.label,
			ni.mac_address::text,
			ni.ip_address::text,
			ni.gateway::text,
			ni.subnet_mask,
			ni.switch_port,
			ni.vlan_id,
			ni.is_primary,
			ni.updated_at
		from server_network_interfaces ni
		join servers s on s.id = ni.server_id
		join racks r on r.id = s.rack_id
		left join network_switches sw on sw.id = ni.switch_id
		left join locations l on l.id = sw.location_id
		order by coalesce(l.name, r.location), server_name, ni.is_primary desc, ni.label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminServerNetworkInterfaceListItem{}
	for rows.Next() {
		var item AdminServerNetworkInterfaceListItem
		if err := rows.Scan(&item.ID, &item.ServerID, &item.ServerName, &item.SwitchID, &item.SwitchLabel, &item.LocationName, &item.Label, &item.MACAddress, &item.IPAddress, &item.Gateway, &item.SubnetMask, &item.SwitchPort, &item.VLANID, &item.IsPrimary, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminProvisioningJobs(ctx context.Context) ([]AdminProvisioningJobListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			pj.id,
			coalesce(nullif(s.metadata->>'hostname', ''), pj.server_id::text) as server,
			o.name as organization,
			coalesce(r.name, pj.rack_id) as rack,
			pj.image_id,
			pj.status,
			u.email as requested_by,
			pj.started_at,
			pj.completed_at,
			pj.failure_reason
		from provisioning_jobs pj
		join organizations o on o.id = pj.organization_id
		join users u on u.id = pj.requested_by_user_id
		join servers s on s.id = pj.server_id
		left join racks r on r.id = pj.rack_id
		order by pj.created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminProvisioningJobListItem{}
	for rows.Next() {
		var item AdminProvisioningJobListItem
		if err := rows.Scan(&item.ID, &item.Server, &item.Organization, &item.Rack, &item.Image, &item.Status, &item.RequestedBy, &item.StartedAt, &item.CompletedAt, &item.FailureReason); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListAdminAuditEvents(ctx context.Context) ([]AdminAuditEventListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			a.id,
			coalesce(u.email, 'system') as actor,
			a.action,
			a.target_type || ':' || a.target_id as target,
			a.organization_id,
			a.server_id,
			a.created_at,
			a.metadata
		from admin_audit_log a
		left join users u on u.id = a.actor_user_id
		order by a.created_at desc
		limit 250`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminAuditEventListItem{}
	for rows.Next() {
		var item AdminAuditEventListItem
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.Actor, &item.Action, &item.Target, &item.Organization, &item.Server, &item.CreatedAt, &metadata); err != nil {
			return nil, err
		}
		item.Metadata = map[string]string{}
		var raw map[string]any
		if err := json.Unmarshal(metadata, &raw); err == nil {
			for key, value := range raw {
				if text, ok := value.(string); ok {
					item.Metadata[key] = text
				}
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) AdminAssignServer(ctx context.Context, serverID, organizationID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`update servers set organization_id = $2 where id = $1`,
		serverID, organizationID,
	)
	if err != nil {
		return fmt.Errorf("assign server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) AdminReleaseServer(ctx context.Context, serverID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`update servers set organization_id = null, project_id = null where id = $1`,
		serverID,
	)
	if err != nil {
		return fmt.Errorf("release server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) AdminRetireServer(ctx context.Context, serverID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`update servers set status = 'terminated', lifecycle_status = 'retired' where id = $1`,
		serverID,
	)
	if err != nil {
		return fmt.Errorf("retire server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
