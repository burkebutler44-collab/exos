package services

import (
	"context"
	"fmt"
	"net"
	"strings"

	"relay/client-backend/internal/store"

	"github.com/google/uuid"
)

const (
	ServerModeBareMetal           = "bare_metal"
	ServerModeVirtualizationHost  = "virtualization_host"
	ServerModeManagedServicesHost = "managed_services_host"
)

type CloudProvisioningService interface {
	RequestStubAction(ctx context.Context, organizationID uuid.UUID, cloudID *uuid.UUID, resourceType string, resourceID *uuid.UUID, actionType, message string) (store.ResourceAction, error)
}

type StubCloudProvisioningService struct {
	repo Repository
}

func (p StubCloudProvisioningService) RequestStubAction(ctx context.Context, organizationID uuid.UUID, cloudID *uuid.UUID, resourceType string, resourceID *uuid.UUID, actionType, message string) (store.ResourceAction, error) {
	return p.repo.AddResourceAction(ctx, organizationID, cloudID, resourceType, resourceID, actionType, "stubbed", message)
}

func (s *Services) cloudProvisioning() CloudProvisioningService {
	return StubCloudProvisioningService{repo: s.repo}
}

func (s *Services) ListClouds(ctx context.Context, organizationID uuid.UUID) ([]store.Cloud, error) {
	return s.repo.ListClouds(ctx, organizationID)
}

func (s *Services) CreateCloud(ctx context.Context, organizationID uuid.UUID, name string, locationID *uuid.UUID, description *string, createDefaultNetwork bool, defaultCIDR string) (store.Cloud, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return store.Cloud{}, ErrInvalidInput
	}
	if createDefaultNetwork {
		if defaultCIDR == "" {
			defaultCIDR = "10.80.0.0/16"
		}
		if err := validateCIDR(defaultCIDR); err != nil {
			return store.Cloud{}, err
		}
	}
	return s.repo.CreateCloud(ctx, store.CreateCloudParams{
		OrganizationID:       organizationID,
		Name:                 name,
		Slug:                 slugify(name),
		LocationID:           locationID,
		Description:          cleanStringPtr(description),
		CreateDefaultNetwork: createDefaultNetwork,
		DefaultCIDR:          defaultCIDR,
	})
}

func (s *Services) GetCloud(ctx context.Context, organizationID, cloudID uuid.UUID) (store.Cloud, error) {
	return s.repo.GetCloud(ctx, organizationID, cloudID)
}

func (s *Services) UpdateCloud(ctx context.Context, organizationID, cloudID uuid.UUID, name string, description *string) (store.Cloud, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return store.Cloud{}, ErrInvalidInput
	}
	return s.repo.UpdateCloud(ctx, organizationID, cloudID, store.UpdateCloudParams{Name: name, Slug: slugify(name), Description: cleanStringPtr(description)})
}

func (s *Services) DeleteCloud(ctx context.Context, organizationID, cloudID uuid.UUID) error {
	return s.repo.DeleteCloud(ctx, organizationID, cloudID)
}

func (s *Services) GetCloudOverview(ctx context.Context, organizationID, cloudID uuid.UUID) (store.CloudOverview, error) {
	return s.repo.GetCloudOverview(ctx, organizationID, cloudID)
}

func (s *Services) ListCloudServers(ctx context.Context, organizationID, cloudID uuid.UUID) ([]store.CloudServer, error) {
	return s.repo.ListCloudServers(ctx, organizationID, cloudID)
}

func (s *Services) ListOrganizationServers(ctx context.Context, organizationID uuid.UUID) ([]store.FleetServer, error) {
	return s.repo.ListOrganizationServers(ctx, organizationID)
}

func (s *Services) AssignServerToCloud(ctx context.Context, organizationID, cloudID, serverID uuid.UUID) (store.CloudServer, error) {
	if _, err := s.repo.GetCloud(ctx, organizationID, cloudID); err != nil {
		return store.CloudServer{}, err
	}
	return s.repo.AssignServerToCloud(ctx, organizationID, cloudID, serverID)
}

