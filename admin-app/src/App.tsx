import { useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { AlertTriangle, Plus, Power, RotateCcw } from 'lucide-react'
import { ConfirmDialog } from './components/ConfirmDialog'
import { Layout, PageHeader, StatusBadge, type View } from './components/Layout'
import { DataTable, type Column } from './components/Table'
import { adminApi } from './lib/adminApi'
import { date, money } from './lib/format'
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
} from './data/adminData'
import type {
  AdminAuditEvent,
  AdminOrganizationListItem,
  AdminServerListItem,
  AdminUserListItem,
  BillingAccountListItem,
  ProvisioningJobListItem,
  RackListItem,
} from './types/admin'

export function App() {
  const [view, setView] = useState<View>('dashboard')
  const [selectedServer, setSelectedServer] = useState<AdminServerListItem | null>(null)
  const [confirm, setConfirm] = useState<{ title: string; detail: string } | null>(null)
  const [reason, setReason] = useState('')
  const session = ADMIN_SESSION

  void adminApi.getSession()

  const page = useMemo(() => {
    if (view === 'dashboard') return <Dashboard onNavigate={setView} />
    if (view === 'users') return <UsersPage />
    if (view === 'organizations') return <OrganizationsPage />
    if (view === 'billing') return <BillingPage />
    if (view === 'servers') {
      return (
        <ServersPage
          selectedServer={selectedServer}
          onSelectServer={setSelectedServer}
          onNewServer={() => setView('server-new')}
          onDangerousAction={(title, detail) => {
            setReason('')
            setConfirm({ title, detail })
          }}
        />
      )
    }
    if (view === 'server-new') return <CreateServerPage onBack={() => setView('servers')} />
    if (view === 'racks') return <RacksPage />
    if (view === 'jobs') return <ProvisioningJobsPage />
    if (view === 'images') return <ImagesPage />
    if (view === 'audit') return <AuditPage />
    return <SettingsPage />
  }, [selectedServer, view])

  return (
    <>
      <Layout view={view} onNavigate={setView} session={session}>
        {page}
      </Layout>
      <ConfirmDialog
        open={Boolean(confirm)}
        title={confirm?.title ?? ''}
        detail={confirm?.detail ?? ''}
        reason={reason}
        onReasonChange={setReason}
        onCancel={() => setConfirm(null)}
        onConfirm={() => setConfirm(null)}
      />
    </>
  )
}

function Dashboard({ onNavigate }: { onNavigate: (view: View) => void }) {
  const cards = [
    ['Users', STATS.totalUsers, 'platform identities'],
    ['Organizations', STATS.totalOrganizations, `${STATS.suspendedOrganizations} suspended`],
    ['Available servers', STATS.availableServers, `${STATS.allocatedServers} allocated`],
    ['Provisioning', STATS.provisioningServers, `${STATS.failedProvisioningJobs} failed jobs`],
    ['Rack health', STATS.offlineRacks, 'degraded/offline racks'],
    ['Past due', STATS.pastDueBillingAccounts, 'billing accounts'],
  ]
  return (
    <>
      <PageHeader eyebrow="Admin / Dashboard" title="Operational Overview" subtitle="Internal platform state across accounts, inventory, racks, provisioning, and billing." />
      <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-6">
        {cards.map(([label, value, detail]) => (
          <button key={label} type="button" onClick={() => label === 'Available servers' && onNavigate('servers')} className="admin-card p-4 text-left">
            <p className="admin-label">{label}</p>
            <p className="mt-3 text-2xl font-semibold text-ink">{value}</p>
            <p className="mt-1 text-[12px] text-muted">{detail}</p>
          </button>
        ))}
      </div>
      <div className="mt-5 grid gap-4 xl:grid-cols-2">
        <RecentPanel title="Recent admin actions" events={AUDIT_EVENTS} />
        <div className="admin-card">
          <div className="border-b border-line px-4 py-3">
            <h2 className="text-[14px] font-semibold">Recent provisioning failures</h2>
          </div>
          <div className="divide-y divide-line">
            {JOBS.filter((job) => job.status === 'failed').map((job) => (
              <div key={job.id} className="px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <p className="font-mono text-[12.5px] text-ink">{job.server}</p>
                  <StatusBadge value={job.status} />
                </div>
                <p className="mt-1 text-[12px] text-muted">{job.failureReason}</p>
              </div>
            ))}
            {JOBS.filter((job) => job.status === 'failed').length === 0 && (
              <div className="px-4 py-10 text-center text-[12.5px] text-muted">
                No provisioning failures have been recorded yet.
              </div>
            )}
          </div>
        </div>
      </div>
    </>
  )
}

