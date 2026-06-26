package central

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"relay/client-backend/internal/provisioning/messages"

	"github.com/google/uuid"
)

var (
	ErrRackOffline       = errors.New("rack is offline")
	ErrServerUnavailable = errors.New("server is unavailable")
	ErrActiveJobExists   = errors.New("active provisioning job already exists")
	ErrRequestTimedOut   = errors.New("rack request timed out")
	ErrInvalidRackEvent  = errors.New("invalid rack event")
)

type ProvisionRequest struct {
	OrganizationID    string
	ProjectID         *string
	ServerID          string
	RequestedByUserID string
	ImageID           string
	Hostname          string
	SSHKeys           []string
	NetworkConfig     messages.NetworkConfig
	HardwareMetadata  map[string]any
}

type PowerRequest struct {
	OrganizationID    string
	ServerID          string
	RequestedByUserID string
	Action            messages.PowerAction
}

type Orchestrator struct {
	repo              Repository
	publisher         CommandPublisher
	requester         RequestClient
	subjects          messages.SubjectBuilder
	metrics           Metrics
	commandExpiration time.Duration
	requestTimeout    time.Duration
	heartbeatOffline  time.Duration
}

func NewOrchestrator(repo Repository, publisher CommandPublisher, requester RequestClient, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		repo:              repo,
		publisher:         publisher,
		requester:         requester,
		subjects:          messages.SubjectBuilder{},
		metrics:           NoopMetrics{},
		commandExpiration: 30 * time.Minute,
		requestTimeout:    3 * time.Second,
		heartbeatOffline:  90 * time.Second,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

type Option func(*Orchestrator)

func WithMetrics(metrics Metrics) Option {
	return func(o *Orchestrator) { o.metrics = metrics }
}

func WithRequestTimeout(timeout time.Duration) Option {
	return func(o *Orchestrator) { o.requestTimeout = timeout }
}

func WithCommandExpiration(duration time.Duration) Option {
	return func(o *Orchestrator) { o.commandExpiration = duration }
}

func (o *Orchestrator) ProvisionServer(ctx context.Context, req ProvisionRequest) (ProvisioningJob, error) {
	server, err := o.repo.GetServerForOrganization(ctx, req.OrganizationID, req.ServerID)
	if err != nil {
		return ProvisioningJob{}, err
	}
	if server.OrganizationID != nil && *server.OrganizationID != req.OrganizationID {
		return ProvisioningJob{}, ErrServerUnavailable
	}
	if !server.Provisionable {
		return ProvisioningJob{}, ErrServerUnavailable
	}
	rack, err := o.repo.GetRack(ctx, server.RackID)
	if err != nil {
		return ProvisioningJob{}, err
	}
	if rack.Status != RackOnline {
		return ProvisioningJob{}, ErrRackOffline
	}
	active, err := o.repo.GetActiveProvisioningJobForServer(ctx, server.ID)
	if err != nil {
		return ProvisioningJob{}, err
	}
	if active != nil {
		return ProvisioningJob{}, ErrActiveJobExists
	}
	if _, err := o.repo.ReserveServerForProvisioning(ctx, req.OrganizationID, server.ID); err != nil {
		return ProvisioningJob{}, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(o.commandExpiration)
	correlationID := uuid.NewString()
	job, err := o.repo.CreateProvisioningJob(ctx, CreateProvisioningJobInput{
		OrganizationID:    req.OrganizationID,
		ProjectID:         req.ProjectID,
		ServerID:          server.ID,
		RackID:            server.RackID,
		RequestedByUserID: req.RequestedByUserID,
		ImageID:           req.ImageID,
		Hostname:          req.Hostname,
		CorrelationID:     correlationID,
		ExpiresAt:         &expiresAt,
	})
	if err != nil {
		return ProvisioningJob{}, err
	}

	payload := messages.ProvisionServerCommand{
		OrganizationID:   req.OrganizationID,
		ProjectID:        req.ProjectID,
		ServerID:         server.ID,
		ImageID:          req.ImageID,
		Hostname:         req.Hostname,
		SSHKeys:          req.SSHKeys,
		NetworkConfig:    req.NetworkConfig,
		HardwareMetadata: req.HardwareMetadata,
	}
	routeLocation := rack.Location
	if routeLocation == "" {
		routeLocation = server.RackID
	}
	env, err := messages.NewEnvelope(messages.ProvisionServerCommandType, routeLocation, payload)
	if err != nil {
		return ProvisioningJob{}, err
	}
	env.CorrelationID = &correlationID
	env.ServerID = &server.ID
	env.JobID = &job.ID
	env.ExpiresAt = &expiresAt
	env.Metadata = map[string]string{"organization_id": req.OrganizationID, "actor_user_id": req.RequestedByUserID}

	raw, _ := json.Marshal(env)
	_, err = o.repo.RecordRackMessage(ctx, RackMessage{
		ID:          uuid.NewString(),
		MessageID:   env.MessageID,
		Direction:   DirectionCentralToRack,
		RackID:      server.RackID,
		ServerID:    &server.ID,
		JobID:       &job.ID,
		MessageType: env.MessageType,
		Status:      MessageReceived,
		Payload:     raw,
		CreatedAt:   now,
	})
	if err != nil {
		return ProvisioningJob{}, err
	}

	subject := o.subjects.DataCenterProvisionRequest(routeLocation)
	if err := o.publisher.PublishCommand(ctx, subject, env); err != nil {
		_ = o.repo.MarkRackMessageProcessed(ctx, env.MessageID, MessageFailed)
		return ProvisioningJob{}, err
	}
	job, err = o.repo.UpdateProvisioningJobCommand(ctx, job.ID, env.MessageID, JobCommandPublished)
	if err != nil {
		return ProvisioningJob{}, err
	}
	o.metrics.Inc("commands_processed_total", map[string]string{"type": env.MessageType, "rack_id": server.RackID})
	return job, nil
}

func (o *Orchestrator) PublishPowerCommand(ctx context.Context, req PowerRequest) (string, error) {
	server, err := o.repo.GetServerForOrganization(ctx, req.OrganizationID, req.ServerID)
	if err != nil {
		return "", err
	}
	rack, err := o.repo.GetRack(ctx, server.RackID)
	if err != nil {
		return "", err
	}
	routeLocation := rack.Location
	if routeLocation == "" {
		routeLocation = server.RackID
	}
	payload := messages.PowerCommand{OrganizationID: req.OrganizationID, ServerID: req.ServerID, Action: req.Action}
	env, err := messages.NewEnvelope(messages.PowerCommandType, routeLocation, payload)
	if err != nil {
		return "", err
	}
	env.ServerID = &server.ID
	env.CorrelationID = ptr(uuid.NewString())
	env.Metadata = map[string]string{"organization_id": req.OrganizationID, "actor_user_id": req.RequestedByUserID}
	raw, _ := json.Marshal(env)
	_, err = o.repo.RecordRackMessage(ctx, RackMessage{
		ID:          uuid.NewString(),
		MessageID:   env.MessageID,
		Direction:   DirectionCentralToRack,
		RackID:      server.RackID,
		ServerID:    &server.ID,
		MessageType: env.MessageType,
		Status:      MessageReceived,
		Payload:     raw,
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	if err := o.publisher.PublishCommand(ctx, o.subjects.DataCenterServerPower(routeLocation), env); err != nil {
		return "", err
	}
	return env.MessageID, nil
}

func (o *Orchestrator) HandleRackEvent(ctx context.Context, env messages.Envelope) error {
	if err := env.Validate(time.Now().UTC()); err != nil {
		return err
	}
	raw, _ := json.Marshal(env)
	created, err := o.repo.RecordRackMessage(ctx, RackMessage{
		ID:          uuid.NewString(),
		MessageID:   env.MessageID,
		Direction:   DirectionRackToCentral,
		RackID:      env.RackID,
		ServerID:    env.ServerID,
		JobID:       env.JobID,
		MessageType: env.MessageType,
		Status:      MessageReceived,
		Payload:     raw,
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	if !created {
		_ = o.repo.MarkRackMessageProcessed(ctx, env.MessageID, MessageIgnoredDuplicate)
		return nil
	}
	switch env.MessageType {
	case messages.ProvisioningStartedEventType, messages.ProvisioningPxeBootSetEventType, messages.ProvisioningInstallingEventType, messages.ProvisioningCompletedEventType, messages.ProvisioningFailedEventType:
		var event messages.ProvisioningEvent
		if err := env.DecodePayload(&event); err != nil {
			return err
		}
		_, _, err := o.repo.ApplyProvisioningEvent(ctx, env, event)
		if err != nil {
			return err
		}
	case messages.RackHeartbeatEventType:
		var heartbeat messages.RackHeartbeat
		if err := env.DecodePayload(&heartbeat); err != nil {
			return err
		}
		_, _, err := o.repo.UpdateRackHeartbeat(ctx, heartbeat)
		if err != nil {
			return err
		}
	default:
		return ErrInvalidRackEvent
	}
	return o.repo.MarkRackMessageProcessed(ctx, env.MessageID, MessageProcessed)
}

func (o *Orchestrator) HandleHeartbeat(ctx context.Context, heartbeat messages.RackHeartbeat) error {
	_, _, err := o.repo.UpdateRackHeartbeat(ctx, heartbeat)
	return err
}

func (o *Orchestrator) MarkMissingHeartbeatsOffline(ctx context.Context, now time.Time) error {
	return o.repo.MarkRacksOfflineBefore(ctx, now.Add(-o.heartbeatOffline))
}

func (o *Orchestrator) GetRackHealth(ctx context.Context, rackID string) (messages.HealthReply, error) {
	env, _ := messages.NewEnvelope("RackHealthRequest", rackID, map[string]string{"rack_id": rackID})
	reply, err := o.requester.Request(ctx, o.subjects.Request(rackID, messages.RequestHealth), env, o.requestTimeout)
	if err != nil {
		return messages.HealthReply{}, ErrRequestTimedOut
	}
	var health messages.HealthReply
	return health, reply.DecodePayload(&health)
}

func ptr[T any](value T) *T {
	return &value
}
