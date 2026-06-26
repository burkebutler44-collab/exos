package adapters

import (
	"context"
	"os"
	"strings"

	"relay/client-backend/internal/provisioning/messages"
)

type MockTinkerbellAdapter struct {
	Calls []string
	Fail  error
}

func (m *MockTinkerbellAdapter) EnsureHardware(ctx context.Context, command messages.ProvisionServerCommand) error {
	m.Calls = append(m.Calls, "ensure_hardware")
	return m.Fail
}

func (m *MockTinkerbellAdapter) CreateWorkflow(ctx context.Context, command messages.ProvisionServerCommand) (string, error) {
	m.Calls = append(m.Calls, "create_workflow")
	return "workflow-" + command.ServerID, m.Fail
}

func (m *MockTinkerbellAdapter) StartWorkflow(ctx context.Context, workflowID string) error {
	m.Calls = append(m.Calls, "start_workflow")
	return m.Fail
}

func (m *MockTinkerbellAdapter) WorkflowStatus(ctx context.Context, workflowID string) (string, error) {
	m.Calls = append(m.Calls, "workflow_status")
	return "completed", m.Fail
}

func (m *MockTinkerbellAdapter) Health(ctx context.Context) (string, error) {
	return "online", m.Fail
}

type MockIPMIAdapter struct {
	Calls []string
	Fail  error
}

func (m *MockIPMIAdapter) PowerOn(ctx context.Context, serverID string) error {
	m.Calls = append(m.Calls, "power_on")
	return m.Fail
}

func (m *MockIPMIAdapter) PowerOff(ctx context.Context, serverID string) error {
	m.Calls = append(m.Calls, "power_off")
	return m.Fail
}

func (m *MockIPMIAdapter) PowerCycle(ctx context.Context, serverID string) error {
	m.Calls = append(m.Calls, "power_cycle")
	return m.Fail
}

func (m *MockIPMIAdapter) Reset(ctx context.Context, serverID string) error {
	m.Calls = append(m.Calls, "reset")
	return m.Fail
}

func (m *MockIPMIAdapter) SetOneTimePXEBoot(ctx context.Context, serverID string) error {
	m.Calls = append(m.Calls, "set_pxe")
	return m.Fail
}

func (m *MockIPMIAdapter) PowerState(ctx context.Context, serverID string) (string, error) {
	m.Calls = append(m.Calls, "power_state")
	return "on", m.Fail
}

func (m *MockIPMIAdapter) CheckBMC(ctx context.Context, serverID string) (bool, error) {
	m.Calls = append(m.Calls, "check_bmc")
	return true, m.Fail
}

type MockNetworkAdapter struct {
	Calls []string
	Fail  error
}

func (m *MockNetworkAdapter) MoveToProvisioningVLAN(ctx context.Context, serverID string) error {
	m.Calls = append(m.Calls, "move_to_provisioning_vlan")
	return m.Fail
}

func (m *MockNetworkAdapter) MoveToCustomerVLAN(ctx context.Context, serverID string, vlanID *int) error {
	m.Calls = append(m.Calls, "move_to_customer_vlan")
	return m.Fail
}

func (m *MockNetworkAdapter) SetPortDescription(ctx context.Context, portID, description string) error {
	m.Calls = append(m.Calls, "set_port_description")
	return m.Fail
}

func (m *MockNetworkAdapter) EnablePort(ctx context.Context, portID string) error {
	m.Calls = append(m.Calls, "enable_port")
	return m.Fail
}

func (m *MockNetworkAdapter) DisablePort(ctx context.Context, portID string) error {
	m.Calls = append(m.Calls, "disable_port")
	return m.Fail
}

type LocalSecretProvider struct{}

func (LocalSecretProvider) GetBMCCredentials(ctx context.Context, serverID string) (BMCCredentials, error) {
	// TODO: Resolve BMC credentials from rack-local Kubernetes secrets or a local secret manager.
	endpoint := os.Getenv("BMC_ENDPOINT")
	if endpoint == "" {
		prefix := strings.TrimRight(os.Getenv("BMC_ENDPOINT_PREFIX"), "/")
		if prefix != "" {
			endpoint = prefix + "/" + serverID
		}
	}
	if endpoint == "" {
		endpoint = serverID
	}
	return BMCCredentials{
		Username: os.Getenv("BMC_USERNAME"),
		Password: os.Getenv("BMC_PASSWORD"),
		Endpoint: endpoint,
	}, nil
}

func (LocalSecretProvider) GetSwitchCredentials(ctx context.Context, switchID string) (SwitchCredentials, error) {
	// TODO: Resolve switch credentials locally. Do not accept raw switch passwords from central.
	return SwitchCredentials{Endpoint: switchID}, nil
}
