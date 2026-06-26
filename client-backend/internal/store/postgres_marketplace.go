package store

import (
	"context"
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
)

var storageTokenPattern = regexp.MustCompile(`(?i)(?:(\d+)\s*x\s*)?(\d+(?:\.\d+)?)\s*(tb|gb)`)

const monthlyBillingHours int64 = 730

type catalogServerRow struct {
	ID                  uuid.UUID
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
			s.rack_id,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as hostname,
			coalesce(nullif(l.keyword, ''), upper(nullif(l.code, '')), upper(r.location)) as location_code,
			coalesce(l.name, r.location) as location_name,
			coalesce(nullif(s.metadata->>'hardware_profile_name', ''), nullif(s.metadata->>'sku', ''), 'Dedicated server') as hardware_profile_name,
			s.metadata
		from servers s
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
		if err := rows.Scan(&row.ID, &row.RackID, &row.Hostname, &row.LocationCode, &row.LocationName, &row.HardwareProfileName, &metadata); err != nil {
			return ServerCatalog{}, err
		}
		row.Metadata = map[string]any{}
		_ = json.Unmarshal(metadata, &row.Metadata)
		configs = append(configs, catalogConfigurationFromRow(row))
	}
	if err := rows.Err(); err != nil {
		return ServerCatalog{}, err
	}
	return groupCatalogPlans(configs), nil
}

