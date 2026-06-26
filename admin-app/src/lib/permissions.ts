import type { AdminSession, PlatformPermission } from '../types/admin'

export function can(session: AdminSession, permission: PlatformPermission) {
  return session.permissions.includes(permission)
}
