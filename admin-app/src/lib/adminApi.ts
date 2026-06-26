import {
  ADMIN_SESSION,
  AUDIT_EVENTS,
  BILLING_ACCOUNTS,
  JOBS,
  ORGANIZATIONS,
  RACKS,
  SERVERS,
  STATS,
  USERS,
} from '../data/adminData'

export const adminApi = {
  getSession: async () => ADMIN_SESSION,
  getStats: async () => STATS,
  listUsers: async () => USERS,
  listOrganizations: async () => ORGANIZATIONS,
  listServers: async () => SERVERS,
  listRacks: async () => RACKS,
  listProvisioningJobs: async () => JOBS,
  listBillingAccounts: async () => BILLING_ACCOUNTS,
  listAuditEvents: async () => AUDIT_EVENTS,
}
