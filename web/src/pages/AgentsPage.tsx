import { useSessions } from '../hooks/useSessions'
import { SessionStatus, type Session, type SessionStatusType } from '../types'
import SharpTag from '../components/SharpTag'
import { relativeTime } from '../utils/relativeTime'

function statusDotClass(status: SessionStatusType): string {
  switch (status) {
    case SessionStatus.Active:
      return 'status-dot status-dot-success'
    case SessionStatus.Stale:
      return 'status-dot status-dot-warning'
    case SessionStatus.Disconnected:
      return 'status-dot'
    default:
      return 'status-dot'
  }
}

function parseCapabilities(capStr?: string): string[] {
  if (!capStr) return []
  try {
    const parsed = JSON.parse(capStr)
    if (Array.isArray(parsed)) return parsed
    return []
  } catch {
    return []
  }
}

function SessionCard({ session }: { session: Session }) {
  const caps = parseCapabilities(session.capabilities)
  return (
    <div className="border border-gray-200 dark:border-gray-border bg-white dark:bg-charcoal-dark p-4 flex flex-col gap-3">
      {/* Header: ID + Status */}
      <div className="flex items-center justify-between">
        <span
          className="font-mono text-xs text-purple-active truncate"
          title={session.id}
        >
          {session.id.slice(0, 12)}…
        </span>
        <span className={statusDotClass(session.status)} />
      </div>

      {/* Harness type badge */}
      <div>
        <span className="sharp-tag sharp-tag-primary">{session.harness_type}</span>
      </div>

      {/* Capabilities */}
      {caps.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {caps.map((cap) => (
            <SharpTag key={cap} label={cap} variant="amber" />
          ))}
        </div>
      )}

      {/* Timestamps */}
      <div className="flex flex-col gap-0.5 mt-auto">
        <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500">
          last seen {relativeTime(session.last_seen_at)}
        </span>
        <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500">
          created {relativeTime(session.created_at)}
        </span>
      </div>
    </div>
  )
}

const STATUS_ORDER: SessionStatusType[] = [
  SessionStatus.Active,
  SessionStatus.Stale,
  SessionStatus.Disconnected,
]

export default function AgentsPage() {
  const { data: sessions, isLoading, error, refetch } = useSessions()

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <span className="font-mono text-sm text-neutral-500 dark:text-amber-muted">
          Loading agents...
        </span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-2">
        <span className="font-mono text-sm text-red-500">
          Error loading agents: {error.message}
        </span>
        <button
          onClick={() => refetch()}
          className="glow-button text-xs"
        >
          Retry
        </button>
      </div>
    )
  }

  // Group sessions by status
  const grouped: Record<string, Session[]> = {}
  for (const s of sessions ?? []) {
    if (!grouped[s.status]) grouped[s.status] = []
    grouped[s.status].push(s)
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-border">
        <span className="text-[10px] uppercase tracking-widest font-bold text-neutral-600 dark:text-neutral-300">
          Agents
        </span>
        <div className="flex gap-3">
          {STATUS_ORDER.map((status) => {
            const count = grouped[status]?.length ?? 0
            return (
              <span
                key={status}
                className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500"
              >
                {status}: [{count}]
              </span>
            )
          })}
        </div>
      </div>

      {/* Agent Cards — grouped by status */}
      <div className="flex-1 overflow-y-auto p-4">
        {sessions && sessions.length > 0 ? (
          STATUS_ORDER.map((status) => {
            const group = grouped[status]
            if (!group || group.length === 0) return null
            return (
              <div key={status} className="mb-6">
                <div className="flex items-center gap-2 mb-3">
                  <span className={statusDotClass(status)} />
                  <span className="text-[10px] uppercase tracking-widest font-bold text-neutral-600 dark:text-neutral-300">
                    {status}
                  </span>
                  <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500">
                    [{group.length}]
                  </span>
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
                  {group.map((session) => (
                    <SessionCard key={session.id} session={session} />
                  ))}
                </div>
              </div>
            )
          })
        ) : (
          <div className="flex items-center justify-center py-16">
            <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-600 uppercase tracking-widest">
              No agents connected
            </span>
          </div>
        )}
      </div>
    </div>
  )
}
