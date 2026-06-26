package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) UpsertHypervisorSnapshot(ctx context.Context, params UpsertHypervisorSnapshotParams) error {
	params.ID = strings.TrimSpace(params.ID)
	if params.ID == "" {
		return pgx.ErrNoRows
	}
	if params.Status == "" {
		params.Status = "online"
	}
	if params.LastReportedAt.IsZero() {
		params.LastReportedAt = time.Now().UTC()
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		with matched_server as (
			select id
			from servers
			where id::text = $1
				or metadata->>'hypervisor_id' = $1
				or lower(coalesce(metadata->>'hostname', metadata->>'label', metadata->>'asset_tag', '')) = lower($2)
			order by updated_at desc
			limit 1
		)
		insert into hypervisors (
			id,
			server_id,
			hostname,
			status,
			vcpus_total,
			vcpus_active,
			memory_total_bytes,
			memory_active_bytes,
			disk_total_bytes,
			disk_available_bytes,
			wireguard_interface,
			control_plane_address,
			last_reported_at,
			metadata
		)
		values (
			$1,
			(select id from matched_server),
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			jsonb_build_object('source', 'hypervisor_agent')
		)
		on conflict (id) do update
		set server_id = coalesce(hypervisors.server_id, excluded.server_id),
			hostname = excluded.hostname,
			status = excluded.status,
			vcpus_total = excluded.vcpus_total,
			vcpus_active = excluded.vcpus_active,
			memory_total_bytes = excluded.memory_total_bytes,
			memory_active_bytes = excluded.memory_active_bytes,
			disk_total_bytes = excluded.disk_total_bytes,
			disk_available_bytes = excluded.disk_available_bytes,
			wireguard_interface = excluded.wireguard_interface,
			control_plane_address = excluded.control_plane_address,
			last_reported_at = excluded.last_reported_at,
			updated_at = now()`,
		params.ID,
		params.Hostname,
		params.Status,
		params.VCPUsTotal,
		params.VCPUsActive,
		params.MemoryTotalBytes,
		params.MemoryActiveBytes,
		params.DiskTotalBytes,
		params.DiskAvailableBytes,
		params.WireguardInterface,
		params.ControlPlaneAddress,
		params.LastReportedAt,
	)
	if err != nil {
		return err
	}

	vmIDs := make([]string, 0, len(params.VMs))
	for index, vm := range params.VMs {
		vm.ID = strings.TrimSpace(vm.ID)
		if vm.ID == "" {
			vm.ID = strings.TrimSpace(vm.Name)
		}
		if vm.ID == "" {
			vm.ID = fmt.Sprintf("%s-vm-%d", params.ID, index+1)
		}
		vm.Name = strings.TrimSpace(vm.Name)
		if vm.Name == "" {
			vm.Name = vm.ID
		}
		if vm.Status == "" {
			vm.Status = "unknown"
		}
		if vm.LastReportedAt.IsZero() {
			vm.LastReportedAt = params.LastReportedAt
		}
		metadata, err := json.Marshal(vm.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			delete from hypervisor_vms
			where hypervisor_id = $1 and lower(name) = lower($2) and id <> $3`,
			params.ID,
			vm.Name,
			vm.ID,
		); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			insert into hypervisor_vms (
				id,
				hypervisor_id,
				name,
				status,
				vcpus,
				memory_bytes,
				disk_bytes,
				mac_addresses,
				ip_addresses,
				metadata,
				last_reported_at
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			on conflict (hypervisor_id, id) do update
			set name = excluded.name,
				status = excluded.status,
				vcpus = excluded.vcpus,
				memory_bytes = excluded.memory_bytes,
				disk_bytes = excluded.disk_bytes,
				mac_addresses = excluded.mac_addresses,
				ip_addresses = excluded.ip_addresses,
				metadata = excluded.metadata,
				last_reported_at = excluded.last_reported_at,
				updated_at = now()`,
			vm.ID,
			params.ID,
			vm.Name,
			vm.Status,
			vm.VCPUs,
			vm.MemoryBytes,
			vm.DiskBytes,
			vm.MACAddresses,
			vm.IPAddresses,
			metadata,
			vm.LastReportedAt,
		)
		if err != nil {
			return err
		}
		vmIDs = append(vmIDs, vm.ID)
	}

	if len(vmIDs) == 0 {
		if _, err := tx.Exec(ctx, `delete from hypervisor_vms where hypervisor_id = $1`, params.ID); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx, `delete from hypervisor_vms where hypervisor_id = $1 and not (id = any($2))`, params.ID, vmIDs); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) ListAdminHypervisors(ctx context.Context) ([]AdminHypervisorListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			h.id,
			h.server_id,
			coalesce(s.metadata->>'hostname', s.metadata->>'label', s.metadata->>'asset_tag', '') as server_hostname,
			h.hostname,
			case
				when h.last_reported_at is null or h.last_reported_at < now() - interval '2 minutes' then 'offline'
				else h.status
			end as status,
			h.vcpus_total,
			h.vcpus_active,
			h.memory_total_bytes,
			h.memory_active_bytes,
			h.disk_total_bytes,
			h.disk_available_bytes,
			h.wireguard_interface,
			h.control_plane_address,
			count(v.id) as vm_count,
			count(v.id) filter (where v.status = 'running') as running_vm_count,
			h.last_reported_at,
			h.updated_at
		from hypervisors h
		left join servers s on s.id = h.server_id
		left join hypervisor_vms v on v.hypervisor_id = h.id
		group by h.id, s.metadata
		order by h.last_reported_at desc nulls last, h.updated_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminHypervisorListItem{}
	for rows.Next() {
		var item AdminHypervisorListItem
		var serverHostname string
		if err := rows.Scan(
			&item.ID,
			&item.ServerID,
			&serverHostname,
			&item.Hostname,
			&item.Status,
			&item.VCPUsTotal,
			&item.VCPUsActive,
			&item.MemoryTotalBytes,
			&item.MemoryActiveBytes,
			&item.DiskTotalBytes,
			&item.DiskAvailableBytes,
			&item.WireguardInterface,
			&item.ControlPlaneAddress,
			&item.VMCount,
			&item.RunningVMCount,
			&item.LastReportedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if serverHostname != "" {
			item.ServerHostname = &serverHostname
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) UpsertHypervisorCommand(ctx context.Context, params UpsertHypervisorCommandParams) error {
	payload := params.Payload
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	_, err := s.pool.Exec(ctx, `
		insert into hypervisor_commands (hypervisor_id, command_id, command_type, status, payload)
		values ($1, $2, $3, 'pending', $4)
		on conflict (command_id) do update
		set hypervisor_id = excluded.hypervisor_id,
			command_type = excluded.command_type,
			payload = excluded.payload,
			updated_at = now()
		where hypervisor_commands.status in ('pending', 'sent')`,
		strings.TrimSpace(params.HypervisorID),
		strings.TrimSpace(params.CommandID),
		strings.TrimSpace(params.CommandType),
		payload,
	)
	return err
}

func (s *PostgresStore) MarkHypervisorCommandSent(ctx context.Context, commandID string) error {
	_, err := s.pool.Exec(ctx, `
		update hypervisor_commands
		set status = 'sent',
			sent_at = coalesce(sent_at, now()),
			updated_at = now()
		where command_id = $1 and status in ('pending', 'sent')`, strings.TrimSpace(commandID))
	return err
}

func (s *PostgresStore) CompleteHypervisorCommand(ctx context.Context, params CompleteHypervisorCommandParams) error {
	result := params.Result
	if len(result) == 0 {
		result = []byte(`{}`)
	}
	status := strings.TrimSpace(params.Status)
	if status != "succeeded" && status != "failed" {
		status = "failed"
	}
	_, err := s.pool.Exec(ctx, `
		update hypervisor_commands
		set status = $2,
			result = $3,
			error_message = $4,
			completed_at = coalesce(completed_at, now()),
			updated_at = now()
		where command_id = $1`, strings.TrimSpace(params.CommandID), status, result, strings.TrimSpace(params.ErrorMessage))
	return err
}

func (s *PostgresStore) ListAdminHypervisorVMs(ctx context.Context, hypervisorID string) ([]AdminHypervisorVMListItem, error) {
	rows, err := s.pool.Query(ctx, `
		select
			id,
			hypervisor_id,
			name,
			status,
			vcpus,
			memory_bytes,
			disk_bytes,
			mac_addresses,
			ip_addresses,
			metadata,
			last_reported_at,
			updated_at
		from hypervisor_vms
		where hypervisor_id = $1
		order by name`, strings.TrimSpace(hypervisorID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminHypervisorVMListItem{}
	for rows.Next() {
		var item AdminHypervisorVMListItem
		var metadata []byte
		if err := rows.Scan(
			&item.ID,
			&item.HypervisorID,
			&item.Name,
			&item.Status,
			&item.VCPUs,
			&item.MemoryBytes,
			&item.DiskBytes,
			&item.MACAddresses,
			&item.IPAddresses,
			&metadata,
			&item.LastReportedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.Metadata = map[string]string{}
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}
