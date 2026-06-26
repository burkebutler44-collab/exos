package messages

import "fmt"

type CommandKind string
type EventKind string
type RequestKind string

const (
	CommandProvision CommandKind = "provision"
	CommandReinstall CommandKind = "reinstall"
	CommandPower     CommandKind = "power"
	CommandRescue    CommandKind = "rescue"
	CommandNetwork   CommandKind = "network"

	EventProvision EventKind = "provision"
	EventPower     EventKind = "power"
	EventInventory EventKind = "inventory"
	EventHealth    EventKind = "health"
	EventNetwork   EventKind = "network"

	RequestHealth             RequestKind = "health"
	RequestPowerState         RequestKind = "power_state"
	RequestBMCCheck           RequestKind = "bmc_check"
	RequestProvisioningStatus RequestKind = "provisioning_status"
	RequestTinkerbellHealth   RequestKind = "tinkerbell_health"
)

type SubjectBuilder struct{}

func (SubjectBuilder) DataCenterProvisionRequest(location string) string {
	return fmt.Sprintf("dc.%s.provision.request", location)
}

func (SubjectBuilder) DataCenterProvisionStatus(location string) string {
	return fmt.Sprintf("dc.%s.provision.status", location)
}

func (SubjectBuilder) DataCenterServerPower(location string) string {
	return fmt.Sprintf("dc.%s.server.power", location)
}

func (SubjectBuilder) DataCenterHardwareSync(location string) string {
	return fmt.Sprintf("dc.%s.hardware.sync", location)
}

func (SubjectBuilder) DataCenterWorkflowCancel(location string) string {
	return fmt.Sprintf("dc.%s.workflow.cancel", location)
}

func (SubjectBuilder) DataCenterRequest(location string, kind RequestKind) string {
	return fmt.Sprintf("dc.%s.request.%s", location, kind)
}

func (SubjectBuilder) DataCenterHeartbeat(location string) string {
	return fmt.Sprintf("dc.%s.heartbeat", location)
}

func (SubjectBuilder) Command(rackID string, kind CommandKind) string {
	return fmt.Sprintf("racks.%s.commands.%s", rackID, kind)
}

func (SubjectBuilder) Event(rackID string, kind EventKind) string {
	return fmt.Sprintf("racks.%s.events.%s", rackID, kind)
}

func (SubjectBuilder) Request(rackID string, kind RequestKind) string {
	return fmt.Sprintf("racks.%s.requests.%s", rackID, kind)
}

func (SubjectBuilder) Heartbeat(rackID string) string {
	return fmt.Sprintf("racks.%s.heartbeats", rackID)
}
