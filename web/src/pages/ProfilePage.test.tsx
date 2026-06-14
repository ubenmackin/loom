import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// ── Mocks ─────────────────────────────────────────────────────────────────

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../api/client', () => ({
  updateMyProfile: vi.fn(),
}))

const mockUpdateUser = vi.fn()

vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(),
}))

// ── Imports (after mocks) ─────────────────────────────────────────────────

import { MemoryRouter } from 'react-router-dom'
import ProfilePage from './ProfilePage'
import { useAuthStore } from '../stores/auth'
import { updateMyProfile } from '../api/client'
import type { User } from '../types'

const mockUseAuthStore = vi.mocked(useAuthStore)
const mockUpdateMyProfile = vi.mocked(updateMyProfile)

type AuthState = {
  user: User | null
  token: string | null
  isAuthenticated: boolean
  login: ReturnType<typeof vi.fn>
  logout: ReturnType<typeof vi.fn>
  updateUser: typeof mockUpdateUser
}

type StoreState = ReturnType<typeof useAuthStore.getState>

// ── Fixtures ──────────────────────────────────────────────────────────────

const defaultUser: User = {
  id: '1',
  username: 'testuser',
  email: 'test@example.com',
  display_name: 'Test User',
  role: 'normal',
  created_at: '2025-01-01T00:00:00Z',
}

const defaultAuthState: AuthState = {
  user: defaultUser,
  token: 'test-token',
  isAuthenticated: true,
  login: vi.fn(),
  logout: vi.fn(),
  updateUser: mockUpdateUser,
}

// ── Helpers ───────────────────────────────────────────────────────────────

function renderProfilePage() {
  return render(
    <MemoryRouter>
      <ProfilePage />
    </MemoryRouter>,
  )
}

function setupAuthState(state: AuthState = defaultAuthState) {
  mockUseAuthStore.mockImplementation((selector?: (s: StoreState) => unknown) => {
    if (selector) return selector(state as unknown as StoreState)
    return state
  })
}

// ── Tests ─────────────────────────────────────────────────────────────────

describe('ProfilePage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setupAuthState()
  })

  it('pre-populates fields from current user', () => {
    renderProfilePage()

    expect(screen.getByDisplayValue('testuser')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Test User')).toBeInTheDocument()
    expect(screen.getByDisplayValue('test@example.com')).toBeInTheDocument()
  })

  it('shows "no changes" error when saving without modifications', async () => {
    renderProfilePage()
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: /save changes/i }))

    expect(screen.getByText('No changes to save')).toBeInTheDocument()
  })

  it('shows validation error when password fields are empty and toggle is active', async () => {
    renderProfilePage()
    const user = userEvent.setup()

    await user.click(screen.getByText('Change Password'))
    await user.click(screen.getByText('Save Changes'))

    expect(
      screen.getByText('Both current and new password are required when changing password'),
    ).toBeInTheDocument()
  })

  it('shows validation error when new password is too short', async () => {
    renderProfilePage()
    const user = userEvent.setup()

    await user.click(screen.getByText('Change Password'))
    await user.type(screen.getByPlaceholderText('Enter current password'), 'current-pass')
    await user.type(screen.getByPlaceholderText('Enter new password (min 6 characters)'), 'ab')
    await user.click(screen.getByText('Save Changes'))

    expect(screen.getByText('New password must be at least 6 characters')).toBeInTheDocument()
  })

  it('calls updateMyProfile with display_name and updates store on save', async () => {
    const updatedUser: User = { ...defaultUser, display_name: 'Updated Name' }
    mockUpdateMyProfile.mockResolvedValue(updatedUser)
    renderProfilePage()
    const user = userEvent.setup()

    const nameInput = screen.getByDisplayValue('Test User')
    await user.clear(nameInput)
    await user.type(nameInput, 'Updated Name')

    await user.click(screen.getByText('Save Changes'))

    expect(mockUpdateMyProfile).toHaveBeenCalledWith({ display_name: 'Updated Name' })
    expect(mockUpdateUser).toHaveBeenCalledWith(updatedUser)
    expect(screen.getByText('Profile updated successfully')).toBeInTheDocument()
  })

  it('calls updateMyProfile with email and updates store on save', async () => {
    const updatedUser: User = { ...defaultUser, email: 'new@example.com' }
    mockUpdateMyProfile.mockResolvedValue(updatedUser)
    renderProfilePage()
    const user = userEvent.setup()

    const emailInput = screen.getByDisplayValue('test@example.com')
    await user.clear(emailInput)
    await user.type(emailInput, 'new@example.com')

    await user.click(screen.getByText('Save Changes'))

    expect(mockUpdateMyProfile).toHaveBeenCalledWith({ email: 'new@example.com' })
    expect(mockUpdateUser).toHaveBeenCalledWith(updatedUser)
  })

  it('calls updateMyProfile with password fields when toggle is active', async () => {
    const updatedUser: User = { ...defaultUser }
    mockUpdateMyProfile.mockResolvedValue(updatedUser)
    renderProfilePage()
    const user = userEvent.setup()

    await user.click(screen.getByText('Change Password'))
    await user.type(screen.getByPlaceholderText('Enter current password'), 'old-password')
    await user.type(screen.getByPlaceholderText('Enter new password (min 6 characters)'), 'new-password-123')

    await user.click(screen.getByText('Save Changes'))

    expect(mockUpdateMyProfile).toHaveBeenCalledWith({
      current_password: 'old-password',
      new_password: 'new-password-123',
    })
    expect(mockUpdateUser).toHaveBeenCalledWith(updatedUser)
  })

  it('displays error message when API call fails', async () => {
    const errorMessage = 'current password is incorrect'
    mockUpdateMyProfile.mockRejectedValue(new Error(errorMessage))
    renderProfilePage()
    const user = userEvent.setup()

    const nameInput = screen.getByDisplayValue('Test User')
    await user.clear(nameInput)
    await user.type(nameInput, 'New Name')

    await user.click(screen.getByText('Save Changes'))

    expect(await screen.findByText(errorMessage)).toBeInTheDocument()
  })

  it('redirects to login when no user is available', () => {
    setupAuthState({ ...defaultAuthState, user: null, isAuthenticated: false })

    renderProfilePage()

    // The <Navigate to="/login" /> replaces the page content — profile fields should not appear
    expect(screen.queryByDisplayValue('testuser')).not.toBeInTheDocument()
    expect(screen.queryByText('Profile')).not.toBeInTheDocument()
  })
})
