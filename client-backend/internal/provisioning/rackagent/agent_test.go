package rackagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"relay/client-backend/internal/provisioning/messages"
	"relay/client-backend/internal/provisioning/rackagent/adapters"
	"relay/client-backend/internal/provisioning/rackagent/storage"
)

func TestConsumesCommandForRackAndProvisionOrder(t *testing.T) {
	agent, store, processed, ipmi, tink, publisher := testAgent()
	env := provisionCommandEnvelope(t, "rack-1", time.Now().Add(time.Minute))
	if err := agent.HandleCommand(context.Background(), env); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	if len(store.jobs) != 1 || !processed.seen[env.MessageID] {
		t.Fatal("command was not persisted and marked processed")
	}
	wantIPMI := []string{"set_pxe", "power_on"}
	for i, call := range wantIPMI {
		if ipmi.Calls[i] != call {
			t.Fatalf("ipmi call %d = %s, want %s", i, ipmi.Calls[i], call)
		}
	}
	wantTinkerbell := []string{"ensure_hardware", "create_workflow", "start_workflow"}
	for i, call := range wantTinkerbell {
		if tink.Calls[i] != call {
			t.Fatalf("tinkerbell call %d = %s, want %s", i, tink.Calls[i], call)
		}
	}
	if publisher.countType(messages.ProvisioningInstallingEventType) != 1 {
		t.Fatal("installing event was not published")
	}
}

func TestRejectsWrongRackAndExpiredCommand(t *testing.T) {
	agent, _, _, _, _, _ := testAgent()
	if err := agent.HandleCommand(context.Background(), provisionCommandEnvelope(t, "rack-2", time.Now().Add(time.Minute))); !errors.Is(err, ErrWrongRack) {
		t.Fatalf("wrong rack error = %v", err)
	}
	if err := agent.HandleCommand(context.Background(), provisionCommandEnvelope(t, "rack-1", time.Now().Add(-time.Minute))); !errors.Is(err, ErrExpiredCommand) {
		t.Fatalf("expired error = %v", err)
	}
}

func TestDuplicateCommandDoesNotRerunDestructiveActions(t *testing.T) {
	agent, _, _, ipmi, _, _ := testAgent()
	env := provisionCommandEnvelope(t, "rack-1", time.Now().Add(time.Minute))
	if err := agent.HandleCommand(context.Background(), env); err != nil {
		t.Fatalf("first HandleCommand returned error: %v", err)
	}
	calls := len(ipmi.Calls)
	if err := agent.HandleCommand(context.Background(), env); !errors.Is(err, ErrDuplicateCommand) {
		t.Fatalf("duplicate error = %v", err)
	}
	if len(ipmi.Calls) != calls {
		t.Fatal("duplicate command reran IPMI actions")
	}
}

func TestProvisionFailurePublishesFailedEvent(t *testing.T) {
	agent, _, _, _, tink, publisher := testAgent()
	tink.Fail = errors.New("workflow failed")
	env := provisionCommandEnvelope(t, "rack-1", time.Now().Add(time.Minute))
	if err := agent.HandleCommand(context.Background(), env); err == nil {
		t.Fatal("HandleCommand returned nil, want failure")
	}
	if publisher.countType(messages.ProvisioningFailedEventType) != 1 {
		t.Fatal("failed event was not published")
	}
}

func TestPowerCommandCallsIPMI(t *testing.T) {
	agent, _, _, ipmi, _, publisher := testAgent()
	env, _ := messages.NewEnvelope(messages.PowerCommandType, "rack-1", messages.PowerCommand{OrganizationID: "org-1", ServerID: "srv-1", Action: messages.PowerCycle})
	env.ServerID = ptr("srv-1")
	if err := agent.HandleCommand(context.Background(), env); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	if ipmi.Calls[0] != "power_cycle" {
		t.Fatalf("first ipmi call = %s, want power_cycle", ipmi.Calls[0])
	}
	if publisher.countType(messages.PowerCommandCompletedEventType) != 1 {
		t.Fatal("power completed event was not published")
	}
}

func TestBareProvisionPayloadIsWrappedForLocationSubject(t *testing.T) {
	agent, _, _, _, _, _ := testAgent()
	agent.Location = "ny"
	env, err := commandEnvelopeFromNATS([]byte(`{"location":"ny","server_id":"srv-123","mac":"aa:bb:cc:dd:ee:ff","ip":"10.10.40.20","gateway":"10.10.40.1","netmask":"255.255.255.0","image":"ubuntu-22.04"}`), agent, messages.ProvisionServerCommandType)
	if err != nil {
		t.Fatalf("commandEnvelopeFromNATS returned error: %v", err)
	}
	if env.RackID != "ny" || env.MessageType != messages.ProvisionServerCommandType || env.MessageID == "" {
		t.Fatalf("wrapped envelope = %+v", env)
	}
	var cmd messages.ProvisionServerCommand
	if err := env.DecodePayload(&cmd); err != nil {
		t.Fatalf("DecodePayload returned error: %v", err)
	}
	if cmd.ImageID != "ubuntu-22.04" || cmd.ServerID != "srv-123" {
		t.Fatalf("decoded command = %+v", cmd)
	}
	if cmd.NetworkConfig["ip"] != "10.10.40.20" || cmd.NetworkConfig["mac"] != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("network_config = %+v", cmd.NetworkConfig)
	}
}

