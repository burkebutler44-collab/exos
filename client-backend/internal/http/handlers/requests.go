package handlers

type createOrganizationRequest struct {
	Name string `json:"name" binding:"required"`
}

type updateOrganizationRequest struct {
	Name string `json:"name" binding:"required"`
}

type inviteMemberRequest struct {
	Email  string `json:"email" binding:"required,email"`
	RoleID string `json:"role_id" binding:"required"`
}

type updateMemberRoleRequest struct {
	RoleID string `json:"role_id" binding:"required"`
}

type createProjectRequest struct {
	Name string `json:"name" binding:"required"`
}

type updateProjectRequest struct {
	Name string `json:"name" binding:"required"`
}

type updateBillingRequest struct {
	BillingEmail string  `json:"billing_email" binding:"required,email"`
	CompanyName  string  `json:"company_name" binding:"required"`
	TaxID        *string `json:"tax_id"`
	Line1        *string `json:"line1"`
	Line2        *string `json:"line2"`
	City         *string `json:"city"`
	State        *string `json:"state"`
	PostalCode   *string `json:"postal_code"`
	Country      *string `json:"country"`
}

type createPaymentMethodRequest struct {
	StripePaymentMethodID string `json:"stripe_payment_method_id" binding:"required"`
	Brand                 string `json:"brand" binding:"required"`
	Last4                 string `json:"last4" binding:"required,len=4"`
	ExpMonth              int32  `json:"exp_month" binding:"required,min=1,max=12"`
	ExpYear               int32  `json:"exp_year" binding:"required"`
	IsDefault             bool   `json:"is_default"`
}

type confirmPaymentMethodSetupRequest struct {
	SetupIntentID string `json:"setup_intent_id" binding:"required"`
}

type updateBillingAccountRequest struct {
	BillingEmail               string `json:"billing_email" binding:"required,email"`
	PaymentTerms               string `json:"payment_terms"`
	AutoRechargeEnabled        bool   `json:"auto_recharge_enabled"`
	AutoRechargeThresholdCents *int64 `json:"auto_recharge_threshold_cents"`
	AutoRechargeAmountCents    *int64 `json:"auto_recharge_amount_cents"`
}