function UsersPage() {
  const columns: Array<Column<AdminUserListItem>> = [
    { key: 'email', label: 'Email', search: (u) => `${u.email} ${u.name} ${u.authProviderSubject}`, render: (u) => <Strong title={u.email} detail={u.name} /> },
    { key: 'sub', label: 'Auth sub', render: (u) => <Mono>{u.authProviderSubject}</Mono> },
    { key: 'orgs', label: 'Organizations', render: (u) => u.organizationCount },
    { key: 'role', label: 'Platform role', render: (u) => u.platformRoles.join(', ') || 'None' },
    { key: 'status', label: 'Status', render: (u) => <StatusBadge value={u.status} /> },
    { key: 'created', label: 'Created', render: (u) => date(u.createdAt) },
    { key: 'login', label: 'Last login', render: (u) => date(u.lastLoginAt) },
  ]
  return <PageWithTable eyebrow="Admin / Users" title="Users" subtitle="Inspect identities, organization memberships, platform roles, and activity." rows={USERS} columns={columns} placeholder="Search email, name, auth0 sub..." />
}

function OrganizationsPage() {
  const columns: Array<Column<AdminOrganizationListItem>> = [
    { key: 'name', label: 'Name', search: (o) => `${o.name} ${o.slug} ${o.billingEmail}`, render: (o) => <Strong title={o.name} detail={o.slug} /> },
    { key: 'billing', label: 'Billing email', render: (o) => o.billingEmail },
    { key: 'status', label: 'Status', render: (o) => <StatusBadge value={o.status} /> },
    { key: 'billingStatus', label: 'Billing', render: (o) => <StatusBadge value={o.billingStatus} /> },
    { key: 'members', label: 'Members', render: (o) => o.memberCount },
    { key: 'servers', label: 'Active servers', render: (o) => o.activeServerCount },
    { key: 'created', label: 'Created', render: (o) => date(o.createdAt) },
  ]
  return <PageWithTable eyebrow="Admin / Organizations" title="Organizations" subtitle="Manage customer account state, members, projects, billing, servers, and audit history." rows={ORGANIZATIONS} columns={columns} placeholder="Search name, slug, billing email..." />
}

function BillingPage() {
  const columns: Array<Column<BillingAccountListItem>> = [
    { key: 'org', label: 'Organization', search: (b) => `${b.organizationName} ${b.billingEmail}`, render: (b) => <Strong title={b.organizationName} detail={b.billingEmail} /> },
    { key: 'status', label: 'Status', render: (b) => <StatusBadge value={b.status} /> },
    { key: 'terms', label: 'Terms', render: (b) => b.paymentTerms },
    { key: 'balance', label: 'Credit balance', render: (b) => <span className={b.creditBalanceCents < 0 ? 'text-bad' : 'text-good'}>{money(b.creditBalanceCents)}</span> },
    { key: 'stripe', label: 'Stripe', render: (b) => <Mono>{b.stripeCustomerId ?? 'Not linked'}</Mono> },
    { key: 'actions', label: 'Actions', render: () => <button className="admin-btn">Adjustment</button> },
  ]
  return <PageWithTable eyebrow="Admin / Billing" title="Billing Accounts" subtitle="View payment terms, credit balances, invoices, orders, and manual adjustment surfaces." rows={BILLING_ACCOUNTS} columns={columns} placeholder="Search organization or billing email..." />
}

function ServersPage({
  selectedServer,
  onSelectServer,
  onNewServer,
  onDangerousAction,
}: {
  selectedServer: AdminServerListItem | null
  onSelectServer: (server: AdminServerListItem) => void
  onNewServer: () => void
  onDangerousAction: (title: string, detail: string) => void
}) {
  const columns: Array<Column<AdminServerListItem>> = [
    { key: 'host', label: 'Hostname', search: (s) => `${s.hostname} ${s.assetTag} ${s.serialNumber} ${s.primaryMacAddress} ${s.bmcAddress} ${s.publicIp ?? ''}`, render: (s) => <button className="text-left" onClick={() => onSelectServer(s)}><Strong title={s.hostname} detail={s.assetTag} /></button> },
    { key: 'status', label: 'Status', render: (s) => <StatusBadge value={s.status} /> },
    { key: 'rack', label: 'Rack', render: (s) => <Strong title={s.rackName} detail={s.locationName} /> },
    { key: 'org', label: 'Organization', render: (s) => s.organizationName ?? 'Unassigned' },
    { key: 'sku', label: 'Hardware', render: (s) => s.hardwareProfileName },
    { key: 'network', label: 'Network', render: (s) => <Strong title={s.publicIp ?? 'No public IP'} detail={s.primaryMacAddress} /> },
    { key: 'bmc', label: 'BMC', render: (s) => <Mono>{s.bmcAddress}</Mono> },
    { key: 'updated', label: 'Updated', render: (s) => date(s.updatedAt) },
  ]
  return (
    <>
      <PageHeader
        eyebrow="Admin / Servers"
        title="Servers & Inventory"
        subtitle="Add, assign, provision, power, retire, and audit physical bare-metal inventory."
        action={<button className="admin-btn-primary" onClick={onNewServer}><Plus size={14} /> Add server</button>}
      />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
        <DataTable rows={SERVERS} columns={columns} placeholder="Search hostname, asset tag, serial, MAC, BMC, public IP..." />
        {selectedServer ? (
          <ServerDetail server={selectedServer} onDangerousAction={onDangerousAction} />
        ) : (
          <div className="admin-card flex min-h-72 items-center justify-center px-6 text-center text-[13px] text-muted">
            Select a server once inventory is available.
          </div>
        )}
      </div>
    </>
  )
}

