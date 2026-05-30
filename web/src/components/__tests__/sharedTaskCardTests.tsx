import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { Task } from '../../types'

export interface TaskCardComponentProps {
  task: Task
  onClick?: (id: string) => void
}

/**
 * Shared test suite for components that render a Task card.
 * Both TaskCard and TaskDragPreview share the same rendering logic,
 * so they can reuse these test cases.
 *
 * @param Component - The component under test (e.g. TaskCard or TaskDragPreview)
 * @param createTask - Factory function to create a Task with optional overrides
 */
export function sharedTaskCardTests(
  Component: React.ComponentType<TaskCardComponentProps>,
  createTask: (overrides?: Partial<Task>) => Task,
) {
  describe('shared task card tests', () => {
    it('renders task title', () => {
      render(<Component task={createTask({ title: 'Test Task Title' })} />)
      expect(screen.getByText('Test Task Title')).toBeInTheDocument()
    })

    it('renders SharpTag with task type label', () => {
      render(<Component task={createTask({ task_type: 'code' })} />)
      // SharpTag renders label inside brackets, e.g. [CODE]
      expect(screen.getByText('[CODE]')).toBeInTheDocument()
    })

    it('renders [BUILD] for build task type', () => {
      render(<Component task={createTask({ task_type: 'build' })} />)
      expect(screen.getByText('[BUILD]')).toBeInTheDocument()
    })

    it('renders [REVIEW] for review task type', () => {
      render(<Component task={createTask({ task_type: 'review' })} />)
      expect(screen.getByText('[REVIEW]')).toBeInTheDocument()
    })

    describe('stale indicator', () => {
      it('shows "stale" text when is_stale is true', () => {
        render(<Component task={createTask({ is_stale: true })} />)
        expect(screen.getByText('stale')).toBeInTheDocument()
      })

      it('does NOT show stale indicator when is_stale is false', () => {
        render(<Component task={createTask({ is_stale: false })} />)
        expect(screen.queryByText('stale')).not.toBeInTheDocument()
      })
    })

    describe('blocked indicator', () => {
      it('shows "blocked" text when status is "blocked"', () => {
        render(<Component task={createTask({ status: 'blocked' as Task['status'] })} />)
        expect(screen.getByText('blocked')).toBeInTheDocument()
      })

      it('does NOT show blocked indicator for "new" status', () => {
        render(<Component task={createTask({ status: 'new' })} />)
        expect(screen.queryByText('blocked')).not.toBeInTheDocument()
      })

      it('does NOT show blocked indicator for "in_progress" status', () => {
        render(<Component task={createTask({ status: 'in_progress' as Task['status'] })} />)
        expect(screen.queryByText('blocked')).not.toBeInTheDocument()
      })
    })

    describe('assigned_to', () => {
      it('shows assigned_to text when present', () => {
        render(<Component task={createTask({ assigned_to: 'agent-42' })} />)
        expect(screen.getByText('agent-42')).toBeInTheDocument()
      })

      it('does NOT show assigned_to when absent', () => {
        render(<Component task={createTask({ assigned_to: undefined })} />)
        expect(screen.queryByText('agent-42')).not.toBeInTheDocument()
      })
    })
  })
}