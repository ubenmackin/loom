import SharpTag from './SharpTag'
import type { Task } from '../types'
import { Status } from '../types'
import { statusVariant, statusDotClass } from '../utils/status'

interface DependencyGraphProps {
  storyId: string
  tasks: Task[]
}

// v1: Displays a task status overview grouped by state (blocked, done, active)
// v2: Planned upgrade to visual graph using SVG/canvas with node-edge layout

export default function DependencyGraph({ tasks }: DependencyGraphProps) {
  if (!tasks || tasks.length === 0) {
    return (
      <div className="px-4 py-3 border border-gray-200 dark:border-gray-border">
        <span className="font-mono text-xs text-neutral-400 dark:text-neutral-500">
          No tasks to display
        </span>
      </div>
    )
  }

  const blockedTasks = tasks.filter((t) => t.status === Status.Blocked)
  const doneTasks = tasks.filter((t) => t.status === Status.Done)
  const activeTasks = tasks.filter(
    (t) => t.status !== Status.Blocked && t.status !== Status.Done
  )

  return (
    <div className="px-4 py-3">
      <label className="text-[10px] uppercase tracking-widest dark:text-amber-primary text-neutral-500 block mb-3">
        Task Status Overview
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

      {/* Task table */}
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
              <span className={statusDotClass(task.status)} />
              <span className="font-mono text-xs text-neutral-800 dark:text-light-neutral">
                {task.id}
              </span>
            </div>
            <div className="flex items-center gap-1">
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
