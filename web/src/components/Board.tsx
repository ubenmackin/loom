import { useState, useMemo, useCallback } from 'react'
import { Plus } from 'lucide-react'
import {
  DndContext,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  verticalListSortingStrategy,
  arrayMove,
} from '@dnd-kit/sortable'
import { useBoard } from '../hooks/useBoard'
import { useCreateStory } from '../hooks/useCreateStory'
import StoryCard from './StoryCard'
import TaskCard from './TaskCard'
import { CellDropZone } from './Column'
import StoryDetail from './StoryDetail'
import TaskDetail from './TaskDetail'
import CreateStoryForm from './CreateStoryForm'
import type { CreateStoryData } from './CreateStoryForm'
import { Status, type StatusType, type Story, type Task, type User, type Session } from '../types'
import { updateStory, updateTask, updateTaskStatus, getUsers, fetchSessions } from '../api/client'
import { statusDotClass } from '../utils/status'
import { useQuery, useQueryClient } from '@tanstack/react-query'

const COLUMNS: { status: StatusType; label: string }[] = [
  { status: Status.New, label: 'New' },
  { status: Status.Ready, label: 'Ready' },
  { status: Status.InProgress, label: 'In Progress' },
  { status: Status.Blocked, label: 'Blocked' },
  { status: Status.Done, label: 'Done' },
]



/** Type-safe access to drag event data — replaces raw `as` assertions */
function getDragData<T>(data: unknown, key: string): T | undefined {
  if (data && typeof data === 'object' && key in data) {
    return (data as Record<string, unknown>)[key] as T
  }
  return undefined
}