function ServerDetail({ server, onDangerousAction }: { server: AdminServerListItem; onDangerousAction: (title: string, detail: string) => void }) {
  const tabs = ['Overview', 'Hardware', 'Networking', 'Assignment', 'Power', 'Provisioning', 'Billing / Service', 'Events']
  return (
    <aside className="admin-card overflow-hidden">
      <div className="border-b border-line px-4 py-3">
        <div className="flex items-start justify-between gap-3">
          <Strong title={server.hostname} detail={`${server.serialNumber} · ${server.assetTag}`} />
          <StatusBadge value={server.status} />
        </div>
      </div>
      <div className="flex gap-1 overflow-x-auto border-b border-line p-2">
        {tabs.map((tab) => (
          <button key={tab} className="rounded px-2 py-1 text-[11.5px] text-gray-500 hover:bg-gray-50">{tab}</button>
        ))}
      </div>
      <div className="grid grid-cols-2 gap-3 p-4 text-[12.5px]">
        <Info label="Organization" value={server.organizationName ?? 'Unassigned'} />
        <Info label="Project" value={server.projectName ?? 'None'} />
        <Info label="Rack" value={`${server.rackName} / ${server.locationName}`} />
        <Info label="Hardware" value={server.hardwareProfileName} />
        <Info label="Public IP" value={server.publicIp ?? 'None'} />
        <Info label="BMC address" value={server.bmcAddress} />
        <Info label="Primary MAC" value={server.primaryMacAddress} />
        <Info label="Provisionable" value={server.provisionable ? 'Yes' : 'No'} />
      </div>
      <div className="grid grid-cols-2 gap-2 border-t border-line p-3">
        <button className="admin-btn" onClick={() => onDangerousAction('Power cycle server', 'This sends a power-cycle command through central backend and rack-agent.')}><Power size={13} /> Power cycle</button>
        <button className="admin-btn" onClick={() => onDangerousAction('Reinstall server', 'This starts a new provisioning workflow and may erase the server.')}><RotateCcw size={13} /> Reinstall</button>
        <button className="admin-btn" onClick={() => onDangerousAction('Retire server', 'Retiring removes this server from allocatable inventory without deleting history.')}><AlertTriangle size={13} /> Retire</button>
      </div>
    </aside>
  )
}

function CreateServerPage({ onBack }: { onBack: () => void }) {
  const required = ['Rack/location', 'Hostname/label', 'Asset tag', 'Serial number', 'Hardware profile/SKU', 'BMC address', 'BMC username secret ref', 'BMC password secret ref', 'Primary MAC address']
  return (
    <>
      <PageHeader eyebrow="Admin / Servers / New" title="Add Server" subtitle="Secret references are stored here, never raw BMC passwords." action={<button className="admin-btn" onClick={onBack}>Back</button>} />
      <div className="admin-card p-4">
        <div className="grid gap-4 md:grid-cols-3">
          {required.map((label) => (
            <label key={label}>
              <span className="admin-label">{label}</span>
              <input className="admin-input mt-1.5" placeholder={label} />
            </label>
          ))}
          <label>
            <span className="admin-label">Provisionable</span>
            <select className="admin-input mt-1.5"><option>true</option><option>false</option></select>
          </label>
          <label className="md:col-span-3">
            <span className="admin-label">Metadata JSON</span>
            <textarea className="admin-input mt-1.5 min-h-28" placeholder='{"cpu":"EPYC 9554","ram_gb":768}' />
          </label>
        </div>
      </div>
    </>
  )
}

