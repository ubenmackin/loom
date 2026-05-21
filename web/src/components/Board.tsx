import { useState } from 'react'
import { Plus } from 'lucide-react'
import { useBoard } from '../hooks/useBoard'
import { useCreateStory } from '../hooks/useCreateStory'
import Column from './Column'
import CreateStoryForm, { type CreateStoryData } from './CreateStoryForm'
import { Status, type StatusType, type Task } from '../types'

const COLUMNS: { status: StatusType; label: string }[] = [
  { status: Status.New, label: 'New' },
  { status: Status.Ready, label: 'Ready' },
  { status: Status.InProgress, label: 'In Progress' },
  { status: Status.Blocked, label: 'Blocked' },
  { status: Status.Done, label: 'Done' },
]

export default function Board() {
  const { data, isLoading, error } = useBoard()
  const [isFormOpen, setIsFormOpen] = useState(false)
  const createStoryMutation = useCreateStory()

  const handleCreate = (data: CreateStoryData) => {
    createStoryMutation.mutate(data, {
      onSuccess: () => {
        setIsFormOpen(false)
      },
    })
  }

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

  const tasksByStatus: Record<string, Task[]> = data?.tasks_by_status ?? {}

  return (
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

      {/* Columns */}
      <div className="flex gap-0 overflow-x-auto flex-1">
        {COLUMNS.map((col, i) => (
          <div
            key={col.status}
            className={`min-w-[260px] md:min-w-0 md:flex-1 flex flex-col ${
              i < COLUMNS.length - 1 ? 'border-r border-gray-200 dark:border-gray-border' : ''
            }`}
          >
            <Column
              status={col.status}
              label={col.label}
              items={tasksByStatus[col.status] ?? []}
            />
          </div>
        ))}
      </div>

      {/* Create Story Modal */}
      <CreateStoryForm
        open={isFormOpen}
        onSubmit={handleCreate}
        onCancel={() => setIsFormOpen(false)}
      />
    </div>
  )
}
