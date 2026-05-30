import { memo } from 'react'
import TaskCardBody from './TaskCardBody'
import type { Task } from '../types'

interface TaskDragPreviewProps {
  task: Task
}

function TaskDragPreview({ task }: TaskDragPreviewProps) {
  return (
    <div className="border border-gray-200 dark:border-gray-border p-2 rounded-none shadow-none bg-white dark:bg-charcoal-dark cursor-pointer">
      <TaskCardBody task={task} />
    </div>
  )
}

export default memo(TaskDragPreview)