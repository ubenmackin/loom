import type { Task, StatusType } from '../types'
import TaskCard from './TaskCard'

interface ColumnProps {
  status: StatusType
  label: string
  items: Task[]
}

function statusDotClass(status: StatusType): string {
  switch (status) {
    case 'new':
      return 'status-dot status-dot-info'
    case 'ready':
      return 'status-dot status-dot-info'
    case 'in_progress':
      return 'status-dot status-dot-warning status-dot-pulse'
    case 'blocked':
      return 'status-dot status-dot-error'
    case 'done':
      return 'status-dot status-dot-success'
    default:
      return 'status-dot'
  }
}

export default function Column({ status, label, items }: ColumnProps) {
  const count = items.length

  return (
    <div className="flex flex-col h-full">
      {/* Column Header */}
      <div className="flex items-center gap-2 px-4 py-3 border-b border-gray-200 dark:border-gray-border">
        <span className={statusDotClass(status)} />
        <span className="text-[10px] uppercase tracking-wider font-bold text-neutral-600 dark:text-neutral-300">
          {label}
        </span>
        <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500 ml-auto">
          [{count}]
        </span>
      </div>

      {/* Cards */}
      <div className="flex-1 overflow-y-auto p-2 space-y-2">
        {items
          .sort((a, b) => a.priority - b.priority)
          .map((task) => (
            <TaskCard key={task.id} task={task} />
          ))}
        {count === 0 && (
          <div className="flex items-center justify-center py-8">
            <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-600 uppercase tracking-widest">
              Empty
            </span>
          </div>
        )}
      </div>
    </div>
  )
}
