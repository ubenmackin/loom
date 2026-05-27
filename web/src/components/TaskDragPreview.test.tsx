import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import TaskDragPreview from './TaskDragPreview'
import type { Task } from '../types'

function createTask(overrides: Partial<Task> = {}): Task {
  return {
    id: 'task-1',
    story_id: 'story-1',
    title: 'Test Task',
    status: 'new',
    task_type: 'code',
    sort_order: 1,
    is_stale: false,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('TaskDragPreview', () => {
  it('renders task title', () => {
    const task = createTask({ title: 'My Task Title' })
    render(<TaskDragPreview task={task} />)
    expect(screen.getByText('My Task Title')).toBeInTheDocument()
  })

  it('renders SharpTag with task type label', () => {
    const task = createTask({ task_type: 'code' })
    render(<TaskDragPreview task={task} />)
    // SharpTag renders label inside brackets, e.g. [CODE]
    expect(screen.getByText('[CODE]')).toBeInTheDocument()
  })

  it('renders [BUILD] for build task type', () => {
    const task = createTask({ task_type: 'build' })
    render(<TaskDragPreview task={task} />)
    expect(screen.getByText('[BUILD]')).toBeInTheDocument()
  })

  it('renders [REVIEW] for review task type', () => {
    const task = createTask({ task_type: 'review' })
    render(<TaskDragPreview task={task} />)
    expect(screen.getByText('[REVIEW]')).toBeInTheDocument()
  })

  describe('stale indicator', () => {
    it('shows "stale" text when is_stale is true', () => {
      const task = createTask({ is_stale: true })
      render(<TaskDragPreview task={task} />)
      expect(screen.getByText('stale')).toBeInTheDocument()
    })

    it('does NOT show stale indicator when is_stale is false', () => {
      const task = createTask({ is_stale: false })
      render(<TaskDragPreview task={task} />)
      expect(screen.queryByText('stale')).not.toBeInTheDocument()
    })
  })

  describe('blocked indicator', () => {
    it('shows "blocked" text when status is "blocked"', () => {
      const task = createTask({ status: 'blocked' })
      render(<TaskDragPreview task={task} />)
      expect(screen.getByText('blocked')).toBeInTheDocument()
    })

    it('does NOT show blocked indicator for "new" status', () => {
      const task = createTask({ status: 'new' })
      render(<TaskDragPreview task={task} />)
      expect(screen.queryByText('blocked')).not.toBeInTheDocument()
    })

    it('does NOT show blocked indicator for "in_progress" status', () => {
      const task = createTask({ status: 'in_progress' })
      render(<TaskDragPreview task={task} />)
      expect(screen.queryByText('blocked')).not.toBeInTheDocument()
    })
  })

  describe('assigned_to', () => {
    it('shows assigned_to when present', () => {
      const task = createTask({ assigned_to: 'agent-42' })
      render(<TaskDragPreview task={task} />)
      expect(screen.getByText('agent-42')).toBeInTheDocument()
    })

    it('does NOT show assigned section when assigned_to is absent', () => {
      const task = createTask({ assigned_to: undefined })
      render(<TaskDragPreview task={task} />)
      expect(screen.queryByText('agent-42')).not.toBeInTheDocument()
    })
  })
})
