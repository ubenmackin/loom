import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import TaskCard from './TaskCard'
import { sharedTaskCardTests } from './__tests__/sharedTaskCardTests'
import type { Task } from '../types'

// Mock @dnd-kit/sortable so TaskCard doesn't need a DndContext wrapper
vi.mock('@dnd-kit/sortable', () => ({
  useSortable: () => ({
    attributes: {},
    listeners: {},
    setNodeRef: () => {},
    transform: null,
    transition: null,
    isDragging: false,
  }),
}))

function createTask(overrides: Partial<Task> = {}): Task {
  return {
    id: 'task-1',
    story_id: 'story-1',
    title: 'Test Task',
    description: 'A test task',
    status: 'new',
    task_type: 'code',
    sort_order: 1,
    is_stale: false,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('TaskCard', () => {
  // Run the shared test suite
  sharedTaskCardTests(TaskCard, createTask)

  describe('onClick', () => {
    it('calls onClick with task.id when clicked', async () => {
      const onClick = vi.fn()
      const user = userEvent.setup()
      render(<TaskCard task={createTask()} onClick={onClick} />)
      await user.click(screen.getByText('Test Task'))
      expect(onClick).toHaveBeenCalledWith('task-1')
    })

    it('does not call onClick when not provided', async () => {
      const user = userEvent.setup()
      render(<TaskCard task={createTask()} />)
      await user.click(screen.getByText('Test Task'))
      expect(screen.queryByText('Test Task')).toBeInTheDocument()
    })
  })
})