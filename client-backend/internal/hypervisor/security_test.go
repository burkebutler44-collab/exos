package hypervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingExecutor struct {
	commands []string
	failUFW  bool
}

func (r *recordingExecutor) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := name + " " + strings.Join(args, " ")
	r.commands = append(r.commands, command)
	if r.failUFW && name == "ufw" {
		return nil, assertErr("ufw unavailable")
	}
	if name == "iptables" && len(args) > 0 && args[0] == "-C" {
		return nil, assertErr("rule missing")
	}
	return []byte("ok"), nil
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

func TestRestrictSSHAllowsOnlyConfiguredCIDR(t *testing.T) {
	exec := &recordingExecutor{}
	manager := NewHostSecurityManager(Config{SSHAllowedCIDR: "34.48.27.200"}, exec)
	if err := manager.RestrictSSH(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(exec.commands, "\n")
	if !strings.Contains(joined, "ufw allow from 34.48.27.200/32 to any port 22 proto tcp") {
		t.Fatalf("missing allow command:\n%s", joined)
	}
	if !strings.Contains(joined, "ufw deny 22/tcp") {
		t.Fatalf("missing deny command:\n%s", joined)
	}
}

func TestRestrictSSHFallsBackToIptables(t *testing.T) {
	exec := &recordingExecutor{failUFW: true}
	manager := NewHostSecurityManager(Config{SSHAllowedCIDR: "34.48.27.200/32"}, exec)
	if err := manager.RestrictSSH(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(exec.commands, "\n")
	if !strings.Contains(joined, "iptables -A INPUT -p tcp --dport 22 -s 34.48.27.200/32 -j ACCEPT") {
		t.Fatalf("missing iptables allow command:\n%s", joined)
	}
	if !strings.Contains(joined, "iptables -A INPUT -p tcp --dport 22 -j DROP") {
		t.Fatalf("missing iptables drop command:\n%s", joined)
	}
}

func TestWriteWireGuardConfigUsesPublicVPNTunnelEndpoint(t *testing.T) {
	tmp := t.TempDir()
	previousRoot := wireGuardConfigRoot
	wireGuardConfigRoot = tmp
	defer func() { wireGuardConfigRoot = previousRoot }()

	manager := NewHostSecurityManager(Config{
		WireGuardInterface:      "wg0",
		WireGuardAddress:        "172.200.1.1/32",
		WireGuardListenPort:     "51820",
		WireGuardPrivateKey:     "private-key",
		WireGuardPeerPublicKey:  "peer-public-key",
		WireGuardPeerEndpoint:   "vpn-1.exos.tech:51820",
		WireGuardPeerAllowedIPs: "172.200.0.1/32",
	}, nil)
	if err := manager.WriteWireGuardConfig(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(tmp, "wg0.conf"))
	if err != nil {
		t.Fatal(err)
	}
	config := string(raw)
	for _, want := range []string{
		"Address = 172.200.1.1/32",
		"AllowedIPs = 172.200.0.1/32",
		"Endpoint = vpn-1.exos.tech:51820",
		"PersistentKeepalive = 25",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("missing %q in config:\n%s", want, config)
		}
	}
}
