package rackagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"relay/client-backend/internal/provisioning/messages"
	"relay/client-backend/internal/provisioning/rackagent/adapters"
	"relay/client-backend/internal/provisioning/rackagent/storage"

	"github.com/google/uuid"
)

var (
	ErrWrongRack        = errors.New("command is for another rack")
	ErrDuplicateCommand = errors.New("duplicate command ignored")
	ErrExpiredCommand   = errors.New("command is expired")
)

type EventPublisher interface {
	PublishEvent(ctx context.Context, kind messages.EventKind, envelope messages.Envelope) error
	PublishHeartbeat(ctx context.Context, envelope messages.Envelope) error
}

type Agent struct {
	RackID         string
	Location       string
	AgentID        string
	Version        string
	CommandStream  string
	CommandDurable string
	CommandAckWait time.Duration
	Storage        storage.LocalJobRepository
	Processed      storage.ProcessedMessageRepository
	Tinkerbell     adapters.TinkerbellAdapter
	IPMI           adapters.IPMIAdapter
	Network        adapters.NetworkAdapter
	Secrets        adapters.RackSecretProvider
	Publisher      EventPublisher
	HeartbeatEvery time.Duration
}

func (a *Agent) HandleCommand(ctx context.Context, env messages.Envelope) error {
	if env.RackID != a.RackID && env.RackID != a.Location {
		return ErrWrongRack
	}
	if err := env.Validate(time.Now().UTC()); err != nil {
		if errors.Is(err, messages.ErrExpiredMessage) {
			return ErrExpiredCommand
		}
		return err
	}
	seen, err := a.Processed.AlreadyProcessed(ctx, env.MessageID)
	if err != nil {
		return err
	}
	if seen {
		_ = a.publishCurrentState(ctx, env)
		return ErrDuplicateCommand
	}
	switch env.MessageType {
	case messages.ProvisionServerCommandType:
		return a.handleProvision(ctx, env)
	case messages.PowerCommandType:
		return a.handlePower(ctx, env)
	default:
		return nil
	}
}

