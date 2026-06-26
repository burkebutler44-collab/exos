package adapters

import (
	"context"
	"strings"
	"testing"

	"relay/client-backend/internal/provisioning/messages"
)

func TestKubernetesTinkerbellAdapterAppliesHardwareMachineAndWorkflow(t *testing.T) {
	applier := &captureApplier{}
	adapter := NewKubernetesTinkerbellAdapter(applier, staticSecrets{}, "tink")
	adapter.ArtifactServer = "10.0.0.10:8080"

	cmd := messages.ProvisionServerCommand{
		OrganizationID: "org-1",
		ServerID:       "server-01",
		ImageID:        "ubuntu-24-04",
		Hostname:       "web-1",
		SSHKeys:        []string{"ssh-ed25519 AAAA"},
		NetworkConfig: messages.NetworkConfig{
			"ip":      "192.0.2.10",
			"gateway": "192.0.2.1",
			"netmask": "255.255.255.0",
		},
		HardwareMetadata: map[string]any{
			"mac":       "52:54:00:aa:bb:cc",
			"disk_name": "/dev/sda",
		},
	}

	if err := adapter.EnsureHardware(context.Background(), cmd); err != nil {
		t.Fatalf("EnsureHardware returned error: %v", err)
	}
	first := string(applier.applied[0])
	for _, want := range []string{"kind: Secret", "kind: Machine", "kind: Hardware", "name: server-01", "mac: 52:54:00:aa:bb:cc"} {
		if !strings.Contains(first, want) {
			t.Fatalf("hardware manifest missing %q:\n%s", want, first)
		}
	}

	workflowID, err := adapter.CreateWorkflow(context.Background(), cmd)
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	if workflowID != "workflow-server-01" {
		t.Fatalf("workflow id = %s", workflowID)
	}
	second := string(applier.applied[1])
	for _, want := range []string{"kind: Workflow", "templateRef: ubuntu-24-04", "hardwareRef: server-01", "hostname: web-1"} {
		if !strings.Contains(second, want) {
			t.Fatalf("workflow manifest missing %q:\n%s", want, second)
		}
	}
}

func TestKubernetesBMCAdapterAppliesPXEJob(t *testing.T) {
	applier := &captureApplier{}
	adapter := NewKubernetesBMCAdapter(applier, "tink")
	if err := adapter.SetOneTimePXEBoot(context.Background(), "server-01"); err != nil {
		t.Fatalf("SetOneTimePXEBoot returned error: %v", err)
	}
	manifest := string(applier.applied[0])
	for _, want := range []string{"kind: Job", "oneTimeBootDeviceAction", `"pxe"`, `powerAction: "on"`} {
		if !strings.Contains(manifest, want) {
			t.Fatalf("pxe job missing %q:\n%s", want, manifest)
		}
	}
}

type captureApplier struct {
	applied [][]byte
}

func (a *captureApplier) Apply(ctx context.Context, manifest []byte) error {
	a.applied = append(a.applied, append([]byte(nil), manifest...))
	return nil
}

func (a *captureApplier) GetJSON(ctx context.Context, resource, name, namespace string) ([]byte, error) {
	return []byte(`{"status":{"state":"completed"}}`), nil
}

type staticSecrets struct{}

func (staticSecrets) GetBMCCredentials(ctx context.Context, serverID string) (BMCCredentials, error) {
	return BMCCredentials{Username: "admin", Password: "password", Endpoint: "192.0.2.50"}, nil
}

func (staticSecrets) GetSwitchCredentials(ctx context.Context, switchID string) (SwitchCredentials, error) {
	return SwitchCredentials{}, nil
}
