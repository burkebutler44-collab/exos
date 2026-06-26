import {
  Activity,
  Banknote,
  Boxes,
  Building2,
  ClipboardList,
  Cpu,
  FileClock,
  Gauge,
  HardDrive,
  Image,
  Search,
  Settings,
  Users,
  type LucideIcon,
} from 'lucide-react'
import type { ReactNode } from 'react'
import type { AdminSession, PlatformPermission } from '../types/admin'
import { can } from '../lib/permissions'

export type View =
  | 'dashboard'
  | 'users'
  | 'organizations'
  | 'billing'
  | 'servers'
  | 'server-new'
  | 'racks'
  | 'jobs'
  | 'images'
  | 'audit'
  | 'settings'

const nav: Array<{ id: View; label: string; icon: LucideIcon; permission: PlatformPermission }> = [
  { id: 'dashboard', label: 'Dashboard', icon: Gauge, permission: 'platform.audit_log.view' },
  { id: 'users', label: 'Users', icon: Users, permission: 'platform.users.view' },
  { id: 'organizations', label: 'Organizations', icon: Building2, permission: 'platform.organizations.view' },
  { id: 'billing', label: 'Billing', icon: Banknote, permission: 'platform.billing.view' },
  { id: 'servers', label: 'Servers', icon: HardDrive, permission: 'platform.servers.view' },
  { id: 'racks', label: 'Racks', icon: Boxes, permission: 'platform.racks.view' },
  { id: 'jobs', label: 'Provisioning Jobs', icon: Cpu, permission: 'platform.provisioning.view' },
  { id: 'images', label: 'Images', icon: Image, permission: 'platform.provisioning.manage' },
  { id: 'audit', label: 'Audit Log', icon: FileClock, permission: 'platform.audit_log.view' },
  { id: 'settings', label: 'Settings', icon: Settings, permission: 'platform.settings.manage' },
]

export function Layout({
  view,
  onNavigate,
  session,
  children,
}: {
  view: View
  onNavigate: (view: View) => void
  session: AdminSession
  children: ReactNode
}) {
  const visibleNav = nav.filter((item) => can(session, item.permission))

  return (
    <div className="min-h-screen bg-bg">
      <aside className="fixed inset-y-0 left-0 hidden w-64 border-r border-line bg-white lg:block">
        <div className="flex h-14 items-center gap-2 border-b border-line px-4">
          <ExosMark className="h-6 w-6 shrink-0" />
          <span className="font-semibold">Exos Admin</span>
        </div>
        <nav className="p-3">
          {visibleNav.map((item) => {
            const Icon = item.icon
            const active = view === item.id || (view === 'server-new' && item.id === 'servers')
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => onNavigate(item.id)}
                className={
                  'mb-1 flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-[13px] transition-colors ' +
                  (active ? 'bg-blue-50 font-medium text-accent' : 'text-gray-600 hover:bg-gray-50 hover:text-ink')
                }
              >
                <Icon size={15} />
                {item.label}
              </button>
            )
          })}
        </nav>
      </aside>

      <div className="lg:pl-64">
        <header className="sticky top-0 z-10 flex h-14 items-center justify-between border-b border-line bg-white/90 px-4 backdrop-blur lg:px-6">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div className="hidden h-8 w-full max-w-lg items-center gap-2 rounded-md border border-line bg-gray-50 px-3 text-[13px] text-gray-400 md:flex">
              <Search size={14} />
              <span>Search users, organizations, servers, racks...</span>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <span className="rounded border border-line bg-gray-50 px-2 py-1 font-mono text-[11px] uppercase text-gray-500">
              {session.environment}
            </span>
            <div className="text-right">
              <p className="text-[12.5px] font-medium text-ink">{session.name}</p>
              <p className="text-[11px] text-gray-400">{session.roles.join(', ')}</p>
            </div>
          </div>
        </header>
        <main className="px-4 py-6 lg:px-6">{children}</main>
      </div>
    </div>
  )
}

function ExosMark({ className = '' }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 80 80"
      fill="none"
      aria-hidden="true"
      style={{ overflow: 'visible' }}
    >
      <ellipse
        cx="40"
        cy="40"
        rx="30"
        ry="10"
        transform="rotate(-24 40 40)"
        stroke="#8DBBFF"
        strokeWidth="5"
        strokeLinecap="round"
      />
      <circle cx="40" cy="40" r="16" fill="#2E75DD" />
      <path
        d="M15.7 48.9C28.6 55.1 49.7 50.6 64.3 31.1"
        stroke="#8DBBFF"
        strokeWidth="5"
        strokeLinecap="round"
      />
      <path
        d="M33 28.4C36.2 27.1 40.4 26.7 44.2 27.7"
        stroke="#F7F8FA"
        strokeOpacity="0.38"
        strokeWidth="3"
        strokeLinecap="round"
      />
    </svg>
  )
}

export function PageHeader({
  eyebrow,
  title,
  subtitle,
  action,
}: {
  eyebrow: string
  title: string
  subtitle?: string
  action?: ReactNode
}) {
  return (
    <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <p className="admin-label">{eyebrow}</p>
        <h1 className="mt-1 text-2xl font-semibold tracking-tight text-ink">{title}</h1>
        {subtitle && <p className="mt-1 max-w-2xl text-[13px] text-muted">{subtitle}</p>}
      </div>
      {action}
    </div>
  )
}

export function StatusBadge({ value }: { value: string }) {
  const tone = value.includes('failed') || value.includes('past_due') || value.includes('offline')
    ? 'bg-red-50 text-bad border-red-100'
    : value.includes('degraded') || value.includes('installing') || value.includes('running')
      ? 'bg-amber-50 text-warn border-amber-100'
      : 'bg-emerald-50 text-good border-emerald-100'
  return <span className={`inline-flex rounded border px-1.5 py-0.5 text-[11px] font-medium ${tone}`}>{value}</span>
}

export function EmptyState({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="admin-card flex min-h-48 flex-col items-center justify-center text-center">
      <ClipboardList size={22} className="text-gray-300" />
      <p className="mt-3 text-[13px] font-medium text-ink">{title}</p>
      <p className="mt-1 max-w-sm text-[12px] text-muted">{detail}</p>
    </div>
  )
}