type createOrderRequest struct {
	OrderType      string  `json:"order_type" binding:"required"`
	SubtotalCents  int64   `json:"subtotal_cents"`
	TaxCents       int64   `json:"tax_cents"`
	TotalCents     int64   `json:"total_cents" binding:"required"`
	ServiceType    string  `json:"service_type"`
	ServiceID      *string `json:"service_id"`
	ProjectID      *string `json:"project_id"`
	BillingMode    string  `json:"billing_mode"`
	Description    string  `json:"description"`
	Unit           string  `json:"unit"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	Quantity       string  `json:"quantity"`
	Currency       string  `json:"currency"`
}

type updateBillableServiceRequest struct {
	Status string `json:"status" binding:"required"`
}

type createCreditCheckoutRequest struct {
	AmountCents int64 `json:"amount_cents" binding:"required"`
}

type manualCreditAdjustmentRequest struct {
	AmountCents int64  `json:"amount_cents" binding:"required"`
	Description string `json:"description" binding:"required"`
}

type generateInvoiceRequest struct {
	ServiceID *string `json:"service_id"`
}

type finalizeInvoiceRequest struct {
	StripeInvoiceID *string `json:"stripe_invoice_id"`
}

type provisionServerRequest struct {
	Location         string         `json:"location"`
	RackID           string         `json:"rack_id"`
	ProjectID        *string        `json:"project_id"`
	ImageID          string         `json:"image_id" binding:"required"`
	Hostname         string         `json:"hostname"`
	SSHKeys          []string       `json:"ssh_keys"`
	NetworkConfig    map[string]any `json:"network_config"`
	HardwareMetadata map[string]any `json:"hardware_metadata"`
}

type powerServerRequest struct {
	Location string `json:"location"`
	RackID   string `json:"rack_id"`
	Action   string `json:"action" binding:"required"`
}

type createCloudRequest struct {
	Name                 string  `json:"name" binding:"required"`
	LocationID           *string `json:"location_id"`
	Description          *string `json:"description"`
	CreateDefaultNetwork bool    `json:"create_default_network"`
	DefaultCIDR          string  `json:"default_cidr"`
}

type allocateServerRequest struct {
	ServerFamilyID    string   `json:"server_family_id" binding:"required"`
	ConfigurationID   string   `json:"configuration_id" binding:"required"`
	ProjectID         *string  `json:"project_id"`
	BillingInterval   string   `json:"billing_interval"`
	HardwareOptionIDs []string `json:"hardware_option_ids"`
}

type updateCloudRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
}

type changeServerModeRequest struct {
	Mode string `json:"mode" binding:"required"`
}

type createPrivateNetworkRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	CIDR        string  `json:"cidr" binding:"required"`
	GatewayIP   *string `json:"gateway_ip"`
}

type createNetworkAttachmentRequest struct {
	ResourceType string  `json:"resource_type" binding:"required"`
	ResourceID   string  `json:"resource_id" binding:"required"`
	PrivateIP    *string `json:"private_ip"`
	MACAddress   *string `json:"mac_address"`
}

type createVirtualMachineRequest struct {
	Name             string  `json:"name" binding:"required"`
	Hostname         string  `json:"hostname"`
	HostServerID     *string `json:"host_server_id"`
	CPUCores         int32   `json:"cpu_cores" binding:"required"`
	MemoryMB         int32   `json:"memory_mb" binding:"required"`
	DiskGB           int32   `json:"disk_gb" binding:"required"`
	ImageID          *string `json:"image_id"`
	OSImage          *string `json:"os_image"`
	PrivateNetworkID *string `json:"private_network_id"`
	PrivateIP        *string `json:"private_ip"`
}

type createAdminServerRequest struct {
	LocationID          string  `json:"location_id" binding:"required"`
	RackID              string  `json:"rack_id"`
	Hostname            string  `json:"hostname" binding:"required"`
	AssetTag            string  `json:"asset_tag"`
	SerialNumber        string  `json:"serial_number"`
	HardwareProfileName string  `json:"hardware_profile_name"`
	CPUProfileID        *string `json:"cpu_profile_id"`
	CPUModel            string  `json:"cpu_model"`
	CPUCount            int32   `json:"cpu_count"`
	CoreCount           int32   `json:"core_count"`
	RAMGB               int32   `json:"ram_gb"`
	DiskName            string  `json:"disk_name"`
	DiskDescription     string  `json:"disk_description"`
	NICDescription      string  `json:"nic_description"`
	PublicIP            *string `json:"public_ip"`
	IPAddress           *string `json:"ip_address"`
	Gateway             *string `json:"gateway"`
	SubnetMask          *string `json:"subnet_mask"`
	VLANID              *int32  `json:"vlan_id"`
	MACAddress          string  `json:"mac_address" binding:"required"`
	BMCAddress          string  `json:"bmc_address" binding:"required"`
	IPMIUsername        string  `json:"ipmi_username" binding:"required"`
	IPMIPassword        string  `json:"ipmi_password" binding:"required"`
	HourlyPriceCents    int64   `json:"hourly_price_cents"`
	MonthlyPriceCents   int64   `json:"monthly_price_cents"`
	QuarterlyPriceCents int64   `json:"quarterly_price_cents"`
	YearlyPriceCents    int64   `json:"yearly_price_cents"`
	Provisionable       bool    `json:"provisionable"`
	Notes               string  `json:"notes"`
}

type createHardwareOptionRequest struct {
	OptionType             string  `json:"option_type" binding:"required"`
	Label                  string  `json:"label" binding:"required"`
	Description            string  `json:"description"`
	Unit                   string  `json:"unit"`
	ValueText              string  `json:"value_text"`
	ValueGB                *int32  `json:"value_gb"`
	PriceDeltaCents        int64   `json:"price_delta_cents"`
	HourlyPriceDeltaCents  int64   `json:"hourly_price_delta_cents"`
	QuarterlyDeltaCents    int64   `json:"quarterly_price_delta_cents"`
	YearlyDeltaCents       int64   `json:"yearly_price_delta_cents"`
	Currency               string  `json:"currency"`
	QuantityAvailable      int32   `json:"quantity_available"`
	FulfillmentMode        string  `json:"fulfillment_mode"`
	EstimatedReadyMinHours int32   `json:"estimated_ready_min_hours"`
	EstimatedReadyMaxHours int32   `json:"estimated_ready_max_hours"`
	LocationID             *string `json:"location_id"`
	HardwareProfileName    string  `json:"hardware_profile_name"`
	CPUModel               string  `json:"cpu_model"`
	Active                 *bool   `json:"active"`
}

type updateVirtualMachineRequest struct {
	Name         string  `json:"name" binding:"required"`
	Hostname     string  `json:"hostname"`
	HostServerID *string `json:"host_server_id"`
	CPUCores     int32   `json:"cpu_cores" binding:"required"`
	MemoryMB     int32   `json:"memory_mb" binding:"required"`
	DiskGB       int32   `json:"disk_gb" binding:"required"`
	ImageID      *string `json:"image_id"`
	OSImage      *string `json:"os_image"`
	PrivateIP    *string `json:"private_ip"`
}

type createPostgresRequest struct {
	Name                string  `json:"name" binding:"required"`
	HostServerID        *string `json:"host_server_id"`
	CPUCores            int32   `json:"cpu_cores" binding:"required"`
	MemoryMB            int32   `json:"memory_mb" binding:"required"`
	StorageGB           int32   `json:"storage_gb" binding:"required"`
	Version             string  `json:"version"`
	PrivateNetworkID    *string `json:"private_network_id"`
	PrivateIP           *string `json:"private_ip"`
	BackupEnabled       bool    `json:"backup_enabled"`
	BackupRetentionDays int32   `json:"backup_retention_days"`
}
