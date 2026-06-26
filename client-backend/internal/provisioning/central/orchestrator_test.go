package central

import (
	"context"
	"errors"
	"testing"
	"time"

	"relay/client-backend/internal/provisioning/messages"
)

func TestProvisioningRequestCreatesJobLocksServerAndPublishes(t *testing.T) {
	repo := newMemoryRepo()
	bus := &memoryBus{}
	orch := NewOrchestrator(repo, bus, bus)
	repo.racks["rack-1"] = Rack{ID: "rack-1", Location: "ny", Status: RackOnline}
	repo.servers["srv-1"] = Server{ID: "srv-1", RackID: "rack-1", Status: ServerAvailable, Provisionable: true}

	job, err := orch.ProvisionServer(context.Background(), ProvisionRequest{OrganizationID: "org-1", ServerID: "srv-1", RequestedByUserID: "usr-1", ImageID: "ubuntu", Hostname: "web-1"})
	if err != nil {
		t.Fatalf("ProvisionServer returned error: %v", err)
	}
	if job.Status != JobCommandPublished {
		t.Fatalf("job status = %s, want command_published", job.Status)
	}
	if repo.servers["srv-1"].Status != ServerProvisioningRequested {
		t.Fatalf("server status = %s, want provisioning_requested", repo.servers["srv-1"].Status)
	}
	if bus.lastSubject != "dc.ny.provision.request" {
		t.Fatalf("subject = %s", bus.lastSubject)
	}
}

func TestCannotCreateTwoActiveProvisioningJobs(t *testing.T) {
	repo := newMemoryRepo()
	bus := &memoryBus{}
	orch := NewOrchestrator(repo, bus, bus)
	repo.racks["rack-1"] = Rack{ID: "rack-1", Location: "ny", Status: RackOnline}
	repo.servers["srv-1"] = Server{ID: "srv-1", RackID: "rack-1", Status: ServerAvailable, Provisionable: true}
	req := ProvisionRequest{OrganizationID: "org-1", ServerID: "srv-1", RequestedByUserID: "usr-1", ImageID: "ubuntu", Hostname: "web-1"}
	if _, err := orch.ProvisionServer(context.Background(), req); err != nil {
		t.Fatalf("first provision returned error: %v", err)
	}
	if _, err := orch.ProvisionServer(context.Background(), req); !errors.Is(err, ErrActiveJobExists) {
		t.Fatalf("second provision error = %v, want ErrActiveJobExists", err)
	}
}

func TestCannotProvisionAnotherOrganizationsServer(t *testing.T) {
	repo := newMemoryRepo()
	bus := &memoryBus{}
	orch := NewOrchestrator(repo, bus, bus)
	other := "org-2"
	repo.racks["rack-1"] = Rack{ID: "rack-1", Location: "ny", Status: RackOnline}
	repo.servers["srv-1"] = Server{ID: "srv-1", RackID: "rack-1", OrganizationID: &other, Status: ServerReserved, Provisionable: true}
	_, err := orch.ProvisionServer(context.Background(), ProvisionRequest{OrganizationID: "org-1", ServerID: "srv-1", RequestedByUserID: "usr-1", ImageID: "ubuntu", Hostname: "web-1"})
	if !errors.Is(err, ErrServerUnavailable) {
		t.Fatalf("error = %v, want ErrServerUnavailable", err)
	}
}

func TestCannotProvisionIfRackOffline(t *testing.T) {
	repo := newMemoryRepo()
	bus := &memoryBus{}
	orch := NewOrchestrator(repo, bus, bus)
	repo.racks["rack-1"] = Rack{ID: "rack-1", Location: "ny", Status: RackOffline}
	repo.servers["srv-1"] = Server{ID: "srv-1", RackID: "rack-1", Status: ServerAvailable, Provisionable: true}
	_, err := orch.ProvisionServer(context.Background(), ProvisionRequest{OrganizationID: "org-1", ServerID: "srv-1", RequestedByUserID: "usr-1", ImageID: "ubuntu", Hostname: "web-1"})
	if !errors.Is(err, ErrRackOffline) {
		t.Fatalf("error = %v, want ErrRackOffline", err)
	}
}

func TestRackEventUpdatesJobAndDuplicateIgnored(t *testing.T) {
	repo := newMemoryRepo()
	bus := &memoryBus{}
	orch := NewOrchestrator(repo, bus, bus)
	repo.racks["rack-1"] = Rack{ID: "rack-1", Location: "ny", Status: RackOnline}
	repo.servers["srv-1"] = Server{ID: "srv-1", RackID: "rack-1", Status: ServerAvailable, Provisionable: true}
	job, _ := orch.ProvisionServer(context.Background(), ProvisionRequest{OrganizationID: "org-1", ServerID: "srv-1", RequestedByUserID: "usr-1", ImageID: "ubuntu", Hostname: "web-1"})
	env, _ := messages.NewEnvelope(messages.ProvisioningCompletedEventType, "rack-1", messages.ProvisioningEvent{OrganizationID: "org-1", ServerID: "srv-1", JobID: job.ID, Status: "completed", Message: "done"})
	env.JobID = &job.ID
	env.ServerID = ptr("srv-1")
	if err := orch.HandleRackEvent(context.Background(), env); err != nil {
		t.Fatalf("HandleRackEvent returned error: %v", err)
	}
	if repo.jobs[job.ID].Status != JobCompleted {
		t.Fatalf("job status = %s, want completed", repo.jobs[job.ID].Status)
	}
	if err := orch.HandleRackEvent(context.Background(), env); err != nil {
		t.Fatalf("duplicate HandleRackEvent returned error: %v", err)
	}
	if len(repo.jobEvents[job.ID]) != 1 {
		t.Fatalf("events = %d, want one after duplicate", len(repo.jobEvents[job.ID]))
	}
}

