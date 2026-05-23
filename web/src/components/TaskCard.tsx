import { memo } from 'react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import SharpTag from './SharpTag'
import type { Task } from '../types'
import { taskTypeLabel, taskTypeVariant } from '../utils/taskType'

interface TaskCardProps {
  task: Task
  onClick?: (taskId: string) => void
  isDraggable?: boolean
}

function TaskCard({ task, onClick, isDraggable = false }: TaskCardProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: task.id,
    disabled: !isDraggable,
    data: {
      type: 'task',
      task,
      storyId: task.story_id,
      status: task.status,
      sortOrder: task.sort_order,
    },
  })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  }

  const sortableProps = isDraggable
    ? { ref: setNodeRef, style, ...attributes, ...listeners }
    : {}

  return (
    <div
      className="border border-gray-200 dark:border-gray-border p-2 rounded-none shadow-none bg-white dark:bg-charcoal-dark cursor-pointer"
      onClick={() => onClick?.(task.id)}
      {...sortableProps}
    >
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
    </div>
  )
}

export default memo(TaskCard)
