import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// ── Mocks (all hoisted) ───────────────────────────────────────────────────

const { mockNavigate, mockLogin, mockGetOnboardingCheck, mockSignup } = vi.hoisted(() => ({
  mockNavigate: vi.fn(),
  mockLogin: vi.fn(),
  mockGetOnboardingCheck: vi.fn(),
  mockSignup: vi.fn(),
}))

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn((selector?: (state: { login: typeof mockLogin }) => unknown) => {
    const state = { login: mockLogin }
    if (selector) return selector(state)
    return state
  }),
}))

vi.mock('../api/client', () => ({
  getOnboardingCheck: mockGetOnboardingCheck,
  signup: mockSignup,
}))

// ── Imports (after mocks) ─────────────────────────────────────────────────

import { MemoryRouter } from 'react-router-dom'
import OnboardingPage from './OnboardingPage'

// ── Helpers ───────────────────────────────────────────────────────────────

function renderOnboarding() {
  return render(
    <MemoryRouter>
      <OnboardingPage />
    </MemoryRouter>,
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────

describe('OnboardingPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetOnboardingCheck.mockResolvedValue({ onboarding_required: true })
  })

  // ── 1. Redirect to /login when onboarding not required ─────────────────
  it('redirects to /login when onboarding is not required', async () => {
    mockGetOnboardingCheck.mockResolvedValue({ onboarding_required: false })

    renderOnboarding()

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login')
    })
  })

  // ── 2. Renders signup form with all fields ────────────────────────────
  it('renders signup form with all fields', async () => {
    renderOnboarding()

    // Wait for the component to settle (onboarding check resolves)
    await waitFor(() => {
      expect(screen.getByText('Create Account')).toBeInTheDocument()
    })

    expect(screen.getByLabelText('Username')).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toBeInTheDocument()
    expect(screen.getByLabelText('Display Name')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByLabelText('Confirm Password')).toBeInTheDocument()
    expect(screen.getByText('Create Account')).toBeInTheDocument()
  })

  // ── 3. "Passwords do not match" validation ────────────────────────────
  it('shows error when passwords do not match', async () => {
    renderOnboarding()

    await waitFor(() => {
      expect(screen.getByText('Create Account')).toBeInTheDocument()
    })

    await userEvent.type(screen.getByLabelText('Username'), 'testuser')
    await userEvent.type(screen.getByLabelText('Email'), 'test@example.com')
    await userEvent.type(screen.getByLabelText('Display Name'), 'Test User')
    await userEvent.type(screen.getByLabelText('Password'), 'password123')
    await userEvent.type(screen.getByLabelText('Confirm Password'), 'differentPassword')

    await userEvent.click(screen.getByText('Create Account'))

    await waitFor(() => {
      expect(screen.getByText('Passwords do not match')).toBeInTheDocument()
    })
  })

  // ── 4. "All fields are required" validation ──────────────────────────
  it('shows error when all fields are not filled', async () => {
    renderOnboarding()

    await waitFor(() => {
      expect(screen.getByText('Create Account')).toBeInTheDocument()
    })

    // Fill only password and confirm password (matching), skip others
    await userEvent.type(screen.getByLabelText('Password'), 'password123')
    await userEvent.type(screen.getByLabelText('Confirm Password'), 'password123')

    // Use fireEvent.submit to bypass HTML5 required validation
    const form = document.querySelector('form')!
    fireEvent.submit(form)

    await waitFor(() => {
      expect(screen.getByText('All fields are required')).toBeInTheDocument()
    })
  })

  // ── 5. Successful signup calls store.login and navigates to '/' ──────
  it('calls login and navigates to / on successful signup', async () => {
    const signupResult = {
      user: {
        id: 'user-new',
        username: 'testuser',
        email: 'test@example.com',
        display_name: 'Test User',
        role: 'admin' as const,
        created_at: '2025-05-26T00:00:00Z',
      },
      token: 'mock-token-123',
    }
    mockSignup.mockResolvedValueOnce(signupResult)

    renderOnboarding()

    await waitFor(() => {
      expect(screen.getByText('Create Account')).toBeInTheDocument()
    })

    await userEvent.type(screen.getByLabelText('Username'), 'testuser')
    await userEvent.type(screen.getByLabelText('Email'), 'test@example.com')
    await userEvent.type(screen.getByLabelText('Display Name'), 'Test User')
    await userEvent.type(screen.getByLabelText('Password'), 'password123')
    await userEvent.type(screen.getByLabelText('Confirm Password'), 'password123')

    await userEvent.click(screen.getByText('Create Account'))

    await waitFor(() => {
      expect(mockSignup).toHaveBeenCalledWith({
        username: 'testuser',
        email: 'test@example.com',
        password: 'password123',
        display_name: 'Test User',
      })
      expect(mockLogin).toHaveBeenCalledWith(signupResult)
      expect(mockNavigate).toHaveBeenCalledWith('/')
    })
  })

  // ── 6. Failed signup shows error message ──────────────────────────────
  it('shows error message when signup fails', async () => {
    mockSignup.mockRejectedValueOnce(new Error('Username already taken'))

    renderOnboarding()

    await waitFor(() => {
      expect(screen.getByText('Create Account')).toBeInTheDocument()
    })

    await userEvent.type(screen.getByLabelText('Username'), 'existinguser')
    await userEvent.type(screen.getByLabelText('Email'), 'existing@example.com')
    await userEvent.type(screen.getByLabelText('Display Name'), 'Existing User')
    await userEvent.type(screen.getByLabelText('Password'), 'password123')
    await userEvent.type(screen.getByLabelText('Confirm Password'), 'password123')

    await userEvent.click(screen.getByText('Create Account'))

    await waitFor(() => {
      expect(screen.getByText('Username already taken')).toBeInTheDocument()
    })
  })

  // ── 7. Shows "Creating..." during loading ────────────────────────────
  it('shows Creating... button text while loading', async () => {
    // Make signup never resolve so loading stays true
    mockSignup.mockReturnValueOnce(new Promise(() => {}))

    renderOnboarding()

    await waitFor(() => {
      expect(screen.getByText('Create Account')).toBeInTheDocument()
    })

    await userEvent.type(screen.getByLabelText('Username'), 'testuser')
    await userEvent.type(screen.getByLabelText('Email'), 'test@example.com')
    await userEvent.type(screen.getByLabelText('Display Name'), 'Test User')
    await userEvent.type(screen.getByLabelText('Password'), 'password123')
    await userEvent.type(screen.getByLabelText('Confirm Password'), 'password123')

    await userEvent.click(screen.getByText('Create Account'))

    await waitFor(() => {
      expect(screen.getByText('Creating...')).toBeInTheDocument()
    })
  })
})