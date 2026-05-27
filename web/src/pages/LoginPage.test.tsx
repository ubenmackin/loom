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
  login: vi.fn(),
}))

const mockLogin = vi.fn()

type AuthState = {
  isAuthenticated: boolean
  login: typeof mockLogin
  user: null
  token: null
  isAdmin: boolean
  logout: ReturnType<typeof vi.fn>
  updateUser: ReturnType<typeof vi.fn>
}

const defaultAuthState: AuthState = {
  isAuthenticated: false,
  login: mockLogin,
  user: null,
  token: null,
  isAdmin: false,
  logout: vi.fn(),
  updateUser: vi.fn(),
}

vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(),
}))

// ── Imports (after mocks) ─────────────────────────────────────────────────

import { MemoryRouter } from 'react-router-dom'
import LoginPage from './LoginPage'
import { useAuthStore } from '../stores/auth'
import { login as apiLogin } from '../api/client'
import type { AuthResponse } from '../types'

const mockUseAuthStore = vi.mocked(useAuthStore)
const mockApiLogin = vi.mocked(apiLogin)

// ── Fixtures ──────────────────────────────────────────────────────────────

const authResponse: AuthResponse = {
  user: {
    id: '1',
    username: 'testuser',
    email: 'test@example.com',
    display_name: 'Test User',
    role: 'normal',
    created_at: '2025-01-01T00:00:00Z',
  },
  token: 'test-token-abc',
}

// ── Helpers ───────────────────────────────────────────────────────────────

function renderLoginPage() {
  return render(
    <MemoryRouter>
      <LoginPage />
    </MemoryRouter>,
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuthStore.mockImplementation((selector?: (state: AuthState) => unknown) => {
      if (selector) return selector(defaultAuthState)
      return defaultAuthState
    })
  })

  it('redirects to "/" when already authenticated', () => {
    mockUseAuthStore.mockImplementation((selector?: (state: AuthState) => unknown) => {
      const authedState = { ...defaultAuthState, isAuthenticated: true }
      if (selector) return selector(authedState)
      return authedState
    })

    renderLoginPage()
    expect(mockNavigate).toHaveBeenCalledWith('/')
  })

  it('renders the login form with username, password fields and SIGN IN button', () => {
    renderLoginPage()

    expect(screen.getByLabelText('Username')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('shows validation error when submitting with empty fields', async () => {
    renderLoginPage()
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: /sign in/i }))

    expect(screen.getByText('Please provide both username and password')).toBeInTheDocument()
  })

  it('calls store.login and navigates to "/" on successful login', async () => {
    mockApiLogin.mockResolvedValue(authResponse)
    renderLoginPage()
    const user = userEvent.setup()

    await user.type(screen.getByLabelText('Username'), 'testuser')
    await user.type(screen.getByLabelText('Password'), 'password123')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    expect(mockApiLogin).toHaveBeenCalledWith({
      username_or_email: 'testuser',
      password: 'password123',
    })
    expect(mockLogin).toHaveBeenCalledWith(authResponse)
    expect(mockNavigate).toHaveBeenCalledWith('/')
  })

  it('displays error message from API when login fails', async () => {
    const errorMessage = 'Invalid credentials'
    mockApiLogin.mockRejectedValue(new Error(errorMessage))
    renderLoginPage()
    const user = userEvent.setup()

    await user.type(screen.getByLabelText('Username'), 'testuser')
    await user.type(screen.getByLabelText('Password'), 'wrongpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    expect(await screen.findByText(errorMessage)).toBeInTheDocument()
  })

  it('renders a "Get Started" link that navigates to /onboarding', () => {
    renderLoginPage()

    const link = screen.getByRole('link', { name: /get started/i })
    expect(link).toBeInTheDocument()
    expect(link).toHaveAttribute('href', '/onboarding')
  })
})