func TestRequestReplyHealthAndHeartbeat(t *testing.T) {
	agent, _, _, _, _, publisher := testAgent()
	reply, err := agent.RespondToRequest(context.Background(), messages.RequestHealth, messages.Envelope{RackID: "rack-1"})
	if err != nil {
		t.Fatalf("RespondToRequest returned error: %v", err)
	}
	var health messages.HealthReply
	_ = reply.DecodePayload(&health)
	if health.Status != "online" {
		t.Fatalf("health status = %s", health.Status)
	}
	if err := agent.PublishHeartbeat(context.Background()); err != nil {
		t.Fatalf("PublishHeartbeat returned error: %v", err)
	}
	if publisher.heartbeats != 1 {
		t.Fatalf("heartbeats = %d, want 1", publisher.heartbeats)
	}
}

func testAgent() (*Agent, *memoryLocalStore, *memoryProcessedStore, *adapters.MockIPMIAdapter, *adapters.MockTinkerbellAdapter, *memoryPublisher) {
	local := &memoryLocalStore{jobs: map[string]storage.LocalJob{}}
	processed := &memoryProcessedStore{seen: map[string]bool{}}
	ipmi := &adapters.MockIPMIAdapter{}
	tinkerbell := &adapters.MockTinkerbellAdapter{}
	publisher := &memoryPublisher{}
	agent := &Agent{RackID: "rack-1", AgentID: "agent-1", Version: "0.1", Storage: local, Processed: processed, Tinkerbell: tinkerbell, IPMI: ipmi, Network: &adapters.MockNetworkAdapter{}, Secrets: adapters.LocalSecretProvider{}, Publisher: publisher}
	return agent, local, processed, ipmi, tinkerbell, publisher
}

func provisionCommandEnvelope(t *testing.T, rackID string, expiresAt time.Time) messages.Envelope {
	t.Helper()
	env, err := messages.NewEnvelope(messages.ProvisionServerCommandType, rackID, messages.ProvisionServerCommand{OrganizationID: "org-1", ServerID: "srv-1", ImageID: "ubuntu", Hostname: "web-1"})
	if err != nil {
		t.Fatal(err)
	}
	env.JobID = ptr("job-1")
	env.ServerID = ptr("srv-1")
	env.CorrelationID = ptr("corr-1")
	env.ExpiresAt = &expiresAt
	return env
}

type memoryPublisher struct {
	events     []messages.Envelope
	heartbeats int
}

func (p *memoryPublisher) PublishEvent(ctx context.Context, kind messages.EventKind, envelope messages.Envelope) error {
	p.events = append(p.events, envelope)
	return nil
}

func (p *memoryPublisher) PublishHeartbeat(ctx context.Context, envelope messages.Envelope) error {
	p.heartbeats++
	return nil
}

func (p *memoryPublisher) countType(messageType string) int {
	var count int
	for _, event := range p.events {
		if event.MessageType == messageType {
			count++
		}
	}
	return count
}

type memoryLocalStore struct {
	jobs map[string]storage.LocalJob
}

func (s *memoryLocalStore) CreateOrGetJob(ctx context.Context, job storage.LocalJob) (storage.LocalJob, bool, error) {
	if existing, ok := s.jobs[job.CentralJobID]; ok {
		return existing, false, nil
	}
	s.jobs[job.CentralJobID] = job
	return job, true, nil
}

func (s *memoryLocalStore) UpdateJobStep(ctx context.Context, centralJobID, status, step string, failureReason *string) error {
	job := s.jobs[centralJobID]
	job.Status = status
	job.LastStep = step
	job.FailureReason = failureReason
	s.jobs[centralJobID] = job
	return nil
}

func (s *memoryLocalStore) GetJobByCentralID(ctx context.Context, centralJobID string) (storage.LocalJob, error) {
	return s.jobs[centralJobID], nil
}

func (s *memoryLocalStore) ActiveJobsCount(ctx context.Context) (int, error) {
	var count int
	for _, job := range s.jobs {
		if job.Status == "running" {
			count++
		}
	}
	return count, nil
}

type memoryProcessedStore struct {
	seen map[string]bool
}

func (s *memoryProcessedStore) AlreadyProcessed(ctx context.Context, messageID string) (bool, error) {
	return s.seen[messageID], nil
}

func (s *memoryProcessedStore) MarkProcessed(ctx context.Context, message storage.ProcessedMessage) error {
	s.seen[message.MessageID] = true
	return nil
}

func ptr[T any](value T) *T {
	return &value
}