export default function Board() {
  const { data, isLoading, error } = useBoard()
  const [isFormOpen, setIsFormOpen] = useState(false)
  const createStoryMutation = useCreateStory()
  const queryClient = useQueryClient()

  const { data: users = [] } = useQuery<User[]>({
    queryKey: ['users'],
    queryFn: getUsers,
  })

  const { data: sessions = [] } = useQuery<Session[]>({
    queryKey: ['sessions'],
    queryFn: fetchSessions,
  })

  const [selectedStoryId, setSelectedStoryId] = useState<string | null>(null)
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null)

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 5 },
    }),
  )

  const handleCreate = useCallback(
    (data: CreateStoryData) => {
      createStoryMutation.mutate(data, {
        onSuccess: () => {
          setIsFormOpen(false)
        },
        onError: (error) => {
          console.error('Failed to create story:', error)
        },
      })
    },
    [createStoryMutation],
  )

  const stories: Story[] = useMemo(
    () => (data?.stories ?? []).sort((a, b) => a.sort_order - b.sort_order),
    [data?.stories],
  )

  const tasksByStoryAndStatus = useMemo(
    () => data?.tasks_by_story_and_status ?? {},
    [data?.tasks_by_story_and_status],
  )

  const allTasks: Task[] = useMemo(() => {
    const tasks: Task[] = []
    if (data?.tasks_by_status) {
      for (const statusList of Object.values(data.tasks_by_status)) {
        for (const task of statusList) {
          tasks.push(task)
        }
      }
    }
    return tasks
  }, [data?.tasks_by_status])

  const assigneeNameMap: Record<string, string> = useMemo(() => {
    const map: Record<string, string> = {}
    for (const u of users) {
      map[u.id] = u.display_name || u.username
    }
    for (const s of sessions) {
      map[s.id] = s.id
    }
    return map
  }, [users, sessions])

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event
      if (!over || active.id === over.id) return

      const activeDataType = getDragData<string>(active.data.current, 'type')

      if (activeDataType === 'story') {
        // Reorder stories
        const oldIndex = stories.findIndex((s) => s.id === active.id)
        const newIndex = stories.findIndex((s) => s.id === over.id)
        if (oldIndex === -1 || newIndex === -1) return

        const newStories = arrayMove(stories, oldIndex, newIndex)
        newStories.forEach((story, idx) => {
          if (story.sort_order !== idx) {
            updateStory(story.id, { sort_order: idx }).catch((err) =>
              console.error('Failed to update story sort_order:', err),
            )
          }
        })
        queryClient.invalidateQueries({ queryKey: ['board'] })
      } else if (activeDataType === 'task') {
        const taskId = String(active.id)
        const sourceStoryId = getDragData<string>(active.data.current, 'storyId')
        const sourceStatus = getDragData<string>(active.data.current, 'status')
        const sourceSortOrder = getDragData<number>(active.data.current, 'sortOrder') ?? 0

        // Determine target story_id and status from the over droppable
        const targetData = over.data.current
        const targetStoryId = getDragData<string>(targetData, 'storyId') ?? sourceStoryId
        const targetStatus = getDragData<StatusType>(targetData, 'status') ?? sourceStatus

        // If dropping onto another task, get its story_id and status
        let targetSortOrder = 0
        if (getDragData<string>(targetData, 'type') === 'task') {
          const overTaskId = String(over.id)
          const overTask = allTasks.find((t) => t.id === overTaskId)
          if (overTask) {
            targetSortOrder = overTask.sort_order
          }
        }

        // Build the update payload
        const updates: Partial<Task> = {}

        // Sort order: place before the target
        if (getDragData<string>(targetData, 'type') === 'task') {
          updates.sort_order = targetSortOrder
        } else {
          // Dropped on a drop zone cell — append to end
          const cellTasks =
            tasksByStoryAndStatus[targetStoryId ?? '']?.[targetStatus ?? ''] ?? []
          updates.sort_order = cellTasks.length > 0 ? Math.max(...cellTasks.map((t) => t.sort_order)) + 1 : 0
        }

        // Status change
        if (targetStatus && targetStatus !== sourceStatus) {
          updateTaskStatus(taskId, targetStatus).catch((err) =>
            console.error('Failed to update task status:', err),
          )
        }

        // Story re-parent
        if (targetStoryId && targetStoryId !== sourceStoryId) {
          updates.story_id = targetStoryId
        }

        if (Object.keys(updates).length > 0) {
          updateTask(taskId, updates).catch((err) =>
            console.error('Failed to update task:', err),
          )
        }

        queryClient.invalidateQueries({ queryKey: ['board'] })
      }
    },
    [stories, allTasks, tasksByStoryAndStatus, queryClient],
  )

  const handleAddTask = useCallback(
    (storyId: string) => {
      setSelectedTaskId(`new-task-${storyId}`)
    },
    [],
  )

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <span className="font-mono text-sm text-neutral-500 dark:text-amber-muted">
          Loading board...
        </span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-64">
        <span className="font-mono text-sm text-red-500">
          Error loading board: {error.message}
        </span>
      </div>
    )
  }

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragEnd={handleDragEnd}
    >
      <div className="flex flex-col h-full">
        {/* Board Header */}
        <div className="flex items-center justify-between px-4 py-2 border-b border-gray-200 dark:border-gray-border">
          <span className="text-[10px] uppercase tracking-widest font-bold text-neutral-600 dark:text-neutral-300">
            Board
          </span>
          <button
            onClick={() => setIsFormOpen(true)}
            className="glow-button flex items-center gap-1.5 text-xs"
          >
            <Plus size={14} />
            Add Story
          </button>
        </div>

      {/* Swimlane Grid */}
      <div className="flex flex-col flex-1 min-h-0">
        {/* Frozen Header Row */}
        <div className="flex shrink-0 border-b border-gray-200 dark:border-gray-border">
          {/* Story header */}
          <div className="min-w-[240px] md:min-w-0 md:w-1/6 flex items-center gap-2 px-4 py-3 border-r border-gray-200 dark:border-gray-border">
            <span className="text-[10px] uppercase tracking-wider font-bold text-neutral-600 dark:text-neutral-300">
              Story
            </span>
            <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500 ml-auto">
              [{stories.length}]
            </span>
          </div>
          {/* Status column headers */}
          {COLUMNS.map((col, i) => {
            let totalCount = 0
            for (const story of stories) {
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

        {/* Scrollable Body */}
        <div className="flex-1 overflow-y-auto">
          <SortableContext items={stories.map((s) => s.id)} strategy={verticalListSortingStrategy}>
            {stories.map((story) => (
              <div key={story.id} className="flex items-stretch border-b border-gray-100 dark:border-gray-border/50">
                {/* Story Card cell */}
                <div className="min-w-[240px] md:min-w-0 md:w-1/6 p-2 border-r border-gray-200 dark:border-gray-border">
                  <StoryCard
                    story={story}
                    isDraggable={true}
                    onClick={() => setSelectedStoryId(story.id)}
                    assigneeName={story.assigned_to ? assigneeNameMap[story.assigned_to] || story.assigned_to : undefined}
                  />
                </div>
                {/* Status column cells */}
                {COLUMNS.map((col, colIdx) => {
                  const cellTasks = (tasksByStoryAndStatus[story.id]?.[col.status] ?? []).sort(
                    (a, b) => a.priority - b.priority,
                  )
                  const droppableId = `cell-${story.id}-${col.status}`
                  return (
                    <div
                      key={col.status}
                      className={`min-w-[200px] md:min-w-0 md:flex-1 p-1.5 ${
                        colIdx < COLUMNS.length - 1 ? 'border-r border-gray-200 dark:border-gray-border' : ''
                      }`}
                    >
                      <CellDropZone id={droppableId} storyId={story.id} status={col.status}>
                        {cellTasks.length > 0 ? (
                          <SortableContext items={cellTasks.map((t) => t.id)} strategy={verticalListSortingStrategy}>
                            <div className="space-y-1.5">
                              {cellTasks.map((task) => (
                                <TaskCard
                                  key={task.id}
                                  task={task}
                                  onClick={(taskId) => setSelectedTaskId(taskId)}
                                  isDraggable={true}
                                />
                              ))}
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
            ))}
          </SortableContext>
          {stories.length === 0 && (
            <div className="flex items-center justify-center py-8">
              <span className="font-mono text-[10px] text-neutral-400 dark:text-neutral-600 uppercase tracking-widest">
                Empty
              </span>
            </div>
          )}
        </div>
      </div>

        {/* Create Story Modal */}
        <CreateStoryForm
          open={isFormOpen}
          onSubmit={handleCreate}
          onCancel={() => setIsFormOpen(false)}
        />

        {/* Story Detail Slide-in Panel */}
        <StoryDetail
          storyId={selectedStoryId}
          onClose={() => setSelectedStoryId(null)}
          onOpenTask={(taskId) => setSelectedTaskId(taskId)}
        />

        {/* Task Detail Slide-in Panel */}
        <TaskDetail
          taskId={selectedTaskId}
          onClose={() => setSelectedTaskId(null)}
        />
    </div>
  </DndContext>
  )
}
