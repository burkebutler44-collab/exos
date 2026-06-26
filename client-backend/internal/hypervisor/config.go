package hypervisor

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HypervisorID             string
	Hostname                 string
	ControlPlaneGRPCEndpoint string
	ControlPlaneAuthority    string
	ControlPlaneTLS          bool
	ControlPlaneAgentToken   string
	ControlPlaneCAFile       string
	ClientTLSCertFile        string
	ClientTLSKeyFile         string
	ReportInterval           time.Duration
	CommandStateDirectory    string
	LibvirtURI               string
	StoragePool              string
	VMImageDirectory         string
	WireGuardInterface       string
	WireGuardAddress         string
	WireGuardListenPort      string
	WireGuardPrivateKey      string
	WireGuardPeerPublicKey   string
	WireGuardPeerEndpoint    string
	WireGuardPeerAllowedIPs  string
	SSHAllowedCIDR           string
	ApplyHostNetworking      bool
}

func LoadConfig() Config {
	hostname, _ := os.Hostname()
	return Config{
		HypervisorID:             env("HYPERVISOR_ID", hostname),
		Hostname:                 env("HYPERVISOR_HOSTNAME", hostname),
		ControlPlaneGRPCEndpoint: env("CONTROL_PLANE_GRPC_ENDPOINT", "172.200.0.1:9090"),
		ControlPlaneAuthority:    os.Getenv("CONTROL_PLANE_GRPC_AUTHORITY"),
		ControlPlaneTLS:          boolEnv("CONTROL_PLANE_GRPC_TLS", false),
		ControlPlaneAgentToken:   os.Getenv("HYPERVISOR_AGENT_TOKEN"),
		ControlPlaneCAFile:       os.Getenv("CONTROL_PLANE_GRPC_CA_FILE"),
		ClientTLSCertFile:        os.Getenv("HYPERVISOR_CLIENT_TLS_CERT_FILE"),
		ClientTLSKeyFile:         os.Getenv("HYPERVISOR_CLIENT_TLS_KEY_FILE"),
		ReportInterval:           durationEnv("HYPERVISOR_REPORT_INTERVAL", 15*time.Second),
		CommandStateDirectory:    env("HYPERVISOR_COMMAND_STATE_DIR", "/var/lib/exos/hypervisor-agent/commands"),
		LibvirtURI:               env("LIBVIRT_URI", "qemu:///system"),
		StoragePool:              env("LIBVIRT_STORAGE_POOL", "default"),
		VMImageDirectory:         env("VM_IMAGE_DIRECTORY", "/var/lib/libvirt/images"),
		WireGuardInterface:       env("WG_INTERFACE", "wg0"),
		WireGuardAddress:         os.Getenv("WG_ADDRESS"),
		WireGuardListenPort:      env("WG_LISTEN_PORT", "51820"),
		WireGuardPrivateKey:      os.Getenv("WG_PRIVATE_KEY"),
		WireGuardPeerPublicKey:   os.Getenv("WG_PEER_PUBLIC_KEY"),
		WireGuardPeerEndpoint:    env("WG_PEER_ENDPOINT", "vpn-1.exos.tech:51820"),
		WireGuardPeerAllowedIPs:  env("WG_PEER_ALLOWED_IPS", "172.200.0.1/32"),
		SSHAllowedCIDR:           normalizeCIDR(env("SSH_ALLOWED_CIDR", "34.48.27.200")),
		ApplyHostNetworking:      boolEnv("APPLY_HOST_NETWORKING", false),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizeCIDR(value string) string {
	if strings.Contains(value, "/") {
		return value
	}
	return value + "/32"
}
