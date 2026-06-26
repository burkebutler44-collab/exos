export type PlatformPermission =
  | 'platform.users.view'
  | 'platform.users.manage'
  | 'platform.organizations.view'
  | 'platform.organizations.manage'
  | 'platform.organizations.suspend'
  | 'platform.organizations.impersonate'
  | 'platform.billing.view'
  | 'platform.billing.manage'
  | 'platform.billing.adjust'
  | 'platform.orders.view'
  | 'platform.invoices.view'
  | 'platform.credits.manage'
  | 'platform.servers.view'
  | 'platform.servers.create'
  | 'platform.servers.update'
  | 'platform.servers.assign'
  | 'platform.servers.retire'
  | 'platform.servers.power.manage'
  | 'platform.servers.provision'
  | 'platform.racks.view'
  | 'platform.racks.manage'
  | 'platform.provisioning.view'
  | 'platform.provisioning.manage'
  | 'platform.audit_log.view'
  | 'platform.settings.manage'

export type PlatformRole =
  | 'SuperAdmin'
  | 'InfrastructureOps'
  | 'BillingOps'
  | 'SupportOps'
  | 'ReadOnlyOps'

export type AdminUserListItem = {
  id: string
  email: string
  name: string
  authProviderSubject: string
  platformRoles: PlatformRole[]
  organizationCount: number
  status: 'active' | 'suspended'
  createdAt: string
  lastLoginAt?: string
}

export type AdminOrganizationListItem = {
  id: string
  name: string
  slug: string
  status: 'active' | 'suspended' | 'closed'
  billingStatus: 'active' | 'past_due' | 'suspended'
  billingEmail: string
  memberCount: number
  activeServerCount: number
  createdAt: string
}

export type AdminServerListItem = {
  id: string
  hostname: string
  assetTag: string
  serialNumber: string
  status:
    | 'available'
    | 'reserved'
    | 'provisioning_requested'
    | 'provisioning_started'
    | 'pxe_booting'
    | 'installing'
    | 'active'
    | 'failed'
    | 'suspended'
    | 'maintenance'
    | 'retired'
    | 'terminated'
  rackName: string
  locationName: string
  organizationName?: string
  projectName?: string
  hardwareProfileName: string
  publicIp?: string
  bmcAddress: string
  primaryMacAddress: string
  provisionable: boolean
  updatedAt: string
}

export type RackListItem = {
  id: string
  name: string
  location: string
  status: 'online' | 'degraded' | 'offline' | 'maintenance'
  lastHeartbeatAt?: string
  agentVersion: string
  availableServers: number
  activeServers: number
  failedJobs: number
}

export type ProvisioningJobListItem = {
  id: string
  server: string
  organization: string
  rack: string
  image: string
  status: 'pending' | 'command_published' | 'accepted_by_rack' | 'running' | 'completed' | 'failed' | 'expired' | 'canceled'
  requestedBy: string
  startedAt?: string
  completedAt?: string
  failureReason?: string
}

export type BillingAccountListItem = {
  id: string
  organizationName: string
  billingEmail: string
  status: 'active' | 'past_due' | 'suspended' | 'closed'
  paymentTerms: 'prepaid' | 'due_on_receipt' | 'net_7' | 'net_15' | 'net_30'
  creditBalanceCents: number
  stripeCustomerId?: string
}

export type AdminAuditEvent = {
  id: string
  actor: string
  action: string
  target: string
  organization?: string
  server?: string
  createdAt: string
  metadata: Record<string, string>
}

export type AdminStats = {
  totalUsers: number
  totalOrganizations: number
  activeOrganizations: number
  suspendedOrganizations: number
  availableServers: number
  allocatedServers: number
  provisioningServers: number
  failedProvisioningJobs: number
  offlineRacks: number
  pastDueBillingAccounts: number
}

export type AdminSession = {
  name: string
  email: string
  environment: 'production' | 'staging'
  roles: PlatformRole[]
  permissions: PlatformPermission[]
}
