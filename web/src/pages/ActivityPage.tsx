import { useActivity } from '../hooks/useActivity'
import { relativeTime } from '../utils/relativeTime'
import type { ActivityLogEntry } from '../types'

function actionColor(action: string): string {
  switch (action) {
    case 'created':
      return 'text-green-500'
    case 'updated':
      return 'text-blue-400'
    case 'deleted':
      return 'text-red-500'
    case 'status_changed':
      return 'text-amber-500'
    default:
      return 'text-neutral-500'
  }
}

function ActionBadge({ action }: { action: string }) {
  return (
    <span
      className={`font-mono text-[10px] uppercase tracking-wider ${actionColor(action)}`}
    >
      {action.replace('_', ' ')}
    </span>
  )
}

function ActivityItem({ entry }: { entry: ActivityLogEntry }) {
  return (
    <div className="flex items-start gap-3 px-4 py-3 border-b border-gray-200 dark:border-gray-border hover:bg-gray-50 dark:hover:bg-charcoal-darkest transition-colors">
      {/* Timestamp */}
      <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500 whitespace-nowrap pt-0.5">
        {relativeTime(entry.created_at)}
      </span>

      {/* Action badge */}
      <ActionBadge action={entry.action} />

      {/* Work item reference */}
      <span className="font-mono text-xs text-neutral-600 dark:text-light-neutral">
        <span className="text-neutral-400 dark:text-neutral-500">
          {entry.work_item_type}
        </span>{' '}
        <span className="text-purple-active">
          {entry.work_item_id.slice(0, 8)}
        </span>
      </span>

      {/* Details */}
      {entry.details && (
        <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500 truncate">
          {entry.details}
        </span>
      )}
    </div>
  )
}

export default function ActivityPage() {
  const { data: entries, isLoading, error, refetch } = useActivity()

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <span className="font-mono text-sm text-neutral-500 dark:text-amber-muted">
          Loading activity...
        </span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-2">
        <span className="font-mono text-sm text-red-500">
          Error loading activity: {error.message}
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

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-border">
        <span className="text-[10px] uppercase tracking-widest font-bold text-neutral-600 dark:text-neutral-300">
          Activity
        </span>
        <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500">
          [{entries?.length ?? 0}]
        </span>
      </div>

      {/* Activity List */}
      <div className="flex-1 overflow-y-auto">
        {entries && entries.length > 0 ? (
          entries.map((entry) => <ActivityItem key={entry.id} entry={entry} />)
        ) : (
          <div className="flex items-center justify-center py-16">
            <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-600 uppercase tracking-widest">
              No activity yet
            </span>
          </div>
        )}
      </div>
    </div>
  )
}