func (s *PostgresStore) AllocateServer(ctx context.Context, params AllocateServerParams) (AllocateServerResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AllocateServerResult{}, err
	}
	defer tx.Rollback(ctx)

	var row catalogServerRow
	var metadataBytes []byte
	if err := tx.QueryRow(ctx, `
		select
			s.id,
			s.rack_id,
			coalesce(nullif(s.metadata->>'hostname', ''), s.id::text) as hostname,
			coalesce(nullif(l.keyword, ''), upper(nullif(l.code, '')), upper(r.location)) as location_code,
			coalesce(l.name, r.location) as location_name,
			coalesce(nullif(s.metadata->>'hardware_profile_name', ''), nullif(s.metadata->>'sku', ''), 'Dedicated server') as hardware_profile_name,
			s.metadata
		from servers s
		join racks r on r.id = s.rack_id
		left join locations l on l.id = r.location_id
		where s.id = $1
			and s.status = 'available'
			and s.organization_id is null
			and s.provisionable = true
		for update`, params.ServerID).Scan(&row.ID, &row.RackID, &row.Hostname, &row.LocationCode, &row.LocationName, &row.HardwareProfileName, &metadataBytes); err != nil {
		return AllocateServerResult{}, mapNoRows(err)
	}
	row.Metadata = map[string]any{}
	_ = json.Unmarshal(metadataBytes, &row.Metadata)
	config := catalogConfigurationFromRow(row)
	interval := normalizeBillingInterval(params.BillingInterval)
	priceCents := config.PriceForInterval(interval)
	billingMode := prepaidBillingModeForInterval(interval)
	billingUnit := billingUnitForInterval(interval)
	now := time.Now().UTC()
	firstPeriodStart := now
	firstPeriodEnd := nextBillingAnchor(now, interval)
	firstPeriodHours := billablePeriodHours(firstPeriodStart, firstPeriodEnd, interval)
	firstPeriodAmountCents := proratedIntervalAmountCents(priceCents, firstPeriodHours, interval)
	if firstPeriodAmountCents < 1 {
		firstPeriodAmountCents = 1
	}

	orderMetadata, _ := json.Marshal(PendingServiceMetadata{
		ServiceType:            domain.ServiceServer,
		ServiceID:              &params.ServerID,
		ProjectID:              params.ProjectID,
		BillingMode:            billingMode,
		BillingInterval:        interval,
		Description:            fmt.Sprintf("%s / %dGB / %s %s rental", config.CPUModel, config.RAMGB, config.DiskDescription, interval),
		Unit:                   billingUnit,
		UnitPriceCents:         priceCents,
		Quantity:               "1",
		Currency:               "usd",
		MonthlyHours:           monthlyBillingHours,
		FirstPeriodStart:       &firstPeriodStart,
		FirstPeriodEnd:         &firstPeriodEnd,
		FirstPeriodHours:       firstPeriodHours,
		FirstPeriodAmountCents: firstPeriodAmountCents,
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
			updated_at = now()
		where id = $1`,
		params.ServerID, params.OrganizationID, params.ProjectID, config.CoreCount, config.RAMGB*1024, config.StorageGB)
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
		if server.ID == params.ServerID {
			return AllocateServerResult{Server: server, Order: order}, nil
		}
	}
	return AllocateServerResult{Order: order}, nil
}

func catalogConfigurationFromRow(row catalogServerRow) ServerCatalogConfiguration {
	cpuModel := firstNonEmptyMetadata(row.Metadata, "cpu_model", "cpu", "processor")
	if cpuModel == "" {
		cpuModel = row.HardwareProfileName
	}
	coreCount := int32(metadataInt(row.Metadata, "core_count", "cores", "cpu_cores"))
	ramGB := int32(metadataInt(row.Metadata, "ram_gb", "memory_gb"))
	diskName := firstNonEmptyMetadata(row.Metadata, "disk_name")
	diskDescription := firstNonEmptyMetadata(row.Metadata, "disk_description", "storage", "disk")
	if diskDescription == "" {
		diskDescription = "Storage unspecified"
	}
	storageGB := parseStorageGB(diskDescription)
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
	planID := normalizeSlug(fmt.Sprintf("%s-%d-core", cpuModel, coreCount))
	return ServerCatalogConfiguration{
		ID:                  row.ID,
		PlanID:              planID,
		Hostname:            row.Hostname,
		LocationCode:        strings.ToUpper(row.LocationCode),
		LocationName:        row.LocationName,
		HardwareProfileName: row.HardwareProfileName,
		CPUModel:            cpuModel,
		CoreCount:           coreCount,
		RAMGB:               ramGB,
		DiskName:            diskName,
		DiskDescription:     diskDescription,
		StorageGB:           storageGB,
		HourlyPriceCents:    hourlyPrice,
		MonthlyPriceCents:   monthlyPrice,
		QuarterlyPriceCents: quarterlyPrice,
		YearlyPriceCents:    yearlyPrice,
		Available:           true,
	}
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

func groupCatalogPlans(configs []ServerCatalogConfiguration) ServerCatalog {
	byID := map[string]*ServerCatalogPlan{}
	for _, config := range configs {
		plan := byID[config.PlanID]
		if plan == nil {
			plan = &ServerCatalogPlan{
				ID:                  config.PlanID,
				Name:                config.CPUModel,
				CPUModel:            config.CPUModel,
				CoreCount:           config.CoreCount,
				Category:            catalogCategory(config),
				StartingPriceCents:  config.MonthlyPriceCents,
				HourlyPriceCents:    config.HourlyPriceCents,
				MonthlyPriceCents:   config.MonthlyPriceCents,
				QuarterlyPriceCents: config.QuarterlyPriceCents,
				YearlyPriceCents:    config.YearlyPriceCents,
				Configurations:      []ServerCatalogConfiguration{},
			}
			byID[config.PlanID] = plan
		}
		if config.MonthlyPriceCents < plan.StartingPriceCents {
			plan.StartingPriceCents = config.MonthlyPriceCents
		}
		plan.HourlyPriceCents = minPositivePrice(plan.HourlyPriceCents, config.HourlyPriceCents)
		plan.MonthlyPriceCents = minPositivePrice(plan.MonthlyPriceCents, config.MonthlyPriceCents)
		plan.QuarterlyPriceCents = minPositivePrice(plan.QuarterlyPriceCents, config.QuarterlyPriceCents)
		plan.YearlyPriceCents = minPositivePrice(plan.YearlyPriceCents, config.YearlyPriceCents)
		plan.Configurations = append(plan.Configurations, config)
		plan.AvailableCount++
	}

	plans := make([]ServerCatalogPlan, 0, len(byID))
	for _, plan := range byID {
		plan.Locations = catalogLocations(plan.Configurations)
		plan.RAMOptionsGB = catalogRAMOptions(plan.Configurations)
		plan.DiskOptions = catalogDiskOptions(plan.Configurations)
		sort.Slice(plan.Configurations, func(i, j int) bool {
			left, right := plan.Configurations[i], plan.Configurations[j]
			if left.MonthlyPriceCents != right.MonthlyPriceCents {
				return left.MonthlyPriceCents < right.MonthlyPriceCents
			}
			if left.RAMGB != right.RAMGB {
				return left.RAMGB < right.RAMGB
			}
			return left.DiskDescription < right.DiskDescription
		})
		plans = append(plans, *plan)
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].StartingPriceCents != plans[j].StartingPriceCents {
			return plans[i].StartingPriceCents < plans[j].StartingPriceCents
		}
		return plans[i].Name < plans[j].Name
	})
	return ServerCatalog{Plans: plans}
}

func catalogCategory(config ServerCatalogConfiguration) string {
	if config.RAMGB >= 384 {
		return "memory"
	}
	return "cpu"
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
