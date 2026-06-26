package hypervisor

import "testing"

func TestParseLibvirtFields(t *testing.T) {
	nodeInfo := `CPU(s):              32
Memory size:        131072000 KiB
`
	if got := parseIntField(nodeInfo, "CPU(s)"); got != 32 {
		t.Fatalf("CPU(s) = %d, want 32", got)
	}
	if got := parseKiBField(nodeInfo, "Memory size"); got != 131072000 {
		t.Fatalf("memory KiB = %d, want 131072000", got)
	}
}

func TestNormalizeVMStatus(t *testing.T) {
	cases := map[string]VMStatus{
		"running\n": VMStatusRunning,
		"shut off":  VMStatusStopped,
		"paused":    VMStatusPaused,
		"crashed":   VMStatusUnknown,
	}
	for input, want := range cases {
		if got := normalizeVMStatus(input); got != want {
			t.Fatalf("normalizeVMStatus(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseVMAddresses(t *testing.T) {
	output := `Name       MAC address          Protocol     Address
-------------------------------------------------------------------------------
vnet0      52:54:00:aa:bb:cc    ipv4         192.168.122.14/24
`
	if got := parseMACAddresses(output); len(got) != 1 || got[0] != "52:54:00:aa:bb:cc" {
		t.Fatalf("macs = %#v", got)
	}
	if got := parseIPAddresses(output); len(got) != 1 || got[0] != "192.168.122.14" {
		t.Fatalf("ips = %#v", got)
	}
}
