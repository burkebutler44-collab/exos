package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

func (s *PostgresStore) GetWireGuardGateway(ctx context.Context, gatewayID string) (WireGuardGateway, error) {
	var gateway WireGuardGateway
	var metadata []byte
	err := s.pool.QueryRow(ctx, `
		select
			id,
			name,
			interface_name,
			public_key,
			endpoint,
			coalesce(tunnel_address::text, ''),
			management_cidr::text,
			control_plane_allowed_ips,
			node_name,
			status,
			metadata
		from wireguard_gateways
		where id = $1`,
		gatewayID,
	).Scan(
		&gateway.ID,
		&gateway.Name,
		&gateway.InterfaceName,
		&gateway.PublicKey,
		&gateway.Endpoint,
		&gateway.TunnelAddress,
		&gateway.ManagementCIDR,
		&gateway.ControlPlaneAllowedIPs,
		&gateway.NodeName,
		&gateway.Status,
		&metadata,
	)
	if err != nil {
		return WireGuardGateway{}, err
	}
	gateway.Metadata = map[string]string{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &gateway.Metadata); err != nil {
			return WireGuardGateway{}, err
		}
	}
	return gateway, nil
}

func (s *PostgresStore) ListWireGuardPeersForGateway(ctx context.Context, gatewayID string) ([]WireGuardPeerDesiredState, error) {
	rows, err := s.pool.Query(ctx, `
		select
			id,
			hypervisor_id,
			gateway_id,
			wireguard_public_key,
			wireguard_management_ip::text,
			allowed_ips,
			endpoint,
			desired_state,
			actual_state,
			last_handshake_at,
			metadata
		from hypervisor_wireguard_peers
		where gateway_id = $1
			and revoked_at is null
		order by hypervisor_id`,
		gatewayID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	peers := []WireGuardPeerDesiredState{}
	for rows.Next() {
		var peer WireGuardPeerDesiredState
		var metadata []byte
		if err := rows.Scan(
			&peer.ID,
			&peer.HypervisorID,
			&peer.GatewayID,
			&peer.WireGuardPublicKey,
			&peer.WireGuardManagementIP,
			&peer.AllowedIPs,
			&peer.Endpoint,
			&peer.DesiredState,
			&peer.ActualState,
			&peer.LastHandshakeAt,
			&metadata,
		); err != nil {
			return nil, err
		}
		peer.Metadata = map[string]string{}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &peer.Metadata); err != nil {
				return nil, err
			}
		}
		peers = append(peers, peer)
	}
	return peers, rows.Err()
}

func (s *PostgresStore) UpdateWireGuardPeerActualState(ctx context.Context, params UpdateWireGuardPeerActualStateParams) error {
	if params.ID == uuid.Nil {
		return nil
	}
	if params.LastReconciledAt.IsZero() {
		params.LastReconciledAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
		update hypervisor_wireguard_peers
		set actual_state = $2,
			last_handshake_at = coalesce($3, last_handshake_at),
			error_message = $4,
			last_reconciled_at = $5,
			revoked_at = case when $6 then coalesce(revoked_at, $5) else revoked_at end,
			updated_at = now()
		where id = $1`,
		params.ID,
		params.ActualState,
		params.LastHandshakeAt,
		params.ErrorMessage,
		params.LastReconciledAt,
		params.MarkRevoked,
	)
	return err
}
