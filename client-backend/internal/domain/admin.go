package domain

import "time"

const (
	PlatformRoleSuperAdmin        = "SuperAdmin"
	PlatformRoleInfrastructureOps = "InfrastructureOps"
	PlatformRoleBillingOps        = "BillingOps"
	PlatformRoleSupportOps        = "SupportOps"
	PlatformRoleReadOnlyOps       = "ReadOnlyOps"
)

const (
	PlatformUsersView                = "platform.users.view"
	PlatformUsersManage              = "platform.users.manage"
	PlatformOrganizationsView        = "platform.organizations.view"
	PlatformOrganizationsManage      = "platform.organizations.manage"
	PlatformOrganizationsSuspend     = "platform.organizations.suspend"
	PlatformOrganizationsImpersonate = "platform.organizations.impersonate"
	PlatformCloudsView               = "platform.clouds.view"
	PlatformCloudsManage             = "platform.clouds.manage"
	PlatformBillingView              = "platform.billing.view"
	PlatformBillingManage            = "platform.billing.manage"
	PlatformBillingAdjust            = "platform.billing.adjust"
	PlatformOrdersView               = "platform.orders.view"
	PlatformInvoicesView             = "platform.invoices.view"
	PlatformCreditsManage            = "platform.credits.manage"
	PlatformServersView              = "platform.servers.view"
	PlatformServersCreate            = "platform.servers.create"
	PlatformServersUpdate            = "platform.servers.update"
	PlatformServersAssign            = "platform.servers.assign"
	PlatformServersRetire            = "platform.servers.retire"
	PlatformServersPowerManage       = "platform.servers.power.manage"
	PlatformServersProvision         = "platform.servers.provision"
	PlatformNetworkView              = "platform.network.view"
	PlatformNetworkManage            = "platform.network.manage"
	PlatformHypervisorsView          = "platform.hypervisors.view"
	PlatformHypervisorsManage        = "platform.hypervisors.manage"
	PlatformRacksView                = "platform.racks.view"
	PlatformRacksManage              = "platform.racks.manage"
	PlatformProvisioningView         = "platform.provisioning.view"
	PlatformProvisioningManage       = "platform.provisioning.manage"
	PlatformAuditLogView             = "platform.audit_log.view"
	PlatformSettingsManage           = "platform.settings.manage"
)

type AdminAuditEntry struct {
	ID             string    `json:"id"`
	ActorUserID    string    `json:"actor_user_id"`
	Action         string    `json:"action"`
	TargetType     string    `json:"target_type"`
	TargetID       string    `json:"target_id"`
	OrganizationID *string   `json:"organization_id,omitempty"`
	ServerID       *string   `json:"server_id,omitempty"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}
