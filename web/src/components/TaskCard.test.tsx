import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import TaskCard from './TaskCard'
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

const baseTask: Task = {
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
}

describe('TaskCard', () => {
  it('renders task title', () => {
    render(<TaskCard task={baseTask} />)
    expect(screen.getByText('Test Task')).toBeInTheDocument()
  })

  it('renders SharpTag with task type label', () => {
    render(<TaskCard task={baseTask} />)
    // SharpTag renders label inside brackets, e.g. [CODE]
    expect(screen.getByText('[CODE]')).toBeInTheDocument()
  })

  it('renders [BUILD] for build task type', () => {
    render(<TaskCard task={{ ...baseTask, task_type: 'build' }} />)
    expect(screen.getByText('[BUILD]')).toBeInTheDocument()
  })

  it('renders [REVIEW] for review task type', () => {
    render(<TaskCard task={{ ...baseTask, task_type: 'review' }} />)
    expect(screen.getByText('[REVIEW]')).toBeInTheDocument()
  })

  describe('stale indicator', () => {
    it('shows "stale" text when is_stale is true', () => {
      render(<TaskCard task={{ ...baseTask, is_stale: true }} />)
      expect(screen.getByText('stale')).toBeInTheDocument()
    })

    it('does NOT show stale indicator when is_stale is false', () => {
      render(<TaskCard task={{ ...baseTask, is_stale: false }} />)
      expect(screen.queryByText('stale')).not.toBeInTheDocument()
    })
  })

  describe('blocked indicator', () => {
    it('shows "blocked" text when status is "blocked"', () => {
      render(<TaskCard task={{ ...baseTask, status: 'blocked' }} />)
      expect(screen.getByText('blocked')).toBeInTheDocument()
    })

    it('does NOT show blocked indicator for "new" status', () => {
      render(<TaskCard task={{ ...baseTask, status: 'new' }} />)
      expect(screen.queryByText('blocked')).not.toBeInTheDocument()
    })

    it('does NOT show blocked indicator for "in_progress" status', () => {
      render(<TaskCard task={{ ...baseTask, status: 'in_progress' }} />)
      expect(screen.queryByText('blocked')).not.toBeInTheDocument()
    })
  })

  describe('assigned_to', () => {
    it('shows assigned_to text when present', () => {
      render(<TaskCard task={{ ...baseTask, assigned_to: 'agent-42' }} />)
      expect(screen.getByText('agent-42')).toBeInTheDocument()
    })

    it('does NOT show assigned_to when absent', () => {
      render(<TaskCard task={baseTask} />)
      expect(screen.queryByText('agent-42')).not.toBeInTheDocument()
    })
  })

  describe('onClick', () => {
    it('calls onClick with task.id when clicked', async () => {
      const onClick = vi.fn()
      const user = userEvent.setup()
      render(<TaskCard task={baseTask} onClick={onClick} />)
      await user.click(screen.getByText('Test Task'))
      expect(onClick).toHaveBeenCalledWith('task-1')
    })

    it('does not call onClick when not provided', async () => {
      const user = userEvent.setup()
      render(<TaskCard task={baseTask} />)
      await user.click(screen.getByText('Test Task'))
      expect(screen.queryByText('Test Task')).toBeInTheDocument()
    })
  })
})