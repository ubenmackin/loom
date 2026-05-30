import { memo, useMemo } from 'react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import TaskCardBody from './TaskCardBody'
import type { Task } from '../types'

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

  const style = useMemo(
    () => ({
      transform: CSS.Transform.toString(transform),
      transition,
      opacity: isDragging ? 0.5 : 1,
    }),
    [transform, transition, isDragging],
  )

  // Always pass sortable props — when isDraggable is false, useSortable disables itself
  // and provides no-op attribute/listener values.
  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      className="border border-gray-200 dark:border-gray-border p-2 rounded-none shadow-none bg-white dark:bg-charcoal-dark cursor-pointer"
      onClick={() => onClick?.(task.id)}
    >
      <TaskCardBody task={task} />
    </div>
  )
}

export default memo(TaskCard)
