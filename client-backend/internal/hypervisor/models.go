package hypervisor

import "time"

type HostSnapshot struct {
	HypervisorID        string    `json:"hypervisor_id"`
	Hostname            string    `json:"hostname"`
	CollectedAt         time.Time `json:"collected_at"`
	VCPUsTotal          int       `json:"vcpus_total"`
	VCPUsActive         int       `json:"vcpus_active"`
	MemoryTotalBytes    uint64    `json:"memory_total_bytes"`
	MemoryActiveBytes   uint64    `json:"memory_active_bytes"`
	DiskTotalBytes      uint64    `json:"disk_total_bytes"`
	DiskAvailableBytes  uint64    `json:"disk_available_bytes"`
	VMs                 []VMInfo  `json:"vms"`
	WireGuardInterface  string    `json:"wireguard_interface"`
	ControlPlaneAddress string    `json:"control_plane_address"`
}

type VMStatus string

const (
	VMStatusRunning VMStatus = "running"
	VMStatusStopped VMStatus = "stopped"
	VMStatusPaused  VMStatus = "paused"
	VMStatusUnknown VMStatus = "unknown"
)

type VMInfo struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Status        VMStatus          `json:"status"`
	VCPUs         int               `json:"vcpus"`
	MemoryBytes   uint64            `json:"memory_bytes"`
	DiskBytes     uint64            `json:"disk_bytes"`
	MACAddresses  []string          `json:"mac_addresses"`
	IPAddresses   []string          `json:"ip_addresses"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	LastUpdatedAt time.Time         `json:"last_updated_at"`
}

type VMCreateRequest struct {
	Name             string            `json:"name"`
	VCPUs            int               `json:"vcpus"`
	MemoryMiB        int               `json:"memory_mib"`
	DiskGiB          int               `json:"disk_gib"`
	ImagePath        string            `json:"image_path"`
	CloudInitISOPath string            `json:"cloud_init_iso_path"`
	NetworkName      string            `json:"network_name"`
	MACAddress       string            `json:"mac_address"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type VMDeleteRequest struct {
	Name          string `json:"name"`
	RemoveDisk    bool   `json:"remove_disk"`
	ForcePowerOff bool   `json:"force_power_off"`
}

type VMCommandResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
