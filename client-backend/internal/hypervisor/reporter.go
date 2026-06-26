package hypervisor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"log"
	"os"

	relayv1 "relay/client-backend/gen/go/relay/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type Reporter interface {
	ReportSnapshot(ctx context.Context, snapshot HostSnapshot) error
	Close() error
}

type CommandStreamer interface {
	RunCommandStream(ctx context.Context, initial HostSnapshot, handler func(context.Context, *relayv1.HypervisorCommand) *relayv1.VMCommandResult) error
}

type LogReporter struct{}

func (LogReporter) ReportSnapshot(ctx context.Context, snapshot HostSnapshot) error {
	payload, _ := json.Marshal(snapshot)
	log.Printf("hypervisor snapshot: %s", payload)
	return nil
}

func (LogReporter) Close() error { return nil }

type GRPCReporter struct {
	conn   *grpc.ClientConn
	client relayv1.HypervisorServiceClient
	token  string
}

func NewGRPCReporter(ctx context.Context, cfg Config) (*GRPCReporter, error) {
	opts := []grpc.DialOption{}
	if cfg.ControlPlaneTLS {
		tlsConfig := &tls.Config{}
		if cfg.ControlPlaneAuthority != "" {
			tlsConfig.ServerName = cfg.ControlPlaneAuthority
		}
		if cfg.ControlPlaneCAFile != "" {
			caPEM, err := os.ReadFile(cfg.ControlPlaneCAFile)
			if err != nil {
				return nil, err
			}
			roots := x509.NewCertPool()
			if roots.AppendCertsFromPEM(caPEM) {
				tlsConfig.RootCAs = roots
			}
		}
		if cfg.ClientTLSCertFile != "" || cfg.ClientTLSKeyFile != "" {
			cert, err := tls.LoadX509KeyPair(cfg.ClientTLSCertFile, cfg.ClientTLSKeyFile)
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.DialContext(ctx, cfg.ControlPlaneGRPCEndpoint, opts...)
	if err != nil {
		return nil, err
	}
	return &GRPCReporter{conn: conn, client: relayv1.NewHypervisorServiceClient(conn), token: cfg.ControlPlaneAgentToken}, nil
}

func (r *GRPCReporter) ReportSnapshot(ctx context.Context, snapshot HostSnapshot) error {
	ctx = r.withAuth(ctx)
	_, err := r.client.ReportSnapshot(ctx, &relayv1.ReportSnapshotRequest{Snapshot: snapshotToProto(snapshot)})
	return err
}

func (r *GRPCReporter) RunCommandStream(ctx context.Context, initial HostSnapshot, handler func(context.Context, *relayv1.HypervisorCommand) *relayv1.VMCommandResult) error {
	ctx = r.withAuth(ctx)
	stream, err := r.client.Connect(ctx)
	if err != nil {
		return err
	}
	if err := stream.Send(&relayv1.HypervisorEvent{
		HypervisorId: initial.HypervisorID,
		Event:        &relayv1.HypervisorEvent_Snapshot{Snapshot: snapshotToProto(initial)},
	}); err != nil {
		return err
	}
	for {
		command, err := stream.Recv()
		if err != nil {
			return err
		}
		result := handler(ctx, command)
		if result == nil {
			continue
		}
		if err := stream.Send(&relayv1.HypervisorEvent{
			HypervisorId: initial.HypervisorID,
			Event:        &relayv1.HypervisorEvent_CommandResult{CommandResult: result},
		}); err != nil {
			return err
		}
	}
}

func (r *GRPCReporter) withAuth(ctx context.Context) context.Context {
	if r.token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "x-exos-hypervisor-token", r.token)
}

func (r *GRPCReporter) Close() error {
	if r.conn == nil {
		return nil
	}
	return r.conn.Close()
}

func snapshotToProto(snapshot HostSnapshot) *relayv1.HostSnapshot {
	vms := make([]*relayv1.VMInfo, 0, len(snapshot.VMs))
	for _, vm := range snapshot.VMs {
		vms = append(vms, &relayv1.VMInfo{
			Id:           vm.ID,
			Name:         vm.Name,
			Status:       string(vm.Status),
			Vcpus:        int32(vm.VCPUs),
			MemoryBytes:  vm.MemoryBytes,
			DiskBytes:    vm.DiskBytes,
			MacAddresses: append([]string(nil), vm.MACAddresses...),
			IpAddresses:  append([]string(nil), vm.IPAddresses...),
			Metadata:     vm.Metadata,
		})
	}
	return &relayv1.HostSnapshot{
		HypervisorId:        snapshot.HypervisorID,
		Hostname:            snapshot.Hostname,
		CollectedAtUnix:     snapshot.CollectedAt.Unix(),
		VcpusTotal:          int32(snapshot.VCPUsTotal),
		VcpusActive:         int32(snapshot.VCPUsActive),
		MemoryTotalBytes:    snapshot.MemoryTotalBytes,
		MemoryActiveBytes:   snapshot.MemoryActiveBytes,
		DiskTotalBytes:      snapshot.DiskTotalBytes,
		DiskAvailableBytes:  snapshot.DiskAvailableBytes,
		Vms:                 vms,
		WireguardInterface:  snapshot.WireGuardInterface,
		ControlPlaneAddress: snapshot.ControlPlaneAddress,
	}
}
