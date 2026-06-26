package httpapi

import (
	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/http/handlers"
	"relay/client-backend/internal/http/middleware"
	"relay/client-backend/internal/services"

	"github.com/gin-gonic/gin"
)

func NewRouter(svc *services.Services, authConfigs ...middleware.AuthConfig) *gin.Engine {
	return NewRouterWithOptions(svc, nil, authConfigs...)
}

func NewRouterWithOptions(svc *services.Services, handlerOptions []handlers.HandlerOption, authConfigs ...middleware.AuthConfig) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.CORS())
	authConfig := middleware.AuthConfig{AllowInsecureDevHeaders: true}
	if len(authConfigs) > 0 {
		authConfig = authConfigs[0]
	}

	h := handlers.New(svc, handlerOptions...)
	router.GET("/healthz", h.Health)
	router.POST("/stripe/webhooks", h.HandleStripeWebhook)

	authenticated := router.Group("/")
	authenticated.Use(middleware.RequireAuth(svc, authConfig))
	authenticated.GET("/session", h.GetSession)
	authenticated.POST("/organizations", h.CreateOrganization)
	authenticated.GET("/organizations", h.ListOrganizations)
	authenticated.POST("/invitations/:token/accept", h.AcceptInvitation)

	organizations := authenticated.Group("/organizations/:organizationId")
	organizations.GET("", middleware.RequireOrganizationPermission(svc, domain.PermissionOrganizationsView), h.GetOrganization)
	organizations.PATCH("", middleware.RequireOrganizationPermission(svc, domain.PermissionOrganizationsUpdate), h.UpdateOrganization)
	organizations.DELETE("", middleware.RequireOrganizationPermission(svc, domain.PermissionOrganizationsDelete), h.DeleteOrganization)
	organizations.GET("/roles", middleware.RequireOrganizationPermission(svc, domain.PermissionMembersView), h.ListRoles)

	organizations.GET("/clouds", middleware.RequireOrganizationPermission(svc, domain.PermissionCloudsView), h.ListClouds)
	organizations.POST("/clouds", middleware.RequireOrganizationPermission(svc, domain.PermissionCloudsManage), h.CreateCloud)
	organizations.GET("/clouds/:cloudId", middleware.RequireOrganizationPermission(svc, domain.PermissionCloudsView), h.GetCloud)
	organizations.PATCH("/clouds/:cloudId", middleware.RequireOrganizationPermission(svc, domain.PermissionCloudsManage), h.UpdateCloud)
	organizations.DELETE("/clouds/:cloudId", middleware.RequireOrganizationPermission(svc, domain.PermissionCloudsManage), h.DeleteCloud)
	organizations.GET("/clouds/:cloudId/overview", middleware.RequireOrganizationPermission(svc, domain.PermissionCloudsView), h.GetCloudOverview)
	organizations.GET("/clouds/:cloudId/servers", middleware.RequireOrganizationPermission(svc, domain.PermissionServersView), h.ListCloudServers)
	organizations.POST("/clouds/:cloudId/servers/:serverId/assign", middleware.RequireOrganizationPermission(svc, domain.PermissionServersCreate), h.AssignServerToCloud)
	organizations.POST("/clouds/:cloudId/servers/:serverId/unassign", middleware.RequireOrganizationPermission(svc, domain.PermissionServersDelete), h.UnassignServerFromCloud)
	organizations.GET("/servers", middleware.RequireOrganizationPermission(svc, domain.PermissionServersView), h.ListOrganizationServers)
	organizations.GET("/server-catalog", middleware.RequireOrganizationPermission(svc, domain.PermissionServersView), h.ListServerCatalog)
	organizations.POST("/server-orders", middleware.RequireOrganizationPermission(svc, domain.PermissionServersCreate), h.AllocateServer)
	organizations.PATCH("/servers/:serverId/mode", middleware.RequireOrganizationPermission(svc, domain.PermissionServersCreate), h.ChangeServerMode)
	organizations.GET("/clouds/:cloudId/private-networks", middleware.RequireOrganizationPermission(svc, domain.PermissionNetworksView), h.ListPrivateNetworks)
	organizations.POST("/clouds/:cloudId/private-networks", middleware.RequireOrganizationPermission(svc, domain.PermissionNetworksManage), h.CreatePrivateNetwork)
	organizations.GET("/clouds/:cloudId/private-networks/:networkId", middleware.RequireOrganizationPermission(svc, domain.PermissionNetworksView), h.GetPrivateNetwork)
	organizations.DELETE("/clouds/:cloudId/private-networks/:networkId", middleware.RequireOrganizationPermission(svc, domain.PermissionNetworksManage), h.DeletePrivateNetwork)
	organizations.GET("/clouds/:cloudId/private-networks/:networkId/attachments", middleware.RequireOrganizationPermission(svc, domain.PermissionNetworksView), h.ListNetworkAttachments)
	organizations.POST("/clouds/:cloudId/private-networks/:networkId/attachments", middleware.RequireOrganizationPermission(svc, domain.PermissionNetworksManage), h.CreateNetworkAttachment)
	organizations.DELETE("/clouds/:cloudId/private-networks/:networkId/attachments/:attachmentId", middleware.RequireOrganizationPermission(svc, domain.PermissionNetworksManage), h.DetachNetworkAttachment)
	organizations.GET("/clouds/:cloudId/vms", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesView), h.ListVirtualMachines)
	organizations.POST("/clouds/:cloudId/vms", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.CreateVirtualMachine)
	organizations.GET("/clouds/:cloudId/vms/:vmId", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesView), h.GetVirtualMachine)
	organizations.PATCH("/clouds/:cloudId/vms/:vmId", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.UpdateVirtualMachine)
	organizations.DELETE("/clouds/:cloudId/vms/:vmId", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.DeleteVirtualMachine)
	organizations.POST("/clouds/:cloudId/vms/:vmId/:action", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.PowerVirtualMachine)
	organizations.GET("/clouds/:cloudId/managed-services", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesView), h.ListManagedServices)
	organizations.POST("/clouds/:cloudId/managed-services/postgres", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.CreatePostgresService)
	organizations.GET("/clouds/:cloudId/managed-services/:serviceId", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesView), h.GetManagedService)
	organizations.DELETE("/clouds/:cloudId/managed-services/:serviceId", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.DeleteManagedService)
	organizations.POST("/clouds/:cloudId/managed-services/:serviceId/:action", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.ActOnManagedService)
	organizations.GET("/clouds/:cloudId/capacity", middleware.RequireOrganizationPermission(svc, domain.PermissionCloudsView), h.GetCloudCapacity)
	organizations.GET("/clouds/:cloudId/placement-options", middleware.RequireOrganizationPermission(svc, domain.PermissionCloudsView), h.ListPlacementOptions)
	organizations.GET("/clouds/:cloudId/activity", middleware.RequireOrganizationPermission(svc, domain.PermissionCloudsView), h.ListCloudActivity)

	organizations.GET("/members", middleware.RequireOrganizationPermission(svc, domain.PermissionMembersView), h.ListMembers)
	organizations.PATCH("/members/:userId/role", middleware.RequireOrganizationPermission(svc, domain.PermissionMembersRolesManage), h.UpdateMemberRole)
	organizations.DELETE("/members/:userId", middleware.RequireOrganizationPermission(svc, domain.PermissionMembersRemove), h.RemoveMember)

	organizations.POST("/invitations", middleware.RequireOrganizationPermission(svc, domain.PermissionMembersInvite), h.InviteMember)
	organizations.GET("/invitations", middleware.RequireOrganizationPermission(svc, domain.PermissionMembersView), h.ListInvitations)
	organizations.POST("/invitations/:invitationId/revoke", middleware.RequireOrganizationPermission(svc, domain.PermissionMembersInvite), h.RevokeInvitation)

	organizations.POST("/projects", middleware.RequireOrganizationPermission(svc, domain.PermissionProjectsCreate), h.CreateProject)
	organizations.GET("/projects", middleware.RequireOrganizationPermission(svc, domain.PermissionProjectsView), h.ListProjects)
	organizations.GET("/projects/:projectId", middleware.RequireOrganizationPermission(svc, domain.PermissionProjectsView), h.GetProject)
	organizations.PATCH("/projects/:projectId", middleware.RequireOrganizationPermission(svc, domain.PermissionProjectsUpdate), h.UpdateProject)
	organizations.DELETE("/projects/:projectId", middleware.RequireOrganizationPermission(svc, domain.PermissionProjectsDelete), h.DeleteProject)

	organizations.GET("/billing", middleware.RequireOrganizationPermission(svc, domain.PermissionBillingView), h.GetBillingAccount)
	organizations.PATCH("/billing", middleware.RequireOrganizationPermission(svc, domain.PermissionBillingManage), h.UpdateBillingAccount)
	organizations.POST("/orders", middleware.RequireOrganizationPermission(svc, domain.PermissionBillingManage), h.CreateOrder)
	organizations.GET("/orders", middleware.RequireOrganizationPermission(svc, domain.PermissionBillingView), h.ListOrders)
	organizations.GET("/orders/:orderId", middleware.RequireOrganizationPermission(svc, domain.PermissionBillingView), h.GetOrder)
	organizations.GET("/billable-services", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesView), h.ListBillableServices)
	organizations.GET("/billable-services/:serviceId", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesView), h.GetBillableService)
	organizations.PATCH("/billable-services/:serviceId", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.UpdateBillableService)
	organizations.POST("/billable-services/:serviceId/cancel", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.CancelBillableService)
	organizations.POST("/billable-services/:serviceId/suspend", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.SuspendBillableService)
	organizations.POST("/billable-services/:serviceId/resume", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesManage), h.ResumeBillableService)
	organizations.GET("/credits", middleware.RequireOrganizationPermission(svc, domain.PermissionCreditsView), h.ListCreditLedger)
	organizations.POST("/credits/checkout", middleware.RequireOrganizationPermission(svc, domain.PermissionCreditsManage), h.CreateCreditCheckout)
	organizations.POST("/credits/manual-adjustment", middleware.RequireOrganizationPermission(svc, domain.PermissionCreditsManage), h.ManualCreditAdjustment)
	organizations.GET("/usage", middleware.RequireOrganizationPermission(svc, domain.PermissionBillingView), h.ListUsage)
	organizations.GET("/billable-services/:serviceId/usage", middleware.RequireOrganizationPermission(svc, domain.PermissionServicesView), h.ListServiceUsage)
	organizations.GET("/payment-methods", middleware.RequireOrganizationPermission(svc, domain.PermissionBillingView), h.ListPaymentMethods)
	organizations.POST("/payment-methods/setup-intent", middleware.RequireOrganizationPermission(svc, domain.PermissionPaymentMethodsManage), h.CreatePaymentMethodSetupIntent)
	organizations.POST("/payment-methods/confirm", middleware.RequireOrganizationPermission(svc, domain.PermissionPaymentMethodsManage), h.ConfirmPaymentMethodSetup)
	organizations.POST("/payment-methods", middleware.RequireOrganizationPermission(svc, domain.PermissionPaymentMethodsManage), h.CreatePaymentMethod)
	organizations.DELETE("/payment-methods/:paymentMethodId", middleware.RequireOrganizationPermission(svc, domain.PermissionPaymentMethodsManage), h.DeletePaymentMethod)
	organizations.GET("/invoices", middleware.RequireOrganizationPermission(svc, domain.PermissionInvoicesView), h.ListInvoices)
	organizations.GET("/invoices/:invoiceId", middleware.RequireOrganizationPermission(svc, domain.PermissionInvoicesView), h.GetInvoiceRecord)
	organizations.POST("/invoices/generate", middleware.RequireOrganizationPermission(svc, domain.PermissionBillingManage), h.GenerateInvoice)
	organizations.POST("/invoices/:invoiceId/finalize", middleware.RequireOrganizationPermission(svc, domain.PermissionBillingManage), h.FinalizeInvoice)
	organizations.POST("/invoices/:invoiceId/void", middleware.RequireOrganizationPermission(svc, domain.PermissionBillingManage), h.VoidInvoice)

	organizations.GET("/audit-log", middleware.RequireOrganizationPermission(svc, domain.PermissionAuditLogView), h.ListAuditLog)

	organizations.POST("/servers/:serverId/provision", middleware.RequireOrganizationPermission(svc, domain.PermissionServersProvision), h.ProvisionServer)
	organizations.POST("/servers/:serverId/reinstall", middleware.RequireOrganizationPermission(svc, domain.PermissionServersReinstall), h.ReinstallServer)
	organizations.GET("/provisioning-jobs/:jobId", middleware.RequireOrganizationPermission(svc, domain.PermissionServersView), h.GetProvisioningJob)
	organizations.GET("/provisioning-jobs/:jobId/events", middleware.RequireOrganizationPermission(svc, domain.PermissionServersView), h.ListProvisioningJobEvents)
	organizations.POST("/servers/:serverId/power", middleware.RequireOrganizationPermission(svc, domain.PermissionServersPowerManage), h.PowerServer)
	organizations.GET("/servers/:serverId/power", middleware.RequireOrganizationPermission(svc, domain.PermissionServersPowerManage), h.GetServerPower)

	admin := authenticated.Group("/admin")
	platformPermission := func(permission string) gin.HandlerFunc {
		return middleware.RequirePlatformPermission(svc, permission)
	}

	admin.GET("/users", platformPermission(domain.PlatformUsersView), h.AdminListUsers)
	admin.GET("/users/:userId", platformPermission(domain.PlatformUsersView), h.AdminGetUser)
	admin.GET("/users/:userId/organizations", platformPermission(domain.PlatformOrganizationsImpersonate), h.AdminListUserOrganizations)
	admin.PATCH("/users/:userId/platform-roles", platformPermission(domain.PlatformUsersManage), h.AdminUpdateUserPlatformRoles)

	admin.GET("/organizations", platformPermission(domain.PlatformOrganizationsView), h.AdminListOrganizations)
	admin.GET("/organizations/:organizationId", platformPermission(domain.PlatformOrganizationsView), h.AdminGetOrganization)
	admin.PATCH("/organizations/:organizationId", platformPermission(domain.PlatformOrganizationsManage), h.AdminUpdateOrganization)
	admin.POST("/organizations/:organizationId/suspend", platformPermission(domain.PlatformOrganizationsSuspend), h.AdminSuspendOrganization)
	admin.POST("/organizations/:organizationId/resume", platformPermission(domain.PlatformOrganizationsSuspend), h.AdminResumeOrganization)
	admin.GET("/cloud-resources", platformPermission(domain.PlatformCloudsView), h.AdminListCloudResources)

	admin.GET("/billing/accounts", platformPermission(domain.PlatformBillingView), h.AdminListBillingAccounts)
	admin.GET("/billing/accounts/:billingAccountId", platformPermission(domain.PlatformBillingView), h.AdminGetBillingAccount)
	admin.POST("/billing/accounts/:billingAccountId/manual-credit", platformPermission(domain.PlatformBillingAdjust), h.AdminManualCredit)
	admin.POST("/billing/accounts/:billingAccountId/manual-debit", platformPermission(domain.PlatformBillingAdjust), h.AdminManualDebit)
	admin.GET("/billing/invoices", platformPermission(domain.PlatformInvoicesView), h.AdminListInvoices)
	admin.GET("/billing/orders", platformPermission(domain.PlatformOrdersView), h.AdminListOrders)
	admin.GET("/billing/credits", platformPermission(domain.PlatformBillingView), h.AdminListCredits)

	admin.GET("/servers", platformPermission(domain.PlatformServersView), h.AdminListServers)
	admin.POST("/servers", platformPermission(domain.PlatformServersCreate), h.AdminCreateServer)
	admin.GET("/servers/:serverId", platformPermission(domain.PlatformServersView), h.AdminGetServer)
	admin.PATCH("/servers/:serverId", platformPermission(domain.PlatformServersUpdate), h.AdminUpdateServer)
	admin.POST("/servers/:serverId/assign", platformPermission(domain.PlatformServersAssign), h.AdminAssignServer)
	admin.POST("/servers/:serverId/release", platformPermission(domain.PlatformServersAssign), h.AdminReleaseServer)
	admin.POST("/servers/:serverId/reserve", platformPermission(domain.PlatformServersAssign), h.AdminReserveServer)
	admin.POST("/servers/:serverId/maintenance", platformPermission(domain.PlatformServersUpdate), h.AdminSetServerMaintenance)
	admin.POST("/servers/:serverId/retire", platformPermission(domain.PlatformServersRetire), h.AdminRetireServer)
	admin.GET("/servers/:serverId/power", platformPermission(domain.PlatformServersPowerManage), h.AdminGetServerPower)
	admin.POST("/servers/:serverId/power", platformPermission(domain.PlatformServersPowerManage), h.AdminPowerServer)
	admin.POST("/servers/:serverId/provision", platformPermission(domain.PlatformServersProvision), h.AdminProvisionServer)
	admin.POST("/servers/:serverId/reinstall", platformPermission(domain.PlatformServersProvision), h.AdminReinstallServer)
	admin.POST("/servers/:serverId/rescue", platformPermission(domain.PlatformServersProvision), h.AdminRescueServer)

	admin.GET("/racks", platformPermission(domain.PlatformRacksView), h.ListRacks)
	admin.POST("/racks", platformPermission(domain.PlatformRacksManage), h.AdminCreateRack)
	admin.GET("/racks/:rackId", platformPermission(domain.PlatformRacksView), h.GetRack)
	admin.PATCH("/racks/:rackId", platformPermission(domain.PlatformRacksManage), h.AdminUpdateRack)
	admin.GET("/racks/:rackId/health", platformPermission(domain.PlatformRacksView), h.GetRackHealth)
	admin.GET("/racks/:rackId/agents", platformPermission(domain.PlatformRacksView), h.ListRackAgents)
	admin.GET("/racks/:rackId/servers", platformPermission(domain.PlatformRacksView), h.AdminListRackServers)
	admin.POST("/racks/:rackId/maintenance", platformPermission(domain.PlatformRacksManage), h.SetRackMaintenance)
	admin.POST("/racks/:rackId/resume", platformPermission(domain.PlatformRacksManage), h.ResumeRack)

	admin.GET("/locations/health", platformPermission(domain.PlatformRacksView), h.AdminListLocationHealth)
	admin.GET("/hardware/cpu-profiles", platformPermission(domain.PlatformServersView), h.AdminListCPUProfiles)
	admin.GET("/network/locations", platformPermission(domain.PlatformNetworkView), h.AdminListLocations)
	admin.GET("/network/switches", platformPermission(domain.PlatformNetworkView), h.AdminListSwitches)
	admin.GET("/network/edge-routers", platformPermission(domain.PlatformNetworkView), h.AdminListEdgeRouters)
	admin.GET("/network/server-interfaces", platformPermission(domain.PlatformNetworkView), h.AdminListServerNetworkInterfaces)
	admin.GET("/hypervisors", platformPermission(domain.PlatformHypervisorsView), h.AdminListHypervisors)
	admin.GET("/hypervisors/:hypervisorId/vms", platformPermission(domain.PlatformHypervisorsView), h.AdminListHypervisorVMs)

	admin.GET("/provisioning-jobs", platformPermission(domain.PlatformProvisioningView), h.AdminListProvisioningJobs)
	admin.GET("/provisioning-jobs/:jobId", platformPermission(domain.PlatformProvisioningView), h.AdminGetProvisioningJob)
	admin.GET("/provisioning-jobs/:jobId/events", platformPermission(domain.PlatformProvisioningView), h.AdminListProvisioningJobEvents)
	admin.POST("/provisioning-jobs/:jobId/retry", platformPermission(domain.PlatformProvisioningManage), h.AdminRetryProvisioningJob)
	admin.POST("/provisioning-jobs/:jobId/cancel", platformPermission(domain.PlatformProvisioningManage), h.AdminCancelProvisioningJob)

	admin.GET("/images", platformPermission(domain.PlatformProvisioningView), h.AdminListImages)
	admin.POST("/images", platformPermission(domain.PlatformProvisioningManage), h.AdminCreateImage)
	admin.GET("/images/:imageId", platformPermission(domain.PlatformProvisioningView), h.AdminGetImage)
	admin.PATCH("/images/:imageId", platformPermission(domain.PlatformProvisioningManage), h.AdminUpdateImage)
	admin.POST("/images/:imageId/enable", platformPermission(domain.PlatformProvisioningManage), h.AdminEnableImage)
	admin.POST("/images/:imageId/disable", platformPermission(domain.PlatformProvisioningManage), h.AdminDisableImage)
	admin.POST("/images/:imageId/make-default", platformPermission(domain.PlatformProvisioningManage), h.AdminMakeDefaultImage)

	admin.GET("/audit-log", platformPermission(domain.PlatformAuditLogView), h.AdminListAuditLog)
	admin.GET("/nats/connections", platformPermission(domain.PlatformSettingsManage), h.AdminListNATSConnections)
	admin.GET("/settings", platformPermission(domain.PlatformSettingsManage), h.AdminGetSettings)
	admin.PATCH("/settings", platformPermission(domain.PlatformSettingsManage), h.AdminUpdateSettings)

	return router
}
