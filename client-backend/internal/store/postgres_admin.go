package store

import (
	"context"
	"encoding/json"
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
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as hostname,
			coalesce(s.metadata->>'asset_tag', '') as asset_tag,
			coalesce(s.metadata->>'serial_number', '') as serial_number,
			s.status,
			r.id as rack_id,
			r.name as rack_name,
			r.location as location_name,
			o.name as organization_name,
			p.name as project_name,
			coalesce(nullif(s.metadata->>'hardware_profile_name', ''), nullif(s.metadata->>'sku', ''), 'Unspecified') as hardware_profile_name,
			nullif(s.metadata->>'public_ip', '') as public_ip,
			s.bmc_address,
			s.mac_address as primary_mac_address,
			s.provisionable,
			s.updated_at
		from servers s
		join racks r on r.id = s.rack_id
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
		if err := rows.Scan(&item.ID, &item.Hostname, &item.AssetTag, &item.SerialNumber, &item.Status, &item.RackID, &item.RackName, &item.LocationName, &item.OrganizationName, &item.ProjectName, &item.HardwareProfileName, &item.PublicIP, &item.BMCAddress, &item.PrimaryMACAddress, &item.Provisionable, &item.UpdatedAt); err != nil {
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

	var locationName, locationKeyword string
	if err := tx.QueryRow(ctx, `
		select name, coalesce(nullif(keyword, ''), upper(code))
		from locations
		where id = $1`, params.LocationID).Scan(&locationName, &locationKeyword); err != nil {
		return AdminServerListItem{}, err
	}
	if params.RackID == "" {
		params.RackID = normalizeSlug(locationKeyword)
	}
	rackName := params.RackID
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
		params.RackID, rackName, locationName, params.LocationID)
	if err != nil {
		return AdminServerListItem{}, err
	}

	if params.CPUProfileID != nil {
		if err := tx.QueryRow(ctx, `
			select name, socket_count, core_count
			from cpu_profiles
			where id = $1`, *params.CPUProfileID).Scan(&params.CPUModel, &params.CPUCount, &params.CoreCount); err != nil {
			return AdminServerListItem{}, err
		}
	}

	metadata := map[string]any{
		"hostname":              params.Hostname,
		"asset_tag":             params.AssetTag,
		"serial_number":         params.SerialNumber,
		"hardware_profile_name": params.HardwareProfileName,
		"cpu_profile_id":        params.CPUProfileID,
		"cpu_model":             params.CPUModel,
		"cpu_count":             params.CPUCount,
		"core_count":            params.CoreCount,
		"ram_gb":                params.RAMGB,
		"disk_name":             params.DiskName,
		"disk_description":      params.DiskDescription,
		"nic_description":       params.NICDescription,
		"ipmi_username":         params.IPMIUsername,
		"ipmi_password":         params.IPMIPassword,
		"hourly_price_cents":    params.HourlyPriceCents,
		"monthly_price_cents":   params.MonthlyPriceCents,
		"quarterly_price_cents": params.QuarterlyPriceCents,
		"yearly_price_cents":    params.YearlyPriceCents,
		"location_keyword":      locationKeyword,
		"notes":                 params.Notes,
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return AdminServerListItem{}, err
	}

	var serverID uuid.UUID
	if err := tx.QueryRow(ctx, `
		insert into servers (rack_id, status, bmc_address, mac_address, provisionable, metadata)
		values ($1, 'available', $2, $3, $4, $5)
		returning id`,
		params.RackID, params.BMCAddress, params.MACAddress, params.Provisionable, metadataBytes).Scan(&serverID); err != nil {
		return AdminServerListItem{}, err
	}

	_, err = tx.Exec(ctx, `
		insert into server_network_interfaces
			(server_id, label, mac_address, ip_address, gateway, subnet_mask, switch_port, vlan_id, is_primary)
		values
			($1, 'primary', $2, nullif($3, '')::inet, nullif($4, '')::inet, $5, '', $6, true)`,
		serverID, params.MACAddress, stringValue(params.IPAddress), stringValue(params.Gateway), stringValue(params.SubnetMask), params.VLANID)
	if err != nil {
		return AdminServerListItem{}, err
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
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as hostname,
			coalesce(s.metadata->>'asset_tag', '') as asset_tag,
			coalesce(s.metadata->>'serial_number', '') as serial_number,
			s.status,
			r.id as rack_id,
			r.name as rack_name,
			coalesce(l.name, r.location) as location_name,
			o.name as organization_name,
			p.name as project_name,
			coalesce(s.metadata->>'hardware_profile_name', '') as hardware_profile_name,
			ni.ip_address::text as public_ip,
			s.bmc_address,
			coalesce(nullif(ni.mac_address::text, ''), s.mac_address) as primary_mac_address,
			s.provisionable,
			s.updated_at
		from servers s
		join racks r on r.id = s.rack_id
		left join locations l on l.id = r.location_id
		left join organizations o on o.id = s.organization_id
		left join projects p on p.id = s.project_id
		left join lateral (
			select *
			from server_network_interfaces
			where server_id = s.id
			order by is_primary desc, created_at asc
			limit 1
		) ni on true
		where s.id = $1`, serverID).Scan(&item.ID, &item.Hostname, &item.AssetTag, &item.SerialNumber, &item.Status, &item.RackID, &item.RackName, &item.LocationName, &item.OrganizationName, &item.ProjectName, &item.HardwareProfileName, &item.PublicIP, &item.BMCAddress, &item.PrimaryMACAddress, &item.Provisionable, &item.UpdatedAt)
	return item, err
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
