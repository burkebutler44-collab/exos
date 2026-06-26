package hypervisor

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Hypervisor interface {
	Snapshot(ctx context.Context) (HostSnapshot, error)
	CreateVM(ctx context.Context, req VMCreateRequest) (VMCommandResult, error)
	DeleteVM(ctx context.Context, req VMDeleteRequest) (VMCommandResult, error)
	StartVM(ctx context.Context, name string) (VMCommandResult, error)
	StopVM(ctx context.Context, name string) (VMCommandResult, error)
}

type VirshHypervisor struct {
	Config   Config
	Executor CommandExecutor
}

func NewVirshHypervisor(cfg Config, executor CommandExecutor) *VirshHypervisor {
	if executor == nil {
		executor = OSExecutor{}
	}
	return &VirshHypervisor{Config: cfg, Executor: executor}
}

func (h *VirshHypervisor) Snapshot(ctx context.Context) (HostSnapshot, error) {
	nodeInfo, err := h.Executor.Run(ctx, "virsh", "-c", h.Config.LibvirtURI, "nodeinfo")
	if err != nil {
		return HostSnapshot{}, err
	}
	poolInfo, err := h.Executor.Run(ctx, "virsh", "-c", h.Config.LibvirtURI, "pool-info", h.Config.StoragePool, "--bytes")
	if err != nil {
		return HostSnapshot{}, err
	}
	vms, err := h.ListVMs(ctx)
	if err != nil {
		return HostSnapshot{}, err
	}
	totalMemory := parseKiBField(string(nodeInfo), "Memory size") * 1024
	activeVCPUs := 0
	activeMemory := uint64(0)
	for _, vm := range vms {
		if vm.Status == VMStatusRunning {
			activeVCPUs += vm.VCPUs
			activeMemory += vm.MemoryBytes
		}
	}
	return HostSnapshot{
		HypervisorID:        h.Config.HypervisorID,
		Hostname:            h.Config.Hostname,
		CollectedAt:         time.Now().UTC(),
		VCPUsTotal:          parseIntField(string(nodeInfo), "CPU(s)"),
		VCPUsActive:         activeVCPUs,
		MemoryTotalBytes:    totalMemory,
		MemoryActiveBytes:   activeMemory,
		DiskTotalBytes:      parseByteField(string(poolInfo), "Capacity"),
		DiskAvailableBytes:  parseByteField(string(poolInfo), "Available"),
		VMs:                 vms,
		WireGuardInterface:  h.Config.WireGuardInterface,
		ControlPlaneAddress: h.Config.ControlPlaneGRPCEndpoint,
	}, nil
}

func (h *VirshHypervisor) ListVMs(ctx context.Context) ([]VMInfo, error) {
	output, err := h.Executor.Run(ctx, "virsh", "-c", h.Config.LibvirtURI, "list", "--all", "--name")
	if err != nil {
		return nil, err
	}
	names := strings.Fields(string(output))
	vms := make([]VMInfo, 0, len(names))
	for _, name := range names {
		vm, err := h.describeVM(ctx, name)
		if err != nil {
			vm = VMInfo{Name: name, Status: VMStatusUnknown, Metadata: map[string]string{"error": err.Error()}, LastUpdatedAt: time.Now().UTC()}
		}
		vms = append(vms, vm)
	}
	return vms, nil
}

func (h *VirshHypervisor) describeVM(ctx context.Context, name string) (VMInfo, error) {
	infoOut, err := h.Executor.Run(ctx, "virsh", "-c", h.Config.LibvirtURI, "dominfo", name)
	if err != nil {
		return VMInfo{}, err
	}
	stateOut, _ := h.Executor.Run(ctx, "virsh", "-c", h.Config.LibvirtURI, "domstate", name)
	ifaceOut, _ := h.Executor.Run(ctx, "virsh", "-c", h.Config.LibvirtURI, "domifaddr", name)
	blkOut, _ := h.Executor.Run(ctx, "virsh", "-c", h.Config.LibvirtURI, "domblklist", name, "--details")
	info := string(infoOut)
	return VMInfo{
		ID:            parseStringField(info, "UUID"),
		Name:          name,
		Status:        normalizeVMStatus(string(stateOut)),
		VCPUs:         parseIntField(info, "CPU(s)"),
		MemoryBytes:   parseKiBField(info, "Max memory") * 1024,
		DiskBytes:     parseDiskBytes(string(blkOut)),
		MACAddresses:  parseMACAddresses(string(ifaceOut)),
		IPAddresses:   parseIPAddresses(string(ifaceOut)),
		LastUpdatedAt: time.Now().UTC(),
	}, nil
}

