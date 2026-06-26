package grpcapi

import (
	"context"
	"crypto/x509"
	"io"
	"log"
	"math"
	"sync"
	"time"

	relayv1 "relay/client-backend/gen/go/relay/v1"
	"relay/client-backend/internal/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type HypervisorRegistry struct {
	mu          sync.RWMutex
	repo        hypervisorSnapshotRepository
	snapshots   map[string]*relayv1.HostSnapshot
	connections map[string]chan *relayv1.HypervisorCommand
	lastSeen    map[string]time.Time
}

type hypervisorSnapshotRepository interface {
	UpsertHypervisorSnapshot(ctx context.Context, params store.UpsertHypervisorSnapshotParams) error
	UpsertHypervisorCommand(ctx context.Context, params store.UpsertHypervisorCommandParams) error
	MarkHypervisorCommandSent(ctx context.Context, commandID string) error
	CompleteHypervisorCommand(ctx context.Context, params store.CompleteHypervisorCommandParams) error
}

func NewHypervisorRegistry(repo hypervisorSnapshotRepository) *HypervisorRegistry {
	return &HypervisorRegistry{
		repo:        repo,
		snapshots:   map[string]*relayv1.HostSnapshot{},
		connections: map[string]chan *relayv1.HypervisorCommand{},
		lastSeen:    map[string]time.Time{},
	}
}

func (r *HypervisorRegistry) RecordSnapshot(ctx context.Context, snapshot *relayv1.HostSnapshot) error {
	if snapshot == nil || snapshot.GetHypervisorId() == "" {
		return status.Error(codes.InvalidArgument, "hypervisor_id is required")
	}
	r.mu.Lock()
	r.snapshots[snapshot.GetHypervisorId()] = snapshot
	r.lastSeen[snapshot.GetHypervisorId()] = time.Now().UTC()
	r.mu.Unlock()
	if r.repo != nil {
		if err := r.repo.UpsertHypervisorSnapshot(ctx, snapshotToStoreParams(snapshot)); err != nil {
			return err
		}
	}
	return nil
}

func (r *HypervisorRegistry) LastSnapshot(hypervisorID string) (*relayv1.HostSnapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot, ok := r.snapshots[hypervisorID]
	return snapshot, ok
}

func (r *HypervisorRegistry) ConnectedHypervisors() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.connections))
	for id := range r.connections {
		ids = append(ids, id)
	}
	return ids
}

func (r *HypervisorRegistry) EnqueueCommand(hypervisorID string, command *relayv1.HypervisorCommand) error {
	if hypervisorID == "" || command == nil || command.GetCommandId() == "" {
		return status.Error(codes.InvalidArgument, "hypervisor_id and command_id are required")
	}
	if r.repo != nil {
		payload, _ := protojson.Marshal(command)
		if err := r.repo.UpsertHypervisorCommand(context.Background(), store.UpsertHypervisorCommandParams{
			HypervisorID: hypervisorID,
			CommandID:    command.GetCommandId(),
			CommandType:  hypervisorCommandType(command),
			Payload:      payload,
		}); err != nil {
			return err
		}
	}
	r.mu.RLock()
	ch, ok := r.connections[hypervisorID]
	r.mu.RUnlock()
	if !ok {
		return status.Error(codes.FailedPrecondition, "hypervisor is not connected")
	}
	select {
	case ch <- command:
		return nil
	default:
		return status.Error(codes.ResourceExhausted, "hypervisor command queue is full")
	}
}

func hypervisorCommandType(command *relayv1.HypervisorCommand) string {
	switch command.GetCommand().(type) {
	case *relayv1.HypervisorCommand_CreateVm:
		return "create_vm"
	case *relayv1.HypervisorCommand_DeleteVm:
		return "delete_vm"
	case *relayv1.HypervisorCommand_PowerVm:
		return "power_vm"
	default:
		return "unknown"
	}
}

