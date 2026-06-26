package hypervisor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type HostSecurityManager struct {
	Config   Config
	Executor CommandExecutor
}

var wireGuardConfigRoot = "/etc/wireguard"

func NewHostSecurityManager(cfg Config, executor CommandExecutor) *HostSecurityManager {
	if executor == nil {
		executor = OSExecutor{}
	}
	return &HostSecurityManager{Config: cfg, Executor: executor}
}

func (m *HostSecurityManager) Apply(ctx context.Context) error {
	if err := m.WriteWireGuardConfig(); err != nil {
		return err
	}
	if !m.Config.ApplyHostNetworking {
		return nil
	}
	if _, err := m.Executor.Run(ctx, "systemctl", "enable", "wg-quick@"+m.Config.WireGuardInterface); err != nil {
		return err
	}
	if _, err := m.Executor.Run(ctx, "systemctl", "restart", "wg-quick@"+m.Config.WireGuardInterface); err != nil {
		return err
	}
	return m.RestrictSSH(ctx)
}

func (m *HostSecurityManager) WriteWireGuardConfig() error {
	if strings.TrimSpace(m.Config.WireGuardPrivateKey) == "" {
		return nil
	}
	if strings.TrimSpace(m.Config.WireGuardAddress) == "" || strings.TrimSpace(m.Config.WireGuardPeerPublicKey) == "" {
		return fmt.Errorf("wireguard address and peer public key are required when WG_PRIVATE_KEY is set")
	}
	config := fmt.Sprintf(`[Interface]
Address = %s
ListenPort = %s
PrivateKey = %s

[Peer]
PublicKey = %s
AllowedIPs = %s
`, m.Config.WireGuardAddress, m.Config.WireGuardListenPort, m.Config.WireGuardPrivateKey, m.Config.WireGuardPeerPublicKey, m.Config.WireGuardPeerAllowedIPs)
	if strings.TrimSpace(m.Config.WireGuardPeerEndpoint) != "" {
		config += "Endpoint = " + strings.TrimSpace(m.Config.WireGuardPeerEndpoint) + "\n"
		config += "PersistentKeepalive = 25\n"
	}
	path := filepath.Join(wireGuardConfigRoot, m.Config.WireGuardInterface+".conf")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(config), 0600)
}

func (m *HostSecurityManager) RestrictSSH(ctx context.Context) error {
	cidr := normalizeCIDR(m.Config.SSHAllowedCIDR)
	// Prefer ufw because it is common on Ubuntu images and easy to inspect.
	if _, err := m.Executor.Run(ctx, "ufw", "allow", "from", cidr, "to", "any", "port", "22", "proto", "tcp"); err != nil {
		return m.restrictSSHWithIptables(ctx, cidr)
	}
	if _, err := m.Executor.Run(ctx, "ufw", "deny", "22/tcp"); err != nil {
		return err
	}
	_, err := m.Executor.Run(ctx, "ufw", "--force", "enable")
	return err
}

func (m *HostSecurityManager) restrictSSHWithIptables(ctx context.Context, cidr string) error {
	if _, err := m.Executor.Run(ctx, "iptables", "-C", "INPUT", "-p", "tcp", "--dport", "22", "-s", cidr, "-j", "ACCEPT"); err != nil {
		if _, addErr := m.Executor.Run(ctx, "iptables", "-A", "INPUT", "-p", "tcp", "--dport", "22", "-s", cidr, "-j", "ACCEPT"); addErr != nil {
			return addErr
		}
	}
	if _, err := m.Executor.Run(ctx, "iptables", "-C", "INPUT", "-p", "tcp", "--dport", "22", "-j", "DROP"); err != nil {
		_, err = m.Executor.Run(ctx, "iptables", "-A", "INPUT", "-p", "tcp", "--dport", "22", "-j", "DROP")
		return err
	}
	return nil
}
