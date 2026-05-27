import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// ── Mocks ─────────────────────────────────────────────────────────────────

const { mockUseQuery, mockUseMutation } = vi.hoisted(() => ({
  mockUseQuery: vi.fn(),
  mockUseMutation: vi.fn(),
}))

vi.mock('@tanstack/react-query', async () => {
  const actual = await vi.importActual('@tanstack/react-query')
  return {
    ...actual,
    useQuery: mockUseQuery,
    useMutation: mockUseMutation,
    useQueryClient: vi.fn().mockReturnValue({
      invalidateQueries: vi.fn(),
    }),
  }
})

vi.mock('../api/client', () => ({
  getUsers: vi.fn(),
  postUser: vi.fn(),
  deleteUser: vi.fn(),
}))

vi.mock('../utils/relativeTime', () => ({
  relativeTime: vi.fn().mockReturnValue('1m ago'),
}))

// ── Imports (after mocks) ─────────────────────────────────────────────────

import UsersPage from './UsersPage'
import type { User } from '../types'

// ── Fixtures ──────────────────────────────────────────────────────────────

const mockUsers: User[] = [
  {
    id: 'user-1',
    username: 'alice',
    email: 'alice@example.com',
    display_name: 'Alice Wonderland',
    role: 'admin',
    created_at: '2025-01-01T00:00:00Z',
  },
  {
    id: 'user-2',
    username: 'bob',
    email: 'bob@example.com',
    display_name: 'Bob Builder',
    role: 'normal',
    created_at: '2025-01-15T00:00:00Z',
  },
]

// ── Factory for createMockMutation ────────────────────────────────────────

function createMockMutation(overrides = {}) {
  return {
    mutate: vi.fn(),
    mutateAsync: vi.fn(),
    isPending: false,
    isSuccess: false,
    isError: false,
    data: null,
    error: null,
    reset: vi.fn(),
    ...overrides,
  }
}

// ── Tests ─────────────────────────────────────────────────────────────────

