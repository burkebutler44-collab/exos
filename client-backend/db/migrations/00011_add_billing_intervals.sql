-- +goose Up
alter table billable_services
	add column billing_interval text not null default 'monthly'
	check (billing_interval in ('hourly', 'monthly', 'quarterly', 'yearly'));

update billable_services
set billing_interval = case
	when billing_mode like 'hourly_%' then 'hourly'
	when billing_mode like 'quarterly_%' then 'quarterly'
	when billing_mode like 'yearly_%' then 'yearly'
	else 'monthly'
end;

alter table billable_services
	drop constraint billable_services_billing_mode_check,
	add constraint billable_services_billing_mode_check
	check (billing_mode in (
		'monthly_prepaid',
		'monthly_postpaid',
		'hourly_prepaid',
		'hourly_postpaid',
		'quarterly_prepaid',
		'quarterly_postpaid',
		'yearly_prepaid',
		'yearly_postpaid',
		'one_time',
		'custom_contract'
	));

alter table billable_service_prices
	drop constraint billable_service_prices_unit_check,
	add constraint billable_service_prices_unit_check
	check (unit in ('month', 'quarter', 'year', 'hour', 'second', 'gb', 'tb', 'ip', 'each'));

alter table usage_ledger
	drop constraint usage_ledger_unit_check,
	add constraint usage_ledger_unit_check
	check (unit in ('month', 'quarter', 'year', 'hour', 'second', 'gb', 'tb', 'ip', 'each'));

alter table invoice_line_items
	drop constraint invoice_line_items_unit_check,
	add constraint invoice_line_items_unit_check
	check (unit in ('month', 'quarter', 'year', 'hour', 'second', 'gb', 'tb', 'ip', 'each'));

create index billable_services_interval_next_invoice_idx
	on billable_services (billing_interval, next_invoice_at)
	where status = 'active';

-- +goose Down
drop index if exists billable_services_interval_next_invoice_idx;

alter table invoice_line_items
	drop constraint invoice_line_items_unit_check,
	add constraint invoice_line_items_unit_check
	check (unit in ('month', 'hour', 'second', 'gb', 'tb', 'ip', 'each'));

alter table usage_ledger
	drop constraint usage_ledger_unit_check,
	add constraint usage_ledger_unit_check
	check (unit in ('month', 'hour', 'second', 'gb', 'tb', 'ip', 'each'));

alter table billable_service_prices
	drop constraint billable_service_prices_unit_check,
	add constraint billable_service_prices_unit_check
	check (unit in ('month', 'hour', 'second', 'gb', 'tb', 'ip', 'each'));

alter table billable_services
	drop constraint billable_services_billing_mode_check,
	add constraint billable_services_billing_mode_check
	check (billing_mode in ('monthly_prepaid', 'monthly_postpaid', 'hourly_prepaid', 'hourly_postpaid', 'one_time', 'custom_contract'));

alter table billable_services
	drop column billing_interval;
