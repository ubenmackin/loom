import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, act, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import CommentThread from './CommentThread'
import type { Comment, ActivityLogEntry, User } from '../types'

// ── Mock Data ─────────────────────────────────────────────────────────────

const mockUsers: User[] = [
  { id: 'user-1', username: 'alice', email: 'alice@test.com', display_name: 'Alice', role: 'admin', created_at: '2025-01-01T00:00:00Z' },
  { id: 'user-2', username: 'bob', email: 'bob@test.com', display_name: 'Bob', role: 'normal', created_at: '2025-01-01T00:00:00Z' },
]

const mockComments: Comment[] = [
  { id: 'c1', work_item_id: 'story-1', work_item_type: 'story', author_id: 'user-1', author_type: 'human', body: 'First comment', created_at: '2025-01-01T12:00:00Z', updated_at: '2025-01-01T12:00:00Z' },
  { id: 'c2', work_item_id: 'story-1', work_item_type: 'story', author_id: 'user-2', author_type: 'human', body: 'Second comment', created_at: '2025-01-01T13:00:00Z', updated_at: '2025-01-01T13:00:00Z' },
]

const mockActivities: ActivityLogEntry[] = [
  { id: 'a1', work_item_id: 'story-1', work_item_type: 'story', action: 'created', details: 'Story created', created_at: '2025-01-01T11:00:00Z' },
]

const mockAgentComment: Comment = {
  id: 'c3', work_item_id: 'story-1', work_item_type: 'story', author_id: 'session-1', author_type: 'session', body: 'Agent comment', created_at: '2025-01-01T14:00:00Z', updated_at: '2025-01-01T14:00:00Z',
}

// ── Mocks ─────────────────────────────────────────────────────────────────

const {
  mockUseQuery,
  mockUseAuthStore,
  mockCancelQueries,
  mockGetQueryData,
  mockSetQueryData,
  mockInvalidateQueries,
  mockAddComment,
  mockUpdateComment,
  mockDeleteComment,
  mockFetchComments,
  mockFetchActivity,
  mockGetUsers,
} = vi.hoisted(() => {
  const mockAuthState = { user: { id: 'user-1', username: 'alice', display_name: 'Alice' } }
  return {
    mockUseQuery: vi.fn(),
    mockUseAuthStore: vi.fn((selector?: (state: typeof mockAuthState) => unknown) => {
      if (selector) return selector(mockAuthState)
      return mockAuthState
    }),
    mockCancelQueries: vi.fn(),
    mockGetQueryData: vi.fn(),
    mockSetQueryData: vi.fn(),
    mockInvalidateQueries: vi.fn(),
    mockAddComment: vi.fn(),
    mockUpdateComment: vi.fn(),
    mockDeleteComment: vi.fn(),
    mockFetchComments: vi.fn(),
    mockFetchActivity: vi.fn(),
    mockGetUsers: vi.fn(),
  }
})

// Mock React Query — useQuery is mocked, but useMutation is REAL
// We keep mocking useQueryClient to return our spy methods for testing
vi.mock('@tanstack/react-query', async () => {
  const actual = await vi.importActual('@tanstack/react-query')
  return {
    ...actual,
    useQuery: mockUseQuery,
    useMutation: actual.useMutation, // REAL useMutation
    // Keep using the mock so callbacks use our spy functions
    useQueryClient: vi.fn().mockReturnValue({
      invalidateQueries: mockInvalidateQueries,
      cancelQueries: mockCancelQueries,
      getQueryData: mockGetQueryData,
      setQueryData: mockSetQueryData,
    }),
  }
})

vi.mock('../api/client', () => ({
  fetchComments: (...args: unknown[]) => mockFetchComments(...args),
  fetchActivity: (...args: unknown[]) => mockFetchActivity(...args),
  addComment: (...args: unknown[]) => mockAddComment(...args),
  updateComment: (...args: unknown[]) => mockUpdateComment(...args),
  deleteComment: (...args: unknown[]) => mockDeleteComment(...args),
  getUsers: (...args: unknown[]) => mockGetUsers(...args),
}))

vi.mock('../stores/auth', () => ({
  useAuthStore: mockUseAuthStore,
}))

// ── Test QueryClient & Render Helpers ─────────────────────────────────────

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
  }
}

function setupQueryMocks(overrides: {
  comments?: Comment[]
  activities?: ActivityLogEntry[]
  users?: User[]
}) {
  const { comments = [], activities = [], users = mockUsers } = overrides
  mockUseQuery.mockImplementation(({ queryKey }: { queryKey: string[] }) => {
    const key = queryKey[0]
    if (key === 'users') {
      return { data: users, isLoading: false }
    }
    if (key === 'comments') {
      return { data: comments, isLoading: false }
    }
    if (key === 'activity') {
      return { data: activities, isLoading: false }
    }
    return { data: [], isLoading: false }
  })
}

