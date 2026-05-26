import { useState, useMemo, useCallback, useEffect } from 'react'
import { Plus } from 'lucide-react'
import {
  DndContext,
  DragOverlay,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
  type DragOverEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  verticalListSortingStrategy,
  arrayMove,
  useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { useBoard } from '../hooks/useBoard'
import { useCreateStory } from '../hooks/useCreateStory'
import StoryCard from './StoryCard'
import TaskCard from './TaskCard'
import TaskDragPreview from './TaskDragPreview'
import { CellDropZone } from './Column'
import StoryDetail from './StoryDetail'
import TaskDetail from './TaskDetail'
import CreateStoryForm from './CreateStoryForm'
import type { CreateStoryData } from './CreateStoryForm'
import { Status, type StatusType, type Story, type Task, type User, type Session, type BoardState } from '../types'
import { batchReorderStories, batchReorderTasks, updateTask, getUsers, fetchSessions } from '../api/client'
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
        return (
          <div
            key={col.status}
            className={`min-w-[200px] md:min-w-0 md:flex-1 p-1.5 ${
              colIdx < COLUMNS.length - 1 ? 'border-r border-gray-200 dark:border-gray-border' : ''
            }`}
          >
            <CellDropZone id={droppableId} storyId={story.id} status={col.status}>
              {cellTasks.length > 0 ? (
                (() => {
                  const cellKey = `${story.id}:${col.status}`
                  const orderedIds = displayTaskOrder[cellKey]
                  const taskIds = orderedIds ?? cellTasks.map((t) => t.id)
                  return (
                    <SortableContext items={taskIds} strategy={verticalListSortingStrategy}>
                      <div className="space-y-1.5">
                        {taskIds.map((taskId) => {
                          const task = cellTasks.find((t) => t.id === taskId) ?? allTasks.find((t) => t.id === taskId)
                          if (!task) return null
                          return (
                            <TaskCard
                              key={task.id}
                              task={task}
                              onClick={(taskId) => onTaskClick(taskId)}
                              isDraggable={true}
                            />
                          )
                        })}
                      </div>
                    </SortableContext>
                  )
                })()
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

export default function Board() {
  const { data, isLoading, error } = useBoard()
  const [displayStories, setDisplayStories] = useState<Story[]>([])
  const [displayTaskOrder, setDisplayTaskOrder] = useState<Record<string, string[]>>({})
  const [activeDragTask, setActiveDragTask] = useState<Task | null>(null)

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

  // Sync displayStories from query data on initial load and refetches
  useEffect(() => {
    if (data?.stories) {
      setDisplayStories(stories)
    }
  }, [stories])

  // Sync displayTaskOrder from query data on initial load and refetches
  useEffect(() => {
    if (data?.tasks_by_story_and_status) {
      const order: Record<string, string[]> = {}
      for (const [storyId, statusMap] of Object.entries(data.tasks_by_story_and_status)) {
        for (const [status, tasks] of Object.entries(statusMap)) {
          const key = `${storyId}:${status}`
          order[key] = [...tasks].sort((a, b) => a.sort_order - b.sort_order).map(t => t.id)
        }
      }
      setDisplayTaskOrder(order)
    }
  }, [data?.tasks_by_story_and_status])

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
      setActiveDragTask(null)
      const { active, over } = event
      console.log('[DragEnd] event', {
        activeId: active.id,
        activeType: getDragData<string>(active.data.current, 'type'),
        overId: over ? over.id : null,
        overType: over ? getDragData<string>(over.data.current, 'type') : null,
        overData: over?.data?.current ?? null,
        activeData: active.data.current,
      })
      if (!over || active.id === over.id) { console.log('[DragEnd] bail: no over or same id'); return }

      const activeDataType = getDragData<string>(active.data.current, 'type')

      if (activeDataType === 'story') {
        // Reorder stories
        const oldIndex = displayStories.findIndex((s) => s.id === active.id)
        console.log('[DragEnd] story', { oldIndex, activeId: active.id })
        if (oldIndex === -1) { console.log('[DragEnd] bail: oldIndex -1'); return }

        // Resolve over to a story ID — over may be a cell or task nested in a story row
        let overStoryId: string | undefined
        const overType = getDragData<string>(over.data.current, 'type')
        if (overType === 'story') {
          overStoryId = String(over.id)
        } else if (overType === 'cell' || overType === 'task') {
          overStoryId = getDragData<string>(over.data.current, 'storyId')
        }
        console.log('[DragEnd] over resolved', { overType, overStoryId, overId: over.id })
        if (!overStoryId) { console.log('[DragEnd] bail: no overStoryId'); return }

        const newIndex = displayStories.findIndex((s) => s.id === overStoryId)
        console.log('[DragEnd] newIndex', { newIndex, overStoryId })
        if (newIndex === -1) { console.log('[DragEnd] bail: newIndex -1'); return }

        const newStories = arrayMove(displayStories, oldIndex, newIndex)

        // Direct state update — follows dnd-kit's standard pattern for synchronous items update
        setDisplayStories(newStories)

        // Also update React Query cache for server-side consistency (async, doesn't affect visual)
        queryClient.setQueryData<BoardState>(['board'], (old) => {
          if (!old) return old
          return {
            ...old,
            stories: newStories.map((story, idx) => ({ ...story, sort_order: idx }))
          }
        })
        console.log('[DragEnd] setQueryData', { cacheUpdated: true })

        // Compute changed items for API — compare by item ID, not by array index
        const reorderItems = newStories
          .map((story, idx) => ({ id: story.id, sort_order: idx }))
          .filter((item) => {
            const original = stories.find((s) => s.id === item.id)
            return original && original.sort_order !== item.sort_order
          })

        console.log('[DragEnd] reorder', { oldIndex, newIndex, reorderItemsLength: reorderItems.length })

        if (reorderItems.length > 0) {
          batchReorderStories(reorderItems)
            .then((res) => console.log('[DragEnd] API success:', res))
            .catch((err) => {
              console.error('Failed to batch reorder stories:', err)
              queryClient.invalidateQueries({ queryKey: ['board'] })
            })
        } else {
          console.log('[DragEnd] no items to reorder')
        }
      } else if (activeDataType === 'task') {
        const taskId = String(active.id)
        const sourceStoryId = getDragData<string>(active.data.current, 'storyId')
        const sourceStatus = getDragData<StatusType>(active.data.current, 'status')
        console.log('[DragEnd] task drag', { taskId, sourceStoryId, sourceStatus })

        // Determine target story_id and status from the over droppable
        const targetData = over.data.current
        const targetStoryId = getDragData<string>(targetData, 'storyId') ?? sourceStoryId
        const targetStatus = getDragData<StatusType>(targetData, 'status') ?? sourceStatus

        // Build a SINGLE update payload
        const updates: Partial<Task> = {}

        // Sort order computation
        if (getDragData<string>(targetData, 'type') === 'task') {
          // Dropped on another task — insert before it
          const overTaskId = String(over.id)
          const overTask = allTasks.find((t) => t.id === overTaskId)
          if (overTask) {
            updates.sort_order = overTask.sort_order
          }
        } else {
          // Dropped on a cell drop zone — append to end
          const cellTasks =
            tasksByStoryAndStatus[targetStoryId ?? '']?.[targetStatus ?? ''] ?? []
          updates.sort_order = cellTasks.length > 0
            ? Math.max(...cellTasks.map((t) => t.sort_order)) + 1
            : 0
        }

        // Include status if changed (folded into the single PUT call)
        if (targetStatus && targetStatus !== sourceStatus) {
          updates.status = targetStatus
        }

        // Include story_id if changed
        if (targetStoryId && targetStoryId !== sourceStoryId) {
          updates.story_id = targetStoryId
        }

        // Synchronous local state update for task sortable items
        const cellKey = `${targetStoryId}:${targetStatus}`
        if (sourceStoryId === targetStoryId && sourceStatus === targetStatus) {
          // Within-cell reorder — update local order synchronously
          const overTaskId = String(over.id)
          setDisplayTaskOrder((prev) => {
            const currentOrder = prev[cellKey]
            if (!currentOrder) return prev
            const oldIdx = currentOrder.indexOf(taskId)
            const newIdx = currentOrder.indexOf(overTaskId)
            if (oldIdx === -1 || newIdx === -1) return prev
            const newOrder = arrayMove([...currentOrder], oldIdx, newIdx)
            return { ...prev, [cellKey]: newOrder }
          })
        } else {
          // Cross-cell drop — remove from source, add at position in target
          const sourceCellKey = `${sourceStoryId}:${sourceStatus}`
          setDisplayTaskOrder((prev) => {
            const next = { ...prev }
            // Remove from source cell if different from target
            if (sourceCellKey !== cellKey && next[sourceCellKey]) {
              next[sourceCellKey] = next[sourceCellKey].filter((id) => id !== taskId)
            }
            // Add to target cell
            const targetOrder = next[cellKey]
            if (targetOrder) {
              next[cellKey] = [...targetOrder, taskId]
            } else {
              next[cellKey] = [taskId]
            }
            return next
          })
        }

        // For within-cell reorder (same story, same status, dropped on a task):
        // batch-reorder all tasks in the cell with sequential sort_order indices.
        // For cross-cell drops, use the existing single-task updateTask.
        if (sourceStoryId === targetStoryId && sourceStatus === targetStatus) {
          // Same cell — only batch when dropped on another task
          if (getDragData<string>(targetData, 'type') === 'task') {
            const batchCellKey = `${targetStoryId}:${targetStatus}`
            const cellTasks = (tasksByStoryAndStatus[targetStoryId ?? '']?.[targetStatus ?? ''] ?? []).sort(
              (a, b) => a.sort_order - b.sort_order,
            )
            const displayOrder = displayTaskOrder[batchCellKey] ?? cellTasks.map((t) => t.id)
            const oldIdx = displayOrder.indexOf(taskId)
            const newIdx = displayOrder.indexOf(String(over.id))
            if (oldIdx !== -1 && newIdx !== -1) {
              const newOrder = arrayMove([...displayOrder], oldIdx, newIdx)
              const batchItems = newOrder.map((id, idx) => ({ id, sort_order: idx }))
              batchReorderTasks(batchItems)
                .then((res) => console.log('[DragEnd] batch reorder success:', res))
                .catch((err) => {
                  console.error('Failed to batch reorder tasks:', err)
                  queryClient.invalidateQueries({ queryKey: ['board'] })
                })
            } else {
              queryClient.invalidateQueries({ queryKey: ['board'] })
            }
          } else {
            // Dropped on cell drop zone within the same cell — no API call needed, just invalidate
            queryClient.invalidateQueries({ queryKey: ['board'] })
          }
        } else {
          // Cross-cell drop — use single updateTask
          if (Object.keys(updates).length > 0) {
            updateTask(taskId, updates)
              .catch((err) => console.error('Failed to update task:', err))
              .finally(() => queryClient.invalidateQueries({ queryKey: ['board'] }))
          } else {
            queryClient.invalidateQueries({ queryKey: ['board'] })
          }
        }
      }
    },
    [displayStories, stories, allTasks, tasksByStoryAndStatus, displayTaskOrder, queryClient],
  )

  const handleDragStart = useCallback(
    (event: DragStartEvent) => {
      const { active } = event
      console.log('[DragStart]', {
        id: active.id,
        type: getDragData<string>(active.data.current, 'type'),
        data: active.data.current,
      })
      if (getDragData<string>(active.data.current, 'type') === 'task') {
        const task = allTasks.find((t) => t.id === active.id)
        if (task) setActiveDragTask(task)
      }
    },
    [allTasks],
  )

  const handleDragOver = useCallback(
    (event: DragOverEvent) => {
      const { active, over } = event
      console.log('[DragOver]', {
        activeId: active.id,
        overId: over ? over.id : null,
        overType: over?.data?.current ? getDragData<string>(over.data.current, 'type') : null,
        overData: over?.data?.current ?? null,
      })

      if (!over || active.id === over.id) return

      const activeType = getDragData<string>(active.data.current, 'type')
      if (activeType !== 'task') return

      const activeStoryId = getDragData<string>(active.data.current, 'storyId')
      const activeStatus = getDragData<string>(active.data.current, 'status')
      const overType = getDragData<string>(over.data.current, 'type')
      const overStoryId = getDragData<string>(over.data.current, 'storyId')
      const overStatus = getDragData<string>(over.data.current, 'status')

      // Only handle within-cell reorder: same story, same status, both are task cards
      if (overType === 'task' && activeStoryId === overStoryId && activeStatus === overStatus) {
        const cellKey = `${activeStoryId}:${activeStatus}`
        setDisplayTaskOrder((prev) => {
          const currentOrder = prev[cellKey]
          if (!currentOrder) return prev
          const oldIdx = currentOrder.indexOf(String(active.id))
          const newIdx = currentOrder.indexOf(String(over.id))
          if (oldIdx === -1 || newIdx === -1) return prev
          return { ...prev, [cellKey]: arrayMove([...currentOrder], oldIdx, newIdx) }
        })
      }
    },
    [setDisplayTaskOrder],
  )

  const handleAddTask = useCallback(
    (storyId: string) => {
      setSelectedTaskId(`new-task-${storyId}`)
      setSelectedStoryId(null)
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
      onDragStart={handleDragStart}
      onDragOver={handleDragOver}
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

          {/* Scrollable Body */}
          <div className="flex-1 overflow-y-auto">
            <SortableContext items={displayStories.map((s) => s.id)} strategy={verticalListSortingStrategy}>
              {displayStories.map((story) => (
                <SortableStoryRow
                  key={story.id}
                  story={story}
                  tasksByStoryAndStatus={tasksByStoryAndStatus}
                  displayTaskOrder={displayTaskOrder}
                  allTasks={allTasks}
                  onStoryClick={(id) => { setSelectedStoryId(id); setSelectedTaskId(null); }}
                  onTaskClick={(id) => { setSelectedTaskId(id); setSelectedStoryId(null); }}
                  assigneeNameMap={assigneeNameMap}
                  handleAddTask={handleAddTask}
                />
              ))}
            </SortableContext>
            {displayStories.length === 0 && (!data?.stories || data.stories.length === 0) && (
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
          onOpenTask={(taskId) => { setSelectedTaskId(taskId); setSelectedStoryId(null); }}
        />

        {/* Task Detail Slide-in Panel */}
        <TaskDetail
          taskId={selectedTaskId}
          onClose={() => setSelectedTaskId(null)}
        />
      </div>
      <DragOverlay>
        {activeDragTask ? <TaskDragPreview task={activeDragTask} /> : null}
      </DragOverlay>
    </DndContext>
  )
}