func (r *HypervisorRegistry) registerConnection(hypervisorID string) (<-chan *relayv1.HypervisorCommand, func()) {
	ch := make(chan *relayv1.HypervisorCommand, 32)
	r.mu.Lock()
	r.connections[hypervisorID] = ch
	r.lastSeen[hypervisorID] = time.Now().UTC()
	r.mu.Unlock()
	cleanup := func() {
		r.mu.Lock()
		if current := r.connections[hypervisorID]; current == ch {
			delete(r.connections, hypervisorID)
			close(ch)
		}
		r.mu.Unlock()
	}
	return ch, cleanup
}

type HypervisorServer struct {
	relayv1.UnimplementedHypervisorServiceServer
	registry                 *HypervisorRegistry
	agentToken               string
	requireClientCertificate bool
}

type HypervisorServerOption func(*HypervisorServer)

func WithHypervisorAgentToken(token string) HypervisorServerOption {
	return func(s *HypervisorServer) {
		s.agentToken = token
	}
}

func WithHypervisorClientCertificateAuth(required bool) HypervisorServerOption {
	return func(s *HypervisorServer) {
		s.requireClientCertificate = required
	}
}

func NewHypervisorServer(registry *HypervisorRegistry, options ...HypervisorServerOption) *HypervisorServer {
	if registry == nil {
		registry = NewHypervisorRegistry(nil)
	}
	server := &HypervisorServer{registry: registry}
	for _, option := range options {
		option(server)
	}
	return server
}

func (s *HypervisorServer) ReportSnapshot(ctx context.Context, req *relayv1.ReportSnapshotRequest) (*relayv1.ReportSnapshotResponse, error) {
	if err := s.authorize(ctx, req.GetSnapshot().GetHypervisorId()); err != nil {
		return nil, err
	}
	if err := s.registry.RecordSnapshot(ctx, req.GetSnapshot()); err != nil {
		return nil, err
	}
	log.Printf("hypervisor snapshot received id=%s vms=%d", req.GetSnapshot().GetHypervisorId(), len(req.GetSnapshot().GetVms()))
	return &relayv1.ReportSnapshotResponse{Status: "accepted"}, nil
}

func (s *HypervisorServer) Connect(stream relayv1.HypervisorService_ConnectServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	hypervisorID := first.GetHypervisorId()
	if hypervisorID == "" {
		if snapshot := first.GetSnapshot(); snapshot != nil {
			hypervisorID = snapshot.GetHypervisorId()
		}
	}
	if hypervisorID == "" {
		return status.Error(codes.InvalidArgument, "hypervisor_id is required")
	}
	if err := s.authorize(stream.Context(), hypervisorID); err != nil {
		return err
	}
	commands, cleanup := s.registry.registerConnection(hypervisorID)
	defer cleanup()
	if err := s.recordEvent(stream.Context(), first); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		for command := range commands {
			if err := stream.Send(command); err != nil {
				errCh <- err
				return
			}
			if s.registry.repo != nil {
				if err := s.registry.repo.MarkHypervisorCommandSent(stream.Context(), command.GetCommandId()); err != nil {
					log.Printf("mark hypervisor command sent failed id=%s: %v", command.GetCommandId(), err)
				}
			}
		}
		errCh <- nil
	}()

	for {
		select {
		case err := <-errCh:
			return err
		default:
		}
		event, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if event.GetHypervisorId() == "" {
			event.HypervisorId = hypervisorID
		}
		if err := s.recordEvent(stream.Context(), event); err != nil {
			return err
		}
	}
}

func (s *HypervisorServer) recordEvent(ctx context.Context, event *relayv1.HypervisorEvent) error {
	if snapshot := event.GetSnapshot(); snapshot != nil {
		if snapshot.GetHypervisorId() == "" {
			snapshot.HypervisorId = event.GetHypervisorId()
		}
		return s.registry.RecordSnapshot(ctx, snapshot)
	}
	if result := event.GetCommandResult(); result != nil {
		log.Printf("hypervisor command result id=%s command=%s vm=%s status=%s", event.GetHypervisorId(), result.GetCommandId(), result.GetName(), result.GetStatus())
		if s.registry.repo != nil && result.GetCommandId() != "" {
			resultJSON, _ := protojson.Marshal(result)
			statusValue := "succeeded"
			if result.GetStatus() == "failed" {
				statusValue = "failed"
			}
			if err := s.registry.repo.CompleteHypervisorCommand(ctx, store.CompleteHypervisorCommandParams{
				CommandID:    result.GetCommandId(),
				Status:       statusValue,
				Result:       resultJSON,
				ErrorMessage: result.GetMessage(),
			}); err != nil {
				return err
			}
		}
		return nil
	}
	return status.Error(codes.InvalidArgument, "hypervisor event payload is required")
}