function renderWithProviders(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return render(ui, { wrapper: createWrapper(queryClient) })
}

// ── Tests ─────────────────────────────────────────────────────────────────

describe('CommentThread', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Default: API calls resolve successfully
    mockAddComment.mockResolvedValue({ id: 'new-c1', body: 'New comment', author_id: 'user-1' })
    mockUpdateComment.mockResolvedValue({ id: 'c1', body: 'Updated' })
    mockDeleteComment.mockResolvedValue(undefined)
  })

  // ── 1. Empty state ────────────────────────────────────────────────────
  it('shows "No activity yet" when there are no comments or activities', () => {
    setupQueryMocks({ comments: [], activities: [] })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)
    expect(screen.getByText('No activity yet')).toBeInTheDocument()
  })

  // ── 2. Renders comments ─────────────────────────────────────────────
  it('renders a comment with body text in the timeline', () => {
    setupQueryMocks({ comments: mockComments })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)
    expect(screen.getByText('First comment')).toBeInTheDocument()
    expect(screen.getByText('Second comment')).toBeInTheDocument()
  })

  // ── 3. Renders activities ─────────────────────────────────────────────
  it('renders an activity entry with action text in the timeline', () => {
    setupQueryMocks({ activities: mockActivities })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)
    // The details text "Story created" is unique and confirms the activity rendered
    expect(screen.getByText(/Story created/)).toBeInTheDocument()
    // The activity div contains the action text; we verify via container text content
    const container = screen.getByText('Activity & Comments').closest('div')!
    expect(container.textContent).toContain('created')
  })

  // ── 4. Chronological ordering ─────────────────────────────────────────
  it('merges and sorts comments and activities by created_at', () => {
    setupQueryMocks({ comments: mockComments, activities: mockActivities })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)
    // Activity at 11:00, comment c1 at 12:00, comment c2 at 13:00
    const container = screen.getByText('Activity & Comments').closest('div')!
    const allText = container.textContent!
    const activityPos = allText.indexOf('created')
    const firstCommentPos = allText.indexOf('First comment')
    const secondCommentPos = allText.indexOf('Second comment')
    expect(activityPos).toBeLessThan(firstCommentPos)
    expect(firstCommentPos).toBeLessThan(secondCommentPos)
  })

  // ── 5. Current user display ───────────────────────────────────────────
  it('shows the current user display_name for comments authored by them', () => {
    setupQueryMocks({ comments: [mockComments[0]] }) // c1 authored by user-1 (Alice)
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
  })

  // ── 6. Other user display ─────────────────────────────────────────────
  it('shows other user name from userMap for comments not by current user', () => {
    setupQueryMocks({ comments: [mockComments[1]] }) // c2 authored by user-2 (Bob)
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)
    expect(screen.getByText('Bob')).toBeInTheDocument()
  })

  // ── 7. Agent comments ─────────────────────────────────────────────────
  it('renders agent comments with session-indicator (purple border)', () => {
    setupQueryMocks({ comments: [mockAgentComment] })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)
    expect(screen.getByText('Agent comment')).toBeInTheDocument()
    // Find the comment container div with the purple border class
    const commentDiv = screen.getByText('Agent comment').closest('div')!
    expect(commentDiv.className).toContain('border-purple-active')
  })

  // ── 8. Edit mode ──────────────────────────────────────────────────────
  it('clicking Pencil on own comment switches to edit textarea', async () => {
    const user = userEvent.setup()
    setupQueryMocks({ comments: [mockComments[0]] }) // c1 is by user-1 (current user)
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    const editButton = screen.getByLabelText('Edit comment')
    await user.click(editButton)

    // Should show a textarea with the comment body, and Save/Cancel buttons
    const textarea = screen.getByDisplayValue('First comment')
    expect(textarea).toBeInTheDocument()
    // Check button (Check icon) and Cancel button
    expect(
      document.querySelector('button svg.lucide-check') || screen.getByText('Cancel'),
    ).toBeTruthy()
  })

  // ── 9. Edit save ──────────────────────────────────────────────────────
  it('clicking Check calls updateComment API', async () => {
    const user = userEvent.setup()
    setupQueryMocks({ comments: [mockComments[0]] })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    // Enter edit mode
    const editButton = screen.getByLabelText('Edit comment')
    await user.click(editButton)

    // Change the text
    const textarea = screen.getByDisplayValue('First comment')
    await user.clear(textarea)
    await user.type(textarea, 'Updated comment')

    // Click the Check (save) button
    const checkButton = document.querySelector('button svg.lucide-check')?.closest('button')
    expect(checkButton).toBeTruthy()
    if (checkButton) {
      await user.click(checkButton)
    }

    // With real useMutation, the mutation function calls the API
    await waitFor(() => {
      expect(mockUpdateComment).toHaveBeenCalledWith('c1', { body: 'Updated comment' })
    })
  })

  // ── 10. Edit cancel ───────────────────────────────────────────────────
  it('clicking Cancel exits edit mode without saving', async () => {
    const user = userEvent.setup()
    setupQueryMocks({ comments: [mockComments[0]] })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    // Enter edit mode
    const editButton = screen.getByLabelText('Edit comment')
    await user.click(editButton)

    // Change the text
    const textarea = screen.getByDisplayValue('First comment')
    await user.clear(textarea)
    await user.type(textarea, 'Should not be saved')

    // Click Cancel
    await user.click(screen.getByText('Cancel'))

    // The API should NOT have been called
    expect(mockUpdateComment).not.toHaveBeenCalled()

    // The original text should be shown again (not in edit mode)
    expect(screen.getByText('First comment')).toBeInTheDocument()
  })

  // ── 11. Delete ────────────────────────────────────────────────────────
  it('clicking X calls deleteComment API', async () => {
    const user = userEvent.setup()
    setupQueryMocks({ comments: [mockComments[0]] })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    const deleteButton = screen.getByLabelText('Delete comment')
    await user.click(deleteButton)

    // With real useMutation, deleteMutation.mutate('c1') is called, which calls deleteComment API
    await waitFor(() => {
      expect(mockDeleteComment).toHaveBeenCalledWith('c1')
    })
  })

  // ── 12. New comment form ──────────────────────────────────────────────
  it('renders the new comment form with textarea and Send button', () => {
    setupQueryMocks({ comments: [], activities: [] })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    const textarea = screen.getByPlaceholderText('Add a comment... (Cmd+Enter to send)')
    expect(textarea).toBeInTheDocument()

    // Send button (with Send icon)
    expect(document.querySelector('button svg.lucide-send')).toBeInTheDocument()
  })

  // ── 13. Send button ───────────────────────────────────────────────────
  it('clicking Send with non-empty body calls addComment API', async () => {
    const user = userEvent.setup()
    setupQueryMocks({ comments: [], activities: [] })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    const textarea = screen.getByPlaceholderText('Add a comment... (Cmd+Enter to send)')
    await user.type(textarea, 'New comment body')

    // Click the Send button
    const sendButton = document.querySelector('button svg.lucide-send')?.closest('button')
    expect(sendButton).toBeTruthy()
    if (sendButton) {
      await user.click(sendButton)
    }

    // With real useMutation, addMutation.mutate('New comment body') is called,
    // which calls addComment API
    await waitFor(() => {
      expect(mockAddComment).toHaveBeenCalledWith('story-1', 'story', {
        body: 'New comment body',
        author_id: 'user-1',
        author_type: 'human',
      })
    })
  })

  // ── 14. Cmd+Enter sends ───────────────────────────────────────────────
  it('pressing Cmd+Enter in the new comment textarea sends the comment', async () => {
    const user = userEvent.setup()
    setupQueryMocks({ comments: [], activities: [] })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    const textarea = screen.getByPlaceholderText('Add a comment... (Cmd+Enter to send)')
    await user.type(textarea, 'Cmd+Enter comment')
    await user.keyboard('{Meta>}{Enter}{/Meta}')

    await waitFor(() => {
      expect(mockAddComment).toHaveBeenCalledWith('story-1', 'story', {
        body: 'Cmd+Enter comment',
        author_id: 'user-1',
        author_type: 'human',
      })
    })
  })

  // ── 15. Empty body disables Send ──────────────────────────────────────
  it('disables Send button when body is empty', () => {
    setupQueryMocks({ comments: [], activities: [] })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    const sendButton = document.querySelector('button svg.lucide-send')?.closest('button')
    expect(sendButton).toBeDisabled()
  })

  // ── 16. Optimistic update — onMutate inserts temp comment ────────────
  it('onMutate inserts a temp comment into the query cache before API responds', async () => {
    const user = userEvent.setup()
    const existingComments: Comment[] = [mockComments[0]]
    setupQueryMocks({ comments: existingComments, activities: [] })

    // Seed getQueryData to return the existing comments when onMutate queries the cache
    mockGetQueryData.mockReturnValue(existingComments)

    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    // Type and send a new comment
    const textarea = screen.getByPlaceholderText('Add a comment... (Cmd+Enter to send)')
    await user.type(textarea, 'New optimistic comment')

    const sendButton = document.querySelector('button svg.lucide-send')?.closest('button')
    await user.click(sendButton!)

    // The mutation callbacks execute synchronously after mutate() is called:
    // 1. onMutate: cancels queries, snapshots previous, adds temp comment
    // 2. mutationFn: calls addComment (which is mocked to resolve)
    // 3. onSettled: invalidates queries

    // Wait for the onMutate to have executed (should be immediate after mutate call)
    await act(async () => {
      // Small delay to ensure onMutate has run
      await new Promise((r) => setTimeout(r, 10))
    })

    // 1. Should have cancelled in-flight queries for the comments key
    expect(mockCancelQueries).toHaveBeenCalledWith({
      queryKey: ['comments', 'story-1', 'story'],
    })

    // 2. Should have snapshot the previous comments
    expect(mockGetQueryData).toHaveBeenCalledWith(['comments', 'story-1', 'story'])

    // 3. Should have set the cache to include the temp comment
    expect(mockSetQueryData).toHaveBeenCalledWith(
      ['comments', 'story-1', 'story'],
      expect.arrayContaining([
        expect.objectContaining({
          id: expect.stringMatching(/^temp-\d+$/),
          body: 'New optimistic comment',
          author_id: 'user-1',
        }),
      ]),
    )

    // Verify the original comment is still present
    const setDataArg = mockSetQueryData.mock.calls[0][1] as Comment[]
    expect(setDataArg).toHaveLength(2)
    expect(setDataArg[0]).toEqual(mockComments[0])
  })

  // ── 17. Error recovery — onError rolls back optimistic update ─────────
  it('onError restores previous comments when addComment fails', async () => {
    const user = userEvent.setup()
    const existingComments: Comment[] = [mockComments[0]]
    setupQueryMocks({ comments: existingComments, activities: [] })

    // Seed getQueryData to return existing comments
    mockGetQueryData.mockReturnValue(existingComments)

    // Make addComment REJECT to trigger onError
    mockAddComment.mockRejectedValue(new Error('Network failure'))

    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    // Type and send a new comment
    const textarea = screen.getByPlaceholderText('Add a comment... (Cmd+Enter to send)')
    await user.type(textarea, 'Comment that will fail')

    const sendButton = document.querySelector('button svg.lucide-send')?.closest('button')
    await user.click(sendButton!)

    // Wait for the mutation to complete (with error).
    // The mutation lifecycle (onMutate → mutationFn → onError → onSettled)
    // completes within the same microtask, so by the time we get here,
    // onError has already called setQueryData for the rollback.
    await waitFor(() => {
      expect(mockAddComment).toHaveBeenCalled()
    })

    // Small pause for React to flush updates
    await act(async () => {
      await new Promise((r) => setTimeout(r, 50))
    })

    // onError should restore the previous comments (rollback).
    // The first setQueryData call is from onMutate (adding the optimistic entry),
    // and the LAST call is from onError (rolling back to the original).
    const allCalls = mockSetQueryData.mock.calls
    const lastCall = allCalls[allCalls.length - 1]
    expect(lastCall).toEqual([
      ['comments', 'story-1', 'story'],
      existingComments,
    ])
  })

  // ── 18. onSettled invalidates queries ──────────────────────────────────
  it('onSettled invalidates the comments query on success or error', async () => {
    const user = userEvent.setup()
    setupQueryMocks({ comments: [mockComments[0]], activities: [] })

    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    const textarea = screen.getByPlaceholderText('Add a comment... (Cmd+Enter to send)')
    await user.type(textarea, 'Test comment for invalidation')

    const sendButton = document.querySelector('button svg.lucide-send')?.closest('button')
    await user.click(sendButton!)

    // Wait for mutation to complete
    await waitFor(() => {
      expect(mockAddComment).toHaveBeenCalled()
    })

    // onSettled should invalidate the comments query
    await waitFor(() => {
      expect(mockInvalidateQueries).toHaveBeenCalledWith({
        queryKey: ['comments', 'story-1', 'story'],
      })
    })
  })

  // ── 19. Hide edit/delete for other users ──────────────────────────────
  it('does not show Edit/Delete buttons for comments authored by other users', () => {
    // Note: This test verifies that non-owner users cannot edit/delete comments.
    // A known limitation: the test does not verify behavior if the current user
    // somehow attempts to edit a comment while another user's comment is being
    // rendered — the UI currently only checks author_id equality and assumes
    // the backend enforces ownership on write operations.
    setupQueryMocks({ comments: [mockComments[1]] })
    renderWithProviders(<CommentThread workItemId="story-1" workItemType="story" />)

    expect(screen.queryByLabelText('Edit comment')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Delete comment')).not.toBeInTheDocument()
  })
})