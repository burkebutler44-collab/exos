package adapters

import (
	"context"

	"relay/client-backend/internal/provisioning/messages"
)

type BMCCredentials struct {
	Username string
	Password string
	Endpoint string
}

type SwitchCredentials struct {
	Username string
	Password string
	Endpoint string
}

type TinkerbellAdapter interface {
	EnsureHardware(ctx context.Context, command messages.ProvisionServerCommand) error
	CreateWorkflow(ctx context.Context, command messages.ProvisionServerCommand) (string, error)
	StartWorkflow(ctx context.Context, workflowID string) error
	WorkflowStatus(ctx context.Context, workflowID string) (string, error)
	Health(ctx context.Context) (string, error)
}

type IPMIAdapter interface {
	PowerOn(ctx context.Context, serverID string) error
	PowerOff(ctx context.Context, serverID string) error
	PowerCycle(ctx context.Context, serverID string) error
	Reset(ctx context.Context, serverID string) error
	SetOneTimePXEBoot(ctx context.Context, serverID string) error
	PowerState(ctx context.Context, serverID string) (string, error)
	CheckBMC(ctx context.Context, serverID string) (bool, error)
}

type NetworkAdapter interface {
	MoveToProvisioningVLAN(ctx context.Context, serverID string) error
	MoveToCustomerVLAN(ctx context.Context, serverID string, vlanID *int) error
	SetPortDescription(ctx context.Context, portID, description string) error
	EnablePort(ctx context.Context, portID string) error
	DisablePort(ctx context.Context, portID string) error
}

type RackSecretProvider interface {
	GetBMCCredentials(ctx context.Context, serverID string) (BMCCredentials, error)
	GetSwitchCredentials(ctx context.Context, switchID string) (SwitchCredentials, error)
}