func TestHeartbeatOnlineAndOfflineThreshold(t *testing.T) {
	repo := newMemoryRepo()
	orch := NewOrchestrator(repo, &memoryBus{}, &memoryBus{})
	if err := orch.HandleHeartbeat(context.Background(), messages.RackHeartbeat{RackID: "rack-1", AgentID: "agent-1", AgentVersion: "0.1", Status: "online", Timestamp: time.Now()}); err != nil {
		t.Fatalf("HandleHeartbeat returned error: %v", err)
	}
	if repo.racks["rack-1"].Status != RackOnline {
		t.Fatalf("rack status = %s, want online", repo.racks["rack-1"].Status)
	}
	if err := orch.MarkMissingHeartbeatsOffline(context.Background(), time.Now().Add(2*time.Minute)); err != nil {
		t.Fatalf("MarkMissingHeartbeatsOffline returned error: %v", err)
	}
	if repo.racks["rack-1"].Status != RackOffline {
		t.Fatalf("rack status = %s, want offline", repo.racks["rack-1"].Status)
	}
}

func TestRequestReplyTimeoutReturnsCleanError(t *testing.T) {
	repo := newMemoryRepo()
	bus := &memoryBus{requestErr: context.DeadlineExceeded}
	orch := NewOrchestrator(repo, bus, bus)
	_, err := orch.GetRackHealth(context.Background(), "rack-1")
	if !errors.Is(err, ErrRequestTimedOut) {
		t.Fatalf("error = %v, want ErrRequestTimedOut", err)
	}
}

func TestPowerCommandPublishes(t *testing.T) {
	repo := newMemoryRepo()
	bus := &memoryBus{}
	orch := NewOrchestrator(repo, bus, bus)
	repo.racks["rack-1"] = Rack{ID: "rack-1", Location: "ny", Status: RackOnline}
	repo.servers["srv-1"] = Server{ID: "srv-1", RackID: "rack-1", OrganizationID: ptr("org-1"), Status: ServerActive, Provisionable: true}
	if _, err := orch.PublishPowerCommand(context.Background(), PowerRequest{OrganizationID: "org-1", ServerID: "srv-1", RequestedByUserID: "usr-1", Action: messages.PowerCycle}); err != nil {
		t.Fatalf("PublishPowerCommand returned error: %v", err)
	}
	if bus.lastSubject != "dc.ny.server.power" {
		t.Fatalf("subject = %s", bus.lastSubject)
	}
}

type memoryBus struct {
	lastSubject  string
	lastEnvelope messages.Envelope
	requestErr   error
}

func (b *memoryBus) PublishCommand(ctx context.Context, subject string, envelope messages.Envelope) error {
	b.lastSubject = subject
	b.lastEnvelope = envelope
	return nil
}

func (b *memoryBus) Request(ctx context.Context, subject string, envelope messages.Envelope, timeout time.Duration) (messages.Envelope, error) {
	if b.requestErr != nil {
		return messages.Envelope{}, b.requestErr
	}
	return messages.NewEnvelope("RackHealthReply", envelope.RackID, messages.HealthReply{RackID: envelope.RackID, Status: "online", Timestamp: time.Now()})
}

type memoryRepo struct {
	racks     map[string]Rack
	agents    map[string]RackAgent
	servers   map[string]Server
	jobs      map[string]ProvisioningJob
	jobEvents map[string][]ProvisioningJobEvent
	messages  map[string]RackMessage
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		racks:     map[string]Rack{},
		agents:    map[string]RackAgent{},
		servers:   map[string]Server{},
		jobs:      map[string]ProvisioningJob{},
		jobEvents: map[string][]ProvisioningJobEvent{},
		messages:  map[string]RackMessage{},
	}
}

func (r *memoryRepo) GetRack(ctx context.Context, rackID string) (Rack, error) {
	rack, ok := r.racks[rackID]
	if !ok {
		return Rack{}, errors.New("rack not found")
	}
	return rack, nil
}

func (r *memoryRepo) ListRacks(ctx context.Context) ([]Rack, error) { return nil, nil }