function RacksPage() {
  const columns: Array<Column<RackListItem>> = [
    { key: 'rack', label: 'Rack', search: (r) => `${r.name} ${r.location}`, render: (r) => <Strong title={r.name} detail={r.location} /> },
    { key: 'status', label: 'Status', render: (r) => <StatusBadge value={r.status} /> },
    { key: 'heartbeat', label: 'Last heartbeat', render: (r) => date(r.lastHeartbeatAt) },
    { key: 'agent', label: 'Agent', render: (r) => r.agentVersion },
    { key: 'available', label: 'Available', render: (r) => r.availableServers },
    { key: 'active', label: 'Active', render: (r) => r.activeServers },
    { key: 'failed', label: 'Failed jobs', render: (r) => r.failedJobs },
  ]
  return <PageWithTable eyebrow="Admin / Racks" title="Racks & Locations" subtitle="Monitor rack agents, heartbeat state, Tinkerbell health, and maintenance mode." rows={RACKS} columns={columns} placeholder="Search rack or location..." />
}

function ProvisioningJobsPage() {
  const columns: Array<Column<ProvisioningJobListItem>> = [
    { key: 'job', label: 'Job', search: (j) => `${j.id} ${j.server} ${j.organization}`, render: (j) => <Strong title={j.id} detail={j.server} /> },
    { key: 'org', label: 'Organization', render: (j) => j.organization },
    { key: 'rack', label: 'Rack', render: (j) => j.rack },
    { key: 'image', label: 'Image', render: (j) => j.image },
    { key: 'status', label: 'Status', render: (j) => <StatusBadge value={j.status} /> },
    { key: 'requested', label: 'Requested by', render: (j) => j.requestedBy },
    { key: 'failure', label: 'Failure', render: (j) => j.failureReason ?? '—' },
  ]
  return <PageWithTable eyebrow="Admin / Provisioning" title="Provisioning Jobs" subtitle="Inspect command publication, rack events, failure reasons, and job timelines." rows={JOBS} columns={columns} placeholder="Search job, server, org..." />
}

function ImagesPage() {
  return <Placeholder title="OS Images" detail="Image/template management route is reserved. Backend endpoint contracts are ready to wire next." />
}

function AuditPage() {
  const columns: Array<Column<AdminAuditEvent>> = [
    { key: 'time', label: 'Time', render: (e) => date(e.createdAt) },
    { key: 'actor', label: 'Actor', search: (e) => `${e.actor} ${e.action} ${e.target}`, render: (e) => e.actor },
    { key: 'action', label: 'Action', render: (e) => <Mono>{e.action}</Mono> },
    { key: 'target', label: 'Target', render: (e) => e.target },
    { key: 'metadata', label: 'Metadata', render: (e) => Object.entries(e.metadata).map(([k, v]) => `${k}: ${v}`).join(', ') },
  ]
  return <PageWithTable eyebrow="Admin / Audit Log" title="Audit Log" subtitle="Sensitive operator actions with actor, target, reason, and metadata." rows={AUDIT_EVENTS} columns={columns} placeholder="Search actor, action, target..." />
}

function SettingsPage() {
  return <Placeholder title="Admin Settings" detail="Platform settings for grace periods, rack heartbeat thresholds, defaults, and feature flags." />
}

function PageWithTable<T>({ eyebrow, title, subtitle, rows, columns, placeholder }: { eyebrow: string; title: string; subtitle: string; rows: T[]; columns: Array<Column<T>>; placeholder: string }) {
  return (
    <>
      <PageHeader eyebrow={eyebrow} title={title} subtitle={subtitle} />
      <DataTable rows={rows} columns={columns} placeholder={placeholder} />
    </>
  )
}

function RecentPanel({ title, events }: { title: string; events: AdminAuditEvent[] }) {
  return (
    <div className="admin-card">
      <div className="border-b border-line px-4 py-3">
        <h2 className="text-[14px] font-semibold">{title}</h2>
      </div>
      <div className="divide-y divide-line">
        {events.map((event) => (
          <div key={event.id} className="px-4 py-3">
            <p className="text-[12.5px] font-medium text-ink">{event.action}</p>
            <p className="mt-1 text-[12px] text-muted">{event.actor} · {event.target} · {date(event.createdAt)}</p>
          </div>
        ))}
        {events.length === 0 && (
          <div className="px-4 py-10 text-center text-[12.5px] text-muted">
            No admin actions have been recorded yet.
          </div>
        )}
      </div>
    </div>
  )
}

function Placeholder({ title, detail }: { title: string; detail: string }) {
  return (
    <>
      <PageHeader eyebrow="Admin" title={title} subtitle={detail} />
      <div className="admin-card flex min-h-72 items-center justify-center text-[13px] text-muted">No data yet</div>
    </>
  )
}

function Strong({ title, detail }: { title: string; detail?: string }) {
  return (
    <div>
      <p className="font-medium text-ink">{title}</p>
      {detail && <p className="mt-0.5 text-[11.5px] text-muted">{detail}</p>}
    </div>
  )
}

function Mono({ children }: { children: ReactNode }) {
  return <span className="font-mono text-[12px] text-gray-600">{children}</span>
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="admin-label">{label}</p>
      <p className="mt-1 text-ink">{value}</p>
    </div>
  )
}
