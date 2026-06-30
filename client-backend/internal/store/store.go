package store

import (
	"context"
	"time"

	"relay/client-backend/internal/domain"

	"github.com/google/uuid"
)

type AuthIdentity struct {
	Auth0Sub string
	Email    string
	Name     string
}

type CreateOrganizationParams struct {
	Name            string
	Slug            string
	CreatedByUserID uuid.UUID
	BillingEmail    string
}

type CreateInvitationParams struct {
	OrganizationID  uuid.UUID
	Email           string
	RoleID          uuid.UUID
	InvitedByUserID uuid.UUID
	Token           string
	ExpiresInHours  int
}

type UpdateBillingProfileParams struct {
	BillingEmail string
	CompanyName  string
	TaxID        *string
	Line1        *string
	Line2        *string
	City         *string
	State        *string
	PostalCode   *string
	Country      *string
}

type CreateCloudParams struct {
	OrganizationID       uuid.UUID
	Name                 string
	Slug                 string
	LocationID           *uuid.UUID
	Description          *string
	CreateDefaultNetwork bool
	DefaultCIDR          string
}

type UpdateCloudParams struct {
	Name        string
	Slug        string
	Description *string
}

type Cloud struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Name           string     `json:"name"`
	Slug           string     `json:"slug"`
	LocationID     *uuid.UUID `json:"location_id"`
	LocationName   *string    `json:"location_name"`
	Description    *string    `json:"description"`
	Status         string     `json:"status"`
	ServerCount    int64      `json:"server_count"`
	VMCount        int64      `json:"vm_count"`
	ServiceCount   int64      `json:"managed_service_count"`
	NetworkCount   int64      `json:"private_network_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at"`
}

type CloudOverview struct {
	Cloud                    Cloud            `json:"cloud"`
	ServerCount              int64            `json:"server_count"`
	VirtualizationHostCount  int64            `json:"virtualization_host_count"`
	ManagedServicesHostCount int64            `json:"managed_services_host_count"`
	VMCount                  int64            `json:"vm_count"`
	ManagedServiceCount      int64            `json:"managed_service_count"`
	PrivateNetworkCount      int64            `json:"private_network_count"`
	Capacity                 CloudCapacity    `json:"capacity"`
	Warnings                 []string         `json:"warnings"`
	RecentActions            []ResourceAction `json:"recent_actions"`
}

type CloudServer struct {
	ID                uuid.UUID  `json:"id"`
	CloudID           *uuid.UUID `json:"cloud_id"`
	Hostname          string     `json:"hostname"`
	Status            string     `json:"status"`
	LocationName      string     `json:"location_name"`
	ServerMode        string     `json:"server_mode"`
	ModeStatus        string     `json:"mode_status"`
	PlatformManaged   bool       `json:"platform_managed"`
	ReservedCPUCores  *int32     `json:"reserved_cpu_cores"`
	ReservedMemoryMB  *int32     `json:"reserved_memory_mb"`
	ReservedStorageGB *int32     `json:"reserved_storage_gb"`
	TotalCPUCores     *int32     `json:"total_cpu_cores"`
	TotalMemoryMB     *int32     `json:"total_memory_mb"`
	TotalStorageGB    *int32     `json:"total_storage_gb"`
	VMCount           int64      `json:"vm_count"`
	ServiceCount      int64      `json:"managed_service_count"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type FleetServer struct {
	ID                  uuid.UUID  `json:"id"`
	Hostname            string     `json:"hostname"`
	Status              string     `json:"status"`
	LocationName        string     `json:"location_name"`
	ProjectID           *uuid.UUID `json:"project_id"`
	ProjectName         *string    `json:"project_name"`
	CloudID             *uuid.UUID `json:"cloud_id"`
	CloudName           *string    `json:"cloud_name"`
	ServerMode          string     `json:"server_mode"`
	ModeStatus          string     `json:"mode_status"`
	ReservedCPUCores    *int32     `json:"reserved_cpu_cores"`
	ReservedMemoryMB    *int32     `json:"reserved_memory_mb"`
	ReservedStorageGB   *int32     `json:"reserved_storage_gb"`
	HardwareProfileName *string    `json:"hardware_profile_name"`
	GPU                 *string    `json:"gpu"`
	PublicIP            *string    `json:"public_ip"`
	VMCount             int64      `json:"vm_count"`
	ServiceCount        int64      `json:"managed_service_count"`
	MonthlyCostCents    *int64     `json:"monthly_cost_cents"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type CreatePrivateNetworkParams struct {
	OrganizationID uuid.UUID
	CloudID        uuid.UUID
	Name           string
	Description    *string
	CIDR           string
	GatewayIP      *string
}

type PrivateNetwork struct {
	ID              uuid.UUID  `json:"id"`
	OrganizationID  uuid.UUID  `json:"organization_id"`
	CloudID         uuid.UUID  `json:"cloud_id"`
	Name            string     `json:"name"`
	Description     *string    `json:"description"`
	CIDR            string     `json:"cidr"`
	GatewayIP       *string    `json:"gateway_ip"`
	NetworkType     string     `json:"network_type"`
	IsolationType   string     `json:"isolation_type"`
	VLANID          *int32     `json:"vlan_id"`
	VNI             *int32     `json:"vni"`
	Status          string     `json:"status"`
	AttachmentCount int64      `json:"attachment_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at"`
}

type CreateNetworkAttachmentParams struct {
	OrganizationID   uuid.UUID
	CloudID          uuid.UUID
	PrivateNetworkID uuid.UUID
	ResourceType     string
	ResourceID       uuid.UUID
	PrivateIP        *string
	MACAddress       *string
}

type NetworkAttachment struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	CloudID          uuid.UUID `json:"cloud_id"`
	PrivateNetworkID uuid.UUID `json:"private_network_id"`
	ResourceType     string    `json:"resource_type"`
	ResourceID       uuid.UUID `json:"resource_id"`
	ResourceName     string    `json:"resource_name"`
	PrivateIP        *string   `json:"private_ip"`
	MACAddress       *string   `json:"mac_address"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreateVirtualMachineParams struct {
	OrganizationID   uuid.UUID
	CloudID          uuid.UUID
	HostServerID     *uuid.UUID
	Name             string
	Hostname         string
	CPUCores         int32
	MemoryMB         int32
	DiskGB           int32
	ImageID          *string
	OSImage          *string
	PrivateNetworkID *uuid.UUID
	PrivateIP        *string
}

type UpdateVirtualMachineParams struct {
	HostServerID *uuid.UUID
	Name         string
	Hostname     string
	CPUCores     int32
	MemoryMB     int32
	DiskGB       int32
	ImageID      *string
	OSImage      *string
	PrivateIP    *string
}

type VirtualMachine struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	CloudID        uuid.UUID  `json:"cloud_id"`
	HostServerID   *uuid.UUID `json:"host_server_id"`
	HostServerName *string    `json:"host_server_name"`
	Name           string     `json:"name"`
	Hostname       string     `json:"hostname"`
	Status         string     `json:"status"`
	PowerState     string     `json:"power_state"`
	CPUCores       int32      `json:"cpu_cores"`
	MemoryMB       int32      `json:"memory_mb"`
	DiskGB         int32      `json:"disk_gb"`
	ImageID        *string    `json:"image_id"`
	OSImage        *string    `json:"os_image"`
	PrivateIP      *string    `json:"private_ip"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at"`
}

