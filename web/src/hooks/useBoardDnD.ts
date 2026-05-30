import { useState, useCallback, useRef } from 'react'
import { arrayMove } from '@dnd-kit/sortable'
import type { DragEndEvent, DragStartEvent } from '@dnd-kit/core'
import { useQueryClient } from '@tanstack/react-query'
import { batchReorderStories, batchReorderTasks, updateTask } from '../api/client'
import type { Story, StatusType, Task, BoardState } from '../types'

/** Type-safe access to drag event data — replaces raw `as` assertions */
function getDragData<T>(data: unknown, key: string): T | undefined {
  if (data && typeof data === 'object' && key in data) {
    return (data as Record<string, unknown>)[key] as T
  }
  return undefined
}

export function useBoardDnD() {
  const queryClient = useQueryClient()
  const [activeDragTask, setActiveDragTask] = useState<Task | null>(null)

  // Stable refs for values that change frequently — avoids recreating drag callbacks
  const allTasksRef = useRef<Task[]>([])
  const tasksByStoryAndStatusRef = useRef<Record<string, Record<string, Task[]>>>({})
  const displayStoriesRef = useRef<Story[]>([])
  const storiesRef = useRef<Story[]>([])
  const displayTaskOrderRef = useRef<Record<string, string[]>>({})

  // Refs are synced on every render via syncRefs.
  // No local state setters — optimistic UI updates go directly to the React Query cache.

  /** Sync all mutable state into the hook's refs. Called on every Board render. */
  const syncRefs = useCallback(
    (opts: {
      allTasks: Task[]
      tasksByStoryAndStatus: Record<string, Record<string, Task[]>>
      displayStories: Story[]
      stories: Story[]
      displayTaskOrder: Record<string, string[]>
    }) => {
      allTasksRef.current = opts.allTasks
      tasksByStoryAndStatusRef.current = opts.tasksByStoryAndStatus
      displayStoriesRef.current = opts.displayStories
      storiesRef.current = opts.stories
      displayTaskOrderRef.current = opts.displayTaskOrder
    },
    [],
  )

  const handleDragStart = useCallback(
    (event: DragStartEvent) => {
      const { active } = event
      if (getDragData<string>(active.data.current, 'type') === 'task') {
        const task = allTasksRef.current.find((t) => t.id === active.id)
        if (task) setActiveDragTask(task)
      }
    },
    [],
  )

  const handleDragOver = useCallback(
    () => {
      // DragOver state updates are intentionally omitted.
      // The @dnd-kit/sortable library handles visual reordering during drag
      // via its built-in animation. Committing state changes on every mouse
      // move is expensive and unnecessary. The final order is computed in
      // handleDragEnd.
    },
    [],
  )

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      setActiveDragTask(null)
      const { active, over } = event
      if (!over || active.id === over.id) return

      const activeDataType = getDragData<string>(active.data.current, 'type')
      const displayStories = displayStoriesRef.current
      const stories = storiesRef.current
      const displayTaskOrder = displayTaskOrderRef.current
      const tasksByStoryAndStatus = tasksByStoryAndStatusRef.current
      const allTasks = allTasksRef.current

      if (activeDataType === 'story') {
        const oldIndex = displayStories.findIndex((s) => s.id === active.id)
        if (oldIndex === -1) return

        let overStoryId: string | undefined
        const overType = getDragData<string>(over.data.current, 'type')
        if (overType === 'story') {
          overStoryId = String(over.id)
        } else if (overType === 'cell' || overType === 'task') {
          overStoryId = getDragData<string>(over.data.current, 'storyId')
        }
        if (!overStoryId) return

        const newIndex = displayStories.findIndex((s) => s.id === overStoryId)
        if (newIndex === -1) return

        const newStories = arrayMove(displayStories, oldIndex, newIndex)
        displayStoriesRef.current = newStories
        storiesRef.current = newStories

        queryClient.setQueryData<BoardState>(['board'], (old) => {
          if (!old) return old
          return {
            ...old,
            stories: newStories.map((story, idx) => ({ ...story, sort_order: idx })),
          }
        })

        const reorderItems = newStories
          .map((story, idx) => ({ id: story.id, sort_order: idx }))
          .filter((item) => {
            const original = stories.find((s) => s.id === item.id)
            return original && original.sort_order !== item.sort_order
          })

        if (reorderItems.length > 0) {
          batchReorderStories(reorderItems)
            .catch((err) => {
              console.error('Failed to batch reorder stories:', err)
              queryClient.invalidateQueries({ queryKey: ['board'] })
            })
        }
      } else if (activeDataType === 'task') {
        const taskId = String(active.id)
        const sourceStoryId = getDragData<string>(active.data.current, 'storyId')
        const sourceStatus = getDragData<StatusType>(active.data.current, 'status')

        const targetData = over.data.current
        const targetStoryId = getDragData<string>(targetData, 'storyId') ?? sourceStoryId
        const targetStatus = getDragData<StatusType>(targetData, 'status') ?? sourceStatus

        const updates: Partial<Task> = {}

        if (getDragData<string>(targetData, 'type') === 'task') {
          const overTaskId = String(over.id)
          const overTask = allTasks.find((t) => t.id === overTaskId)
          if (overTask) {
            updates.sort_order = overTask.sort_order
          }
        } else {
          const cellTasks =
            tasksByStoryAndStatus[targetStoryId ?? '']?.[targetStatus ?? ''] ?? []
          updates.sort_order = cellTasks.length > 0
            ? Math.max(...cellTasks.map((t) => t.sort_order)) + 1
            : 0
        }

        if (targetStatus && targetStatus !== sourceStatus) {
          updates.status = targetStatus
        }
        if (targetStoryId && targetStoryId !== sourceStoryId) {
          updates.story_id = targetStoryId
        }

        const cellKey = `${targetStoryId}:${targetStatus}`
        if (sourceStoryId === targetStoryId && sourceStatus === targetStatus) {
          const overTaskId = String(over.id)
          const currentOrder = displayTaskOrder[cellKey]
          if (currentOrder) {
            const oldIdx = currentOrder.indexOf(taskId)
            const newIdx = currentOrder.indexOf(overTaskId)
            if (oldIdx !== -1 && newIdx !== -1) {
              const newOrder = arrayMove([...currentOrder], oldIdx, newIdx)
              queryClient.setQueryData<BoardState>(['board'], (old) => {
                if (!old) return old
                const storyTasks = old.tasks_by_story_and_status?.[targetStoryId ?? '']?.[targetStatus ?? '']
                if (!storyTasks) return old
                const updatedTasks = [...storyTasks]
                for (let i = 0; i < newOrder.length; i++) {
                  const task = updatedTasks.find((t) => t.id === newOrder[i])
                  if (task && task.sort_order !== i) {
                    updatedTasks[updatedTasks.indexOf(task)] = { ...task, sort_order: i }
                  }
                }
                return {
                  ...old,
                  tasks_by_story_and_status: {
                    ...old.tasks_by_story_and_status,
                    [targetStoryId ?? '']: {
                      ...old.tasks_by_story_and_status?.[targetStoryId ?? ''],
                      [targetStatus ?? '']: updatedTasks,
                    },
                  },
                }
              })
            }
          }
        } else {
          // Cross-cell move: skip optimistic cache update — invalidateQueries
          // is called immediately below, which will refetch the correct data.
        }

        if (sourceStoryId === targetStoryId && sourceStatus === targetStatus) {
          if (getDragData<string>(targetData, 'type') === 'task') {
            const batchCellKey = `${targetStoryId}:${targetStatus}`
            const cellTasks = (tasksByStoryAndStatus[targetStoryId ?? '']?.[targetStatus ?? ''] ?? []).sort(
              (a, b) => a.sort_order - b.sort_order,
            )
            const curOrder = displayTaskOrder[batchCellKey] ?? cellTasks.map((t) => t.id)
            const oldIdx = curOrder.indexOf(taskId)
            const newIdx = curOrder.indexOf(String(over.id))
            if (oldIdx !== -1 && newIdx !== -1) {
              const newOrder = arrayMove([...curOrder], oldIdx, newIdx)
              const batchItems = newOrder.map((id, idx) => ({ id, sort_order: idx }))
              batchReorderTasks(batchItems)
                .catch((err) => {
                  console.error('Failed to batch reorder tasks:', err)
                  queryClient.invalidateQueries({ queryKey: ['board'] })
                })
            } else {
              queryClient.invalidateQueries({ queryKey: ['board'] })
            }
          } else {
            queryClient.invalidateQueries({ queryKey: ['board'] })
          }
        } else {
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
    [queryClient],
  )

  return {
    activeDragTask,
    handleDragStart,
    handleDragOver,
    handleDragEnd,
    syncRefs,
  }
}