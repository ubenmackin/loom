import { describe } from 'vitest'
import TaskDragPreview from './TaskDragPreview'
import { sharedTaskCardTests } from './__tests__/sharedTaskCardTests'
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
  // Run the shared test suite
  sharedTaskCardTests(TaskDragPreview, createTask)
})