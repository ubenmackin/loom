import type { Story, Task, StatusType } from '../types'
import { Status } from '../types'
import { statusDotClass } from '../utils/status'

const COLUMNS: { status: StatusType; label: string }[] = [
  { status: Status.New, label: 'New' },
  { status: Status.Ready, label: 'Ready' },
  { status: Status.InProgress, label: 'In Progress' },
  { status: Status.Blocked, label: 'Blocked' },
  { status: Status.Done, label: 'Done' },
]

interface GridHeaderProps {
  displayStories: Story[]
  tasksByStoryAndStatus: Record<string, Record<string, Task[]>>
}

export default function GridHeader({ displayStories, tasksByStoryAndStatus }: GridHeaderProps) {
  return (
    <div className="flex shrink-0 border-b border-gray-200 dark:border-gray-border">
      {/* Story header */}
      <div className="min-w-[240px] md:min-w-0 md:w-1/6 flex items-center gap-2 px-4 py-3 border-r border-gray-200 dark:border-gray-border">
        <span className="text-[10px] uppercase tracking-wider font-bold text-neutral-600 dark:text-neutral-300">
          Story
        </span>
        <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500 ml-auto">
          [{displayStories.length}]
        </span>
      </div>
      {/* Status column headers */}
      {COLUMNS.map((col, i) => {
        let totalCount = 0
        for (const story of displayStories) {
          totalCount += (tasksByStoryAndStatus[story.id]?.[col.status] ?? []).length
        }
        return (
          <div
            key={col.status}
            className={`min-w-[200px] md:min-w-0 md:flex-1 flex items-center gap-2 px-4 py-3 ${
              i < COLUMNS.length - 1 ? 'border-r border-gray-200 dark:border-gray-border' : ''
            }`}
          >
            <span className={statusDotClass(col.status)} />
            <span className="text-[10px] uppercase tracking-wider font-bold text-neutral-600 dark:text-neutral-300">
              {col.label}
            </span>
            <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500 ml-auto">
              [{totalCount}]
            </span>
          </div>
        )
      })}
    </div>
  )
}