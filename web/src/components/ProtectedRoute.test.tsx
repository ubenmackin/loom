import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'

// ── Mocks ─────────────────────────────────────────────────────────────────

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return actual
})

vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(),
}))

// ── Imports (after mocks) ─────────────────────────────────────────────────

import { MemoryRouter, Routes, Route } from 'react-router-dom'
import ProtectedRoute from './ProtectedRoute'
import { useAuthStore } from '../stores/auth'
import type { User } from '../types'

const mockUseAuthStore = vi.mocked(useAuthStore)

// ── Types ───────────────────────────────────────────────────────────────────

type AuthState = {
  user: User | null
  token: string | null
  isAuthenticated: boolean
}

// ── Fixtures ──────────────────────────────────────────────────────────────

function MockChild() {
  return <div data-testid="protected-content">Protected Content</div>
}

function LoginPage() {
  return <div data-testid="login-page">Login Page</div>
}

function HomePage() {
  return <div data-testid="home-page">Home Page</div>
}

// ── Helpers ───────────────────────────────────────────────────────────────

function renderProtected(requireAdmin = false) {
  return render(
    <MemoryRouter initialEntries={['/protected']}>
      <Routes>
        <Route element={<ProtectedRoute requireAdmin={requireAdmin} />}>
          <Route path="/protected" element={<MockChild />} />
        </Route>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/" element={<HomePage />} />
      </Routes>
    </MemoryRouter>,
  )
}

let authState: AuthState = {
  user: null,
  token: null,
  isAuthenticated: false,
}

// ── Tests ─────────────────────────────────────────────────────────────────

describe('ProtectedRoute', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authState = { user: null, token: null, isAuthenticated: false }
    type MockSelectorFn = (selector?: (state: AuthState) => unknown) => unknown

    ;(mockUseAuthStore as unknown as { mockImplementation: (fn: MockSelectorFn) => void }).mockImplementation((selector?) => {
      if (selector) return selector(authState)
      return authState
    })
  })

  describe('when not authenticated', () => {
    it('navigates to login page', () => {
      renderProtected()
      expect(screen.getByTestId('login-page')).toBeInTheDocument()
    })

    it('does not render the child route content', () => {
      renderProtected()
      expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument()
    })

    it('does not render the home page', () => {
      renderProtected()
      expect(screen.getByTestId('login-page')).toBeInTheDocument()
      expect(screen.queryByTestId('home-page')).not.toBeInTheDocument()
    })
  })

  describe('when authenticated (non-admin)', () => {
    beforeEach(() => {
      authState = {
        user: {
          id: '1',
          username: 'testuser',
          email: 'test@example.com',
          role: 'normal',
          created_at: '2025-01-01T00:00:00Z',
        },
        token: 'token',
        isAuthenticated: true,
      }
    })

    it('does not navigate (stays on protected route)', () => {
      renderProtected()
      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
    })

    it('renders the child route content via Outlet', () => {
      renderProtected()
      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
      expect(screen.getByText('Protected Content')).toBeInTheDocument()
    })
  })

  describe('when requireAdmin is true and user is not admin', () => {
    beforeEach(() => {
      authState = {
        user: {
          id: '1',
          username: 'testuser',
          email: 'test@example.com',
          role: 'normal',
          created_at: '2025-01-01T00:00:00Z',
        },
        token: 'token',
        isAuthenticated: true,
      }
    })

    it('navigates to home page', () => {
      renderProtected(true)
      expect(screen.getByTestId('home-page')).toBeInTheDocument()
    })

    it('does not render the child route content', () => {
      renderProtected(true)
      expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument()
    })
  })

  describe('when authenticated and admin with requireAdmin', () => {
    beforeEach(() => {
      authState = {
        user: {
          id: '1',
          username: 'admin',
          email: 'admin@example.com',
          role: 'admin',
          created_at: '2025-01-01T00:00:00Z',
        },
        token: 'token',
        isAuthenticated: true,
      }
    })

    it('does not navigate (stays on protected route)', () => {
      renderProtected(true)
      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
    })

    it('renders the child route content via Outlet', () => {
      renderProtected(true)
      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
      expect(screen.getByText('Protected Content')).toBeInTheDocument()
    })
  })

  describe('when requireAdmin is false but user is admin', () => {
    beforeEach(() => {
      authState = {
        user: {
          id: '1',
          username: 'admin',
          email: 'admin@example.com',
          role: 'admin',
          created_at: '2025-01-01T00:00:00Z',
        },
        token: 'token',
        isAuthenticated: true,
      }
    })

    it('does not navigate (stays on protected route)', () => {
      renderProtected(false)
      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
    })

    it('renders the child route content', () => {
      renderProtected(false)
      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
    })
  })
})