type CreateManagedServiceParams struct {
	OrganizationID      uuid.UUID
	CloudID             uuid.UUID
	HostServerID        *uuid.UUID
	ServiceType         string
	Name                string
	CPUCores            int32
	MemoryMB            int32
	StorageGB           int32
	Version             string
	PrivateNetworkID    *uuid.UUID
	PrivateIP           *string
	BackupEnabled       bool
	BackupRetentionDays int32
	EndpointHostname    string
}

type ManagedService struct {
	ID                  uuid.UUID  `json:"id"`
	OrganizationID      uuid.UUID  `json:"organization_id"`
	CloudID             uuid.UUID  `json:"cloud_id"`
	HostServerID        *uuid.UUID `json:"host_server_id"`
	HostServerName      *string    `json:"host_server_name"`
	ServiceType         string     `json:"service_type"`
	Name                string     `json:"name"`
	Status              string     `json:"status"`
	PlanName            *string    `json:"plan_name"`
	CPUCores            int32      `json:"cpu_cores"`
	MemoryMB            int32      `json:"memory_mb"`
	StorageGB           int32      `json:"storage_gb"`
	Version             *string    `json:"version"`
	EndpointHostname    *string    `json:"endpoint_hostname"`
	PrivateIP           *string    `json:"private_ip"`
	Port                *int32     `json:"port"`
	BackupEnabled       bool       `json:"backup_enabled"`
	BackupRetentionDays int32      `json:"backup_retention_days"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DeletedAt           *time.Time `json:"deleted_at"`
}

type ResourceAction struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	CloudID        *uuid.UUID `json:"cloud_id"`
	ResourceType   string     `json:"resource_type"`
	ResourceID     *uuid.UUID `json:"resource_id"`
	ActionType     string     `json:"action_type"`
	Status         string     `json:"status"`
	Message        string     `json:"message"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type CloudCapacity struct {
	TotalCPUCores             *int64 `json:"total_cpu_cores"`
	TotalMemoryMB             *int64 `json:"total_memory_mb"`
	TotalStorageGB            *int64 `json:"total_storage_gb"`
	AllocatedVMCPUCores       int64  `json:"allocated_vm_cpu_cores"`
	AllocatedVMMemoryMB       int64  `json:"allocated_vm_memory_mb"`
	AllocatedVMDiskGB         int64  `json:"allocated_vm_disk_gb"`
	AllocatedServiceCPUCores  int64  `json:"allocated_service_cpu_cores"`
	AllocatedServiceMemoryMB  int64  `json:"allocated_service_memory_mb"`
	AllocatedServiceStorageGB int64  `json:"allocated_service_storage_gb"`
	RemainingCPUCores         *int64 `json:"remaining_cpu_cores"`
	RemainingMemoryMB         *int64 `json:"remaining_memory_mb"`
	RemainingStorageGB        *int64 `json:"remaining_storage_gb"`
	EstimateAvailable         bool   `json:"estimate_available"`
}

type PlacementOption struct {
	ServerID           uuid.UUID `json:"server_id"`
	Hostname           string    `json:"hostname"`
	ServerMode         string    `json:"server_mode"`
	ModeStatus         string    `json:"mode_status"`
	LocationName       string    `json:"location_name"`
	RemainingCPUCores  *int64    `json:"remaining_cpu_cores"`
	RemainingMemoryMB  *int64    `json:"remaining_memory_mb"`
	RemainingStorageGB *int64    `json:"remaining_storage_gb"`
	Warnings           []string  `json:"warnings"`
}

type PlatformSession struct {
	Roles       []string
	Permissions []string
}

type AdminUserListItem struct {
	ID                  uuid.UUID  `json:"id"`
	Email               string     `json:"email"`
	Name                string     `json:"name"`
	AuthProviderSubject string     `json:"auth_provider_subject"`
	PlatformRoles       []string   `json:"platform_roles"`
	OrganizationCount   int64      `json:"organization_count"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
	LastLoginAt         *time.Time `json:"last_login_at"`
}

type AdminOrganizationListItem struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	Slug              string    `json:"slug"`
	Status            string    `json:"status"`
	BillingStatus     string    `json:"billing_status"`
	BillingEmail      string    `json:"billing_email"`
	MemberCount       int64     `json:"member_count"`
	ActiveServerCount int64     `json:"active_server_count"`
	CreatedAt         time.Time `json:"created_at"`
}

type AdminBillingAccountListItem struct {
	ID                 uuid.UUID `json:"id"`
	OrganizationName   string    `json:"organization_name"`
	BillingEmail       string    `json:"billing_email"`
	Status             string    `json:"status"`
	PaymentTerms       string    `json:"payment_terms"`
	CreditBalanceCents int64     `json:"credit_balance_cents"`
	StripeCustomerID   *string   `json:"stripe_customer_id"`
}

type AdminServerListItem struct {
	ID                  uuid.UUID `json:"id"`
	Hostname            string    `json:"hostname"`
	AssetTag            string    `json:"asset_tag"`
	SerialNumber        string    `json:"serial_number"`
	Status              string    `json:"status"`
	RackID              string    `json:"rack_id"`
	RackName            string    `json:"rack_name"`
	LocationName        string    `json:"location_name"`
	OrganizationName    *string   `json:"organization_name"`
	ProjectName         *string   `json:"project_name"`
	HardwareProfileName string    `json:"hardware_profile_name"`
	PublicIP            *string   `json:"public_ip"`
	BMCAddress          string    `json:"bmc_address"`
	PrimaryMACAddress   string    `json:"primary_mac_address"`
	Provisionable       bool      `json:"provisionable"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type AdminRackListItem struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Location         string     `json:"location"`
	LocationCode     string     `json:"location_code"`
	Status           string     `json:"status"`
	LastHeartbeatAt  *time.Time `json:"last_heartbeat_at"`
	AgentVersion     string     `json:"agent_version"`
	AvailableServers int64      `json:"available_servers"`
	ActiveServers    int64      `json:"active_servers"`
	FailedJobs       int64      `json:"failed_jobs"`
}

type AdminLocationListItem struct {
	ID      uuid.UUID `json:"id"`
	Code    string    `json:"code"`
	Keyword string    `json:"keyword"`
	Name    string    `json:"name"`
	City    string    `json:"city"`
	Region  string    `json:"region"`
	Country string    `json:"country"`
}

type AdminOSImageListItem struct {
	ID                    uuid.UUID      `json:"id"`
	Name                  string         `json:"name"`
	Slug                  string         `json:"slug"`
	Version               string         `json:"version"`
	Family                string         `json:"family"`
	Architecture          string         `json:"architecture"`
	Enabled               bool           `json:"enabled"`
	IsDefault             bool           `json:"is_default"`
	TinkerbellTemplateRef string         `json:"tinkerbell_template_ref"`
	ArtifactName          string         `json:"artifact_name"`
	ArtifactFile          string         `json:"artifact_file"`
	Metadata              map[string]any `json:"metadata"`
}

type AdminSwitchListItem struct {
	ID               uuid.UUID `json:"id"`
	Label            string    `json:"label"`
	IPAddress        string    `json:"ip_address"`
	ManagementIP     *string   `json:"management_ip"`
	LocationID       uuid.UUID `json:"location_id"`
	LocationName     string    `json:"location_name"`
	RackID           *string   `json:"rack_id"`
	RackName         *string   `json:"rack_name"`
	Vendor           string    `json:"vendor"`
	Model            string    `json:"model"`
	SerialNumber     string    `json:"serial_number"`
	PortCount        int32     `json:"port_count"`
	DefaultPortSpeed string    `json:"default_port_speed"`
	Status           string    `json:"status"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type AdminEdgeRouterListItem struct {
	ID           uuid.UUID `json:"id"`
	Label        string    `json:"label"`
	IPAddress    string    `json:"ip_address"`
	ManagementIP *string   `json:"management_ip"`
	LocationID   uuid.UUID `json:"location_id"`
	LocationName string    `json:"location_name"`
	Vendor       string    `json:"vendor"`
	Model        string    `json:"model"`
	SerialNumber string    `json:"serial_number"`
	ASN          int32     `json:"asn"`
	UpstreamISPs []string  `json:"upstream_isps"`
	PortCount    int32     `json:"port_count"`
	PortSpeed    string    `json:"port_speed"`
	Status       string    `json:"status"`
	Notes        string    `json:"notes"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AdminServerNetworkInterfaceListItem struct {
	ID           uuid.UUID  `json:"id"`
	ServerID     uuid.UUID  `json:"server_id"`
	ServerName   string     `json:"server_name"`
	SwitchID     *uuid.UUID `json:"switch_id"`
	SwitchLabel  *string    `json:"switch_label"`
	LocationName string     `json:"location_name"`
	Label        string     `json:"label"`
	MACAddress   string     `json:"mac_address"`
	IPAddress    *string    `json:"ip_address"`
	Gateway      *string    `json:"gateway"`
	SubnetMask   *string    `json:"subnet_mask"`
	SwitchPort   string     `json:"switch_port"`
	VLANID       *int32     `json:"vlan_id"`
	IsPrimary    bool       `json:"is_primary"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type AdminCPUProfileListItem struct {
	ID            uuid.UUID      `json:"id"`
	Name          string         `json:"name"`
	Vendor        string         `json:"vendor"`
	Model         string         `json:"model"`
	SocketCount   int32          `json:"socket_count"`
	CoreCount     int32          `json:"core_count"`
	ThreadCount   int32          `json:"thread_count"`
	BaseClockGHz  *float64       `json:"base_clock_ghz"`
	BoostClockGHz *float64       `json:"boost_clock_ghz"`
	Architecture  string         `json:"architecture"`
	Metadata      map[string]any `json:"metadata"`
}

type AdminProvisioningJobListItem struct {
	ID            uuid.UUID  `json:"id"`
	Server        string     `json:"server"`
	Organization  string     `json:"organization"`
	Rack          string     `json:"rack"`
	Image         string     `json:"image"`
	Status        string     `json:"status"`
	RequestedBy   string     `json:"requested_by"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	FailureReason *string    `json:"failure_reason"`
}

type ServerCatalog struct {
	Families []ServerCatalogPlan `json:"server_families"`
}

type ServerCatalogPlan struct {
	ID                  string                        `json:"id"`
	Name                string                        `json:"display_name"`
	Slug                string                        `json:"slug"`
	CPUManufacturer     string                        `json:"cpu_manufacturer"`
	CPUModel            string                        `json:"cpu_model"`
	CoreCount           int32                         `json:"core_count"`
	ThreadCount         int32                         `json:"thread_count"`
	BaseClockGHz        *float64                      `json:"base_clock_ghz,omitempty"`
	BoostClockGHz       *float64                      `json:"boost_clock_ghz,omitempty"`
	Generation          string                        `json:"generation"`
	Category            string                        `json:"category"`
	Description         string                        `json:"description"`
	FeatureBadges       []string                      `json:"feature_badges"`
	StartingPriceCents  int64                         `json:"starting_price_cents"`
	HourlyPriceCents    int64                         `json:"hourly_price_cents"`
	MonthlyPriceCents   int64                         `json:"monthly_price_cents"`
	QuarterlyPriceCents int64                         `json:"quarterly_price_cents"`
	YearlyPriceCents    int64                         `json:"yearly_price_cents"`
	AvailableCount      int64                         `json:"available_count"`
	Locations           []ServerCatalogLocation       `json:"locations"`
	RAMOptionsGB        []int32                       `json:"ram_options_gb"`
	DiskOptions         []ServerCatalogDiskOption     `json:"disk_options"`
	HardwareOptions     []ServerCatalogHardwareOption `json:"hardware_options"`
	Configurations      []ServerCatalogConfiguration  `json:"configurations"`
}

type ServerCatalogLocation struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type ServerCatalogDiskOption struct {
	Label     string `json:"label"`
	StorageGB int32  `json:"storage_gb"`
}

type ServerCatalogHardwareOption struct {
	ID                     uuid.UUID  `json:"id"`
	OptionType             string     `json:"option_type"`
	Label                  string     `json:"label"`
	Description            string     `json:"description"`
	Unit                   string     `json:"unit"`
	ValueText              string     `json:"value_text"`
	ValueGB                *int32     `json:"value_gb,omitempty"`
	PriceDeltaCents        int64      `json:"price_delta_cents"`
	HourlyPriceDeltaCents  int64      `json:"hourly_price_delta_cents"`
	QuarterlyDeltaCents    int64      `json:"quarterly_price_delta_cents"`
	YearlyDeltaCents       int64      `json:"yearly_price_delta_cents"`
	Currency               string     `json:"currency"`
	QuantityAvailable      int32      `json:"quantity_available"`
	FulfillmentMode        string     `json:"fulfillment_mode"`
	EstimatedReadyMinHours int32      `json:"estimated_ready_min_hours"`
	EstimatedReadyMaxHours int32      `json:"estimated_ready_max_hours"`
	LocationID             *uuid.UUID `json:"location_id,omitempty"`
	LocationCode           *string    `json:"location_code,omitempty"`
	LocationName           *string    `json:"location_name,omitempty"`
	HardwareProfileName    string     `json:"hardware_profile_name,omitempty"`
	CPUModel               string     `json:"cpu_model,omitempty"`
	RequiresInstall        bool       `json:"requires_install"`
	Active                 bool       `json:"active"`
	CreatedAt              time.Time  `json:"created_at,omitempty"`
	UpdatedAt              time.Time  `json:"updated_at,omitempty"`
}

type ServerCatalogConfiguration struct {
	ID                     string                        `json:"id"`
	ServerFamilyID         string                        `json:"server_family_id"`
	LocationCode           string                        `json:"location_code"`
	LocationName           string                        `json:"location_name"`
	HardwareProfileName    string                        `json:"hardware_profile_name"`
	CPUModel               string                        `json:"cpu_model"`
	CoreCount              int32                         `json:"core_count"`
	RAMGB                  int32                         `json:"ram_gb"`
	DiskName               string                        `json:"disk_name"`
	DiskDescription        string                        `json:"disk_description"`
	DiskCount              int32                         `json:"disk_count"`
	DiskSizeGB             int32                         `json:"disk_size_gb"`
	DiskType               string                        `json:"disk_type"`
	StorageGB              int32                         `json:"storage_gb"`
	NetworkCapacity        string                        `json:"network_capacity"`
	HourlyPriceCents       int64                         `json:"hourly_price_cents"`
	MonthlyPriceCents      int64                         `json:"monthly_price_cents"`
	QuarterlyPriceCents    int64                         `json:"quarterly_price_cents"`
	YearlyPriceCents       int64                         `json:"yearly_price_cents"`
	Available              bool                          `json:"available"`
	AvailableQuantity      int32                         `json:"available_quantity"`
	EstimatedReadyMinHours int32                         `json:"estimated_ready_min_hours"`
	EstimatedReadyMaxHours int32                         `json:"estimated_ready_max_hours"`
	HardwareOptions        []ServerCatalogHardwareOption `json:"hardware_options"`
	PhysicalServerIDs      []uuid.UUID                   `json:"-"`
	FamilyDisplayName      string                        `json:"-"`
	FamilySlug             string                        `json:"-"`
	CPUManufacturer        string                        `json:"-"`
	ThreadCount            int32                         `json:"-"`
	BaseClockGHz           *float64                      `json:"-"`
	BoostClockGHz          *float64                      `json:"-"`
	Generation             string                        `json:"-"`
	WorkloadCategory       string                        `json:"-"`
	FamilyDescription      string                        `json:"-"`
	FeatureBadges          []string                      `json:"-"`
	DisplayOrder           int32                         `json:"-"`
}

type AllocateServerParams struct {
	OrganizationID    uuid.UUID
	ProjectID         *uuid.UUID
	ServerFamilyID    uuid.UUID
	ConfigurationID   string
	CreatedByUserID   uuid.UUID
	BillingInterval   domain.BillingInterval
	HardwareOptionIDs []uuid.UUID
}

type AllocateServerResult struct {
	Server               FleetServer  `json:"server"`
	Order                domain.Order `json:"order"`
	CheckoutURL          string       `json:"checkout_url,omitempty"`
	BillableServiceID    *uuid.UUID   `json:"billable_service_id,omitempty"`
	ReservationExpiresAt *time.Time   `json:"reservation_expires_at,omitempty"`
}

type CreateHardwareOptionParams struct {
	OptionType             string
	Label                  string
	Description            string
	Unit                   string
	ValueText              string
	ValueGB                *int32
	PriceDeltaCents        int64
	HourlyPriceDeltaCents  int64
	QuarterlyDeltaCents    int64
	YearlyDeltaCents       int64
	Currency               string
	QuantityAvailable      int32
	FulfillmentMode        string
	EstimatedReadyMinHours int32
	EstimatedReadyMaxHours int32
	LocationID             *uuid.UUID
	HardwareProfileName    string
	CPUModel               string
	Active                 bool
}

type HardwareFulfillmentOrder struct {
	OrderID                uuid.UUID               `json:"order_id"`
	OrganizationID         uuid.UUID               `json:"organization_id"`
	OrganizationName       string                  `json:"organization_name"`
	ServerID               *uuid.UUID              `json:"server_id,omitempty"`
	ServerHostname         string                  `json:"server_hostname"`
	Status                 domain.OrderStatus      `json:"status"`
	TotalCents             int64                   `json:"total_cents"`
	RequiresModification   bool                    `json:"requires_modification"`
	FulfillmentReady       bool                    `json:"fulfillment_ready"`
	EstimatedReadyMinHours int32                   `json:"estimated_ready_min_hours"`
	EstimatedReadyMaxHours int32                   `json:"estimated_ready_max_hours"`
	HardwareOptions        []PendingHardwareOption `json:"hardware_options"`
	CreatedAt              time.Time               `json:"created_at"`
	UpdatedAt              time.Time               `json:"updated_at"`
}

type CreateAdminServerParams struct {
	LocationID          uuid.UUID
	RackID              string
	Hostname            string
	AssetTag            string
	SerialNumber        string
	HardwareProfileName string
	CPUProfileID        *uuid.UUID
	CPUModel            string
	CPUCount            int32
	CoreCount           int32
	RAMGB               int32
	DiskName            string
	DiskDescription     string
	NICDescription      string
	PublicIP            *string
	Gateway             *string
	SubnetMask          *string
	VLANID              *int32
	MACAddress          string
	IPAddress           *string
	BMCAddress          string
	IPMIUsername        string
	IPMIPassword        string
	HourlyPriceCents    int64
	MonthlyPriceCents   int64
	QuarterlyPriceCents int64
	YearlyPriceCents    int64
	Provisionable       bool
	Notes               string
}

type ProvisioningServerInventory struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	ProjectID      *uuid.UUID     `json:"project_id"`
	RackID         string         `json:"rack_id"`
	RackLocation   string         `json:"rack_location"`
	Status         string         `json:"status"`
	BMCAddress     string         `json:"bmc_address"`
	MACAddress     string         `json:"mac_address"`
	IPAddress      *string        `json:"ip_address"`
	Gateway        *string        `json:"gateway"`
	SubnetMask     *string        `json:"subnet_mask"`
	DiskName       string         `json:"disk_name"`
	Hostname       string         `json:"hostname"`
	Metadata       map[string]any `json:"metadata"`
}

type UpsertHypervisorSnapshotParams struct {
	ID                  string
	Hostname            string
	Status              string
	VCPUsTotal          int32
	VCPUsActive         int32
	MemoryTotalBytes    int64
	MemoryActiveBytes   int64
	DiskTotalBytes      int64
	DiskAvailableBytes  int64
	WireguardInterface  string
	ControlPlaneAddress string
	LastReportedAt      time.Time
	VMs                 []UpsertHypervisorVMParams
}

type UpsertHypervisorVMParams struct {
	ID             string
	Name           string
	Status         string
	VCPUs          int32
	MemoryBytes    int64
	DiskBytes      int64
	MACAddresses   []string
	IPAddresses    []string
	Metadata       map[string]string
	LastReportedAt time.Time
}

type AdminHypervisorListItem struct {
	ID                  string     `json:"id"`
	ServerID            *uuid.UUID `json:"server_id"`
	ServerHostname      *string    `json:"server_hostname"`
	Hostname            string     `json:"hostname"`
	Status              string     `json:"status"`
	VCPUsTotal          int32      `json:"vcpus_total"`
	VCPUsActive         int32      `json:"vcpus_active"`
	MemoryTotalBytes    int64      `json:"memory_total_bytes"`
	MemoryActiveBytes   int64      `json:"memory_active_bytes"`
	DiskTotalBytes      int64      `json:"disk_total_bytes"`
	DiskAvailableBytes  int64      `json:"disk_available_bytes"`
	WireguardInterface  string     `json:"wireguard_interface"`
	ControlPlaneAddress string     `json:"control_plane_address"`
	VMCount             int64      `json:"vm_count"`
	RunningVMCount      int64      `json:"running_vm_count"`
	LastReportedAt      *time.Time `json:"last_reported_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type AdminHypervisorVMListItem struct {
	ID             string            `json:"id"`
	HypervisorID   string            `json:"hypervisor_id"`
	Name           string            `json:"name"`
	Status         string            `json:"status"`
	VCPUs          int32             `json:"vcpus"`
	MemoryBytes    int64             `json:"memory_bytes"`
	DiskBytes      int64             `json:"disk_bytes"`
	MACAddresses   []string          `json:"mac_addresses"`
	IPAddresses    []string          `json:"ip_addresses"`
	Metadata       map[string]string `json:"metadata"`
	LastReportedAt *time.Time        `json:"last_reported_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type UpsertHypervisorCommandParams struct {
	HypervisorID string
	CommandID    string
	CommandType  string
	Payload      []byte
}

type CompleteHypervisorCommandParams struct {
	CommandID    string
	Status       string
	Result       []byte
	ErrorMessage string
}

type WireGuardGateway struct {
	ID                     string            `json:"id"`
	Name                   string            `json:"name"`
	InterfaceName          string            `json:"interface_name"`
	PublicKey              string            `json:"public_key"`
	Endpoint               string            `json:"endpoint"`
	TunnelAddress          string            `json:"tunnel_address"`
	ManagementCIDR         string            `json:"management_cidr"`
	ControlPlaneAllowedIPs []string          `json:"control_plane_allowed_ips"`
	NodeName               string            `json:"node_name"`
	Status                 string            `json:"status"`
	Metadata               map[string]string `json:"metadata"`
}

type WireGuardPeerDesiredState struct {
	ID                    uuid.UUID         `json:"id"`
	HypervisorID          string            `json:"hypervisor_id"`
	GatewayID             string            `json:"gateway_id"`
	WireGuardPublicKey    string            `json:"wireguard_public_key"`
	WireGuardManagementIP string            `json:"wireguard_management_ip"`
	AllowedIPs            []string          `json:"allowed_ips"`
	Endpoint              *string           `json:"endpoint"`
	DesiredState          string            `json:"desired_state"`
	ActualState           string            `json:"actual_state"`
	LastHandshakeAt       *time.Time        `json:"last_handshake_at"`
	Metadata              map[string]string `json:"metadata"`
}

type UpdateWireGuardPeerActualStateParams struct {
	ID               uuid.UUID
	ActualState      string
	LastHandshakeAt  *time.Time
	ErrorMessage     string
	MarkRevoked      bool
	LastReconciledAt time.Time
}

type AdminAuditEventListItem struct {
	ID           uuid.UUID         `json:"id"`
	Actor        string            `json:"actor"`
	Action       string            `json:"action"`
	Target       string            `json:"target"`
	Organization *uuid.UUID        `json:"organization_id"`
	Server       *uuid.UUID        `json:"server_id"`
	CreatedAt    time.Time         `json:"created_at"`
	Metadata     map[string]string `json:"metadata"`
}

type Repository interface {
	BillingRepository

	UpsertUser(ctx context.Context, identity AuthIdentity) (domain.User, error)
	GetUserByAuth0Sub(ctx context.Context, auth0Sub string) (domain.User, error)

	CreateOrganization(ctx context.Context, params CreateOrganizationParams) (domain.Organization, error)
	ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]domain.Organization, error)
	GetOrganizationForUser(ctx context.Context, organizationID, userID uuid.UUID) (domain.Organization, error)
	GetOrganizationByID(ctx context.Context, organizationID uuid.UUID) (domain.Organization, error)
	UpdateOrganization(ctx context.Context, organizationID uuid.UUID, name, slug string) (domain.Organization, error)
	DeleteOrganization(ctx context.Context, organizationID uuid.UUID) error
	CountActiveBillableResources(ctx context.Context, organizationID uuid.UUID) (int64, error)

	ListClouds(ctx context.Context, organizationID uuid.UUID) ([]Cloud, error)
	CreateCloud(ctx context.Context, params CreateCloudParams) (Cloud, error)
	GetCloud(ctx context.Context, organizationID, cloudID uuid.UUID) (Cloud, error)
	UpdateCloud(ctx context.Context, organizationID, cloudID uuid.UUID, params UpdateCloudParams) (Cloud, error)
	DeleteCloud(ctx context.Context, organizationID, cloudID uuid.UUID) error
	GetCloudOverview(ctx context.Context, organizationID, cloudID uuid.UUID) (CloudOverview, error)
	ListOrganizationServers(ctx context.Context, organizationID uuid.UUID) ([]FleetServer, error)
	ListServerCatalog(ctx context.Context) (ServerCatalog, error)
	AllocateServer(ctx context.Context, params AllocateServerParams) (AllocateServerResult, error)
	ListCloudServers(ctx context.Context, organizationID, cloudID uuid.UUID) ([]CloudServer, error)
	AssignServerToCloud(ctx context.Context, organizationID, cloudID, serverID uuid.UUID) (CloudServer, error)
	UnassignServerFromCloud(ctx context.Context, organizationID, cloudID, serverID uuid.UUID) (CloudServer, error)
	ChangeServerMode(ctx context.Context, organizationID, serverID uuid.UUID, mode string) (CloudServer, error)
	ListPrivateNetworks(ctx context.Context, organizationID, cloudID uuid.UUID) ([]PrivateNetwork, error)
	CreatePrivateNetwork(ctx context.Context, params CreatePrivateNetworkParams) (PrivateNetwork, error)
	GetPrivateNetwork(ctx context.Context, organizationID, cloudID, networkID uuid.UUID) (PrivateNetwork, error)
	DeletePrivateNetwork(ctx context.Context, organizationID, cloudID, networkID uuid.UUID) error
	ListNetworkAttachments(ctx context.Context, organizationID, cloudID, networkID uuid.UUID) ([]NetworkAttachment, error)
	CreateNetworkAttachment(ctx context.Context, params CreateNetworkAttachmentParams) (NetworkAttachment, error)
	DetachNetworkAttachment(ctx context.Context, organizationID, cloudID, networkID, attachmentID uuid.UUID) error
	ListVirtualMachines(ctx context.Context, organizationID, cloudID uuid.UUID) ([]VirtualMachine, error)
	CreateVirtualMachine(ctx context.Context, params CreateVirtualMachineParams) (VirtualMachine, error)
	GetVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID) (VirtualMachine, error)
	UpdateVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID, params UpdateVirtualMachineParams) (VirtualMachine, error)
	DeleteVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID) error
	PowerVirtualMachine(ctx context.Context, organizationID, cloudID, vmID uuid.UUID, action string) (VirtualMachine, error)
	ListManagedServices(ctx context.Context, organizationID, cloudID uuid.UUID) ([]ManagedService, error)
	CreateManagedService(ctx context.Context, params CreateManagedServiceParams) (ManagedService, error)
	GetManagedService(ctx context.Context, organizationID, cloudID, serviceID uuid.UUID) (ManagedService, error)
	DeleteManagedService(ctx context.Context, organizationID, cloudID, serviceID uuid.UUID) error
	ActOnManagedService(ctx context.Context, organizationID, cloudID, serviceID uuid.UUID, action string) (ManagedService, error)
	GetCloudCapacity(ctx context.Context, organizationID, cloudID uuid.UUID) (CloudCapacity, error)
	ListPlacementOptions(ctx context.Context, organizationID, cloudID uuid.UUID, resourceType string) ([]PlacementOption, error)
	ListResourceActions(ctx context.Context, organizationID uuid.UUID, cloudID *uuid.UUID) ([]ResourceAction, error)
	AddResourceAction(ctx context.Context, organizationID uuid.UUID, cloudID *uuid.UUID, resourceType string, resourceID *uuid.UUID, actionType, status, message string) (ResourceAction, error)

	GetSystemRoleByName(ctx context.Context, name string) (domain.Role, error)
	GetRole(ctx context.Context, roleID uuid.UUID) (domain.Role, error)
	ListRoles(ctx context.Context, organizationID uuid.UUID) ([]domain.Role, error)
	HasPermission(ctx context.Context, userID, organizationID uuid.UUID, permission string) (bool, error)
	HasPlatformPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error)
	GetPlatformSession(ctx context.Context, userID uuid.UUID) (PlatformSession, error)
	AddAdminAuditLog(ctx context.Context, actorUserID uuid.UUID, action, targetType, targetID, reason string, metadata []byte) error
	ListAdminUsers(ctx context.Context) ([]AdminUserListItem, error)
	ListAdminUserOrganizations(ctx context.Context, userID uuid.UUID) ([]AdminOrganizationListItem, error)
	ListAdminOrganizations(ctx context.Context) ([]AdminOrganizationListItem, error)
	ListAdminBillingAccounts(ctx context.Context) ([]AdminBillingAccountListItem, error)
	ListAdminServers(ctx context.Context) ([]AdminServerListItem, error)
	CreateAdminServer(ctx context.Context, params CreateAdminServerParams) (AdminServerListItem, error)
	ListHardwareOptions(ctx context.Context) ([]ServerCatalogHardwareOption, error)
	CreateHardwareOption(ctx context.Context, params CreateHardwareOptionParams) (ServerCatalogHardwareOption, error)
	ListHardwareFulfillmentOrders(ctx context.Context) ([]HardwareFulfillmentOrder, error)
	MarkHardwareFulfillmentReady(ctx context.Context, orderID uuid.UUID) (HardwareFulfillmentOrder, error)
	ListAdminRacks(ctx context.Context) ([]AdminRackListItem, error)
	ListAdminLocations(ctx context.Context) ([]AdminLocationListItem, error)
	ListAdminCPUProfiles(ctx context.Context) ([]AdminCPUProfileListItem, error)
	ListAdminOSImages(ctx context.Context) ([]AdminOSImageListItem, error)
	ListAdminSwitches(ctx context.Context) ([]AdminSwitchListItem, error)
	ListAdminEdgeRouters(ctx context.Context) ([]AdminEdgeRouterListItem, error)
	ListAdminServerNetworkInterfaces(ctx context.Context) ([]AdminServerNetworkInterfaceListItem, error)
	UpsertHypervisorSnapshot(ctx context.Context, params UpsertHypervisorSnapshotParams) error
	UpsertHypervisorCommand(ctx context.Context, params UpsertHypervisorCommandParams) error
	MarkHypervisorCommandSent(ctx context.Context, commandID string) error
	CompleteHypervisorCommand(ctx context.Context, params CompleteHypervisorCommandParams) error
	GetWireGuardGateway(ctx context.Context, gatewayID string) (WireGuardGateway, error)
	ListWireGuardPeersForGateway(ctx context.Context, gatewayID string) ([]WireGuardPeerDesiredState, error)
	UpdateWireGuardPeerActualState(ctx context.Context, params UpdateWireGuardPeerActualStateParams) error
	ListAdminHypervisors(ctx context.Context) ([]AdminHypervisorListItem, error)
	ListAdminHypervisorVMs(ctx context.Context, hypervisorID string) ([]AdminHypervisorVMListItem, error)
	ListAdminCloudResources(ctx context.Context) ([]Cloud, []VirtualMachine, []ManagedService, []PrivateNetwork, []ResourceAction, error)
	ListAdminProvisioningJobs(ctx context.Context) ([]AdminProvisioningJobListItem, error)
	ListAdminAuditEvents(ctx context.Context) ([]AdminAuditEventListItem, error)
	GetProvisioningServerInventory(ctx context.Context, organizationID, serverID uuid.UUID) (ProvisioningServerInventory, error)

	ListMembers(ctx context.Context, organizationID uuid.UUID) ([]domain.Member, error)
	UpdateMemberRole(ctx context.Context, organizationID, targetUserID, roleID uuid.UUID) (domain.Member, error)
	RemoveMember(ctx context.Context, organizationID, targetUserID uuid.UUID) error
	CountActiveOwners(ctx context.Context, organizationID uuid.UUID) (int64, error)
	IsOwnerRole(ctx context.Context, roleID uuid.UUID) (bool, error)
	IsActiveMemberEmail(ctx context.Context, organizationID uuid.UUID, email string) (bool, error)

	CreateInvitation(ctx context.Context, params CreateInvitationParams) (domain.Invitation, error)
	ListInvitations(ctx context.Context, organizationID uuid.UUID) ([]domain.Invitation, error)
	GetInvitationByToken(ctx context.Context, token string) (domain.Invitation, error)
	AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (domain.Invitation, error)
	RevokeInvitation(ctx context.Context, organizationID, invitationID uuid.UUID) (domain.Invitation, error)

	CreateProject(ctx context.Context, organizationID uuid.UUID, name, slug string) (domain.Project, error)
	ListProjects(ctx context.Context, organizationID uuid.UUID) ([]domain.Project, error)
	GetProject(ctx context.Context, organizationID, projectID uuid.UUID) (domain.Project, error)
	UpdateProject(ctx context.Context, organizationID, projectID uuid.UUID, name, slug string) (domain.Project, error)
	DeleteProject(ctx context.Context, organizationID, projectID uuid.UUID) error

	GetBillingProfile(ctx context.Context, organizationID uuid.UUID) (domain.BillingProfile, error)
	UpdateBillingProfile(ctx context.Context, organizationID uuid.UUID, params UpdateBillingProfileParams) (domain.BillingProfile, error)
	SetBillingAccountStripeCustomerID(ctx context.Context, organizationID uuid.UUID, stripeCustomerID string) (domain.BillingAccount, error)
	ListPaymentMethods(ctx context.Context, organizationID uuid.UUID) ([]domain.PaymentMethod, error)
	CreatePaymentMethod(ctx context.Context, method domain.PaymentMethod) (domain.PaymentMethod, error)
	DeletePaymentMethod(ctx context.Context, organizationID, paymentMethodID uuid.UUID) error
	ListInvoices(ctx context.Context, organizationID uuid.UUID) ([]domain.Invoice, error)

	AddAuditLog(ctx context.Context, organizationID uuid.UUID, actorUserID *uuid.UUID, action, entityType string, entityID *uuid.UUID, metadata []byte) error
	ListAuditLog(ctx context.Context, organizationID uuid.UUID) ([]domain.AuditLogEntry, error)

	ResourceBelongsToOrganization(ctx context.Context, resourceType string, resourceID, organizationID uuid.UUID) (bool, error)
}
