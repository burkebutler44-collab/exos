package wireguard

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"
	"time"

	"relay/client-backend/internal/store"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Repository interface {
	GetWireGuardGateway(ctx context.Context, gatewayID string) (store.WireGuardGateway, error)
	ListWireGuardPeersForGateway(ctx context.Context, gatewayID string) ([]store.WireGuardPeerDesiredState, error)
	UpdateWireGuardPeerActualState(ctx context.Context, params store.UpdateWireGuardPeerActualStateParams) error
}

type DeviceClient interface {
	Device(name string) (*wgtypes.Device, error)
	ConfigureDevice(name string, cfg wgtypes.Config) error
	Close() error
}

type Controller struct {
	repo      Repository
	client    DeviceClient
	gatewayID string
	interval  time.Duration
	logger    *log.Logger
}

type Option func(*Controller)

func WithClient(client DeviceClient) Option {
	return func(c *Controller) {
		c.client = client
	}
}

func WithLogger(logger *log.Logger) Option {
	return func(c *Controller) {
		c.logger = logger
	}
}

func NewController(repo Repository, gatewayID string, interval time.Duration, opts ...Option) (*Controller, error) {
	if strings.TrimSpace(gatewayID) == "" {
		return nil, errors.New("wireguard gateway id is required")
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}
	controller := &Controller{
		repo:      repo,
		gatewayID: gatewayID,
		interval:  interval,
		logger:    log.Default(),
	}
	for _, opt := range opts {
		opt(controller)
	}
	if controller.client == nil {
		client, err := wgctrl.New()
		if err != nil {
			return nil, fmt.Errorf("open wgctrl: %w", err)
		}
		controller.client = client
	}
	return controller, nil
}

func (c *Controller) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (c *Controller) Run(ctx context.Context) error {
	if err := c.Reconcile(ctx); err != nil {
		c.logger.Printf("wireguard reconcile failed: %v", err)
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.Reconcile(ctx); err != nil {
				c.logger.Printf("wireguard reconcile failed: %v", err)
			}
		}
	}
}

func (c *Controller) Reconcile(ctx context.Context) error {
	gateway, err := c.repo.GetWireGuardGateway(ctx, c.gatewayID)
	if err != nil {
		return fmt.Errorf("load gateway: %w", err)
	}
	if gateway.Status != "active" {
		return nil
	}

	device, err := c.client.Device(gateway.InterfaceName)
	if err != nil {
		return fmt.Errorf("load wireguard device %q: %w", gateway.InterfaceName, err)
	}
	desiredPeers, err := c.repo.ListWireGuardPeersForGateway(ctx, gateway.ID)
	if err != nil {
		return fmt.Errorf("load peers: %w", err)
	}

	known := map[wgtypes.Key]wgtypes.Peer{}
	for _, peer := range device.Peers {
		known[peer.PublicKey] = peer
	}

	type pendingUpdate struct {
		params store.UpdateWireGuardPeerActualStateParams
	}
	peerConfigs := []wgtypes.PeerConfig{}
	pending := []pendingUpdate{}
	now := time.Now().UTC()
	for _, desired := range desiredPeers {
		cfg, state, stateErr := peerConfig(desired)
		if stateErr != nil {
			if err := c.repo.UpdateWireGuardPeerActualState(ctx, store.UpdateWireGuardPeerActualStateParams{
				ID:               desired.ID,
				ActualState:      "failed",
				ErrorMessage:     stateErr.Error(),
				LastReconciledAt: now,
			}); err != nil {
				return err
			}
			continue
		}
		peerConfigs = append(peerConfigs, cfg)
		pending = append(pending, pendingUpdate{params: store.UpdateWireGuardPeerActualStateParams{
			ID:               desired.ID,
			ActualState:      state,
			LastHandshakeAt:  handshakeFor(known[cfg.PublicKey]),
			MarkRevoked:      desired.DesiredState == "absent",
			LastReconciledAt: now,
		}})
	}
	if len(peerConfigs) == 0 {
		return nil
	}

	if err := c.client.ConfigureDevice(gateway.InterfaceName, wgtypes.Config{Peers: peerConfigs}); err != nil {
		return fmt.Errorf("configure wireguard device %q: %w", gateway.InterfaceName, err)
	}
	for _, update := range pending {
		if err := c.repo.UpdateWireGuardPeerActualState(ctx, update.params); err != nil {
			return err
		}
	}
	return nil
}

func peerConfig(peer store.WireGuardPeerDesiredState) (wgtypes.PeerConfig, string, error) {
	key, err := wgtypes.ParseKey(peer.WireGuardPublicKey)
	if err != nil {
		return wgtypes.PeerConfig{}, "failed", fmt.Errorf("parse public key for %s: %w", peer.HypervisorID, err)
	}
	if peer.DesiredState == "absent" {
		return wgtypes.PeerConfig{
			PublicKey: key,
			Remove:    true,
		}, "removed", nil
	}

	allowedIPs := []net.IPNet{}
	for _, value := range peer.AllowedIPs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			return wgtypes.PeerConfig{}, "failed", fmt.Errorf("parse allowed ip %q for %s: %w", value, peer.HypervisorID, err)
		}
		allowedIPs = append(allowedIPs, net.IPNet{
			IP:   net.IP(prefix.Addr().AsSlice()),
			Mask: net.CIDRMask(prefix.Bits(), prefix.Addr().BitLen()),
		})
	}
	if len(allowedIPs) == 0 {
		prefix, err := netip.ParsePrefix(peer.WireGuardManagementIP + "/32")
		if err != nil {
			return wgtypes.PeerConfig{}, "failed", fmt.Errorf("parse management ip for %s: %w", peer.HypervisorID, err)
		}
		allowedIPs = append(allowedIPs, net.IPNet{
			IP:   net.IP(prefix.Addr().AsSlice()),
			Mask: net.CIDRMask(prefix.Bits(), prefix.Addr().BitLen()),
		})
	}

	cfg := wgtypes.PeerConfig{
		PublicKey:         key,
		ReplaceAllowedIPs: true,
		AllowedIPs:        allowedIPs,
	}
	if peer.Endpoint != nil && strings.TrimSpace(*peer.Endpoint) != "" {
		endpoint, err := net.ResolveUDPAddr("udp", strings.TrimSpace(*peer.Endpoint))
		if err != nil {
			return wgtypes.PeerConfig{}, "failed", fmt.Errorf("parse endpoint for %s: %w", peer.HypervisorID, err)
		}
		cfg.Endpoint = endpoint
	}
	return cfg, "applied", nil
}

func handshakeFor(peer wgtypes.Peer) *time.Time {
	if peer.LastHandshakeTime.IsZero() {
		return nil
	}
	return &peer.LastHandshakeTime
}