func (h *VirshHypervisor) CreateVM(ctx context.Context, req VMCreateRequest) (VMCommandResult, error) {
	if strings.TrimSpace(req.Name) == "" || req.VCPUs < 1 || req.MemoryMiB < 256 || req.DiskGiB < 1 {
		return VMCommandResult{}, fmt.Errorf("invalid vm create request")
	}
	diskPath := filepath.Join(h.Config.VMImageDirectory, req.Name+".qcow2")
	if _, err := h.Executor.Run(ctx, "qemu-img", "create", "-f", "qcow2", "-b", req.ImagePath, "-F", "qcow2", diskPath, fmt.Sprintf("%dG", req.DiskGiB)); err != nil {
		return VMCommandResult{}, err
	}
	args := []string{
		"--connect", h.Config.LibvirtURI,
		"--name", req.Name,
		"--vcpus", strconv.Itoa(req.VCPUs),
		"--memory", strconv.Itoa(req.MemoryMiB),
		"--disk", "path=" + diskPath + ",format=qcow2,bus=virtio",
		"--import",
		"--os-variant", "ubuntu24.04",
		"--graphics", "none",
		"--noautoconsole",
	}
	if req.NetworkName != "" {
		network := "network=" + req.NetworkName + ",model=virtio"
		if req.MACAddress != "" {
			network += ",mac=" + req.MACAddress
		}
		args = append(args, "--network", network)
	}
	if req.CloudInitISOPath != "" {
		args = append(args, "--disk", "path="+req.CloudInitISOPath+",device=cdrom")
	}
	if _, err := h.Executor.Run(ctx, "virt-install", args...); err != nil {
		return VMCommandResult{}, err
	}
	return VMCommandResult{Name: req.Name, Status: "created"}, nil
}

func (h *VirshHypervisor) DeleteVM(ctx context.Context, req VMDeleteRequest) (VMCommandResult, error) {
	if req.ForcePowerOff {
		_, _ = h.Executor.Run(ctx, "virsh", "-c", h.Config.LibvirtURI, "destroy", req.Name)
	}
	args := []string{"-c", h.Config.LibvirtURI, "undefine", req.Name}
	if req.RemoveDisk {
		args = append(args, "--remove-all-storage")
	}
	if _, err := h.Executor.Run(ctx, "virsh", args...); err != nil {
		return VMCommandResult{}, err
	}
	return VMCommandResult{Name: req.Name, Status: "deleted"}, nil
}

func (h *VirshHypervisor) StartVM(ctx context.Context, name string) (VMCommandResult, error) {
	_, err := h.Executor.Run(ctx, "virsh", "-c", h.Config.LibvirtURI, "start", name)
	return VMCommandResult{Name: name, Status: "started"}, err
}

func (h *VirshHypervisor) StopVM(ctx context.Context, name string) (VMCommandResult, error) {
	_, err := h.Executor.Run(ctx, "virsh", "-c", h.Config.LibvirtURI, "shutdown", name)
	return VMCommandResult{Name: name, Status: "stopping"}, err
}

func parseStringField(output, field string) string {
	prefix := field + ":"
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func parseIntField(output, field string) int {
	value, _ := strconv.Atoi(strings.Fields(parseStringField(output, field))[0])
	return value
}

func parseKiBField(output, field string) uint64 {
	parts := strings.Fields(parseStringField(output, field))
	if len(parts) == 0 {
		return 0
	}
	value, _ := strconv.ParseUint(parts[0], 10, 64)
	return value
}

func parseByteField(output, field string) uint64 {
	parts := strings.Fields(parseStringField(output, field))
	if len(parts) == 0 {
		return 0
	}
	value, _ := strconv.ParseUint(parts[0], 10, 64)
	return value
}

func normalizeVMStatus(value string) VMStatus {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "running":
		return VMStatusRunning
	case "shut off", "shutoff", "shutdown":
		return VMStatusStopped
	case "paused":
		return VMStatusPaused
	default:
		return VMStatusUnknown
	}
}

func parseMACAddresses(output string) []string {
	var values []string
	for _, line := range strings.Split(output, "\n") {
		for _, field := range strings.Fields(line) {
			if strings.Count(field, ":") == 5 {
				values = append(values, field)
			}
		}
	}
	return values
}

func parseIPAddresses(output string) []string {
	var values []string
	for _, line := range strings.Split(output, "\n") {
		for _, field := range strings.Fields(line) {
			if strings.Contains(field, "/") && strings.Count(field, ".") == 3 {
				values = append(values, strings.Split(field, "/")[0])
			}
		}
	}
	return values
}

func parseDiskBytes(output string) uint64 {
	// virsh does not report allocated disk sizes in domblklist consistently.
	// TODO: query qemu-img info --output=json for each block path and sum virtual-size.
	return 0
}
