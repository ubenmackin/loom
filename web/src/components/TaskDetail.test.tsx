import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import TaskDetail from './TaskDetail'
import type { TaskDetailResponse } from '../types'

// Shared mock for edit mode tests
const mockTaskResponse: TaskDetailResponse = {
  task: {
    id: 'task-1',
    story_id: 'story-1',
    title: 'Test Task',
    description: 'Task description',
    status: 'in_progress',
    task_type: 'code',
    assigned_to: 'user-1',
    assignee_type: 'human',
    instructions: 'Do the thing',
    is_stale: false,
    sort_order: 1,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
  dependencies: ['dep-task-1'],
  dependents: [
    {
      id: 'dep-task-2',
      story_id: 'story-1',
      title: 'Dependent Task',
      status: 'new',
      task_type: 'build',
      is_stale: false,
      sort_order: 2,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
  ],
}

// Mock @tanstack/react-query - use implementation that returns data by query key
const mockUseQuery = vi.fn()
const mockUseMutation = vi.fn()

type UseQueryOptions = { queryKey?: unknown[] }
type UseMutationOptions = unknown

vi.mock('@tanstack/react-query', () => ({
  useQuery: (...args: UseQueryOptions[]) => mockUseQuery(...args),
  useMutation: (...args: UseMutationOptions[]) => mockUseMutation(...args),
  useQueryClient: vi.fn(() => ({ invalidateQueries: vi.fn(), cancelQueries: vi.fn(), getQueryData: vi.fn(), setQueryData: vi.fn() })),
}))

// Default useQuery mock: returns appropriate data based on query key
function defaultUseQuery(options: UseQueryOptions): { data: unknown; isLoading: boolean } {
  const key = options.queryKey?.[0]
  if (key === 'task') {
    return { data: mockTaskResponse, isLoading: false }
  }
  if (key === 'blockers') {
    return { data: [], isLoading: false }
  }
  // Default for users, sessions, story
  return { data: [], isLoading: false }
}

vi.mock('../api/client', () => ({
  fetchTask: vi.fn(),
  updateTask: vi.fn(),
  updateTaskStatus: vi.fn(),
  fetchBlockers: vi.fn().mockResolvedValue([]),
  addDependency: vi.fn(),
  removeDependency: vi.fn(),
  createTask: vi.fn(),
  deleteTask: vi.fn(),
  getUsers: vi.fn().mockResolvedValue([]),
  fetchSessions: vi.fn().mockResolvedValue([]),
  fetchStory: vi.fn(),
}))

describe('TaskDetail create mode', () => {
  let mockOnClose: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.clearAllMocks()
    mockUseQuery.mockImplementation(defaultUseQuery)
    mockUseMutation.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      isSuccess: false,
      isError: false,
    })
    mockOnClose = vi.fn()
  })

  const renderComponent = (taskId: string) => {
    return render(<TaskDetail taskId={taskId} onClose={mockOnClose} />)
  }

  it('renders create mode UI with "Create Task" header when taskId starts with new-task-', () => {
    renderComponent('new-task-story-1')
    // There are two elements with "Create Task" text - the header (in a span) and the button
    // Check for the header specifically (in a span element with the font-mono class)
    const header = screen.getByText('Create Task', { selector: 'span' })
    expect(header).toBeInTheDocument()
    expect(header).toHaveClass('font-mono')
  })

  it('renders title input with autoFocus', () => {
    renderComponent('new-task-story-1')
    const titleInput = screen.getByPlaceholderText('Task title...')
    expect(titleInput).toBeInTheDocument()
    expect(titleInput).toHaveFocus()
  })

  it('renders task type selector with CODE, BUILD, REVIEW buttons', () => {
    renderComponent('new-task-story-1')
    expect(screen.getByText('CODE')).toBeInTheDocument()
    expect(screen.getByText('BUILD')).toBeInTheDocument()
    expect(screen.getByText('REVIEW')).toBeInTheDocument()
  })

  it('selects CODE by default (highlighted)', () => {
    renderComponent('new-task-story-1')
    const codeButton = screen.getByText('CODE')
    // CODE should be highlighted (has the darker background style)
    expect(codeButton).toHaveClass('border-neutral-800')
  })

  it('allows selecting BUILD task type', async () => {
    const user = userEvent.setup()
    renderComponent('new-task-story-1')

    const buildButton = screen.getByText('BUILD')
    await user.click(buildButton)

    // BUILD should now be highlighted
    expect(buildButton).toHaveClass('border-neutral-800')
    // CODE should no longer be highlighted
    expect(screen.getByText('CODE')).not.toHaveClass('border-neutral-800')
  })

  it('allows selecting REVIEW task type', async () => {
    const user = userEvent.setup()
    renderComponent('new-task-story-1')

    const reviewButton = screen.getByText('REVIEW')
    await user.click(reviewButton)

    // REVIEW should now be highlighted
    expect(reviewButton).toHaveClass('border-neutral-800')
    // CODE should no longer be highlighted
    expect(screen.getByText('CODE')).not.toHaveClass('border-neutral-800')
  })

  it('calls onClose when Escape key is pressed', async () => {
    const user = userEvent.setup()
    renderComponent('new-task-story-1')

    await user.keyboard('{Escape}')

    // onClose is called at least once (from ESC key handler)
    expect(mockOnClose).toHaveBeenCalled()
  })

  it('calls onClose when close button (X) is clicked', async () => {
    const user = userEvent.setup()
    renderComponent('new-task-story-1')

    const closeButton = screen.getByRole('button', { name: /close/i })
    await user.click(closeButton)

    expect(mockOnClose).toHaveBeenCalledTimes(1)
  })

  it('renders Create Task button', () => {
    renderComponent('new-task-story-1')
    expect(screen.getByRole('button', { name: /create task/i })).toBeInTheDocument()
  })

  it('disables Create button when title is empty', () => {
    renderComponent('new-task-story-1')
    const createButton = screen.getByRole('button', { name: /create task/i })
    expect(createButton).toBeDisabled()
  })

  it('disables Create button when title is only whitespace', async () => {
    const user = userEvent.setup()
    renderComponent('new-task-story-1')

    const titleInput = screen.getByPlaceholderText('Task title...')
    await user.type(titleInput, '   ')

    const createButton = screen.getByRole('button', { name: /create task/i })
    expect(createButton).toBeDisabled()
  })

  it('enables Create button when title has content', async () => {
    const user = userEvent.setup()
    renderComponent('new-task-story-1')

    const titleInput = screen.getByPlaceholderText('Task title...')
    await user.type(titleInput, 'Valid task title')

    const createButton = screen.getByRole('button', { name: /create task/i })
    expect(createButton).toBeEnabled()
  })

  it('renders Task Type label in create mode', () => {
    renderComponent('new-task-story-1')
    // Should have task type selector in create mode
    expect(screen.getByText('Task Type')).toBeInTheDocument()
  })
})

