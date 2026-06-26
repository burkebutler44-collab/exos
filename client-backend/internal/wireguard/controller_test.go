package wireguard

import (
	"context"
	"errors"
	"testing"
	"time"

	"relay/client-backend/internal/store"

	"github.com/google/uuid"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type fakeRepo struct {
	gateway store.WireGuardGateway
	peers   []store.WireGuardPeerDesiredState
	updates []store.UpdateWireGuardPeerActualStateParams
}

func (r *fakeRepo) GetWireGuardGateway(ctx context.Context, gatewayID string) (store.WireGuardGateway, error) {
	if r.gateway.ID != gatewayID {
		return store.WireGuardGateway{}, errors.New("not found")
	}
	return r.gateway, nil
}

func (r *fakeRepo) ListWireGuardPeersForGateway(ctx context.Context, gatewayID string) ([]store.WireGuardPeerDesiredState, error) {
	return r.peers, nil
}

func (r *fakeRepo) UpdateWireGuardPeerActualState(ctx context.Context, params store.UpdateWireGuardPeerActualStateParams) error {
	r.updates = append(r.updates, params)
	return nil
}

type fakeDeviceClient struct {
	device      *wgtypes.Device
	configured  []wgtypes.PeerConfig
	configError error
}

func (c *fakeDeviceClient) Device(name string) (*wgtypes.Device, error) {
	return c.device, nil
}

func (c *fakeDeviceClient) ConfigureDevice(name string, cfg wgtypes.Config) error {
	c.configured = append(c.configured, cfg.Peers...)
	return c.configError
}

func (c *fakeDeviceClient) Close() error { return nil }

func TestControllerReconcileAppliesPeer(t *testing.T) {
	key := mustKey(t)
	peerID := uuid.New()
	repo := &fakeRepo{
		gateway: store.WireGuardGateway{ID: "wg-control-plane-1", InterfaceName: "wg0", Status: "active"},
		peers: []store.WireGuardPeerDesiredState{{
			ID:                    peerID,
			HypervisorID:          "hv-1",
			GatewayID:             "wg-control-plane-1",
			WireGuardPublicKey:    key.PublicKey().String(),
			WireGuardManagementIP: "172.200.1.10",
			AllowedIPs:            []string{"172.200.1.10/32"},
			DesiredState:          "present",
		}},
	}
	client := &fakeDeviceClient{device: &wgtypes.Device{Name: "wg0"}}
	controller, err := NewController(repo, "wg-control-plane-1", time.Hour, WithClient(client))
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(client.configured) != 1 {
		t.Fatalf("expected 1 configured peer, got %d", len(client.configured))
	}
	if got := client.configured[0].PublicKey.String(); got != key.PublicKey().String() {
		t.Fatalf("unexpected public key %q", got)
	}
	if len(repo.updates) != 1 || repo.updates[0].ActualState != "applied" {
		t.Fatalf("expected applied update, got %#v", repo.updates)
	}
}

func TestControllerReconcileRemovesAbsentPeer(t *testing.T) {
	key := mustKey(t)
	peerID := uuid.New()
	repo := &fakeRepo{
		gateway: store.WireGuardGateway{ID: "wg-control-plane-1", InterfaceName: "wg0", Status: "active"},
		peers: []store.WireGuardPeerDesiredState{{
			ID:                 peerID,
			HypervisorID:       "hv-1",
			GatewayID:          "wg-control-plane-1",
			WireGuardPublicKey: key.PublicKey().String(),
			DesiredState:       "absent",
		}},
	}
	client := &fakeDeviceClient{device: &wgtypes.Device{Name: "wg0"}}
	controller, err := NewController(repo, "wg-control-plane-1", time.Hour, WithClient(client))
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(client.configured) != 1 || !client.configured[0].Remove {
		t.Fatalf("expected remove peer config, got %#v", client.configured)
	}
	if len(repo.updates) != 1 || repo.updates[0].ActualState != "removed" || !repo.updates[0].MarkRevoked {
		t.Fatalf("expected removed/revoked update, got %#v", repo.updates)
	}
}

func TestControllerReconcileMarksInvalidPeerFailed(t *testing.T) {
	peerID := uuid.New()
	repo := &fakeRepo{
		gateway: store.WireGuardGateway{ID: "wg-control-plane-1", InterfaceName: "wg0", Status: "active"},
		peers: []store.WireGuardPeerDesiredState{{
			ID:                 peerID,
			HypervisorID:       "hv-1",
			GatewayID:          "wg-control-plane-1",
			WireGuardPublicKey: "not-a-key",
			DesiredState:       "present",
		}},
	}
	client := &fakeDeviceClient{device: &wgtypes.Device{Name: "wg0"}}
	controller, err := NewController(repo, "wg-control-plane-1", time.Hour, WithClient(client))
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(client.configured) != 0 {
		t.Fatalf("expected no device config for invalid peer, got %#v", client.configured)
	}
	if len(repo.updates) != 1 || repo.updates[0].ActualState != "failed" {
		t.Fatalf("expected failed update, got %#v", repo.updates)
	}
}

func mustKey(t *testing.T) wgtypes.Key {
	t.Helper()
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}