func (s *Services) UnassignServerFromCloud(ctx context.Context, organizationID, cloudID, serverID uuid.UUID) (store.CloudServer, error) {
	return s.repo.UnassignServerFromCloud(ctx, organizationID, cloudID, serverID)
}

func (s *Services) ChangeServerMode(ctx context.Context, organizationID, serverID uuid.UUID, mode string) (store.CloudServer, error) {
	switch mode {
	case ServerModeBareMetal, ServerModeVirtualizationHost, ServerModeManagedServicesHost:
	default:
		return store.CloudServer{}, ErrInvalidInput
	}
	return s.repo.ChangeServerMode(ctx, organizationID, serverID, mode)
}

func (s *Services) ListPrivateNetworks(ctx context.Context, organizationID, cloudID uuid.UUID) ([]store.PrivateNetwork, error) {
	return s.repo.ListPrivateNetworks(ctx, organizationID, cloudID)
}

func (s *Services) CreatePrivateNetwork(ctx context.Context, organizationID, cloudID uuid.UUID, name string, description *string, cidr string, gatewayIP *string) (store.PrivateNetwork, error) {
	name = strings.TrimSpace(name)
	if name == "" || validateCIDR(cidr) != nil {
		return store.PrivateNetwork{}, ErrInvalidInput
	}
	if err := validateOptionalIP(gatewayIP); err != nil {
		return store.PrivateNetwork{}, err
	}
	return s.repo.CreatePrivateNetwork(ctx, store.CreatePrivateNetworkParams{
		OrganizationID: organizationID,
		CloudID:        cloudID,
		Name:           name,
		Description:    cleanStringPtr(description),
		CIDR:           cidr,
		GatewayIP:      cleanStringPtr(gatewayIP),
	})
}

func (s *Services) GetPrivateNetwork(ctx context.Context, organizationID, cloudID, networkID uuid.UUID) (store.PrivateNetwork, error) {
	return s.repo.GetPrivateNetwork(ctx, organizationID, cloudID, networkID)
}

func (s *Services) DeletePrivateNetwork(ctx context.Context, organizationID, cloudID, networkID uuid.UUID) error {
	return s.repo.DeletePrivateNetwork(ctx, organizationID, cloudID, networkID)
}

func (s *Services) ListNetworkAttachments(ctx context.Context, organizationID, cloudID, networkID uuid.UUID) ([]store.NetworkAttachment, error) {
	return s.repo.ListNetworkAttachments(ctx, organizationID, cloudID, networkID)
}

func (s *Services) CreateNetworkAttachment(ctx context.Context, params store.CreateNetworkAttachmentParams) (store.NetworkAttachment, error) {
	if params.ResourceType != "server" && params.ResourceType != "virtual_machine" && params.ResourceType != "managed_service" {
		return store.NetworkAttachment{}, ErrInvalidInput
	}
	if err := validateOptionalIP(params.PrivateIP); err != nil {
		return store.NetworkAttachment{}, err
	}
	if params.ResourceType == "server" {
		servers, err := s.repo.ListCloudServers(ctx, params.OrganizationID, params.CloudID)
		if err != nil {
			return store.NetworkAttachment{}, err
		}
		found := false
		for _, server := range servers {
			if server.ID == params.ResourceID {
				found = true
				break
			}
		}
		if !found {
			return store.NetworkAttachment{}, ErrInvalidInput
		}
		networks, err := s.repo.ListPrivateNetworks(ctx, params.OrganizationID, params.CloudID)
		if err != nil {
			return store.NetworkAttachment{}, err
		}
		for _, network := range networks {
			attachments, err := s.repo.ListNetworkAttachments(ctx, params.OrganizationID, params.CloudID, network.ID)
			if err != nil {
				return store.NetworkAttachment{}, err
			}
			for _, attachment := range attachments {
				if attachment.ResourceType == "server" && attachment.ResourceID == params.ResourceID && attachment.Status != "detached" && network.ID != params.PrivateNetworkID {
					return store.NetworkAttachment{}, ErrConflict
				}
			}
		}
	}
	return s.repo.CreateNetworkAttachment(ctx, params)
}

