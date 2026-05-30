import { useState, useCallback, useMemo } from 'react'
import { Plus } from 'lucide-react'
import {
  DndContext,
  DragOverlay,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import {
  SortableContext,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { useBoardState } from '../hooks/useBoardState'
import { useCreateStory } from '../hooks/useCreateStory'
import { useBoardDnD } from '../hooks/useBoardDnD'
import SortableStoryRow from './SortableStoryRow'
import TaskDragPreview from './TaskDragPreview'
import StoryDetail from './StoryDetail'
import TaskDetail from './TaskDetail'
import CreateStoryForm from './CreateStoryForm'
import GridHeader from './GridHeader'
import EmptyState from './EmptyState'
import type { CreateStoryData } from './CreateStoryForm'

export default function Board() {
  const {
    isLoading,
    error,
    displayStories,
    displayTaskOrder,
    tasksByStoryAndStatus,
    allTasks,
    assigneeNameMap,
  } = useBoardState()

  const stories = useMemo(
    () => [...displayStories].sort((a, b) => a.sort_order - b.sort_order),
    [displayStories],
  )

  const [isFormOpen, setIsFormOpen] = useState(false)
  const createStoryMutation = useCreateStory()
  const [selectedStoryId, setSelectedStoryId] = useState<string | null>(null)
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null)

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  )

  const { activeDragTask, handleDragStart, handleDragOver, handleDragEnd, syncRefs } = useBoardDnD()

  // Keep DnD hook refs in sync with latest state — runs on every render
  syncRefs({
    allTasks,
    tasksByStoryAndStatus,
    displayStories,
    stories,
    displayTaskOrder,
  })

  const handleCreate = useCallback(
    (formData: CreateStoryData) => {
      createStoryMutation.mutate(formData, {
        onSuccess: () => setIsFormOpen(false),
        onError: (err) => { /* mutation handles toast */ void err },
      })
    },
    [createStoryMutation],
  )

  const handleStoryClick = useCallback((id: string) => {
    setSelectedStoryId(id)
    setSelectedTaskId(null)
  }, [])

  const handleTaskClick = useCallback((id: string) => {
    setSelectedTaskId(id)
    setSelectedStoryId(null)
  }, [])

  const handleAddTask = useCallback((storyId: string) => {
    setSelectedTaskId(`new-task-${storyId}`)
    setSelectedStoryId(null)
  }, [])

  const closeStory = useCallback(() => setSelectedStoryId(null), [])
  const closeTask = useCallback(() => setSelectedTaskId(null), [])
  const openTaskFromStory = useCallback((taskId: string) => {
    setSelectedTaskId(taskId)
    setSelectedStoryId(null)
  }, [])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <span className="font-mono text-sm text-neutral-500 dark:text-amber-muted">Loading board...</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-64">
        <span className="font-mono text-sm text-red-500">Error loading board: {error.message}</span>
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
        <div className="flex items-center justify-between px-4 py-2 border-b border-gray-200 dark:border-gray-border">
          <span className="text-[10px] uppercase tracking-widest font-bold text-neutral-600 dark:text-neutral-300">
            Board
          </span>
          <button onClick={() => setIsFormOpen(true)} className="glow-button flex items-center gap-1.5 text-xs">
            <Plus size={14} /> Add Story
          </button>
        </div>

        <div className="flex flex-col flex-1 min-h-0">
          <GridHeader displayStories={displayStories} tasksByStoryAndStatus={tasksByStoryAndStatus} />
          <div className="flex-1 overflow-y-auto">
            <SortableContext items={displayStories.map((s) => s.id)} strategy={verticalListSortingStrategy}>
              {displayStories.map((story) => (
                <SortableStoryRow
                  key={story.id}
                  story={story}
                  tasksByStoryAndStatus={tasksByStoryAndStatus}
                  displayTaskOrder={displayTaskOrder}
                  allTasks={allTasks}
                  onStoryClick={handleStoryClick}
                  onTaskClick={handleTaskClick}
                  assigneeNameMap={assigneeNameMap}
                  handleAddTask={handleAddTask}
                />
              ))}
            </SortableContext>
            <EmptyState show={displayStories.length === 0} />
          </div>
        </div>

        <CreateStoryForm open={isFormOpen} onSubmit={handleCreate} onCancel={() => setIsFormOpen(false)} isPending={createStoryMutation.isPending} />
        <StoryDetail storyId={selectedStoryId} onClose={closeStory} onOpenTask={openTaskFromStory} />
        <TaskDetail taskId={selectedTaskId} onClose={closeTask} />
      </div>
      <DragOverlay>
        {activeDragTask ? <TaskDragPreview task={activeDragTask} /> : null}
      </DragOverlay>
    </DndContext>
  )
}
