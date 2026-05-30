import SharpTag from './SharpTag'
import type { Task } from '../types'
import { taskTypeLabel, taskTypeVariant } from '../utils/taskType'

/** The visual body of a TaskCard — shared between TaskCard and TaskDragPreview */
export default function TaskCardBody({ task }: { task: Task }) {
  return (
    <>
      {/* Title */}
      <div className="text-xs font-bold text-neutral-800 dark:text-light-neutral leading-tight">
        {task.title}
      </div>

      {/* Tags row */}
      <div className="flex items-center gap-1.5 mt-1.5 flex-wrap">
        <SharpTag label={taskTypeLabel(task.task_type)} variant={taskTypeVariant(task.task_type)} />
      </div>

      {/* Dependency count */}
      {task.is_stale && (
        <div className="mt-1 flex items-center gap-1">
          <span className="status-dot status-dot-warning" />
          <span className="font-mono text-[10px] text-amber-500">stale</span>
        </div>
      )}

      {/* Unread comments indicator */}
      {task.has_unread_comments && (
        <div className="mt-1 flex items-center gap-1">
          <span className="status-dot status-dot-info" />
          <span className="font-mono text-[10px] text-sky-500">unread</span>
        </div>
      )}

      {/* Blocked indicator */}
      {task.status === 'blocked' && (
        <div className="mt-1 flex items-center gap-1">
          <span className="status-dot status-dot-error" />
          <span className="mono-bracket">blocked</span>
        </div>
      )}

      {/* Assigned agent */}
      {task.assigned_to && (
        <div className="mt-0.5 font-mono text-[10px] dark:text-amber-primary text-neutral-500">
          {task.assigned_to}
        </div>
      )}
    </>
  )
}