func (a *Agent) handleProvision(ctx context.Context, env messages.Envelope) error {
	var cmd messages.ProvisionServerCommand
	if err := env.DecodePayload(&cmd); err != nil {
		return err
	}
	jobID := valueOr(env.JobID, uuid.NewString())
	now := time.Now().UTC()
	if _, _, err := a.Storage.CreateOrGetJob(ctx, storage.LocalJob{
		ID:               uuid.NewString(),
		CentralJobID:     jobID,
		CommandMessageID: env.MessageID,
		RackID:           env.RackID,
		ServerID:         cmd.ServerID,
		Status:           "running",
		LastStep:         "accepted",
		StartedAt:        &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		return err
	}
	if err := a.publishProvisionEvent(ctx, env, messages.ProvisioningStartedEventType, cmd, "running", "Provisioning accepted"); err != nil {
		return err
	}
	if err := a.Tinkerbell.EnsureHardware(ctx, cmd); err != nil {
		return a.failProvision(ctx, env, cmd, err)
	}
	workflowID, err := a.Tinkerbell.CreateWorkflow(ctx, cmd)
	if err != nil {
		return a.failProvision(ctx, env, cmd, err)
	}
	if err := a.publishProvisionEvent(ctx, env, messages.ProvisioningTinkerbellWorkflowCreatedEventType, cmd, "running", workflowID); err != nil {
		return err
	}
	if err := a.Tinkerbell.StartWorkflow(ctx, workflowID); err != nil {
		return a.failProvision(ctx, env, cmd, err)
	}
	if err := a.IPMI.SetOneTimePXEBoot(ctx, cmd.ServerID); err != nil {
		return a.failProvision(ctx, env, cmd, err)
	}
	if err := a.publishProvisionEvent(ctx, env, messages.ProvisioningPxeBootSetEventType, cmd, "running", "PXE boot set"); err != nil {
		return err
	}
	if err := a.IPMI.PowerOn(ctx, cmd.ServerID); err != nil {
		return a.failProvision(ctx, env, cmd, err)
	}
	if err := a.publishProvisionEvent(ctx, env, messages.ProvisioningPowerCycledEventType, cmd, "running", "Server powered on"); err != nil {
		return err
	}
	if err := a.publishProvisionEvent(ctx, env, messages.ProvisioningInstallingEventType, cmd, "running", "Installing image"); err != nil {
		return err
	}
	_ = a.Storage.UpdateJobStep(ctx, jobID, "running", "workflow_started", nil)
	return a.markProcessed(ctx, env, "processed")
}

func (a *Agent) handlePower(ctx context.Context, env messages.Envelope) error {
	var cmd messages.PowerCommand
	if err := env.DecodePayload(&cmd); err != nil {
		return err
	}
	var err error
	switch cmd.Action {
	case messages.PowerOn:
		err = a.IPMI.PowerOn(ctx, cmd.ServerID)
	case messages.PowerOff:
		err = a.IPMI.PowerOff(ctx, cmd.ServerID)
	case messages.PowerCycle:
		err = a.IPMI.PowerCycle(ctx, cmd.ServerID)
	case messages.PowerReset:
		err = a.IPMI.Reset(ctx, cmd.ServerID)
	case messages.PowerStatus:
		_, err = a.IPMI.PowerState(ctx, cmd.ServerID)
	}
	eventType := messages.PowerCommandCompletedEventType
	message := "Power command completed"
	if err != nil {
		eventType = messages.PowerCommandFailedEventType
		message = err.Error()
	}
	state, _ := a.IPMI.PowerState(ctx, cmd.ServerID)
	payload := messages.PowerEvent{OrganizationID: cmd.OrganizationID, ServerID: cmd.ServerID, Action: string(cmd.Action), PowerState: state, Message: message}
	out, _ := messages.NewEnvelope(eventType, env.RackID, payload)
	out.ServerID = &cmd.ServerID
	out.JobID = env.JobID
	out.CorrelationID = env.CorrelationID
	out.CausationID = &env.MessageID
	if pubErr := a.Publisher.PublishEvent(ctx, messages.EventPower, out); pubErr != nil {
		return pubErr
	}
	if err != nil {
		_ = a.markProcessed(ctx, env, "failed")
		return err
	}
	return a.markProcessed(ctx, env, "processed")
}

func (a *Agent) RespondToRequest(ctx context.Context, kind messages.RequestKind, env messages.Envelope) (messages.Envelope, error) {
	switch kind {
	case messages.RequestHealth:
		return messages.NewEnvelope("RackHealthReply", a.RackID, messages.HealthReply{RackID: a.RackID, Status: "online", Timestamp: time.Now().UTC()})
	case messages.RequestPowerState:
		serverID := valueOr(env.ServerID, "")
		state, err := a.IPMI.PowerState(ctx, serverID)
		if err != nil {
			return messages.Envelope{}, err
		}
		return messages.NewEnvelope("PowerStateReply", a.RackID, messages.PowerStateReply{ServerID: serverID, PowerState: state})
	case messages.RequestBMCCheck:
		serverID := valueOr(env.ServerID, "")
		ok, err := a.IPMI.CheckBMC(ctx, serverID)
		if err != nil {
			return messages.Envelope{}, err
		}
		return messages.NewEnvelope("BMCCheckReply", a.RackID, messages.BMCCheckReply{ServerID: serverID, Reachable: ok})
	case messages.RequestTinkerbellHealth:
		status, err := a.Tinkerbell.Health(ctx)
		if err != nil {
			return messages.Envelope{}, err
		}
		return messages.NewEnvelope("TinkerbellHealthReply", a.RackID, messages.HealthReply{RackID: a.RackID, Status: status, Timestamp: time.Now().UTC()})
	case messages.RequestProvisioningStatus:
		jobID := valueOr(env.JobID, "")
		job, err := a.Storage.GetJobByCentralID(ctx, jobID)
		if err != nil {
			return messages.Envelope{}, err
		}
		return messages.NewEnvelope("ProvisioningStatusReply", a.RackID, messages.ProvisioningStatusReply{JobID: jobID, Status: job.Status, LastStep: job.LastStep})
	default:
		return messages.Envelope{}, errors.New("unknown request kind")
	}
}

func (a *Agent) PublishHeartbeat(ctx context.Context) error {
	activeJobs, _ := a.Storage.ActiveJobsCount(ctx)
	tbStatus, _ := a.Tinkerbell.Health(ctx)
	payload := messages.RackHeartbeat{RackID: a.RackID, AgentID: a.AgentID, AgentVersion: a.Version, Status: "online", TinkerbellStatus: tbStatus, BMCNetworkStatus: "online", ProvisioningNetworkStatus: "online", ActiveJobsCount: activeJobs, Timestamp: time.Now().UTC()}
	env, _ := messages.NewEnvelope(messages.RackHeartbeatEventType, a.RackID, payload)
	return a.Publisher.PublishHeartbeat(ctx, env)
}

func (a *Agent) failProvision(ctx context.Context, env messages.Envelope, cmd messages.ProvisionServerCommand, cause error) error {
	reason := cause.Error()
	if env.JobID != nil {
		_ = a.Storage.UpdateJobStep(ctx, *env.JobID, "failed", "failed", &reason)
	}
	_ = a.publishProvisionEvent(ctx, env, messages.ProvisioningFailedEventType, cmd, "failed", reason)
	_ = a.markProcessed(ctx, env, "failed")
	return cause
}

func (a *Agent) publishProvisionEvent(ctx context.Context, env messages.Envelope, eventType string, cmd messages.ProvisionServerCommand, status, message string) error {
	jobID := valueOr(env.JobID, "")
	payload := messages.ProvisioningEvent{OrganizationID: cmd.OrganizationID, ServerID: cmd.ServerID, JobID: jobID, Status: status, Message: message}
	out, err := messages.NewEnvelope(eventType, env.RackID, payload)
	if err != nil {
		return err
	}
	out.ServerID = &cmd.ServerID
	out.JobID = env.JobID
	out.CorrelationID = env.CorrelationID
	out.CausationID = &env.MessageID
	return a.Publisher.PublishEvent(ctx, messages.EventProvision, out)
}

func (a *Agent) publishCurrentState(ctx context.Context, env messages.Envelope) error {
	if env.JobID == nil {
		return nil
	}
	job, err := a.Storage.GetJobByCentralID(ctx, *env.JobID)
	if err != nil {
		return nil
	}
	payload := messages.ProvisioningEvent{ServerID: job.ServerID, JobID: job.CentralJobID, Status: job.Status, Message: "Duplicate command ignored; current state returned"}
	out, _ := messages.NewEnvelope(duplicateEventType(env.MessageType), env.RackID, payload)
	out.JobID = env.JobID
	out.ServerID = &job.ServerID
	out.CorrelationID = env.CorrelationID
	out.CausationID = &env.MessageID
	return a.Publisher.PublishEvent(ctx, messages.EventProvision, out)
}

func (a *Agent) markProcessed(ctx context.Context, env messages.Envelope, status string) error {
	hash := sha256.Sum256(env.Payload)
	return a.Processed.MarkProcessed(ctx, storage.ProcessedMessage{ID: uuid.NewString(), MessageID: env.MessageID, MessageType: env.MessageType, ProcessedAt: time.Now().UTC(), ResultStatus: status, PayloadHash: hex.EncodeToString(hash[:]), CreatedAt: time.Now().UTC()})
}

func valueOr(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

func duplicateEventType(messageType string) string {
	if messageType == messages.PowerCommandType {
		return messages.PowerCommandCompletedEventType
	}
	return messages.ProvisioningStartedEventType
}