describe('edit mode', () => {
  let mockOnClose: ReturnType<typeof vi.fn>
  let mutateMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.clearAllMocks()
    mutateMock = vi.fn()
    mockUseQuery.mockImplementation(defaultUseQuery)
    mockUseMutation.mockReturnValue({
      mutate: mutateMock,
      isPending: false,
      isSuccess: false,
      isError: false,
    })
    mockOnClose = vi.fn()
  })

  const renderComponent = (taskId: string | null) => {
    return render(<TaskDetail taskId={taskId} onClose={mockOnClose} />)
  }

  it('returns null when taskId is null', () => {
    const { container } = renderComponent(null)
    expect(container.innerHTML).toBe('')
  })

  it('shows loading state with PanelLoading "Loading task..."', () => {
    mockUseQuery.mockImplementation((options: UseQueryOptions) => {
      const key = options.queryKey?.[0]
      if (key === 'task') {
        return { data: undefined, isLoading: true }
      }
      return { data: [], isLoading: false }
    })
    renderComponent('task-1')
    expect(screen.getByText('Loading task...')).toBeInTheDocument()
  })

  it('shows not-found state with PanelNotFound "Task not found"', () => {
    mockUseQuery.mockImplementation((options: UseQueryOptions) => {
      const key = options.queryKey?.[0]
      if (key === 'task') {
        return { data: undefined, isLoading: false }
      }
      return { data: [], isLoading: false }
    })
    renderComponent('task-1')
    expect(screen.getByText('Task not found')).toBeInTheDocument()
  })

  it('renders task data including ID, task type SharpTag, title, description, status', () => {
    renderComponent('task-1')
    // Task ID
    expect(screen.getByText('task-1')).toBeInTheDocument()
    // Task type SharpTag (CODE with primary variant) - appears in header and body
    const codeTags = screen.getAllByText('[CODE]')
    expect(codeTags.length).toBeGreaterThanOrEqual(2)
    // Title (rendered via EditableTitle as a button showing the value)
    expect(screen.getByText('Test Task')).toBeInTheDocument()
    // Description
    expect(screen.getByText('Task description')).toBeInTheDocument()
    // Status select should have "In Progress" selected
    const statusSelect = screen.getByRole('combobox')
    expect(statusSelect).toHaveValue('in_progress')
  })

  it('shows instructions section when task has instructions', () => {
    renderComponent('task-1')
    // Instructions (advanced) collapsible
    expect(screen.getByText('Instructions (advanced)')).toBeInTheDocument()
    // Instruction content should be present
    expect(screen.getByText('Do the thing')).toBeInTheDocument()
  })

  it('does not render instructions section when task has no instructions', () => {
    const noInstructionsTask: TaskDetailResponse = {
      ...mockTaskResponse,
      task: { ...mockTaskResponse.task, instructions: undefined },
    }
    mockUseQuery.mockImplementation((options: UseQueryOptions) => {
      const key = options.queryKey?.[0]
      if (key === 'task') {
        return { data: noInstructionsTask, isLoading: false }
      }
      return { data: [], isLoading: false }
    })
    renderComponent('task-1')
    expect(screen.queryByText('Instructions (advanced)')).not.toBeInTheDocument()
  })

  it('renders Depends On blockers with remove buttons', () => {
    mockUseQuery.mockImplementation((options: UseQueryOptions) => {
      const key = options.queryKey?.[0]
      if (key === 'task') {
        return { data: mockTaskResponse, isLoading: false }
      }
      if (key === 'blockers') {
        return {
          data: [
            {
              id: 'dep-task-1',
              story_id: 'story-1',
              title: 'Blocker Task',
              status: 'new',
              task_type: 'code',
              is_stale: false,
              sort_order: 1,
              created_at: '2025-01-01T00:00:00Z',
              updated_at: '2025-01-01T00:00:00Z',
            },
          ],
        }
      }
      return { data: [], isLoading: false }
    })
    renderComponent('task-1')
    expect(screen.getByText('Depends On')).toBeInTheDocument()
    expect(screen.getByText('[dep-task-1] Blocker Task')).toBeInTheDocument()
    // There should be a remove button for each blocker
    const removeButtons = screen.getAllByRole('button', { name: /remove dependency/i })
    expect(removeButtons.length).toBeGreaterThanOrEqual(1)
  })

  it('shows "No dependencies" message when blockers are empty', () => {
    renderComponent('task-1')
    expect(screen.getByText('No dependencies')).toBeInTheDocument()
  })

  it('shows "+ Add" button for adding dependencies', () => {
    // Need story data so availableDepTasks is populated
    mockUseQuery.mockImplementation((options: UseQueryOptions) => {
      const key = options.queryKey?.[0]
      if (key === 'task') {
        return { data: mockTaskResponse, isLoading: false }
      }
      if (key === 'story') {
        return {
          data: {
            story: { id: 'story-1', title: 'Test Story', status: 'new', sort_order: 1, created_at: '', updated_at: '' },
            tasks: [
              { id: 'available-task-1', story_id: 'story-1', title: 'Available Task', status: 'new', task_type: 'review', is_stale: false, sort_order: 1, created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' },
            ],
          },
        }
      }
      return { data: [], isLoading: false }
    })
    renderComponent('task-1')
    expect(screen.getByText('+ Add')).toBeInTheDocument()
  })

  it('calls addDependency mutation when clicking an available task in dropdown', async () => {
    const user = userEvent.setup()
    mockUseQuery.mockImplementation((options: UseQueryOptions) => {
      const key = options.queryKey?.[0]
      if (key === 'task') {
        return { data: mockTaskResponse, isLoading: false }
      }
      if (key === 'story') {
        return {
          data: {
            story: { id: 'story-1', title: 'Test Story', status: 'new', sort_order: 1, created_at: '', updated_at: '' },
            tasks: [
              { id: 'available-task-1', story_id: 'story-1', title: 'Available Task', status: 'new', task_type: 'review', is_stale: false, sort_order: 1, created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' },
            ],
          },
        }
      }
      return { data: [], isLoading: false }
    })
    renderComponent('task-1')

    const addButton = screen.getByText('+ Add')
    await user.click(addButton)

    // The dropdown should show available tasks
    expect(screen.getByText('[available-task-1] Available Task')).toBeInTheDocument()
    await user.click(screen.getByText('[available-task-1] Available Task'))

    // The add dependency mutation should be called
    expect(mutateMock).toHaveBeenCalledWith('available-task-1')
  })

  it('calls removeDependency mutation when clicking X on a blocker', async () => {
    const user = userEvent.setup()
    mockUseQuery.mockImplementation((options: UseQueryOptions) => {
      const key = options.queryKey?.[0]
      if (key === 'task') {
        return { data: mockTaskResponse, isLoading: false }
      }
      if (key === 'blockers') {
        return {
          data: [
            {
              id: 'dep-task-1',
              story_id: 'story-1',
              title: 'Blocker Task',
              status: 'new',
              task_type: 'code',
              is_stale: false,
              sort_order: 1,
              created_at: '2025-01-01T00:00:00Z',
              updated_at: '2025-01-01T00:00:00Z',
            },
          ],
        }
      }
      return { data: [], isLoading: false }
    })
    renderComponent('task-1')

    const removeButton = screen.getByRole('button', { name: /remove dependency/i })
    await user.click(removeButton)

    expect(mutateMock).toHaveBeenCalledWith('dep-task-1')
  })

  it('renders Depended On By list with dependents', () => {
    renderComponent('task-1')
    expect(screen.getByText('Depended On By')).toBeInTheDocument()
    expect(screen.getByText('[dep-task-2] Dependent Task')).toBeInTheDocument()
  })

  it('enables Save button when a field is changed (dirty detection)', async () => {
    const user = userEvent.setup()
    renderComponent('task-1')

    // Initially Save button is disabled (no changes)
    const saveButton = screen.getByRole('button', { name: /^save$/i })
    expect(saveButton).toBeDisabled()

    // Change the description
    const textarea = screen.getByPlaceholderText('Markdown description...')
    await user.clear(textarea)
    await user.type(textarea, 'Updated description')

    // Save should now be enabled
    expect(saveButton).toBeEnabled()
  })

  it('calls updateTask on Save when title is changed', async () => {
    const user = userEvent.setup()
    renderComponent('task-1')

    // Click on the title to enter edit mode
    const titleButton = screen.getByText('Test Task')
    await user.click(titleButton)

    // Clear and type new title in the input
    const titleInput = screen.getByDisplayValue('Test Task')
    await user.clear(titleInput)
    await user.type(titleInput, 'Updated Title')

    // Click the check (save) button to save the title edit
    // In EditableTitle's editing mode, the check button is the first button
    // inside the title editing area
    const titleEditContainer = document.querySelector('.mt-2.flex.gap-2')
    const checkButton = titleEditContainer?.querySelector('button')
    expect(checkButton).toBeTruthy()
    if (checkButton) {
      await user.click(checkButton)
    }

    // Now Save should be enabled
    const saveButton = screen.getByRole('button', { name: /^save$/i })
    await user.click(saveButton)

    // updateMutation.mutate should have been called with title change
    expect(mutateMock).toHaveBeenCalled()
    const callArg = mutateMock.mock.calls[mutateMock.mock.calls.length - 1][0]
    expect(callArg).toHaveProperty('title', 'Updated Title')
  })

  it('calls updateTaskStatus separately on Save when only status is changed', async () => {
    const user = userEvent.setup()
    renderComponent('task-1')

    // Change status
    const statusSelect = screen.getByRole('combobox')
    await user.selectOptions(statusSelect, 'done')

    // Click Save
    const saveButton = screen.getByRole('button', { name: /^save$/i })
    await user.click(saveButton)

    // mutateMock should be the statusMutation since only status changed
    // (updateTask won't be called when only status changes)
    // statusMutation.mutate('done') should have been called
    expect(mutateMock).toHaveBeenCalledWith('done')
  })

  it('calls Cancel to reset draft and close', async () => {
    const user = userEvent.setup()
    const { unmount } = render(<TaskDetail taskId="task-1" onClose={mockOnClose} />)

    // Make a change to description
    const textarea = screen.getByPlaceholderText('Markdown description...')
    await user.clear(textarea)
    await user.type(textarea, 'Changed description')

    // Click Cancel
    const cancelButton = screen.getByText('Cancel')
    await user.click(cancelButton)

    // onClose should be called
    expect(mockOnClose).toHaveBeenCalled()

    // Unmount the first render and re-render fresh
    unmount()
    const { container } = render(<TaskDetail taskId="task-1" onClose={mockOnClose} />)
    // The description textarea should contain the original value (after useEffect syncs draft)
    const textareas = container.querySelectorAll('textarea')
    const descTextarea = Array.from(textareas).find(t => t.value === 'Task description')
    expect(descTextarea).toBeTruthy()
  })

  it('shows ConfirmModal on Delete click and calls deleteMutation on confirm', async () => {
    const user = userEvent.setup()
    renderComponent('task-1')

    // Click Delete Task button
    const deleteButton = screen.getByText('Delete Task')
    await user.click(deleteButton)

    // ConfirmModal should be visible with title "Delete Task"
    expect(screen.getByText('Delete Task', { selector: 'h2' })).toBeInTheDocument()
    expect(screen.getByText('Are you sure you want to delete this task? This action cannot be undone.')).toBeInTheDocument()

    // Click Confirm
    const confirmButton = screen.getByText('Confirm')
    await user.click(confirmButton)

    // deleteMutation.mutate should have been called
    expect(mutateMock).toHaveBeenCalled()
  })
})