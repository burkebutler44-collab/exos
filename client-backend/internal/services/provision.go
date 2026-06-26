package services

import "context"

type ProvisionService struct{}

func NewProvisionService() *ProvisionService {
	return &ProvisionService{}
}

func (s *ProvisionService) ProvisionServer(ctx context.Context, serverID string) error {
	// TODO: Call the bare metal provider workflow once server provisioning is designed.
	return nil
}

func (s *ProvisionService) PowerCycleServer(ctx context.Context, serverID string) error {
	// TODO: Wire power management to the provider API.
	return nil
}

func (s *ProvisionService) ReinstallServer(ctx context.Context, serverID string) error {
	// TODO: Queue OS reinstall once image/install flows exist.
	return nil
}

func (s *ProvisionService) DeprovisionServer(ctx context.Context, serverID string) error {
	// TODO: Release hardware and cleanup provider-side networking when server lifecycle lands.
	return nil
}