func (s *Services) DetachNetworkAttachment(ctx context.Context, organizationID, cloudID, networkID, attachmentID uuid.UUID) error {
	return s.repo.DetachNetworkAttachment(ctx, organizationID, cloudID, networkID, attachmentID)
}

func (s *Services) ListVirtualMachines(ctx context.Context, organizationID, cloudID uuid.UUID) ([]store.VirtualMachine, error) {
	return s.repo.ListVirtualMachines(ctx, organizationID, cloudID)
}

func (s *Services) CreateVirtualMachine(ctx context.Context, params store.CreateVirtualMachineParams) (store.VirtualMachine, error) {
	if strings.TrimSpace(params.Name) == "" || params.CPUCores <= 0 || params.MemoryMB <= 0 || params.DiskGB <= 0 {
		return store.VirtualMachine{}, ErrInvalidInput
	}
	if err := validateOptionalIP(params.PrivateIP); err != nil {
		return store.VirtualMachine{}, err
	}
	if params.PrivateNetworkID != nil && params.PrivateIP != nil {
		network, err := s.repo.GetPrivateNetwork(ctx, params.OrganizationID, params.CloudID, *params.PrivateNetworkID)
		if err != nil {
			return store.VirtualMachine{}, err
		}
		if !ipInCIDR(*params.PrivateIP, network.CIDR) {
			return store.VirtualMachine{}, ErrInvalidInput
		}
	}
	if params.HostServerID != nil {
		if err := s.requireEligibleHost(ctx, params.OrganizationID, params.CloudID, *params.HostServerID, "vm"); err != nil {
			return store.VirtualMachine{}, err
		}
	}
	params.Name = strings.TrimSpace(params.Name)
	params.Hostname = strings.TrimSpace(params.Hostname)
	if params.Hostname == "" {
		params.Hostname = slugify(params.Name)
	}
	return s.repo.CreateVirtualMachine(ctx, params)
}

func (s *Services) GetVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID) (store.VirtualMachine, error) {
	return s.repo.GetVirtualMachine(ctx, organizationID, cloudID, vmID)
}

func (s *Services) UpdateVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID, params store.UpdateVirtualMachineParams) (store.VirtualMachine, error) {
	if strings.TrimSpace(params.Name) == "" || params.CPUCores <= 0 || params.MemoryMB <= 0 || params.DiskGB <= 0 {
		return store.VirtualMachine{}, ErrInvalidInput
	}
	if err := validateOptionalIP(params.PrivateIP); err != nil {
		return store.VirtualMachine{}, err
	}
	if params.HostServerID != nil {
		if err := s.requireEligibleHost(ctx, organizationID, cloudID, *params.HostServerID, "vm"); err != nil {
			return store.VirtualMachine{}, err
		}
	}
	params.Name = strings.TrimSpace(params.Name)
	params.Hostname = strings.TrimSpace(params.Hostname)
	if params.Hostname == "" {
		params.Hostname = slugify(params.Name)
	}
	return s.repo.UpdateVirtualMachine(ctx, organizationID, cloudID, vmID, params)
}

func (s *Services) DeleteVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID) error {
	return s.repo.DeleteVirtualMachine(ctx, organizationID, cloudID, vmID)
}

func (s *Services) PowerVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID, action string) (store.VirtualMachine, error) {
	return s.repo.PowerVirtualMachine(ctx, organizationID, cloudID, vmID, action)
}

func (s *Services) ListManagedServices(ctx context.Context, organizationID, cloudID uuid.UUID) ([]store.ManagedService, error) {
	return s.repo.ListManagedServices(ctx, organizationID, cloudID)
}

