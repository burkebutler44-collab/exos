package grpcapi

import (
	"context"

	relayv1 "relay/client-backend/gen/go/relay/v1"
	"relay/client-backend/internal/services"
)

type ProvisionerServer struct {
	relayv1.UnimplementedProvisionerServiceServer
	provision *services.ProvisionService
}

func NewProvisionerServer(provision *services.ProvisionService) *ProvisionerServer {
	return &ProvisionerServer{provision: provision}
}

func (s *ProvisionerServer) ProvisionServer(ctx context.Context, req *relayv1.ProvisionServerRequest) (*relayv1.ProvisionServerResponse, error) {
	if err := s.provision.ProvisionServer(ctx, req.GetServerId()); err != nil {
		return nil, err
	}
	return &relayv1.ProvisionServerResponse{ServerId: req.GetServerId(), Status: "queued"}, nil
}

func (s *ProvisionerServer) PowerCycleServer(ctx context.Context, req *relayv1.PowerCycleServerRequest) (*relayv1.PowerCycleServerResponse, error) {
	if err := s.provision.PowerCycleServer(ctx, req.GetServerId()); err != nil {
		return nil, err
	}
	return &relayv1.PowerCycleServerResponse{ServerId: req.GetServerId(), Status: "queued"}, nil
}

func (s *ProvisionerServer) ReinstallServer(ctx context.Context, req *relayv1.ReinstallServerRequest) (*relayv1.ReinstallServerResponse, error) {
	if err := s.provision.ReinstallServer(ctx, req.GetServerId()); err != nil {
		return nil, err
	}
	return &relayv1.ReinstallServerResponse{ServerId: req.GetServerId(), Status: "queued"}, nil
}

func (s *ProvisionerServer) DeprovisionServer(ctx context.Context, req *relayv1.DeprovisionServerRequest) (*relayv1.DeprovisionServerResponse, error) {
	if err := s.provision.DeprovisionServer(ctx, req.GetServerId()); err != nil {
		return nil, err
	}
	return &relayv1.DeprovisionServerResponse{ServerId: req.GetServerId(), Status: "queued"}, nil
}
