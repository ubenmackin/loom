import { memo } from 'react'
import { Plus } from 'lucide-react'
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import StoryCard from './StoryCard'
import TaskCard from './TaskCard'
import { CellDropZone } from './Column'
import type { Story, Task, StatusType } from '../types'
import { Status } from '../types'

const COLUMNS: { status: StatusType; label: string }[] = [
  { status: Status.New, label: 'New' },
  { status: Status.Ready, label: 'Ready' },
  { status: Status.InProgress, label: 'In Progress' },
  { status: Status.Blocked, label: 'Blocked' },
  { status: Status.Done, label: 'Done' },
]

interface SortableStoryRowProps {
  story: Story
  tasksByStoryAndStatus: Record<string, Record<string, Task[]>>
  displayTaskOrder: Record<string, string[]>
  allTasks: Task[]
  onStoryClick: (id: string) => void
  onTaskClick: (id: string) => void
  assigneeNameMap: Record<string, string>
  handleAddTask: (storyId: string) => void
}

function SortableStoryRow({
  story,
  tasksByStoryAndStatus,
  displayTaskOrder,
  allTasks,
  onStoryClick,
  onTaskClick,
  assigneeNameMap,
  handleAddTask,
}: SortableStoryRowProps) {
  const {
    attributes, listeners, setNodeRef, transform, transition, isDragging,
  } = useSortable({
    id: story.id,
    data: { type: 'story', story },
  })
  const rowStyle = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  }
  return (
    <div
      ref={setNodeRef}
      style={rowStyle}
      {...attributes}
      {...listeners}
      className="flex items-stretch border-b border-gray-100 dark:border-gray-border/50"
    >
      {/* Story Card cell */}
      <div className="min-w-[240px] md:min-w-0 md:w-1/6 p-2 border-r border-gray-200 dark:border-gray-border">
        <StoryCard
          story={story}
          onClick={() => onStoryClick(story.id)}
          assigneeName={story.assigned_to ? assigneeNameMap[story.assigned_to] || story.assigned_to : undefined}
        />
      </div>
      {/* Status column cells */}
      {COLUMNS.map((col, colIdx) => {
        const cellTasks = (tasksByStoryAndStatus[story.id]?.[col.status] ?? []).sort(
          (a, b) => a.sort_order - b.sort_order,
        )
        const droppableId = `cell-${story.id}-${col.status}`
        const cellKey = `${story.id}:${col.status}`
        const orderedIds = displayTaskOrder[cellKey]
        const taskIds = orderedIds ?? cellTasks.map((t) => t.id)
        return (
          <div
            key={col.status}
            className={`min-w-[200px] md:min-w-0 md:flex-1 p-1.5 ${
              colIdx < COLUMNS.length - 1 ? 'border-r border-gray-200 dark:border-gray-border' : ''
            }`}
          >
            <CellDropZone id={droppableId} storyId={story.id} status={col.status}>
              {taskIds.length > 0 ? (
                <SortableContext items={taskIds} strategy={verticalListSortingStrategy}>
                  <div className="space-y-1.5">
                    {taskIds.map((taskId) => {
                      const task = cellTasks.find((t) => t.id === taskId) ?? allTasks.find((t) => t.id === taskId)
                      if (!task) return null
                      return (
                        <TaskCard
                          key={task.id}
                          task={task}
                          onClick={(id) => onTaskClick(id)}
                          isDraggable={true}
                        />
                      )
                    })}
                  </div>
                </SortableContext>
              ) : (
                <div className="flex items-center justify-center py-3">
                  <span className="font-mono text-[10px] text-neutral-300 dark:text-neutral-600 uppercase tracking-widest">
                    —
                  </span>
                </div>
              )}
              {col.status === Status.New && (
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    handleAddTask(story.id)
                  }}
                  className="w-full mt-1 flex items-center justify-center gap-1 py-1 border border-dashed border-neutral-300 dark:border-neutral-600 text-neutral-400 dark:text-neutral-500 hover:text-neutral-600 dark:hover:text-neutral-300 hover:border-neutral-400 dark:hover:border-neutral-500 transition-colors rounded-none"
                >
                  <Plus size={12} />
                  <span className="font-mono text-[10px] uppercase tracking-widest">Add Task</span>
                </button>
              )}
            </CellDropZone>
          </div>
        )
      })}
    </div>
  )
}

export default memo(SortableStoryRow, (prev, next) => {
  // Custom comparator — only re-render if relevant props changed
  if (prev.story !== next.story) return false
  if (prev.allTasks !== next.allTasks) return false
  if (prev.assigneeNameMap !== next.assigneeNameMap) return false
  // Deep-compare displayTaskOrder for this story's keys
  const prevOrder = prev.displayTaskOrder
  const nextOrder = next.displayTaskOrder
  for (const col of COLUMNS) {
    const key = `${prev.story.id}:${col.status}`
    if (prevOrder[key] !== nextOrder[key]) return false
  }
  // Deep-compare tasksByStoryAndStatus for this story
  const prevTasks = prev.tasksByStoryAndStatus[prev.story.id]
  const nextTasks = next.tasksByStoryAndStatus[next.story.id]
  if (prevTasks !== nextTasks) return false
  return true
})