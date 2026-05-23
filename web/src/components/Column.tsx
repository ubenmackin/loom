import { memo } from 'react'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { useDroppable } from '@dnd-kit/core'
import type { Story, Task, StatusType } from '../types'
import TaskCard from './TaskCard'
import { statusDotClass } from '../utils/status'

interface ColumnProps {
  status: StatusType
  label: string
  stories: Story[]
  tasksByStory: Record<string, Task[]>
  onTaskClick?: (taskId: string) => void
}

function Column({ status, label, stories, tasksByStory, onTaskClick }: ColumnProps) {
  let totalCount = 0
  for (const story of stories) {
    totalCount += (tasksByStory[story.id] ?? []).length
  }

  return (
    <div className="flex flex-col h-full">
      {/* Column Header */}
      <div className="flex items-center gap-2 px-4 py-3 border-b border-gray-200 dark:border-gray-border">
        <span className={statusDotClass(status)} />
        <span className="text-[10px] uppercase tracking-wider font-bold text-neutral-600 dark:text-neutral-300">
          {label}
        </span>
        <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500 ml-auto">
          [{totalCount}]
        </span>
      </div>

      {/* Swimlane Rows */}
      <div className="flex-1 overflow-y-auto">
        {stories.map((story) => {
          const cellTasks = (tasksByStory[story.id] ?? []).sort(
            (a, b) => a.priority - b.priority,
          )
          const droppableId = `cell-${story.id}-${status}`

          return (
            <div
              key={droppableId}
              className="border-b border-gray-100 dark:border-gray-border/50"
            >
              <CellDropZone id={droppableId} storyId={story.id} status={status}>
                {cellTasks.length > 0 && (
                  <SortableContext
                    items={cellTasks.map((t) => t.id)}
                    strategy={verticalListSortingStrategy}
                  >
                    <div className="p-1.5 space-y-1.5">
                      {cellTasks.map((task) => (
                        <TaskCard
                          key={task.id}
                          task={task}
                          onClick={onTaskClick}
                          isDraggable={true}
                        />
                      ))}
                    </div>
                  </SortableContext>
                )}
                {cellTasks.length === 0 && (
                  <div className="flex items-center justify-center py-3">
                    <span className="font-mono text-[10px] text-neutral-300 dark:text-neutral-600 uppercase tracking-widest">
                      —
                    </span>
                  </div>
                )}
              </CellDropZone>
            </div>
          )
        })}
        {stories.length === 0 && (
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

export default memo(Column)

/** Internal droppable wrapper for a single (story × status) cell */
function CellDropZone({
  id,
  storyId,
  status,
  children,
}: {
  id: string
  storyId: string
  status: StatusType
  children: React.ReactNode
}) {
  const { setNodeRef } = useDroppable({
    id,
    data: {
      type: 'cell',
      storyId,
      status,
    },
  })

  return (
    <div ref={setNodeRef} className="min-h-[40px]">
      {children}
    </div>
  )
}