describe('UsersPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // ── 1. Loading state ──────────────────────────────────────────────────
  it('renders loading state', () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
      refetch: vi.fn(),
    })
    mockUseMutation.mockReturnValue(createMockMutation())

    render(<UsersPage />)
    expect(screen.getByText('Loading users...')).toBeInTheDocument()
  })

  // ── 2. Error state ────────────────────────────────────────────────────
  it('renders error state with Retry button', async () => {
    const refetch = vi.fn()
    mockUseQuery.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Failed to fetch'),
      refetch,
    })
    mockUseMutation.mockReturnValue(createMockMutation())

    render(<UsersPage />)

    expect(screen.getByText(/Error loading users:/)).toBeInTheDocument()
    // Error message text is split across child elements; use a matcher function
    expect(screen.getByText((content) => content.includes('Failed to fetch'))).toBeInTheDocument()

    const retryButton = screen.getByText('Retry')
    expect(retryButton).toBeInTheDocument()

    await userEvent.click(retryButton)
    expect(refetch).toHaveBeenCalled()
  })

  // ── 3. Empty state ────────────────────────────────────────────────────
  it('renders "No users found" when data is empty', () => {
    mockUseQuery.mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    })
    mockUseMutation.mockReturnValue(createMockMutation())

    render(<UsersPage />)
    expect(screen.getByText('No users found')).toBeInTheDocument()
  })

  // ── 4. Data state: renders user rows ──────────────────────────────────
  it('renders user rows with username, email, and role', () => {
    mockUseQuery.mockReturnValue({
      data: mockUsers,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    })
    mockUseMutation.mockReturnValue(createMockMutation())

    render(<UsersPage />)

    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
    expect(screen.getByText('alice@example.com')).toBeInTheDocument()
    expect(screen.getByText('bob@example.com')).toBeInTheDocument()
    expect(screen.getByText('admin')).toBeInTheDocument()
    expect(screen.getByText('normal')).toBeInTheDocument()
    expect(screen.getByText('Alice Wonderland')).toBeInTheDocument()
    expect(screen.getByText('Bob Builder')).toBeInTheDocument()
  })

  // ── 5. Create User button opens modal with form fields ────────────────
  it('opens modal with form fields when Create User is clicked', async () => {
    mockUseQuery.mockReturnValue({
      data: mockUsers,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    })
    mockUseMutation.mockReturnValue(createMockMutation())

    render(<UsersPage />)

    // Click CREATE USER button
    await userEvent.click(screen.getByText('CREATE USER'))

    // Modal should now be visible with form fields
    expect(screen.getByText('Create User')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Enter username')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Enter email')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Enter display name')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Enter password')).toBeInTheDocument()
    expect(screen.getByRole('combobox')).toBeInTheDocument()
  })

  // ── 6. Form validation shows error for missing fields ─────────────────
  it('shows validation error when submitting with missing fields', async () => {
    mockUseQuery.mockReturnValue({
      data: mockUsers,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    })
    mockUseMutation.mockReturnValue(createMockMutation())

    render(<UsersPage />)

    // Open modal
    await userEvent.click(screen.getByText('CREATE USER'))

    // Submit the form directly (userEvent.click won't bypass HTML5 validation on required fields)
    const form = document.querySelector('form')!
    fireEvent.submit(form)

    // Validation error should appear
    expect(screen.getByText('Username, email, and password are required')).toBeInTheDocument()
  })

  // ── 7. Submit creates user and closes modal ──────────────────────────
  it('creates user and closes modal on successful submit', async () => {
    const createMutate = vi.fn((_data, { onSuccess }) => {
      onSuccess()
    })

    mockUseQuery.mockReturnValue({
      data: mockUsers,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    })
    mockUseMutation.mockReturnValue(createMockMutation({ mutate: createMutate }))

    render(<UsersPage />)

    // Open modal
    await userEvent.click(screen.getByText('CREATE USER'))

    // Fill in form fields
    await userEvent.type(screen.getByPlaceholderText('Enter username'), 'charlie')
    await userEvent.type(screen.getByPlaceholderText('Enter email'), 'charlie@example.com')
    await userEvent.type(screen.getByPlaceholderText('Enter display name'), 'Charlie Brown')
    await userEvent.type(screen.getByPlaceholderText('Enter password'), 'secret123')

    // Submit
    await userEvent.click(screen.getByText('CREATE'))

    // Modal should close — "Create User" heading should be gone
    await waitFor(() => {
      expect(screen.queryByText('Create User')).not.toBeInTheDocument()
    })

    // Mutation should have been called with the right data
    expect(createMutate).toHaveBeenCalledWith(
      {
        username: 'charlie',
        email: 'charlie@example.com',
        display_name: 'Charlie Brown',
        password: 'secret123',
        role: 'normal',
      },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    )
  })

  // ── 8. Cancel closes modal without creating ──────────────────────────
  it('closes modal when Cancel is clicked', async () => {
    mockUseQuery.mockReturnValue({
      data: mockUsers,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    })
    mockUseMutation.mockReturnValue(createMockMutation())

    render(<UsersPage />)

    // Open modal
    await userEvent.click(screen.getByText('CREATE USER'))
    expect(screen.getByText('Create User')).toBeInTheDocument()

    // Click Cancel
    await userEvent.click(screen.getByText('Cancel'))

    // Modal should close
    await waitFor(() => {
      expect(screen.queryByText('Create User')).not.toBeInTheDocument()
    })
  })

  // ── 9. Delete button calls deleteMutation ────────────────────────────
  it('calls delete mutation when delete is confirmed', async () => {
    const originalConfirm = window.confirm
    window.confirm = vi.fn(() => true)

    const deleteMutate = vi.fn()
    mockUseQuery.mockReturnValue({
      data: mockUsers,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    })
    mockUseMutation.mockReturnValue(createMockMutation({ mutate: deleteMutate }))

    render(<UsersPage />)

    // Click first DELETE button
    const deleteButtons = screen.getAllByText('DELETE')
    await userEvent.click(deleteButtons[0])

    expect(window.confirm).toHaveBeenCalledWith('Are you sure you want to delete this user?')
    expect(deleteMutate).toHaveBeenCalledWith('user-1')

    window.confirm = originalConfirm
  })
})