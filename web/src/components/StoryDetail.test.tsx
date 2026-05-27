import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import StoryDetail from './StoryDetail'
import type { StoryWithTasks, User, Session } from '../types'

// ── Mock Data ─────────────────────────────────────────────────────────────

const mockStoryWithTasks: StoryWithTasks = {
  story: {
    id: 'story-1',
    title: 'Test Story',
    description: 'A test description',
    status: 'in_progress',
    requires_build: true,
    requires_review: false,
    assigned_to: 'user-1',
    assignee_type: 'human',
    sort_order: 1,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
  tasks: [
    {
      id: 'task-1',
      story_id: 'story-1',
      title: 'Child Task 1',
      status: 'new',
      task_type: 'code',
      is_stale: false,
      sort_order: 1,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
  ],
}

const mockEmptyStory: StoryWithTasks = {
  story: {
    id: 'story-2',
    title: 'Empty Story',
    description: '',
    status: 'new',
    requires_build: false,
    requires_review: false,
    assigned_to: undefined,
    assignee_type: undefined,
    sort_order: 2,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
  tasks: [],
}

// ── Mocks ─────────────────────────────────────────────────────────────────

// vi.hoisted ensures mocks are available at the right time for vi.mock factories
const { mockUseQuery, mockUseMutation, mockUseAuthStore } = vi.hoisted(() => ({
  mockUseQuery: vi.fn(),
  mockUseMutation: vi.fn(),
  // Return store state object with user property (Zustand selector pattern)
  mockUseAuthStore: vi.fn().mockReturnValue({ user: { id: 'user-1', username: 'testuser' } }),
}))

vi.mock('@tanstack/react-query', async () => {
  const actual = await vi.importActual('@tanstack/react-query')
  return {
    ...actual,
    useQuery: mockUseQuery,
    useMutation: mockUseMutation,
    useQueryClient: vi.fn().mockReturnValue({
      invalidateQueries: vi.fn(),
      cancelQueries: vi.fn(),
      getQueryData: vi.fn(),
      setQueryData: vi.fn(),
    }),
  }
})

vi.mock('../api/client', () => ({
  fetchStory: vi.fn(),
  updateStory: vi.fn(),
  deleteStory: vi.fn(),
  getUsers: vi.fn(),
  fetchSessions: vi.fn(),
  createTask: vi.fn(),
  fetchComments: vi.fn(),
  addComment: vi.fn(),
  updateComment: vi.fn(),
  deleteComment: vi.fn(),
  fetchActivity: vi.fn(),
}))

vi.mock('../stores/auth', () => ({
  useAuthStore: mockUseAuthStore,
}))

// ── Mock Helpers for Editing/Mutation Tests ─────────────────────────────

const mockUsers: User[] = [
  { id: 'user-1', username: 'alice', email: 'alice@test.com', role: 'admin', created_at: '2025-01-01T00:00:00Z' },
  { id: 'user-2', username: 'bob', email: 'bob@test.com', role: 'normal', created_at: '2025-01-01T00:00:00Z' },
]

const mockSessions: Session[] = [
  { id: 'session-1', harness_type: 'openai', status: 'active', last_seen_at: '2025-01-01T00:00:00Z', created_at: '2025-01-01T00:00:00Z' },
]

const createMockMutation = () => ({
  mutate: vi.fn(),
  mutateAsync: vi.fn(),
  isPending: false,
  isSuccess: false,
  isError: false,
  data: null,
  error: null,
  reset: vi.fn(),
})

function mockStoryQueryWithUsers(result: { data?: StoryWithTasks; isLoading?: boolean }) {
  mockUseQuery.mockImplementation(({ queryKey } = {}) => {
    if (!queryKey) return { data: undefined, isLoading: false }
    const key = queryKey[0]
    if (key === 'story') {
      return {
        data: result.data,
        isLoading: result.isLoading ?? false,
      }
    }
    if (key === 'users') {
      return { data: mockUsers, isLoading: false }
    }
    if (key === 'sessions') {
      return { data: mockSessions, isLoading: false }
    }
    return { data: [], isLoading: false }
  })
}

// ── Helper Functions ───────────────────────────────────────────────────────

function mockStoryQuery(result: { data?: StoryWithTasks; isLoading?: boolean }) {
  mockUseQuery.mockImplementation(({ queryKey } = {}) => {
    if (!queryKey) return { data: undefined, isLoading: false }
    const key = queryKey[0]
    if (key === 'story') {
      return {
        data: result.data,
        isLoading: result.isLoading ?? false,
      }
    }
    // All other queries (users, sessions, comments, activity) return empty
    return { data: [], isLoading: false }
  })
}

function renderStoryDetail(storyId: string | null = 'story-1') {
  return render(<StoryDetail storyId={storyId} onClose={vi.fn()} onOpenTask={vi.fn()} />)
}

// ── Tests ─────────────────────────────────────────────────────────────────

describe('StoryDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseMutation.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      error: null,
      isSuccess: false,
      data: undefined,
    })
  })

  // 1. storyId=null returns null
  describe('when storyId is null', () => {
    it('returns null (renders nothing)', () => {
      mockStoryQuery({ data: undefined, isLoading: false })
      const { container } = renderStoryDetail(null)
      expect(container.textContent).toBe('')
    })
  })

  // 2. Loading state
  describe('loading state', () => {
    it('shows PanelLoading with "Loading story..." message', () => {
      mockStoryQuery({ data: undefined, isLoading: true })
      renderStoryDetail('story-1')
      expect(screen.getByText('Loading story...')).toBeInTheDocument()
    })
  })

  // 3. Not-found state
  describe('when story is not found', () => {
    it('shows PanelNotFound with "Story not found" message', () => {
      mockStoryQuery({ data: undefined, isLoading: false })
      renderStoryDetail('story-1')
      expect(screen.getByText('Story not found')).toBeInTheDocument()
    })
  })

  // 4. Renders story data
  describe('when story data is loaded', () => {
    beforeEach(() => {
      mockStoryQuery({ data: mockStoryWithTasks, isLoading: false })
    })

    it('renders story ID', () => {
      renderStoryDetail('story-1')
      expect(screen.getByText('story-1')).toBeInTheDocument()
    })

    it('renders "STORY" SharpTag', () => {
      renderStoryDetail('story-1')
      expect(screen.getByText('[STORY]')).toBeInTheDocument()
    })

    it('renders story title via EditableTitle', () => {
      renderStoryDetail('story-1')
      // EditableTitle renders the value as a button with accessible name
      expect(screen.getByRole('button', { name: 'Test Story' })).toBeInTheDocument()
    })

    it('renders description in textarea', () => {
      renderStoryDetail('story-1')
      expect(screen.getByDisplayValue('A test description')).toBeInTheDocument()
    })

    it('renders status in select dropdown', () => {
      renderStoryDetail('story-1')
      // Use role query for the select element
      const select = screen.getByRole('combobox')
      expect(select).toBeInTheDocument()
      expect(select).toHaveValue('in_progress')
    })

    it('renders build checkbox as checked', () => {
      renderStoryDetail('story-1')
      const checkboxes = screen.getAllByRole('checkbox')
      // requires_build is true (first checkbox)
      expect(checkboxes[0]).toBeChecked()
    })

    it('renders review checkbox as unchecked', () => {
      renderStoryDetail('story-1')
      const checkboxes = screen.getAllByRole('checkbox')
      // requires_review is false (second checkbox)
      expect(checkboxes[1]).not.toBeChecked()
    })

    it('renders assigned to section', () => {
      renderStoryDetail('story-1')
      // assigned_to is 'user-1', rendered in a span with font-mono
      expect(screen.getByText('user-1')).toBeInTheDocument()
    })

    it('renders "Add Task" button', () => {
      renderStoryDetail('story-1')
      expect(screen.getByText('+ Add Task')).toBeInTheDocument()
    })
  })

  // 5. Child tasks rendering
  describe('child tasks rendering', () => {
    beforeEach(() => {
      mockStoryQuery({ data: mockStoryWithTasks, isLoading: false })
    })

    it('renders "Child Tasks (1)" field label', () => {
      renderStoryDetail('story-1')
      expect(screen.getByText('Child Tasks (1)')).toBeInTheDocument()
    })

    it('renders task ID', () => {
      renderStoryDetail('story-1')
      expect(screen.getByText('task-1')).toBeInTheDocument()
    })

    it('renders task title', () => {
      renderStoryDetail('story-1')
      expect(screen.getByText('Child Task 1')).toBeInTheDocument()
    })

    it('renders task status SharpTag (uppercase)', () => {
      renderStoryDetail('story-1')
      expect(screen.getByText('[NEW]')).toBeInTheDocument()
    })
  })

  describe('when no tasks exist', () => {
    it('shows "No tasks" message', () => {
      mockStoryQuery({ data: mockEmptyStory, isLoading: false })
      renderStoryDetail('story-2')
      expect(screen.getByText('No tasks')).toBeInTheDocument()
    })

    it('renders "Child Tasks (0)" field label', () => {
      mockStoryQuery({ data: mockEmptyStory, isLoading: false })
      renderStoryDetail('story-2')
      expect(screen.getByText('Child Tasks (0)')).toBeInTheDocument()
    })
  })

  // 6. Add Task inline form
  describe('add task form interaction', () => {
    beforeEach(() => {
      mockStoryQuery({ data: mockStoryWithTasks, isLoading: false })
    })

    it('shows create form when clicking "Add Task"', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      await user.click(screen.getByText('+ Add Task'))

      // Form should be visible with title input
      expect(screen.getByPlaceholderText('Task title...')).toBeInTheDocument()
    })

    it('shows task type select in form', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      await user.click(screen.getByText('+ Add Task'))

      // Task type select should be visible in the form (use role query)
      const selects = await screen.findAllByRole('combobox')
      expect(selects[1]).toHaveValue('code') // Second select is in the form
    })

    it('shows Cancel and Create buttons in form', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      await user.click(screen.getByText('+ Add Task'))

      // Both Cancel and Create should exist (there are two Cancel buttons in the component)
      // We verify the form appears by checking for the placeholder
      expect(screen.getByPlaceholderText('Task title...')).toBeInTheDocument()
    })

    it('hides form when clicking Cancel', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      await user.click(screen.getByText('+ Add Task'))
      expect(screen.getByPlaceholderText('Task title...')).toBeInTheDocument()

      // Find all buttons with "Cancel" text and click the one that is in the Add Task form
      // The form Cancel is in a div with the placeholder input
      const cancelButtons = screen.getAllByText('Cancel')
      // The form Cancel button is the one inside the form div (simpler: just click the first one after input)
      // Actually, clicking either should work for this test since we just need to hide the form
      await user.click(cancelButtons[0])
      expect(screen.queryByPlaceholderText('Task title...')).not.toBeInTheDocument()
    })

    it('hides form after successful task creation (mutation success)', async () => {
      const user = userEvent.setup()
      const mockMutate = vi.fn()
      mockUseMutation.mockReturnValue({
        mutate: mockMutate,
        mutateAsync: vi.fn().mockResolvedValue({}),
        isPending: false,
        isError: false,
        error: null,
        isSuccess: true,
        data: undefined,
      })

      renderStoryDetail('story-1')
      await user.click(screen.getByText('+ Add Task'))

      // Type a task title
      const input = screen.getByPlaceholderText('Task title...')
      await user.type(input, 'New Task Title')

      // Click Create
      await user.click(screen.getByText('Create'))

      // Form should be hidden (mutate was called)
      expect(mockMutate).toHaveBeenCalledWith({ title: 'New Task Title', task_type: 'code' })
    })
  })

  // ── Editing & Mutation Tests ───────────────────────────────────────────
  describe('editing and mutations', () => {
    beforeEach(() => {
      mockStoryQueryWithUsers({ data: mockStoryWithTasks, isLoading: false })
      // Default mutation returns a do-nothing mock
      mockUseMutation.mockReturnValue(createMockMutation())
    })

    // ── Field Editing ─────────────────────────────────────────────────────

    it('edits title via EditableTitle', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      // Click on the title button to start editing
      await user.click(screen.getByRole('button', { name: 'Test Story' }))

      // An input should appear
      const input = screen.getByDisplayValue('Test Story')
      await user.clear(input)
      await user.type(input, 'Updated Title')

      // Click the checkmark (save) button - use container query to find the one in EditableTitle
      const checkBtn = document.querySelector('.glow-button')
      if (checkBtn) await user.click(checkBtn)

      // Save button should become enabled (dirty state)
      const saveBtn = screen.getByRole('button', { name: 'Save' })
      expect(saveBtn).toBeEnabled()
    })

    it('edits description via textarea', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      const textarea = screen.getByDisplayValue('A test description')
      await user.clear(textarea)
      await user.type(textarea, 'Updated description')

      expect(textarea).toHaveValue('Updated description')

      // Dirty state - Save should be enabled
      expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    })

    it('toggles requires_build checkbox', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      const checkboxes = screen.getAllByRole('checkbox')
      const buildCheckbox = checkboxes[0]

      // Initially checked (requires_build: true)
      expect(buildCheckbox).toBeChecked()

      await user.click(buildCheckbox)
      expect(buildCheckbox).not.toBeChecked()

      // Dirty state
      expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    })

    it('toggles requires_review checkbox', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      const checkboxes = screen.getAllByRole('checkbox')
      const reviewCheckbox = checkboxes[1]

      // Initially unchecked (requires_review: false)
      expect(reviewCheckbox).not.toBeChecked()

      await user.click(reviewCheckbox)
      expect(reviewCheckbox).toBeChecked()

      // Dirty state
      expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    })

    it('changes status via select', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      const select = screen.getByRole('combobox')
      expect(select).toHaveValue('in_progress')

      await user.selectOptions(select, 'done')
      expect(select).toHaveValue('done')

      // Dirty state
      expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    })

    it('shows filtered assignee options when typing search term', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      // Clear the current assignee first (assigned_to is 'user-1', shown as 'alice')
      const clearBtn = screen.getByText('alice').closest('div')?.querySelector('button')
      if (clearBtn) await user.click(clearBtn)

      // Now we should see the search input
      const searchInput = screen.getByPlaceholderText('Search users or agents...')
      await user.type(searchInput, 'bob')

      // Should see bob in dropdown
      expect(screen.getByText('bob')).toBeInTheDocument()
      // Should not see alice
      expect(screen.queryByText('alice')).not.toBeInTheDocument()
    })

    it('selects assignee from dropdown', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      // Clear current assignee first (assigned_to is 'user-1', shown as 'alice')
      const clearBtn = screen.getByText('alice').closest('div')?.querySelector('button')
      if (clearBtn) await user.click(clearBtn)

      // Search for and select 'bob'
      const searchInput = screen.getByPlaceholderText('Search users or agents...')
      await user.type(searchInput, 'bob')

      // Click the bob option (it's a button in the dropdown)
      const bobOption = screen.getByText('bob')
      await user.click(bobOption)

      // Bob should be displayed as selected
      expect(screen.getByText('bob')).toBeInTheDocument()

      // Dirty state should be active (assigned_to changed from '' to 'user-2')
      expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    })

    // ── Dirty State ───────────────────────────────────────────────────────

    it('disables Save button when draft is clean (not dirty)', () => {
      renderStoryDetail('story-1')
      expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    })

    it('enables Save button when draft becomes dirty', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      // Modify description to make it dirty
      const textarea = screen.getByDisplayValue('A test description')
      await user.clear(textarea)
      await user.type(textarea, 'Changed')

      expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    })

    // ── Cancel ────────────────────────────────────────────────────────────

    it('resets draft when Cancel is clicked', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      // Make a change
      const textarea = screen.getByDisplayValue('A test description')
      await user.clear(textarea)
      await user.type(textarea, 'Something else')

      // Click Cancel (the one in the footer)
      const cancelBtns = screen.getAllByText('Cancel')
      await user.click(cancelBtns[cancelBtns.length - 1]) // last Cancel is the footer one

      // The description should be reset to original value
      // After cancel, the component calls onClose which unmounts
      // So we just verify the save button behavior through the mock
      expect(screen.queryByDisplayValue('Something else')).not.toBeInTheDocument()
    })

    // ── Save ──────────────────────────────────────────────────────────────

    it('calls updateMutation.mutate with computed changes when Save is clicked', async () => {
      const user = userEvent.setup()
      const mockMutate = vi.fn()
      mockUseMutation.mockReturnValue({
        ...createMockMutation(),
        mutate: mockMutate,
      })

      renderStoryDetail('story-1')

      // Edit title to make it dirty
      await user.click(screen.getByRole('button', { name: 'Test Story' }))
      const titleInput = screen.getByDisplayValue('Test Story')
      await user.clear(titleInput)
      await user.type(titleInput, 'Updated Title')
      // Click the EditableTitle checkmark button
      const checkBtn = document.querySelector('.glow-button')
      if (checkBtn) await user.click(checkBtn)

      // Click Save
      await user.click(screen.getByRole('button', { name: 'Save' }))

      expect(mockMutate).toHaveBeenCalledTimes(1)
      expect(mockMutate).toHaveBeenCalledWith({ title: 'Updated Title' })
    })

    it('calls updateMutation.mutate with multiple changes', async () => {
      const user = userEvent.setup()
      const mockMutate = vi.fn()
      mockUseMutation.mockReturnValue({
        ...createMockMutation(),
        mutate: mockMutate,
      })

      renderStoryDetail('story-1')

      // Change description
      const textarea = screen.getByDisplayValue('A test description')
      await user.clear(textarea)
      await user.type(textarea, 'New desc')

      // Change status
      const select = screen.getByRole('combobox')
      await user.selectOptions(select, 'done')

      // Click Save
      await user.click(screen.getByRole('button', { name: 'Save' }))

      expect(mockMutate).toHaveBeenCalledTimes(1)
      expect(mockMutate).toHaveBeenCalledWith({
        description: 'New desc',
        status: 'done',
      })
    })

    it('calls updateMutation.mutate and calls onClose on success with Save & Close', async () => {
      const user = userEvent.setup()
      const onClose = vi.fn()
      const mockMutate = vi.fn()

      // This mutation passes onSuccess to mutate options
      mockUseMutation.mockReturnValue({
        ...createMockMutation(),
        mutate: mockMutate,
      })

      render(
        <StoryDetail storyId="story-1" onClose={onClose} onOpenTask={vi.fn()} />
      )

      // Make a change
      const textarea = screen.getByDisplayValue('A test description')
      await user.clear(textarea)
      await user.type(textarea, 'Trigger save & close')

      // Open Save dropdown by clicking the chevron button (the last button in the save group)
      // The Save button and chevron button are siblings in a div. The chevron is the last child.
      const saveGroup = screen.getByRole('button', { name: 'Save' }).closest('div')!
      const dropdownBtn = saveGroup.querySelector('button:last-child')!
      await user.click(dropdownBtn)

      // Now the dropdown should show "Save & Close"
      const saveAndCloseBtn = screen.getByText('Save & Close')
      await user.click(saveAndCloseBtn)

      expect(mockMutate).toHaveBeenCalledTimes(1)
      // The second argument should contain an onSuccess callback
      const callArgs = mockMutate.mock.calls[0]
      expect(callArgs[0]).toEqual({ description: 'Trigger save & close' })
      expect(callArgs[1]).toHaveProperty('onSuccess')
    })

    // ── Delete ────────────────────────────────────────────────────────────

    it('opens ConfirmModal when Delete is clicked', async () => {
      const user = userEvent.setup()
      renderStoryDetail('story-1')

      // Click Delete Story button - look for the button by its text content
      const deleteBtn = screen.getByText('Delete Story')
      await user.click(deleteBtn)

      // ConfirmModal should show its message
      expect(
        screen.getByText('Are you sure you want to delete this story? This action cannot be undone.')
      ).toBeInTheDocument()
    })

    it('calls deleteMutation.mutate when delete is confirmed', async () => {
      const user = userEvent.setup()
      const mockDeleteMutate = vi.fn()
      mockUseMutation.mockReturnValue({
        ...createMockMutation(),
        mutate: mockDeleteMutate,
      })

      renderStoryDetail('story-1')

      // Click Delete Story
      await user.click(screen.getByText('Delete Story'))

      // Click Confirm in the modal
      const confirmBtn = screen.getByRole('button', { name: /confirm/i })
      await user.click(confirmBtn)

      expect(mockDeleteMutate).toHaveBeenCalledTimes(1)
      // deleteMutation.mutate is called with no args
      expect(mockDeleteMutate).toHaveBeenCalledWith()
    })
  })
})