func (s *HypervisorServer) authorize(ctx context.Context, hypervisorID string) error {
	if s.agentToken != "" {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Error(codes.Unauthenticated, "hypervisor credentials are required")
		}
		values := md.Get("x-exos-hypervisor-token")
		if len(values) == 0 || values[0] != s.agentToken {
			return status.Error(codes.Unauthenticated, "invalid hypervisor credentials")
		}
	}
	if !s.requireClientCertificate {
		return nil
	}
	certID, err := hypervisorIDFromClientCertificate(ctx)
	if err != nil {
		return err
	}
	if certID != hypervisorID {
		return status.Errorf(codes.PermissionDenied, "hypervisor certificate identity %q cannot act as %q", certID, hypervisorID)
	}
	return nil
}

func hypervisorIDFromClientCertificate(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.AuthInfo == nil {
		return "", status.Error(codes.Unauthenticated, "hypervisor client certificate is required")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.PeerCertificates) == 0 {
		return "", status.Error(codes.Unauthenticated, "hypervisor client certificate is required")
	}
	cert := tlsInfo.State.PeerCertificates[0]
	if id := certificateHypervisorID(cert); id != "" {
		return id, nil
	}
	return "", status.Error(codes.Unauthenticated, "hypervisor client certificate has no usable identity")
}

func certificateHypervisorID(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	for _, uri := range cert.URIs {
		if uri.Scheme == "exos-hypervisor" && uri.Host != "" {
			return uri.Host
		}
	}
	return cert.Subject.CommonName
}

func snapshotToStoreParams(snapshot *relayv1.HostSnapshot) store.UpsertHypervisorSnapshotParams {
	reportedAt := time.Now().UTC()
	if snapshot.GetCollectedAtUnix() > 0 {
		reportedAt = time.Unix(snapshot.GetCollectedAtUnix(), 0).UTC()
	}
	vms := make([]store.UpsertHypervisorVMParams, 0, len(snapshot.GetVms()))
	for _, vm := range snapshot.GetVms() {
		vms = append(vms, store.UpsertHypervisorVMParams{
			ID:             vm.GetId(),
			Name:           vm.GetName(),
			Status:         normalizeVMStatus(vm.GetStatus()),
			VCPUs:          vm.GetVcpus(),
			MemoryBytes:    uint64ToInt64(vm.GetMemoryBytes()),
			DiskBytes:      uint64ToInt64(vm.GetDiskBytes()),
			MACAddresses:   vm.GetMacAddresses(),
			IPAddresses:    vm.GetIpAddresses(),
			Metadata:       vm.GetMetadata(),
			LastReportedAt: reportedAt,
		})
	}
	return store.UpsertHypervisorSnapshotParams{
		ID:                  snapshot.GetHypervisorId(),
		Hostname:            snapshot.GetHostname(),
		Status:              "online",
		VCPUsTotal:          snapshot.GetVcpusTotal(),
		VCPUsActive:         snapshot.GetVcpusActive(),
		MemoryTotalBytes:    uint64ToInt64(snapshot.GetMemoryTotalBytes()),
		MemoryActiveBytes:   uint64ToInt64(snapshot.GetMemoryActiveBytes()),
		DiskTotalBytes:      uint64ToInt64(snapshot.GetDiskTotalBytes()),
		DiskAvailableBytes:  uint64ToInt64(snapshot.GetDiskAvailableBytes()),
		WireguardInterface:  snapshot.GetWireguardInterface(),
		ControlPlaneAddress: snapshot.GetControlPlaneAddress(),
		LastReportedAt:      reportedAt,
		VMs:                 vms,
	}
}

func uint64ToInt64(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
}

func normalizeVMStatus(statusValue string) string {
	switch statusValue {
	case "running", "stopped", "paused":
		return statusValue
	default:
		return "unknown"
	}
}
