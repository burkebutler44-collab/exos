package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"relay/client-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var storageTokenPattern = regexp.MustCompile(`(?i)(?:(\d+)\s*x\s*)?(\d+(?:\.\d+)?)\s*(tb|gb)`)

const monthlyBillingHours int64 = 730

type catalogServerRow struct {
	ID                  uuid.UUID
	ServerFamilyID      uuid.UUID
	FamilyDisplayName   string
	FamilySlug          string
	CPUManufacturer     string
	FamilyCPUModel      string
	FamilyCoreCount     int32
	FamilyThreadCount   int32
	BaseClockGHz        float64
	BoostClockGHz       float64
	Generation          string
	WorkloadCategory    string
	FamilyDescription   string
	FeatureBadges       []string
	DisplayOrder        int32
	RackID              string
	Hostname            string
	LocationCode        string
	LocationName        string
	HardwareProfileName string
	Metadata            map[string]any
}

func (s *PostgresStore) ListServerCatalog(ctx context.Context) (ServerCatalog, error) {
	rows, err := s.pool.Query(ctx, `
		select
			s.id,
			sf.id,
			sf.display_name,
			sf.slug,
			sf.cpu_manufacturer,
			sf.cpu_model,
			sf.core_count,
			sf.thread_count,
			coalesce(sf.base_clock_ghz, 0)::float8,
			coalesce(sf.boost_clock_ghz, 0)::float8,
			sf.generation,
			sf.workload_category,
			sf.description,
			sf.feature_badges,
			sf.display_order,
			s.rack_id,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as hostname,
			coalesce(nullif(l.keyword, ''), upper(nullif(l.code, '')), upper(r.location)) as location_code,
			coalesce(l.name, r.location) as location_name,
			coalesce(nullif(s.metadata->>'hardware_profile_name', ''), nullif(s.metadata->>'sku', ''), 'Dedicated server') as hardware_profile_name,
			s.metadata
		from servers s
		join server_families sf on sf.id = s.server_family_id and sf.active = true
		join racks r on r.id = s.rack_id
		left join locations l on l.id = r.location_id
		where s.status = 'available'
			and s.organization_id is null
			and s.provisionable = true
		order by location_name, hardware_profile_name, hostname`)
	if err != nil {
		return ServerCatalog{}, err
	}
	defer rows.Close()

	configs := []ServerCatalogConfiguration{}
	for rows.Next() {
		var row catalogServerRow
		var metadata []byte
		var badges []byte
		if err := rows.Scan(
			&row.ID,
			&row.ServerFamilyID,
			&row.FamilyDisplayName,
			&row.FamilySlug,
			&row.CPUManufacturer,
			&row.FamilyCPUModel,
			&row.FamilyCoreCount,
			&row.FamilyThreadCount,
			&row.BaseClockGHz,
			&row.BoostClockGHz,
			&row.Generation,
			&row.WorkloadCategory,
			&row.FamilyDescription,
			&badges,
			&row.DisplayOrder,
			&row.RackID,
			&row.Hostname,
			&row.LocationCode,
			&row.LocationName,
			&row.HardwareProfileName,
			&metadata,
		); err != nil {
			return ServerCatalog{}, err
		}
		row.Metadata = map[string]any{}
		_ = json.Unmarshal(metadata, &row.Metadata)
		_ = json.Unmarshal(badges, &row.FeatureBadges)
		configs = append(configs, catalogConfigurationFromRow(row))
	}
	if err := rows.Err(); err != nil {
		return ServerCatalog{}, err
	}
	options, err := s.listCatalogHardwareOptions(ctx)
	if err != nil {
		return ServerCatalog{}, err
	}
	attachHardwareOptions(configs, options)
	return groupCatalogPlans(configs), nil
}

func (s *PostgresStore) listCatalogHardwareOptions(ctx context.Context) (map[uuid.UUID][]ServerCatalogHardwareOption, error) {
	rows, err := s.pool.Query(ctx, catalogHardwareOptionsQuery("", ""))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	options := map[uuid.UUID][]ServerCatalogHardwareOption{}
	for rows.Next() {
		serverID, option, err := scanCatalogHardwareOption(rows)
		if err != nil {
			return nil, err
		}
		options[serverID] = append(options[serverID], option)
	}
	return options, rows.Err()
}

