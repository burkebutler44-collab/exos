package messages

import (
	"testing"
	"time"
)

func TestSubjectBuilder(t *testing.T) {
	builder := SubjectBuilder{}
	if got := builder.DataCenterProvisionRequest("ny"); got != "dc.ny.provision.request" {
		t.Fatalf("datacenter provision request subject = %s", got)
	}
	if got := builder.DataCenterProvisionStatus("ny"); got != "dc.ny.provision.status" {
		t.Fatalf("datacenter provision status subject = %s", got)
	}
	if got := builder.DataCenterServerPower("ny"); got != "dc.ny.server.power" {
		t.Fatalf("datacenter server power subject = %s", got)
	}
	if got := builder.DataCenterHardwareSync("ny"); got != "dc.ny.hardware.sync" {
		t.Fatalf("datacenter hardware sync subject = %s", got)
	}
	if got := builder.DataCenterWorkflowCancel("ny"); got != "dc.ny.workflow.cancel" {
		t.Fatalf("datacenter workflow cancel subject = %s", got)
	}
	if got := builder.Command("rack-a", CommandProvision); got != "racks.rack-a.commands.provision" {
		t.Fatalf("command subject = %s", got)
	}
	if got := builder.Event("rack-a", EventPower); got != "racks.rack-a.events.power" {
		t.Fatalf("event subject = %s", got)
	}
	if got := builder.Request("rack-a", RequestBMCCheck); got != "racks.rack-a.requests.bmc_check" {
		t.Fatalf("request subject = %s", got)
	}
	if got := builder.Heartbeat("rack-a"); got != "racks.rack-a.heartbeats" {
		t.Fatalf("heartbeat subject = %s", got)
	}
}

func TestEnvelopeValidationAndExpiration(t *testing.T) {
	env, err := NewEnvelope(ProvisionServerCommandType, "rack-a", ProvisionServerCommand{ServerID: "srv-1"})
	if err != nil {
		t.Fatalf("NewEnvelope returned error: %v", err)
	}
	expired := time.Now().Add(-time.Minute)
	env.ExpiresAt = &expired
	if err := env.Validate(time.Now()); err != ErrExpiredMessage {
		t.Fatalf("Validate error = %v, want ErrExpiredMessage", err)
	}
}

func TestCorrelationIsPreserved(t *testing.T) {
	env, _ := NewEnvelope(ProvisioningStartedEventType, "rack-a", ProvisioningEvent{})
	env = env.WithCorrelation("corr-1").WithCausation("cmd-1")
	if env.CorrelationID == nil || *env.CorrelationID != "corr-1" {
		t.Fatal("correlation_id was not set")
	}
	if env.CausationID == nil || *env.CausationID != "cmd-1" {
		t.Fatal("causation_id was not set")
	}
}
