package domain

const (
	RoleOwner          = "Owner"
	RoleAdmin          = "Admin"
	RoleInfrastructure = "Infrastructure"
	RoleBilling        = "Billing"
	RoleReadOnly       = "Read-only"
)

const (
	PermissionOrganizationsView    = "organizations.view"
	PermissionOrganizationsUpdate  = "organizations.update"
	PermissionOrganizationsDelete  = "organizations.delete"
	PermissionCloudsView           = "clouds.view"
	PermissionCloudsManage         = "clouds.manage"
	PermissionServersView          = "servers.view"
	PermissionServersProvision     = "servers.provision"
	PermissionServersCreate        = "servers.create"
	PermissionServersPowerManage   = "servers.power.manage"
	PermissionServersReinstall     = "servers.reinstall"
	PermissionServersDelete        = "servers.delete"
	PermissionNetworksView         = "networks.view"
	PermissionNetworksManage       = "networks.manage"
	PermissionSSHKeysView          = "ssh_keys.view"
	PermissionSSHKeysManage        = "ssh_keys.manage"
	PermissionBillingView          = "billing.view"
	PermissionBillingManage        = "billing.manage"
	PermissionInvoicesView         = "invoices.view"
	PermissionPaymentMethodsManage = "payment_methods.manage"
	PermissionCreditsView          = "credits.view"
	PermissionCreditsManage        = "credits.manage"
	PermissionServicesView         = "services.view"
	PermissionServicesManage       = "services.manage"
	PermissionMembersView          = "members.view"
	PermissionMembersInvite        = "members.invite"
	PermissionMembersRemove        = "members.remove"
	PermissionMembersRolesManage   = "members.roles.manage"
	PermissionAPIKeysView          = "api_keys.view"
	PermissionAPIKeysCreate        = "api_keys.create"
	PermissionAPIKeysRevoke        = "api_keys.revoke"
	PermissionProjectsView         = "projects.view"
	PermissionProjectsCreate       = "projects.create"
	PermissionProjectsUpdate       = "projects.update"
	PermissionProjectsDelete       = "projects.delete"
	PermissionAuditLogView         = "audit_log.view"
)

var SystemRolePermissions = map[string][]string{
	RoleOwner: {
		PermissionOrganizationsView, PermissionOrganizationsUpdate, PermissionOrganizationsDelete,
		PermissionCloudsView, PermissionCloudsManage,
		PermissionServersView, PermissionServersProvision, PermissionServersCreate, PermissionServersPowerManage, PermissionServersReinstall, PermissionServersDelete,
		PermissionNetworksView, PermissionNetworksManage,
		PermissionSSHKeysView, PermissionSSHKeysManage,
		PermissionBillingView, PermissionBillingManage, PermissionInvoicesView, PermissionPaymentMethodsManage, PermissionCreditsView, PermissionCreditsManage, PermissionServicesView, PermissionServicesManage,
		PermissionMembersView, PermissionMembersInvite, PermissionMembersRemove, PermissionMembersRolesManage,
		PermissionAPIKeysView, PermissionAPIKeysCreate, PermissionAPIKeysRevoke,
		PermissionProjectsView, PermissionProjectsCreate, PermissionProjectsUpdate, PermissionProjectsDelete,
		PermissionAuditLogView,
	},
	RoleAdmin: {
		PermissionOrganizationsView, PermissionOrganizationsUpdate,
		PermissionCloudsView, PermissionCloudsManage,
		PermissionServersView, PermissionServersProvision, PermissionServersCreate, PermissionServersPowerManage, PermissionServersReinstall, PermissionServersDelete,
		PermissionNetworksView, PermissionNetworksManage,
		PermissionSSHKeysView, PermissionSSHKeysManage,
		PermissionBillingView, PermissionInvoicesView, PermissionCreditsView,
		PermissionMembersView, PermissionMembersInvite, PermissionMembersRemove, PermissionMembersRolesManage,
		PermissionAPIKeysView, PermissionAPIKeysCreate, PermissionAPIKeysRevoke, PermissionServicesView, PermissionServicesManage,
		PermissionProjectsView, PermissionProjectsCreate, PermissionProjectsUpdate, PermissionProjectsDelete,
		PermissionAuditLogView,
	},
	RoleInfrastructure: {
		PermissionOrganizationsView,
		PermissionCloudsView, PermissionCloudsManage,
		PermissionServersView, PermissionServersProvision, PermissionServersCreate, PermissionServersPowerManage, PermissionServersReinstall, PermissionServersDelete,
		PermissionNetworksView, PermissionNetworksManage,
		PermissionSSHKeysView, PermissionSSHKeysManage,
		PermissionAPIKeysView, PermissionAPIKeysCreate, PermissionAPIKeysRevoke,
		PermissionProjectsView, PermissionServicesView, PermissionServicesManage,
	},
	RoleBilling: {
		PermissionOrganizationsView,
		PermissionCloudsView,
		PermissionBillingView, PermissionBillingManage, PermissionInvoicesView, PermissionPaymentMethodsManage, PermissionCreditsView, PermissionCreditsManage, PermissionServicesView,
	},
	RoleReadOnly: {
		PermissionOrganizationsView,
		PermissionCloudsView,
		PermissionServersView, PermissionNetworksView, PermissionSSHKeysView,
		PermissionBillingView, PermissionInvoicesView, PermissionCreditsView, PermissionServicesView, PermissionMembersView,
		PermissionAPIKeysView, PermissionProjectsView, PermissionAuditLogView,
	},
}
