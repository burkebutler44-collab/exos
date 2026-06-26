import type {
  AdminAuditEvent,
  AdminOrganizationListItem,
  AdminServerListItem,
  AdminSession,
  AdminStats,
  AdminUserListItem,
  BillingAccountListItem,
  ProvisioningJobListItem,
  RackListItem,
} from '../types/admin'

export const ADMIN_SESSION: AdminSession = {
  name: 'Operator',
  email: '',
  environment: 'production',
  roles: ['ReadOnlyOps'],
  permissions: [
    'platform.users.view',
    'platform.organizations.view',
    'platform.billing.view',
    'platform.servers.view',
    'platform.racks.view',
    'platform.provisioning.view',
    'platform.audit_log.view',
  ],
}

export const USERS: AdminUserListItem[] = []
export const ORGANIZATIONS: AdminOrganizationListItem[] = []
export const SERVERS: AdminServerListItem[] = []
export const RACKS: RackListItem[] = []
export const JOBS: ProvisioningJobListItem[] = []
export const BILLING_ACCOUNTS: BillingAccountListItem[] = []
export const AUDIT_EVENTS: AdminAuditEvent[] = []

export const STATS: AdminStats = {
  totalUsers: 0,
  totalOrganizations: 0,
  activeOrganizations: 0,
  suspendedOrganizations: 0,
  availableServers: 0,
  allocatedServers: 0,
  provisioningServers: 0,
  failedProvisioningJobs: 0,
  offlineRacks: 0,
  pastDueBillingAccounts: 0,
}