func listCatalogHardwareOptionsForServerTx(ctx context.Context, tx pgx.Tx, serverID uuid.UUID, optionIDs []uuid.UUID) ([]ServerCatalogHardwareOption, error) {
	optionIDs = uniqueUUIDs(optionIDs)
	if len(optionIDs) == 0 {
		return nil, nil
	}

	rows, err := tx.Query(ctx, catalogHardwareOptionsQuery("and o.id = any($1) and s.id = $2", "for update of o"), optionIDs, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	options := []ServerCatalogHardwareOption{}
	for rows.Next() {
		_, option, err := scanCatalogHardwareOption(rows)
		if err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if len(options) != len(optionIDs) {
		return nil, ErrInvalidInput
	}
	for _, option := range options {
		if option.QuantityAvailable < 1 {
			return nil, ErrInvalidInput
		}
		tag, err := tx.Exec(ctx, `
			update hardware_option_inventory
			set quantity_available = quantity_available - 1,
				updated_at = now()
			where id = $1
				and quantity_available > 0`, option.ID)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, ErrInvalidInput
		}
	}
	return options, nil
}

func catalogHardwareOptionsQuery(extraWhere, lockClause string) string {
	return `
		with available_servers as (
			select
				s.id,
				r.location_id,
				coalesce(nullif(l.keyword, ''), upper(nullif(l.code, '')), upper(r.location)) as location_code,
				coalesce(l.name, r.location) as location_name,
				coalesce(nullif(s.metadata->>'hardware_profile_name', ''), nullif(s.metadata->>'sku', ''), 'Dedicated server') as hardware_profile_name,
				coalesce(nullif(s.metadata->>'cpu_model', ''), nullif(s.metadata->>'cpu', ''), nullif(s.metadata->>'processor', ''), coalesce(nullif(s.metadata->>'hardware_profile_name', ''), nullif(s.metadata->>'sku', ''), 'Dedicated server')) as cpu_model
			from servers s
			join racks r on r.id = s.rack_id
			left join locations l on l.id = r.location_id
			where s.status = 'available'
				and s.organization_id is null
				and s.provisionable = true
		)
		select
			s.id as server_id,
			o.id,
			o.option_type,
			o.label,
			o.description,
			o.unit,
			o.value_text,
			o.value_gb,
			o.price_delta_cents,
			o.hourly_price_delta_cents,
			o.quarterly_price_delta_cents,
			o.yearly_price_delta_cents,
			o.currency,
			o.quantity_available,
			o.fulfillment_mode,
			o.estimated_ready_min_hours,
			o.estimated_ready_max_hours,
			coalesce(nullif(lo.keyword, ''), upper(nullif(lo.code, '')), s.location_code) as option_location_code,
			coalesce(lo.name, s.location_name) as option_location_name
		from available_servers s
		join hardware_option_inventory o on o.active = true
			and o.quantity_available > 0
		left join locations lo on lo.id = o.location_id
		left join server_catalog_option_overrides override on override.server_id = s.id and override.hardware_option_id = o.id
		where coalesce(override.compatible, true) = true
			and (
				override.compatible = true
				or (
					(o.location_id is null or o.location_id = s.location_id)
					and (o.hardware_profile_name = '' or lower(o.hardware_profile_name) = lower(s.hardware_profile_name))
					and (o.cpu_model = '' or lower(o.cpu_model) = lower(s.cpu_model))
				)
			)
			` + extraWhere + `
		order by o.option_type, o.value_gb nulls last, o.label
		` + lockClause
}

func scanCatalogHardwareOption(rows pgx.Rows) (uuid.UUID, ServerCatalogHardwareOption, error) {
	var serverID uuid.UUID
	var option ServerCatalogHardwareOption
	var valueGB pgtype.Int4
	var locationCode pgtype.Text
	var locationName pgtype.Text
	if err := rows.Scan(
		&serverID,
		&option.ID,
		&option.OptionType,
		&option.Label,
		&option.Description,
		&option.Unit,
		&option.ValueText,
		&valueGB,
		&option.PriceDeltaCents,
		&option.HourlyPriceDeltaCents,
		&option.QuarterlyDeltaCents,
		&option.YearlyDeltaCents,
		&option.Currency,
		&option.QuantityAvailable,
		&option.FulfillmentMode,
		&option.EstimatedReadyMinHours,
		&option.EstimatedReadyMaxHours,
		&locationCode,
		&locationName,
	); err != nil {
		return uuid.Nil, ServerCatalogHardwareOption{}, err
	}
	if valueGB.Valid {
		value := valueGB.Int32
		option.ValueGB = &value
	}
	if locationCode.Valid {
		value := strings.ToUpper(locationCode.String)
		option.LocationCode = &value
	}
	if locationName.Valid {
		value := locationName.String
		option.LocationName = &value
	}
	option.RequiresInstall = option.FulfillmentMode == "requires_install" || option.FulfillmentMode == "special_order" || option.FulfillmentMode == "manual"
	option.Active = true
	return serverID, option, nil
}

func (s *PostgresStore) ListHardwareOptions(ctx context.Context) ([]ServerCatalogHardwareOption, error) {
	rows, err := s.pool.Query(ctx, `
		select
			o.id,
			o.option_type,
			o.label,
			o.description,
			o.unit,
			o.value_text,
			o.value_gb,
			o.price_delta_cents,
			o.hourly_price_delta_cents,
			o.quarterly_price_delta_cents,
			o.yearly_price_delta_cents,
			o.currency,
			o.quantity_available,
			o.fulfillment_mode,
			o.estimated_ready_min_hours,
			o.estimated_ready_max_hours,
			o.location_id,
			coalesce(nullif(l.keyword, ''), upper(nullif(l.code, ''))) as location_code,
			l.name as location_name,
			o.hardware_profile_name,
			o.cpu_model,
			o.active,
			o.created_at,
			o.updated_at
		from hardware_option_inventory o
		left join locations l on l.id = o.location_id
		order by o.active desc, o.option_type, o.value_gb nulls last, o.label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ServerCatalogHardwareOption{}
	for rows.Next() {
		item, err := scanAdminHardwareOption(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) CreateHardwareOption(ctx context.Context, params CreateHardwareOptionParams) (ServerCatalogHardwareOption, error) {
	if params.Currency == "" {
		params.Currency = "usd"
	}
	if params.Unit == "" {
		params.Unit = "each"
	}
	if params.FulfillmentMode == "" {
		params.FulfillmentMode = "available"
	}
	item, err := scanAdminHardwareOption(s.pool.QueryRow(ctx, `
		insert into hardware_option_inventory (
			option_type, label, description, unit, value_text, value_gb,
			price_delta_cents, hourly_price_delta_cents, quarterly_price_delta_cents, yearly_price_delta_cents,
			currency, quantity_available, fulfillment_mode, estimated_ready_min_hours, estimated_ready_max_hours,
			location_id, hardware_profile_name, cpu_model, active
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		returning
			id, option_type, label, description, unit, value_text, value_gb,
			price_delta_cents, hourly_price_delta_cents, quarterly_price_delta_cents, yearly_price_delta_cents,
			currency, quantity_available, fulfillment_mode, estimated_ready_min_hours, estimated_ready_max_hours,
			location_id, null::text as location_code, null::text as location_name,
			hardware_profile_name, cpu_model, active, created_at, updated_at`,
		params.OptionType, params.Label, params.Description, params.Unit, params.ValueText, params.ValueGB,
		params.PriceDeltaCents, params.HourlyPriceDeltaCents, params.QuarterlyDeltaCents, params.YearlyDeltaCents,
		params.Currency, params.QuantityAvailable, params.FulfillmentMode, params.EstimatedReadyMinHours, params.EstimatedReadyMaxHours,
		params.LocationID, params.HardwareProfileName, params.CPUModel, params.Active))
	return item, mapConstraint(err)
}

type hardwareOptionScanner interface {
	Scan(dest ...any) error
}

func scanAdminHardwareOption(row hardwareOptionScanner) (ServerCatalogHardwareOption, error) {
	var item ServerCatalogHardwareOption
	var valueGB pgtype.Int4
	var locationID pgtype.UUID
	var locationCode pgtype.Text
	var locationName pgtype.Text
	if err := row.Scan(
		&item.ID,
		&item.OptionType,
		&item.Label,
		&item.Description,
		&item.Unit,
		&item.ValueText,
		&valueGB,
		&item.PriceDeltaCents,
		&item.HourlyPriceDeltaCents,
		&item.QuarterlyDeltaCents,
		&item.YearlyDeltaCents,
		&item.Currency,
		&item.QuantityAvailable,
		&item.FulfillmentMode,
		&item.EstimatedReadyMinHours,
		&item.EstimatedReadyMaxHours,
		&locationID,
		&locationCode,
		&locationName,
		&item.HardwareProfileName,
		&item.CPUModel,
		&item.Active,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return ServerCatalogHardwareOption{}, err
	}
	if valueGB.Valid {
		value := valueGB.Int32
		item.ValueGB = &value
	}
	if locationID.Valid {
		value := uuid.UUID(locationID.Bytes)
		item.LocationID = &value
	}
	if locationCode.Valid {
		value := strings.ToUpper(locationCode.String)
		item.LocationCode = &value
	}
	if locationName.Valid {
		value := locationName.String
		item.LocationName = &value
	}
	item.RequiresInstall = item.FulfillmentMode == "requires_install" || item.FulfillmentMode == "special_order" || item.FulfillmentMode == "manual"
	return item, nil
}

func (s *PostgresStore) ListHardwareFulfillmentOrders(ctx context.Context) ([]HardwareFulfillmentOrder, error) {
	rows, err := s.pool.Query(ctx, `
		select
			o.id,
			o.organization_id,
			org.name,
			o.status,
			o.total_cents,
			o.metadata,
			o.created_at,
			o.updated_at
		from orders o
		join organizations org on org.id = o.organization_id
		where o.order_type = 'server_purchase'
			and coalesce((o.metadata->>'requires_modification')::boolean, false) = true
		order by coalesce((o.metadata->>'fulfillment_ready')::boolean, false), o.created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []HardwareFulfillmentOrder{}
	for rows.Next() {
		item, err := scanHardwareFulfillmentOrder(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) MarkHardwareFulfillmentReady(ctx context.Context, orderID uuid.UUID) (HardwareFulfillmentOrder, error) {
	var metadata []byte
	var organizationID uuid.UUID
	if err := s.pool.QueryRow(ctx, `
		select organization_id, metadata
		from orders
		where id = $1
			and order_type = 'server_purchase'
		for update`, orderID).Scan(&organizationID, &metadata); err != nil {
		return HardwareFulfillmentOrder{}, mapNoRows(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(metadata, &payload); err != nil {
		return HardwareFulfillmentOrder{}, err
	}
	if requiresModification, _ := payload["requires_modification"].(bool); !requiresModification {
		return HardwareFulfillmentOrder{}, ErrInvalidInput
	}
	payload["fulfillment_ready"] = true
	payload["fulfillment_ready_at"] = time.Now().UTC().Format(time.RFC3339)
	updatedMetadata, err := json.Marshal(payload)
	if err != nil {
		return HardwareFulfillmentOrder{}, err
	}

	row := s.pool.QueryRow(ctx, `
		update orders
		set metadata = $2,
			updated_at = now()
		where id = $1
		returning id, organization_id, (select name from organizations where id = orders.organization_id), status, total_cents, metadata, created_at, updated_at`,
		orderID, updatedMetadata)
	item, err := scanHardwareFulfillmentOrder(row)
	return item, mapNoRows(err)
}

type hardwareFulfillmentScanner interface {
	Scan(dest ...any) error
}

func scanHardwareFulfillmentOrder(row hardwareFulfillmentScanner) (HardwareFulfillmentOrder, error) {
	var item HardwareFulfillmentOrder
	var metadata []byte
	if err := row.Scan(&item.OrderID, &item.OrganizationID, &item.OrganizationName, &item.Status, &item.TotalCents, &metadata, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return HardwareFulfillmentOrder{}, err
	}
	var pending PendingServiceMetadata
	if err := json.Unmarshal(metadata, &pending); err != nil {
		return HardwareFulfillmentOrder{}, err
	}
	item.ServerID = pending.ServiceID
	item.RequiresModification = pending.RequiresModification
	item.EstimatedReadyMinHours = pending.EstimatedReadyMinHours
	item.EstimatedReadyMaxHours = pending.EstimatedReadyMaxHours
	item.HardwareOptions = pending.HardwareOptions
	item.ServerHostname = pending.Description
	var raw map[string]any
	if err := json.Unmarshal(metadata, &raw); err == nil {
		if ready, ok := raw["fulfillment_ready"].(bool); ok {
			item.FulfillmentReady = ready
		}
	}
	return item, nil
}

func (s *PostgresStore) AllocateServer(ctx context.Context, params AllocateServerParams) (AllocateServerResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AllocateServerResult{}, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		select
			s.id,
			sf.id,
			sf.display_name,
			sf.slug,
			sf.cpu_manufacturer,
			sf.cpu_model,
			sf.core_count,
			sf.thread_count,
			coalesce(sf.base_clock_ghz, 0)::float8,
			coalesce(sf.boost_clock_ghz, 0)::float8,
			sf.generation,
			sf.workload_category,
			sf.description,
			sf.feature_badges,
			sf.display_order,
			s.rack_id,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as hostname,
			coalesce(nullif(l.keyword, ''), upper(nullif(l.code, '')), upper(r.location)) as location_code,
			coalesce(l.name, r.location) as location_name,
			coalesce(nullif(s.metadata->>'hardware_profile_name', ''), nullif(s.metadata->>'sku', ''), 'Dedicated server') as hardware_profile_name,
			s.metadata
		from servers s
		join server_families sf on sf.id = s.server_family_id and sf.active = true
		join racks r on r.id = s.rack_id
		left join locations l on l.id = r.location_id
		where s.server_family_id = $1
			and s.status = 'available'
			and s.organization_id is null
			and s.provisionable = true
		order by s.created_at
		for update of s skip locked`, params.ServerFamilyID)
	if err != nil {
		return AllocateServerResult{}, err
	}
	defer rows.Close()

	var selectedServerID uuid.UUID
	var config ServerCatalogConfiguration
	matchingServerIDs := []uuid.UUID{}
	for rows.Next() {
		var row catalogServerRow
		var metadataBytes []byte
		var badges []byte
		if err := rows.Scan(
			&row.ID,
			&row.ServerFamilyID,
			&row.FamilyDisplayName,
			&row.FamilySlug,
			&row.CPUManufacturer,
			&row.FamilyCPUModel,
			&row.FamilyCoreCount,
			&row.FamilyThreadCount,
			&row.BaseClockGHz,
			&row.BoostClockGHz,
			&row.Generation,
			&row.WorkloadCategory,
			&row.FamilyDescription,
			&badges,
			&row.DisplayOrder,
			&row.RackID,
			&row.Hostname,
			&row.LocationCode,
			&row.LocationName,
			&row.HardwareProfileName,
			&metadataBytes,
		); err != nil {
			return AllocateServerResult{}, err
		}
		row.Metadata = map[string]any{}
		_ = json.Unmarshal(metadataBytes, &row.Metadata)
		_ = json.Unmarshal(badges, &row.FeatureBadges)
		candidate := catalogConfigurationFromRow(row)
		if candidate.ID == params.ConfigurationID {
			matchingServerIDs = append(matchingServerIDs, row.ID)
			if config.ID == "" {
				config = candidate
			}
		}
	}
	if err := rows.Err(); err != nil {
		return AllocateServerResult{}, err
	}
	rows.Close()
	if len(matchingServerIDs) == 0 {
		return AllocateServerResult{}, ErrNotFound
	}
	interval := normalizeBillingInterval(params.BillingInterval)
	var selectedOptions []ServerCatalogHardwareOption
	for _, candidateID := range matchingServerIDs {
		options, optionErr := listCatalogHardwareOptionsForServerTx(ctx, tx, candidateID, params.HardwareOptionIDs)
		if optionErr == ErrInvalidInput {
			continue
		}
		if optionErr != nil {
			return AllocateServerResult{}, optionErr
		}
		selectedServerID = candidateID
		selectedOptions = options
		break
	}
	if selectedServerID == uuid.Nil {
		return AllocateServerResult{}, ErrInvalidInput
	}
	config.HardwareOptions = selectedOptions
	priceCents := config.PriceForInterval(interval) + hardwareOptionsPriceDelta(selectedOptions, interval)
	billingMode := prepaidBillingModeForInterval(interval)
	billingUnit := billingUnitForInterval(interval)
	now := time.Now().UTC()
	reservationExpiresAt := now.Add(15 * time.Minute)
	firstPeriodStart := now
	firstPeriodEnd := nextBillingAnchor(now, interval)
	firstPeriodHours := billablePeriodHours(firstPeriodStart, firstPeriodEnd, interval)
	firstPeriodAmountCents := proratedIntervalAmountCents(priceCents, firstPeriodHours, interval)
	if firstPeriodAmountCents < 1 {
		firstPeriodAmountCents = 1
	}

	requiresModification, minReadyHours, maxReadyHours := hardwareOptionsFulfillmentWindow(selectedOptions)
	orderMetadata, _ := json.Marshal(PendingServiceMetadata{
		ServiceType:            domain.ServiceServer,
		ServiceID:              &selectedServerID,
		ProjectID:              params.ProjectID,
		BillingMode:            billingMode,
		BillingInterval:        interval,
		Description:            serverOrderDescription(config, selectedOptions, interval),
		Unit:                   billingUnit,
		UnitPriceCents:         priceCents,
		Quantity:               "1",
		Currency:               "usd",
		MonthlyHours:           monthlyBillingHours,
		FirstPeriodStart:       &firstPeriodStart,
		FirstPeriodEnd:         &firstPeriodEnd,
		FirstPeriodHours:       firstPeriodHours,
		FirstPeriodAmountCents: firstPeriodAmountCents,
		HardwareOptions:        pendingHardwareOptions(selectedOptions),
		RequiresModification:   requiresModification,
		EstimatedReadyMinHours: minReadyHours,
		EstimatedReadyMaxHours: maxReadyHours,
	})

	order, err := scanOrder(tx.QueryRow(ctx, `
		insert into orders (organization_id, created_by_user_id, status, order_type, subtotal_cents, tax_cents, total_cents, metadata)
		values ($1, $2, 'pending', 'server_purchase', $3, 0, $3, $4)
		returning id, organization_id, created_by_user_id, status, order_type, subtotal_cents, tax_cents, total_cents, stripe_checkout_session_id, stripe_payment_intent_id, metadata, created_at, updated_at`,
		params.OrganizationID, params.CreatedByUserID, firstPeriodAmountCents, orderMetadata))
	if err != nil {
		return AllocateServerResult{}, err
	}

	_, err = tx.Exec(ctx, `
		update servers
		set organization_id = $2,
			project_id = $3,
			status = 'reserved',
			reserved_cpu_cores = $4,
			reserved_memory_mb = $5,
			reserved_storage_gb = $6,
			reservation_expires_at = $7,
			updated_at = now()
		where id = $1`,
		selectedServerID, params.OrganizationID, params.ProjectID, config.CoreCount, config.RAMGB*1024, config.StorageGB, reservationExpiresAt)
	if err != nil {
		return AllocateServerResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AllocateServerResult{}, err
	}

	servers, err := s.ListOrganizationServers(ctx, params.OrganizationID)
	if err != nil {
		return AllocateServerResult{}, err
	}
	for _, server := range servers {
		if server.ID == selectedServerID {
			return AllocateServerResult{Server: server, Order: order, ReservationExpiresAt: &reservationExpiresAt}, nil
		}
	}
	return AllocateServerResult{Order: order, ReservationExpiresAt: &reservationExpiresAt}, nil
}

func catalogConfigurationFromRow(row catalogServerRow) ServerCatalogConfiguration {
	cpuModel := row.FamilyCPUModel
	coreCount := row.FamilyCoreCount
	ramGB := int32(metadataInt(row.Metadata, "ram_gb", "memory_gb"))
	diskName := firstNonEmptyMetadata(row.Metadata, "disk_name")
	diskDescription := firstNonEmptyMetadata(row.Metadata, "disk_description", "storage", "disk")
	if diskDescription == "" {
		diskDescription = "Storage unspecified"
	}
	storageGB := parseStorageGB(diskDescription)
	diskCount, diskSizeGB, diskType := parseDiskShape(diskDescription)
	networkCapacity := firstNonEmptyMetadata(row.Metadata, "network_capacity", "network_speed", "nic_speed", "nic_description")
	if networkCapacity == "" {
		networkCapacity = "10 Gbps"
	}
	estimatedReadyMinHours := int32(metadataInt(row.Metadata, "estimated_ready_min_hours", "provisioning_min_hours"))
	estimatedReadyMaxHours := int32(metadataInt(row.Metadata, "estimated_ready_max_hours", "provisioning_max_hours"))
	if estimatedReadyMaxHours == 0 {
		estimatedReadyMaxHours = estimatedReadyMinHours
	}
	monthlyPrice := metadataInt(row.Metadata, "monthly_price_cents", "monthly_cost_cents", "price_cents")
	if monthlyPrice <= 0 {
		monthlyPrice = derivedMonthlyPriceCents(coreCount, ramGB, storageGB)
	}
	hourlyPrice := metadataInt(row.Metadata, "hourly_price_cents", "hourly_cost_cents")
	if hourlyPrice <= 0 {
		hourlyPrice = int64(math.Ceil(float64(monthlyPrice) / float64(monthlyBillingHours)))
	}
	quarterlyPrice := metadataInt(row.Metadata, "quarterly_price_cents", "quarterly_cost_cents")
	if quarterlyPrice <= 0 {
		quarterlyPrice = monthlyPrice * 3
	}
	yearlyPrice := metadataInt(row.Metadata, "yearly_price_cents", "yearly_cost_cents", "annual_price_cents", "annual_cost_cents")
	if yearlyPrice <= 0 {
		yearlyPrice = monthlyPrice * 12
	}
	config := ServerCatalogConfiguration{
		ServerFamilyID:         row.ServerFamilyID.String(),
		LocationCode:           strings.ToUpper(row.LocationCode),
		LocationName:           row.LocationName,
		HardwareProfileName:    row.HardwareProfileName,
		CPUModel:               cpuModel,
		CoreCount:              coreCount,
		RAMGB:                  ramGB,
		DiskName:               diskName,
		DiskDescription:        diskDescription,
		DiskCount:              diskCount,
		DiskSizeGB:             diskSizeGB,
		DiskType:               diskType,
		StorageGB:              storageGB,
		NetworkCapacity:        networkCapacity,
		HourlyPriceCents:       hourlyPrice,
		MonthlyPriceCents:      monthlyPrice,
		QuarterlyPriceCents:    quarterlyPrice,
		YearlyPriceCents:       yearlyPrice,
		Available:              true,
		AvailableQuantity:      1,
		EstimatedReadyMinHours: estimatedReadyMinHours,
		EstimatedReadyMaxHours: estimatedReadyMaxHours,
		PhysicalServerIDs:      []uuid.UUID{row.ID},
		FamilyDisplayName:      row.FamilyDisplayName,
		FamilySlug:             row.FamilySlug,
		CPUManufacturer:        row.CPUManufacturer,
		ThreadCount:            row.FamilyThreadCount,
		Generation:             row.Generation,
		WorkloadCategory:       row.WorkloadCategory,
		FamilyDescription:      row.FamilyDescription,
		FeatureBadges:          row.FeatureBadges,
		DisplayOrder:           row.DisplayOrder,
	}
	if row.BaseClockGHz > 0 {
		config.BaseClockGHz = &row.BaseClockGHz
	}
	if row.BoostClockGHz > 0 {
		config.BoostClockGHz = &row.BoostClockGHz
	}
	config.ID = inventoryConfigurationID(config)
	return config
}

func (config ServerCatalogConfiguration) PriceForInterval(interval domain.BillingInterval) int64 {
	switch normalizeBillingInterval(interval) {
	case domain.IntervalHourly:
		return config.HourlyPriceCents
	case domain.IntervalQuarterly:
		return config.QuarterlyPriceCents
	case domain.IntervalYearly:
		return config.YearlyPriceCents
	default:
		return config.MonthlyPriceCents
	}
}

func hardwareOptionsPriceDelta(options []ServerCatalogHardwareOption, interval domain.BillingInterval) int64 {
	var total int64
	for _, option := range options {
		switch normalizeBillingInterval(interval) {
		case domain.IntervalHourly:
			if option.HourlyPriceDeltaCents > 0 {
				total += option.HourlyPriceDeltaCents
			} else {
				total += int64(math.Ceil(float64(option.PriceDeltaCents) / float64(monthlyBillingHours)))
			}
		case domain.IntervalQuarterly:
			if option.QuarterlyDeltaCents > 0 {
				total += option.QuarterlyDeltaCents
			} else {
				total += option.PriceDeltaCents * 3
			}
		case domain.IntervalYearly:
			if option.YearlyDeltaCents > 0 {
				total += option.YearlyDeltaCents
			} else {
				total += option.PriceDeltaCents * 12
			}
		default:
			total += option.PriceDeltaCents
		}
	}
	return total
}

func hardwareOptionsFulfillmentWindow(options []ServerCatalogHardwareOption) (bool, int32, int32) {
	var requiresModification bool
	var minHours int32
	var maxHours int32
	for _, option := range options {
		if option.RequiresInstall {
			requiresModification = true
		}
		if option.EstimatedReadyMinHours > minHours {
			minHours = option.EstimatedReadyMinHours
		}
		if option.EstimatedReadyMaxHours > maxHours {
			maxHours = option.EstimatedReadyMaxHours
		}
	}
	return requiresModification, minHours, maxHours
}

func pendingHardwareOptions(options []ServerCatalogHardwareOption) []PendingHardwareOption {
	pending := make([]PendingHardwareOption, 0, len(options))
	for _, option := range options {
		pending = append(pending, PendingHardwareOption{
			ID:                     option.ID,
			OptionType:             option.OptionType,
			Label:                  option.Label,
			Description:            option.Description,
			Unit:                   option.Unit,
			ValueText:              option.ValueText,
			ValueGB:                option.ValueGB,
			PriceDeltaCents:        option.PriceDeltaCents,
			HourlyPriceDeltaCents:  option.HourlyPriceDeltaCents,
			QuarterlyDeltaCents:    option.QuarterlyDeltaCents,
			YearlyDeltaCents:       option.YearlyDeltaCents,
			FulfillmentMode:        option.FulfillmentMode,
			EstimatedReadyMinHours: option.EstimatedReadyMinHours,
			EstimatedReadyMaxHours: option.EstimatedReadyMaxHours,
		})
	}
	return pending
}

func serverOrderDescription(config ServerCatalogConfiguration, options []ServerCatalogHardwareOption, interval domain.BillingInterval) string {
	description := fmt.Sprintf("%s / %dGB / %s %s rental", config.CPUModel, config.RAMGB, config.DiskDescription, interval)
	if len(options) == 0 {
		return description
	}
	labels := make([]string, 0, len(options))
	for _, option := range options {
		labels = append(labels, option.Label)
	}
	return fmt.Sprintf("%s with %s", description, strings.Join(labels, ", "))
}

func groupCatalogPlans(configs []ServerCatalogConfiguration) ServerCatalog {
	byID := map[string]*ServerCatalogPlan{}
	for _, config := range configs {
		family := byID[config.ServerFamilyID]
		if family == nil {
			family = &ServerCatalogPlan{
				ID:                  config.ServerFamilyID,
				Name:                config.FamilyDisplayName,
				Slug:                config.FamilySlug,
				CPUManufacturer:     config.CPUManufacturer,
				CPUModel:            config.CPUModel,
				CoreCount:           config.CoreCount,
				ThreadCount:         config.ThreadCount,
				BaseClockGHz:        config.BaseClockGHz,
				BoostClockGHz:       config.BoostClockGHz,
				Generation:          config.Generation,
				Category:            config.WorkloadCategory,
				Description:         config.FamilyDescription,
				FeatureBadges:       config.FeatureBadges,
				StartingPriceCents:  config.MonthlyPriceCents,
				HourlyPriceCents:    config.HourlyPriceCents,
				MonthlyPriceCents:   config.MonthlyPriceCents,
				QuarterlyPriceCents: config.QuarterlyPriceCents,
				YearlyPriceCents:    config.YearlyPriceCents,
				Configurations:      []ServerCatalogConfiguration{},
			}
			byID[config.ServerFamilyID] = family
		}
		if config.MonthlyPriceCents < family.StartingPriceCents {
			family.StartingPriceCents = config.MonthlyPriceCents
		}
		family.HourlyPriceCents = minPositivePrice(family.HourlyPriceCents, config.HourlyPriceCents)
		family.MonthlyPriceCents = minPositivePrice(family.MonthlyPriceCents, config.MonthlyPriceCents)
		family.QuarterlyPriceCents = minPositivePrice(family.QuarterlyPriceCents, config.QuarterlyPriceCents)
		family.YearlyPriceCents = minPositivePrice(family.YearlyPriceCents, config.YearlyPriceCents)
		family.AvailableCount++
		merged := false
		for i := range family.Configurations {
			if family.Configurations[i].ID == config.ID {
				family.Configurations[i].AvailableQuantity += config.AvailableQuantity
				family.Configurations[i].PhysicalServerIDs = append(family.Configurations[i].PhysicalServerIDs, config.PhysicalServerIDs...)
				family.Configurations[i].HardwareOptions = mergeHardwareOptions(family.Configurations[i].HardwareOptions, config.HardwareOptions)
				merged = true
				break
			}
		}
		if !merged {
			family.Configurations = append(family.Configurations, config)
		}
	}

	families := make([]ServerCatalogPlan, 0, len(byID))
	for _, family := range byID {
		family.Locations = catalogLocations(family.Configurations)
		family.RAMOptionsGB = catalogRAMOptions(family.Configurations)
		family.DiskOptions = catalogDiskOptions(family.Configurations)
		family.HardwareOptions = catalogHardwareOptions(family.Configurations)
		sort.Slice(family.Configurations, func(i, j int) bool {
			left, right := family.Configurations[i], family.Configurations[j]
			if left.MonthlyPriceCents != right.MonthlyPriceCents {
				return left.MonthlyPriceCents < right.MonthlyPriceCents
			}
			if left.RAMGB != right.RAMGB {
				return left.RAMGB < right.RAMGB
			}
			return left.DiskDescription < right.DiskDescription
		})
		families = append(families, *family)
	}
	sort.Slice(families, func(i, j int) bool {
		if families[i].StartingPriceCents != families[j].StartingPriceCents {
			return families[i].StartingPriceCents < families[j].StartingPriceCents
		}
		return families[i].Name < families[j].Name
	})
	return ServerCatalog{Families: families}
}

func attachHardwareOptions(configs []ServerCatalogConfiguration, options map[uuid.UUID][]ServerCatalogHardwareOption) {
	for i := range configs {
		if len(configs[i].PhysicalServerIDs) > 0 {
			configs[i].HardwareOptions = options[configs[i].PhysicalServerIDs[0]]
		}
	}
}

func mergeHardwareOptions(left, right []ServerCatalogHardwareOption) []ServerCatalogHardwareOption {
	seen := map[uuid.UUID]ServerCatalogHardwareOption{}
	for _, option := range append(left, right...) {
		if existing, ok := seen[option.ID]; ok && existing.QuantityAvailable < option.QuantityAvailable {
			continue
		}
		seen[option.ID] = option
	}
	items := make([]ServerCatalogHardwareOption, 0, len(seen))
	for _, option := range seen {
		items = append(items, option)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	return items
}

func catalogHardwareOptions(configs []ServerCatalogConfiguration) []ServerCatalogHardwareOption {
	seen := map[uuid.UUID]ServerCatalogHardwareOption{}
	for _, config := range configs {
		for _, option := range config.HardwareOptions {
			seen[option.ID] = option
		}
	}
	items := make([]ServerCatalogHardwareOption, 0, len(seen))
	for _, item := range seen {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].OptionType != items[j].OptionType {
			return items[i].OptionType < items[j].OptionType
		}
		if items[i].ValueGB != nil && items[j].ValueGB != nil && *items[i].ValueGB != *items[j].ValueGB {
			return *items[i].ValueGB < *items[j].ValueGB
		}
		return items[i].Label < items[j].Label
	})
	return items
}

func catalogLocations(configs []ServerCatalogConfiguration) []ServerCatalogLocation {
	seen := map[string]ServerCatalogLocation{}
	for _, config := range configs {
		seen[config.LocationCode] = ServerCatalogLocation{Code: config.LocationCode, Name: config.LocationName}
	}
	items := make([]ServerCatalogLocation, 0, len(seen))
	for _, item := range seen {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Code < items[j].Code })
	return items
}

func catalogRAMOptions(configs []ServerCatalogConfiguration) []int32 {
	seen := map[int32]bool{}
	for _, config := range configs {
		if config.RAMGB > 0 {
			seen[config.RAMGB] = true
		}
	}
	items := make([]int32, 0, len(seen))
	for value := range seen {
		items = append(items, value)
	}
	sort.Slice(items, func(i, j int) bool { return items[i] < items[j] })
	return items
}

func catalogDiskOptions(configs []ServerCatalogConfiguration) []ServerCatalogDiskOption {
	seen := map[string]ServerCatalogDiskOption{}
	for _, config := range configs {
		seen[config.DiskDescription] = ServerCatalogDiskOption{Label: config.DiskDescription, StorageGB: config.StorageGB}
	}
	items := make([]ServerCatalogDiskOption, 0, len(seen))
	for _, item := range seen {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].StorageGB != items[j].StorageGB {
			return items[i].StorageGB < items[j].StorageGB
		}
		return items[i].Label < items[j].Label
	})
	return items
}

func firstNonEmptyMetadata(metadata map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := metadata[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case float64:
			if value != 0 {
				return strconv.FormatInt(int64(value), 10)
			}
		}
	}
	return ""
}

func metadataInt(metadata map[string]any, keys ...string) int64 {
	for _, key := range keys {
		switch value := metadata[key].(type) {
		case float64:
			return int64(value)
		case int64:
			return value
		case int:
			return int64(value)
		case string:
			parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func uniqueUUIDs(values []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	items := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if value == uuid.Nil || seen[value] {
			continue
		}
		seen[value] = true
		items = append(items, value)
	}
	return items
}

func parseStorageGB(description string) int32 {
	matches := storageTokenPattern.FindAllStringSubmatch(description, -1)
	var total float64
	for _, match := range matches {
		count := 1.0
		if match[1] != "" {
			if parsed, err := strconv.ParseFloat(match[1], 64); err == nil {
				count = parsed
			}
		}
		value, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			continue
		}
		if strings.EqualFold(match[3], "tb") {
			value *= 1000
		}
		total += count * value
	}
	return int32(math.Round(total))
}

func parseDiskShape(description string) (int32, int32, string) {
	match := storageTokenPattern.FindStringSubmatch(description)
	if len(match) == 0 {
		return 0, 0, "unknown"
	}
	count := int32(1)
	if match[1] != "" {
		if parsed, err := strconv.ParseInt(match[1], 10, 32); err == nil {
			count = int32(parsed)
		}
	}
	size, _ := strconv.ParseFloat(match[2], 64)
	if strings.EqualFold(match[3], "tb") {
		size *= 1000
	}
	diskType := "disk"
	lower := strings.ToLower(description)
	switch {
	case strings.Contains(lower, "nvme"):
		diskType = "nvme"
	case strings.Contains(lower, "ssd"):
		diskType = "ssd"
	case strings.Contains(lower, "hdd") || strings.Contains(lower, "sas") || strings.Contains(lower, "sata"):
		diskType = "hdd"
	}
	return count, int32(math.Round(size)), diskType
}

func inventoryConfigurationID(config ServerCatalogConfiguration) string {
	value := fmt.Sprintf(
		"%s|%s|%d|%s|%d|%d|%s|%s|%d|%d|%d|%d|%d|%d",
		config.ServerFamilyID,
		config.LocationCode,
		config.RAMGB,
		strings.ToLower(config.DiskDescription),
		config.DiskCount,
		config.DiskSizeGB,
		config.DiskType,
		strings.ToLower(config.NetworkCapacity),
		config.HourlyPriceCents,
		config.MonthlyPriceCents,
		config.QuarterlyPriceCents,
		config.YearlyPriceCents,
		config.EstimatedReadyMinHours,
		config.EstimatedReadyMaxHours,
	)
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("cfg_%x", sum[:10])
}

func derivedMonthlyPriceCents(cores, ramGB, storageGB int32) int64 {
	price := int64(12900)
	price += int64(cores) * 900
	price += int64(ramGB) * 75
	price += int64(storageGB) * 5
	if price < 12900 {
		return 12900
	}
	return price
}

func nextMonthlyBillingAnchor(now time.Time) time.Time {
	return nextBillingAnchor(now, domain.IntervalMonthly)
}

func normalizeBillingInterval(interval domain.BillingInterval) domain.BillingInterval {
	switch interval {
	case domain.IntervalHourly, domain.IntervalMonthly, domain.IntervalQuarterly, domain.IntervalYearly:
		return interval
	default:
		return domain.IntervalMonthly
	}
}

func prepaidBillingModeForInterval(interval domain.BillingInterval) domain.BillingMode {
	switch normalizeBillingInterval(interval) {
	case domain.IntervalHourly:
		return domain.BillingHourlyPrepaid
	case domain.IntervalQuarterly:
		return domain.BillingQuarterlyPrepaid
	case domain.IntervalYearly:
		return domain.BillingYearlyPrepaid
	default:
		return domain.BillingMonthlyPrepaid
	}
}

func billingUnitForInterval(interval domain.BillingInterval) domain.BillingUnit {
	switch normalizeBillingInterval(interval) {
	case domain.IntervalHourly:
		return domain.UnitHour
	case domain.IntervalQuarterly:
		return domain.UnitQuarter
	case domain.IntervalYearly:
		return domain.UnitYear
	default:
		return domain.UnitMonth
	}
}

func nextBillingAnchor(now time.Time, interval domain.BillingInterval) time.Time {
	utc := now.UTC()
	month := utc.Month()
	switch normalizeBillingInterval(interval) {
	case domain.IntervalHourly:
		return utc.Truncate(time.Hour).Add(time.Hour)
	case domain.IntervalQuarterly:
		nextQuarterMonth := time.Month(((int(month)-1)/3+1)*3 + 1)
		year := utc.Year()
		if nextQuarterMonth > 12 {
			nextQuarterMonth -= 12
			year++
		}
		return time.Date(year, nextQuarterMonth, 1, 0, 0, 0, 0, time.UTC)
	case domain.IntervalYearly:
		return time.Date(utc.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	}
}

func addBillingInterval(start time.Time, interval domain.BillingInterval) time.Time {
	switch normalizeBillingInterval(interval) {
	case domain.IntervalHourly:
		return start.Add(time.Hour)
	case domain.IntervalQuarterly:
		return start.AddDate(0, 3, 0)
	case domain.IntervalYearly:
		return start.AddDate(1, 0, 0)
	default:
		return start.AddDate(0, 1, 0)
	}
}

func billablePeriodHours(start, end time.Time, interval domain.BillingInterval) int64 {
	hours := int64(math.Ceil(end.Sub(start).Hours()))
	if hours < 1 {
		return 1
	}
	full := fullIntervalHours(interval)
	if full > 0 && hours > full {
		return full
	}
	return hours
}

func fullIntervalHours(interval domain.BillingInterval) int64 {
	switch normalizeBillingInterval(interval) {
	case domain.IntervalHourly:
		return 1
	case domain.IntervalQuarterly:
		return monthlyBillingHours * 3
	case domain.IntervalYearly:
		return monthlyBillingHours * 12
	default:
		return monthlyBillingHours
	}
}

func proratedIntervalAmountCents(priceCents, hours int64, interval domain.BillingInterval) int64 {
	full := fullIntervalHours(interval)
	if full <= 1 {
		return priceCents * hours
	}
	return int64(math.Ceil(float64(priceCents) * float64(hours) / float64(full)))
}

func minPositivePrice(current, next int64) int64 {
	if current <= 0 || (next > 0 && next < current) {
		return next
	}
	return current
}
