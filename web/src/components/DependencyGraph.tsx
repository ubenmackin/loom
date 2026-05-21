import SharpTag from './SharpTag'
import type { Task, StatusType } from '../types'
import { Status } from '../types'
import { statusVariant } from '../utils/statusVariant'

interface DependencyGraphProps {
  storyId: string
  tasks: Task[]
}

// TODO: v2 upgrade to visual graph using SVG/canvas with node-edge layout
// Current v1: list/table format grouped by dependent task

function depStatusDotVariant(status: StatusType): string {
  switch (status) {
    case 'done':
      return 'status-dot-success'
    case 'blocked':
      return 'status-dot-error'
    case 'in_progress':
    case 'ready':
      return 'status-dot-warning'
    default:
      return 'status-dot-info'
  }
}

export default function DependencyGraph({ tasks }: DependencyGraphProps) {
  // Build dependency map: taskId -> list of tasks it depends on
  // For v1, we show a flat list since deps would come from enriched task data
  // In a real API, each task would have a `dependencies` or `blockers` field

  if (!tasks || tasks.length === 0) {
    return (
      <div className="px-4 py-3 border border-gray-200 dark:border-gray-border">
        <span className="font-mono text-xs text-neutral-400 dark:text-neutral-500">
          No tasks to display dependencies
        </span>
      </div>
    )
  }

  // For v1, show all tasks with their status as a dependency overview
  // Blocked tasks are highlighted in red
  const blockedTasks = tasks.filter((t) => t.status === Status.Blocked)
  const doneTasks = tasks.filter((t) => t.status === Status.Done)
  const activeTasks = tasks.filter(
    (t) => t.status !== Status.Blocked && t.status !== Status.Done
  )

  return (
    <div className="px-4 py-3">
      <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-3">
        Dependencies
      </label>

      {/* Summary stats */}
      <div className="flex gap-3 mb-3">
        <div className="flex items-center gap-1">
          <span className="status-dot status-dot-success" />
          <span className="font-mono text-[10px] text-neutral-500 dark:text-neutral-400">
            {doneTasks.length} resolved
          </span>
        </div>
        <div className="flex items-center gap-1">
          <span className="status-dot status-dot-warning" />
          <span className="font-mono text-[10px] text-neutral-500 dark:text-neutral-400">
            {activeTasks.length} pending
          </span>
        </div>
        {blockedTasks.length > 0 && (
          <div className="flex items-center gap-1">
            <span className="status-dot status-dot-error" />
            <span className="font-mono text-[10px] text-neutral-500 dark:text-neutral-400">
              {blockedTasks.length} blocked
            </span>
          </div>
        )}
      </div>

      {/* Dependency table */}
      <div className="border border-gray-200 dark:border-gray-border">
        {/* Header */}
        <div className="grid grid-cols-3 px-4 py-3 border-b border-gray-200 dark:border-gray-border bg-neutral-50 dark:bg-neutral-900/50">
          <span className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
            Task
          </span>
          <span className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
            Depends On
          </span>
          <span className="text-[10px] uppercase tracking-widest text-neutral-500 dark:text-neutral-400">
            Status
          </span>
        </div>

        {/* Rows */}
        {tasks.map((task) => (
          <div
            key={task.id}
            className="grid grid-cols-3 px-4 py-1 border-b border-gray-200 dark:border-gray-border last:border-b-0"
          >
            <div className="flex items-center gap-2">
              <span className={`status-dot ${depStatusDotVariant(task.status)}`} />
              <span className="font-mono text-xs text-neutral-800 dark:text-light-neutral">
                {task.id}
              </span>
            </div>
            <div className="flex items-center gap-1">
              {/* In v1, show placeholder — real deps come from API */}
              <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-600">
                —
              </span>
            </div>
            <div>
              <SharpTag
                label={task.status.toUpperCase()}
                variant={statusVariant(task.status)}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
