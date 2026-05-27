import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Status, type Task, type TaskTypeType } from '../types'
import DependencyGraph from './DependencyGraph'

const baseTask = (overrides: Partial<Task> = {}): Task => ({
  id: 'TASK-001',
  numeric_id: 1,
  story_id: 'story-1',
  title: 'Test task',
  description: '',
  status: Status.New,
  task_type: 'code' as TaskTypeType,
  assigned_to: '',
  assignee_type: 'human',
  sort_order: 1,
  instructions: '',
  is_stale: false,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  ...overrides,
})

describe('DependencyGraph', () => {
  it('renders "No tasks to display" when tasks array is empty', () => {
    render(<DependencyGraph storyId="story-1" tasks={[]} />)
    expect(screen.getByText('No tasks to display')).toBeInTheDocument()
  })

  it('renders "No tasks to display" when tasks is undefined', () => {
    render(<DependencyGraph storyId="story-1" tasks={undefined as unknown as Task[]} />)
    expect(screen.getByText('No tasks to display')).toBeInTheDocument()
  })

  it('renders the Task Status Overview label when there are tasks', () => {
    render(<DependencyGraph storyId="story-1" tasks={[baseTask()]} />)
    expect(screen.getByText('Task Status Overview')).toBeInTheDocument()
  })

  it('renders summary counts for resolved tasks', () => {
    const tasks = [
      baseTask({ id: 'TASK-001', status: Status.Done }),
      baseTask({ id: 'TASK-002', status: Status.Done }),
      baseTask({ id: 'TASK-003', status: Status.New }),
    ]
    render(<DependencyGraph storyId="story-1" tasks={tasks} />)
    expect(screen.getByText('2 resolved')).toBeInTheDocument()
  })

  it('renders summary counts for pending (active) tasks', () => {
    const tasks = [
      baseTask({ id: 'TASK-001', status: Status.New }),
      baseTask({ id: 'TASK-002', status: Status.Ready }),
      baseTask({ id: 'TASK-003', status: Status.InProgress }),
      baseTask({ id: 'TASK-004', status: Status.Done }),
    ]
    // New, Ready, InProgress are not blocked/done → 3 active
    render(<DependencyGraph storyId="story-1" tasks={tasks} />)
    expect(screen.getByText('3 pending')).toBeInTheDocument()
  })

  it('shows blocked count when there are blocked tasks', () => {
    const tasks = [
      baseTask({ id: 'TASK-001', status: Status.Blocked }),
      baseTask({ id: 'TASK-002', status: Status.Blocked }),
    ]
    render(<DependencyGraph storyId="story-1" tasks={tasks} />)
    expect(screen.getByText('2 blocked')).toBeInTheDocument()
  })

  it('does not show blocked count when no tasks are blocked', () => {
    const tasks = [
      baseTask({ id: 'TASK-001', status: Status.Done }),
      baseTask({ id: 'TASK-002', status: Status.New }),
    ]
    render(<DependencyGraph storyId="story-1" tasks={tasks} />)
    expect(screen.queryByText(/blocked/)).not.toBeInTheDocument()
  })

  it('renders task rows with task ID', () => {
    const tasks = [
      baseTask({ id: 'TASK-001', status: Status.New }),
      baseTask({ id: 'TASK-002', status: Status.Done }),
    ]
    render(<DependencyGraph storyId="story-1" tasks={tasks} />)
    expect(screen.getByText('TASK-001')).toBeInTheDocument()
    expect(screen.getByText('TASK-002')).toBeInTheDocument()
  })

  it('renders each task row with a SharpTag for the status', () => {
    const tasks = [
      baseTask({ id: 'TASK-001', status: Status.New }),
      baseTask({ id: 'TASK-002', status: Status.Done }),
      baseTask({ id: 'TASK-003', status: Status.Blocked }),
    ]
    render(<DependencyGraph storyId="story-1" tasks={tasks} />)
    // SharpTag renders labels as [UPPERCASE_STATUS]
    expect(screen.getByText('[NEW]')).toBeInTheDocument()
    expect(screen.getByText('[DONE]')).toBeInTheDocument()
    expect(screen.getByText('[BLOCKED]')).toBeInTheDocument()
  })

  it('renders the correct number of task rows', () => {
    const tasks = [
      baseTask({ id: 'TASK-001', status: Status.New }),
      baseTask({ id: 'TASK-002', status: Status.Ready }),
      baseTask({ id: 'TASK-003', status: Status.InProgress }),
    ]
    render(<DependencyGraph storyId="story-1" tasks={tasks} />)
    // The task rows have the task IDs
    expect(screen.getByText('TASK-001')).toBeInTheDocument()
    expect(screen.getByText('TASK-002')).toBeInTheDocument()
    expect(screen.getByText('TASK-003')).toBeInTheDocument()
  })

  it('renders header columns', () => {
    const tasks = [baseTask()]
    render(<DependencyGraph storyId="story-1" tasks={tasks} />)
    expect(screen.getByText('Task')).toBeInTheDocument()
    expect(screen.getByText('Depends On')).toBeInTheDocument()
    expect(screen.getByText('Status')).toBeInTheDocument()
  })

  it('applies status-dot class elements for each task row', () => {
    const tasks = [
      baseTask({ id: 'TASK-001', status: Status.New }),
      baseTask({ id: 'TASK-002', status: Status.Done }),
    ]
    const { container } = render(<DependencyGraph storyId="story-1" tasks={tasks} />)
    // Each task row has a status-dot span
    const dots = container.querySelectorAll('.status-dot')
    // 2 summary dots (status-dot-success, status-dot-warning) + 2 task row dots
    expect(dots.length).toBeGreaterThanOrEqual(4)
  })

  it('shows correct summary with mixed statuses', () => {
    const tasks = [
      baseTask({ id: 'TASK-001', status: Status.Done }),
      baseTask({ id: 'TASK-002', status: Status.InProgress }),
      baseTask({ id: 'TASK-003', status: Status.Blocked }),
      baseTask({ id: 'TASK-004', status: Status.New }),
    ]
    render(<DependencyGraph storyId="story-1" tasks={tasks} />)
    expect(screen.getByText('1 resolved')).toBeInTheDocument()
    expect(screen.getByText('2 pending')).toBeInTheDocument() // InProgress + New = 2
    expect(screen.getByText('1 blocked')).toBeInTheDocument()
  })
})
