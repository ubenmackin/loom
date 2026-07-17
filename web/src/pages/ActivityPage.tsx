import { useState } from 'react'
import { useActivity } from '../hooks/useActivity'
import { relativeTime } from '../utils/relativeTime'
import type { ActivityLogEntry } from '../types'
import StoryDetail from '../components/StoryDetail'
import AsyncBoundary from '../components/AsyncBoundary'

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

// Format a work item reference as "STO-{id}: {title}" or "TSK-{id}: {title}".
// Uses a short slice of the UUID as the id portion (the API does not expose
// the numeric id in the activity response, so we fall back to a stable short
// identifier derived from the work_item_id).
function formatWorkItemLabel(workItemType: string, workItemId: string, title: string): string {
  const prefix = workItemType === 'story' ? 'STO' : 'TSK'
  const shortId = workItemId.length >= 8 ? workItemId.slice(0, 8) : workItemId
  return `${prefix}-${shortId}: ${title}`
}

function ProjectPill({ name }: { name: string }) {
  if (!name) return null
  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-mono uppercase tracking-wider bg-purple-50 text-purple-700 dark:bg-purple-active/10 dark:text-purple-active border border-purple-200 dark:border-purple-active/30">
      {name}
    </span>
  )
}

function ActivityItem({ entry, onStoryClick }: { entry: ActivityLogEntry; onStoryClick: (storyId: string) => void }) {
  const label = formatWorkItemLabel(entry.work_item_type, entry.work_item_id, entry.work_item_title)

  return (
    <div className="flex items-start gap-3 px-4 py-3 border-b border-gray-200 dark:border-gray-border hover:bg-gray-50 dark:hover:bg-charcoal-darkest transition-colors">
      {/* Timestamp */}
      <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500 whitespace-nowrap pt-0.5">
        {relativeTime(entry.created_at)}
      </span>

      {/* Action badge */}
      <ActionBadge action={entry.action} />

      {/* Project name pill */}
      <ProjectPill name={entry.project_name} />

      {/* Work item reference */}
      <span className="font-mono text-xs text-neutral-600 dark:text-light-neutral">
        {entry.work_item_type === 'story' ? (
          <button
            onClick={() => onStoryClick(entry.work_item_id)}
            className="text-purple-active cursor-pointer hover:underline font-bold"
            title={entry.work_item_id}
          >
            {label}
          </button>
        ) : (
          <span className="text-purple-active" title={entry.work_item_id}>
            {label}
          </span>
        )}
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
  const [selectedStoryId, setSelectedStoryId] = useState<string | null>(null)

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center px-4 py-3 border-b border-gray-200 dark:border-gray-border">
        <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500">
          [{entries?.length ?? 0}]
        </span>
      </div>

      {/* Activity List */}
      <div className="flex-1 overflow-y-auto">
        <AsyncBoundary
          isLoading={isLoading}
          error={error}
          onRetry={refetch}
          isEmpty={!entries || entries.length === 0}
          emptyMessage="No activity yet"
        >
          {entries?.map((entry) => (
            <ActivityItem key={entry.id} entry={entry} onStoryClick={setSelectedStoryId} />
          ))}
        </AsyncBoundary>
      </div>

      {selectedStoryId && <StoryDetail storyId={selectedStoryId} onClose={() => setSelectedStoryId(null)} />}
    </div>
  )
}
