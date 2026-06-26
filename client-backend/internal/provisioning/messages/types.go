package messages

import "time"

const (
	ProvisionServerCommandType = "ProvisionServerCommand"
	ReinstallServerCommandType = "ReinstallServerCommand"
	PowerCommandType           = "PowerCommand"
	RescueModeCommandType      = "RescueModeCommand"
	NetworkCommandType         = "NetworkCommand"

	ProvisioningStartedEventType                   = "ProvisioningStartedEvent"
	ProvisioningPxeBootSetEventType                = "ProvisioningPxeBootSetEvent"
	ProvisioningPowerCycledEventType               = "ProvisioningPowerCycledEvent"
	ProvisioningTinkerbellWorkflowCreatedEventType = "ProvisioningTinkerbellWorkflowCreatedEvent"
	ProvisioningInstallingEventType                = "ProvisioningInstallingEvent"
	ProvisioningCompletedEventType                 = "ProvisioningCompletedEvent"
	ProvisioningFailedEventType                    = "ProvisioningFailedEvent"
	PowerStateChangedEventType                     = "PowerStateChangedEvent"
	PowerCommandCompletedEventType                 = "PowerCommandCompletedEvent"
	PowerCommandFailedEventType                    = "PowerCommandFailedEvent"
	RackHeartbeatEventType                         = "RackHeartbeatEvent"
	RackHealthChangedEventType                     = "RackHealthChangedEvent"
	TinkerbellHealthChangedEventType               = "TinkerbellHealthChangedEvent"
	NetworkCommandCompletedEventType               = "NetworkCommandCompletedEvent"
	NetworkCommandFailedEventType                  = "NetworkCommandFailedEvent"
)

type NetworkConfig map[string]any

type ProvisionServerCommand struct {
	OrganizationID   string         `json:"organization_id"`
	ProjectID        *string        `json:"project_id,omitempty"`
	ServerID         string         `json:"server_id"`
	ImageID          string         `json:"image_id"`
	Hostname         string         `json:"hostname"`
	SSHKeys          []string       `json:"ssh_keys"`
	NetworkConfig    NetworkConfig  `json:"network_config"`
	HardwareMetadata map[string]any `json:"hardware_metadata"`
}

type ReinstallServerCommand struct {
	OrganizationID string        `json:"organization_id"`
	ServerID       string        `json:"server_id"`
	ImageID        string        `json:"image_id"`
	Hostname       string        `json:"hostname"`
	SSHKeys        []string      `json:"ssh_keys"`
	NetworkConfig  NetworkConfig `json:"network_config"`
}

type PowerAction string

const (
	PowerOn     PowerAction = "power_on"
	PowerOff    PowerAction = "power_off"
	PowerCycle  PowerAction = "power_cycle"
	PowerReset  PowerAction = "reset"
	PowerStatus PowerAction = "status"
)

type PowerCommand struct {
	OrganizationID string      `json:"organization_id"`
	ServerID       string      `json:"server_id"`
	Action         PowerAction `json:"action"`
}

type RescueModeCommand struct {
	OrganizationID string   `json:"organization_id"`
	ServerID       string   `json:"server_id"`
	RescueImageID  string   `json:"rescue_image_id"`
	SSHKeys        []string `json:"ssh_keys"`
}

type NetworkCommand struct {
	OrganizationID string         `json:"organization_id"`
	ServerID       string         `json:"server_id"`
	Action         string         `json:"action"`
	VLANID         *int           `json:"vlan_id,omitempty"`
	PortID         *string        `json:"port_id,omitempty"`
	NetworkConfig  map[string]any `json:"network_config,omitempty"`
}

type ProvisioningEvent struct {
	OrganizationID string         `json:"organization_id"`
	ServerID       string         `json:"server_id"`
	JobID          string         `json:"job_id"`
	Status         string         `json:"status"`
	Message        string         `json:"message"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type PowerEvent struct {
	OrganizationID string `json:"organization_id"`
	ServerID       string `json:"server_id"`
	Action         string `json:"action"`
	PowerState     string `json:"power_state"`
	Message        string `json:"message"`
}

type RackHeartbeat struct {
	RackID                    string         `json:"rack_id"`
	AgentID                   string         `json:"agent_id"`
	AgentVersion              string         `json:"agent_version"`
	Status                    string         `json:"status"`
	TinkerbellStatus          string         `json:"tinkerbell_status"`
	BMCNetworkStatus          string         `json:"bmc_network_status"`
	ProvisioningNetworkStatus string         `json:"provisioning_network_status"`
	ActiveJobsCount           int            `json:"active_jobs_count"`
	Timestamp                 time.Time      `json:"timestamp"`
	Metadata                  map[string]any `json:"metadata,omitempty"`
}

type HealthReply struct {
	RackID    string    `json:"rack_id"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type PowerStateReply struct {
	ServerID   string `json:"server_id"`
	PowerState string `json:"power_state"`
}

type BMCCheckReply struct {
	ServerID  string `json:"server_id"`
	Reachable bool   `json:"reachable"`
}

type ProvisioningStatusReply struct {
	JobID    string `json:"job_id"`
	Status   string `json:"status"`
	LastStep string `json:"last_step"`
}