func (r *memoryRepo) UpdateRackHeartbeat(ctx context.Context, heartbeat messages.RackHeartbeat) (Rack, RackAgent, error) {
	now := heartbeat.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	status := RackOnline
	if heartbeat.Status == "degraded" {
		status = RackDegraded
	}
	rack := r.racks[heartbeat.RackID]
	rack.ID = heartbeat.RackID
	rack.Status = status
	rack.LastHeartbeatAt = &now
	rack.LastSeenAt = &now
	r.racks[heartbeat.RackID] = rack
	agent := RackAgent{ID: heartbeat.AgentID, RackID: heartbeat.RackID, AgentID: heartbeat.AgentID, Version: heartbeat.AgentVersion, Status: AgentOnline, LastHeartbeatAt: &now}
	r.agents[heartbeat.AgentID] = agent
	return rack, agent, nil
}

func (r *memoryRepo) MarkRacksOfflineBefore(ctx context.Context, cutoff time.Time) error {
	for id, rack := range r.racks {
		if rack.LastHeartbeatAt != nil && rack.LastHeartbeatAt.Before(cutoff) {
			rack.Status = RackOffline
			r.racks[id] = rack
		}
	}
	return nil
}

func (r *memoryRepo) SetRackStatus(ctx context.Context, rackID string, status RackStatus) (Rack, error) {
	rack := r.racks[rackID]
	rack.Status = status
	r.racks[rackID] = rack
	return rack, nil
}

func (r *memoryRepo) GetServerForOrganization(ctx context.Context, organizationID, serverID string) (Server, error) {
	server, ok := r.servers[serverID]
	if !ok {
		return Server{}, errors.New("server not found")
	}
	return server, nil
}

func (r *memoryRepo) ReserveServerForProvisioning(ctx context.Context, organizationID, serverID string) (Server, error) {
	server := r.servers[serverID]
	server.OrganizationID = &organizationID
	server.Status = ServerProvisioningRequested
	r.servers[serverID] = server
	return server, nil
}

func (r *memoryRepo) UpdateServerStatus(ctx context.Context, serverID string, status ServerStatus) error {
	server := r.servers[serverID]
	server.Status = status
	r.servers[serverID] = server
	return nil
}

func (r *memoryRepo) CreateProvisioningJob(ctx context.Context, input CreateProvisioningJobInput) (ProvisioningJob, error) {
	job := ProvisioningJob{ID: "job-" + input.ServerID, OrganizationID: input.OrganizationID, ProjectID: input.ProjectID, ServerID: input.ServerID, RackID: input.RackID, RequestedByUserID: input.RequestedByUserID, ImageID: input.ImageID, Hostname: input.Hostname, Status: JobPending, CorrelationID: input.CorrelationID, RequestedAt: time.Now(), ExpiresAt: input.ExpiresAt}
	r.jobs[job.ID] = job
	return job, nil
}

func (r *memoryRepo) GetProvisioningJob(ctx context.Context, organizationID, jobID string) (ProvisioningJob, error) {
	return r.jobs[jobID], nil
}

func (r *memoryRepo) GetActiveProvisioningJobForServer(ctx context.Context, serverID string) (*ProvisioningJob, error) {
	for _, job := range r.jobs {
		if job.ServerID == serverID && job.Status != JobCompleted && job.Status != JobFailed && job.Status != JobCanceled && job.Status != JobExpired {
			return &job, nil
		}
	}
	return nil, nil
}

func (r *memoryRepo) UpdateProvisioningJobCommand(ctx context.Context, jobID, messageID string, status JobStatus) (ProvisioningJob, error) {
	job := r.jobs[jobID]
	job.CommandMessageID = &messageID
	job.Status = status
	r.jobs[jobID] = job
	return job, nil
}

func (r *memoryRepo) ApplyProvisioningEvent(ctx context.Context, envelope messages.Envelope, event messages.ProvisioningEvent) (ProvisioningJob, bool, error) {
	job := r.jobs[event.JobID]
	switch envelope.MessageType {
	case messages.ProvisioningCompletedEventType:
		job.Status = JobCompleted
		_ = r.UpdateServerStatus(ctx, event.ServerID, ServerActive)
	case messages.ProvisioningFailedEventType:
		job.Status = JobFailed
		job.FailureReason = &event.Message
		_ = r.UpdateServerStatus(ctx, event.ServerID, ServerFailed)
	default:
		job.Status = JobRunning
	}
	r.jobs[job.ID] = job
	r.jobEvents[job.ID] = append(r.jobEvents[job.ID], ProvisioningJobEvent{ID: envelope.MessageID, ProvisioningJobID: job.ID, EventType: envelope.MessageType, Message: event.Message, CreatedAt: time.Now()})
	return job, true, nil
}

func (r *memoryRepo) ListProvisioningJobEvents(ctx context.Context, organizationID, jobID string) ([]ProvisioningJobEvent, error) {
	return r.jobEvents[jobID], nil
}

func (r *memoryRepo) RecordRackMessage(ctx context.Context, message RackMessage) (bool, error) {
	if _, ok := r.messages[message.MessageID]; ok {
		return false, nil
	}
	r.messages[message.MessageID] = message
	return true, nil
}

func (r *memoryRepo) MarkRackMessageProcessed(ctx context.Context, messageID string, status MessageStatus) error {
	message := r.messages[messageID]
	message.Status = status
	now := time.Now()
	message.ProcessedAt = &now
	r.messages[messageID] = message
	return nil
}