func (s *Services) CreatePostgresService(ctx context.Context, params store.CreateManagedServiceParams) (store.ManagedService, error) {
	if strings.TrimSpace(params.Name) == "" || params.CPUCores <= 0 || params.MemoryMB <= 0 || params.StorageGB <= 0 {
		return store.ManagedService{}, ErrInvalidInput
	}
	if err := validateOptionalIP(params.PrivateIP); err != nil {
		return store.ManagedService{}, err
	}
	if params.PrivateNetworkID != nil && params.PrivateIP != nil {
		network, err := s.repo.GetPrivateNetwork(ctx, params.OrganizationID, params.CloudID, *params.PrivateNetworkID)
		if err != nil {
			return store.ManagedService{}, err
		}
		if !ipInCIDR(*params.PrivateIP, network.CIDR) {
			return store.ManagedService{}, ErrInvalidInput
		}
	}
	if params.HostServerID != nil {
		if err := s.requireEligibleHost(ctx, params.OrganizationID, params.CloudID, *params.HostServerID, "postgres"); err != nil {
			return store.ManagedService{}, err
		}
	}
	params.ServiceType = "postgres"
	params.Name = strings.TrimSpace(params.Name)
	params.Version = strings.TrimSpace(params.Version)
	if params.Version == "" {
		params.Version = "16"
	}
	if params.BackupRetentionDays <= 0 {
		params.BackupRetentionDays = 7
	}
	cloud, err := s.repo.GetCloud(ctx, params.OrganizationID, params.CloudID)
	if err != nil {
		return store.ManagedService{}, err
	}
	params.EndpointHostname = fmt.Sprintf("postgres-%s.%s.internal", slugify(params.Name), cloud.Slug)
	return s.repo.CreateManagedService(ctx, params)
}

func (s *Services) GetManagedService(ctx context.Context, organizationID, cloudID, serviceID uuid.UUID) (store.ManagedService, error) {
	return s.repo.GetManagedService(ctx, organizationID, cloudID, serviceID)
}

func (s *Services) DeleteManagedService(ctx context.Context, organizationID, cloudID, serviceID uuid.UUID) error {
	return s.repo.DeleteManagedService(ctx, organizationID, cloudID, serviceID)
}

func (s *Services) ActOnManagedService(ctx context.Context, organizationID, cloudID, serviceID uuid.UUID, action string) (store.ManagedService, error) {
	return s.repo.ActOnManagedService(ctx, organizationID, cloudID, serviceID, action)
}

func (s *Services) GetCloudCapacity(ctx context.Context, organizationID, cloudID uuid.UUID) (store.CloudCapacity, error) {
	return s.repo.GetCloudCapacity(ctx, organizationID, cloudID)
}

func (s *Services) ListPlacementOptions(ctx context.Context, organizationID, cloudID uuid.UUID, resourceType string) ([]store.PlacementOption, error) {
	if resourceType != "vm" && resourceType != "postgres" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListPlacementOptions(ctx, organizationID, cloudID, resourceType)
}

func (s *Services) ListResourceActions(ctx context.Context, organizationID uuid.UUID, cloudID *uuid.UUID) ([]store.ResourceAction, error) {
	return s.repo.ListResourceActions(ctx, organizationID, cloudID)
}

func (s *Services) ListAdminCloudResources(ctx context.Context) ([]store.Cloud, []store.VirtualMachine, []store.ManagedService, []store.PrivateNetwork, []store.ResourceAction, error) {
	return s.repo.ListAdminCloudResources(ctx)
}

func (s *Services) requireEligibleHost(ctx context.Context, organizationID, cloudID, hostID uuid.UUID, resourceType string) error {
	options, err := s.repo.ListPlacementOptions(ctx, organizationID, cloudID, resourceType)
	if err != nil {
		return err
	}
	for _, option := range options {
		if option.ServerID == hostID {
			return nil
		}
	}
	return ErrInvalidInput
}

func validateCIDR(value string) error {
	if strings.TrimSpace(value) == "" {
		return ErrInvalidInput
	}
	if _, _, err := net.ParseCIDR(value); err != nil {
		return ErrInvalidInput
	}
	return nil
}

func validateOptionalIP(value *string) error {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	if net.ParseIP(strings.TrimSpace(*value)) == nil {
		return ErrInvalidInput
	}
	clean := strings.TrimSpace(*value)
	*value = clean
	return nil
}

func ipInCIDR(ipValue, cidrValue string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipValue))
	_, network, err := net.ParseCIDR(strings.TrimSpace(cidrValue))
	if ip == nil || err != nil {
		return false
	}
	return network.Contains(ip)
}

func cleanStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	clean := strings.TrimSpace(*value)
	if clean == "" {
		return nil
	}
	return &clean
}
