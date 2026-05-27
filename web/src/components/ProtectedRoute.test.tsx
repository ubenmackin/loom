import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'

// ── Mocks ─────────────────────────────────────────────────────────────────

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(),
}))

// ── Imports (after mocks) ─────────────────────────────────────────────────

import { MemoryRouter, Routes, Route } from 'react-router-dom'
import ProtectedRoute from './ProtectedRoute'
import { useAuthStore } from '../stores/auth'

const mockUseAuthStore = vi.mocked(useAuthStore)

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

// ── Tests ─────────────────────────────────────────────────────────────────

describe('ProtectedRoute', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuthStore.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isAdmin: false,
    })
  })

  describe('when not authenticated', () => {
    it('calls navigate with "/login"', () => {
      renderProtected()
      expect(mockNavigate).toHaveBeenCalledWith('/login')
    })

    it('does not render the child route content', () => {
      renderProtected()
      expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument()
    })

    it('renders nothing (returns null)', () => {
      const { container } = renderProtected()
      // ProtectedRoute returns null, so the Routes match but nothing renders
      expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument()
      expect(screen.queryByTestId('login-page')).not.toBeInTheDocument()
      // The Outlet is never reached, so no child renders
      expect(container.textContent).toBe('')
    })
  })

  describe('when authenticated (non-admin)', () => {
    beforeEach(() => {
      mockUseAuthStore.mockReturnValue({
        user: {
          id: '1',
          username: 'testuser',
          email: 'test@example.com',
          role: 'normal',
          created_at: '2025-01-01T00:00:00Z',
        },
        isAuthenticated: true,
        isAdmin: false,
      })
    })

    it('does not call navigate', () => {
      renderProtected()
      expect(mockNavigate).not.toHaveBeenCalled()
    })

    it('renders the child route content via Outlet', () => {
      renderProtected()
      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
      expect(screen.getByText('Protected Content')).toBeInTheDocument()
    })
  })

  describe('when requireAdmin is true and user is not admin', () => {
    beforeEach(() => {
      mockUseAuthStore.mockReturnValue({
        user: {
          id: '1',
          username: 'testuser',
          email: 'test@example.com',
          role: 'normal',
          created_at: '2025-01-01T00:00:00Z',
        },
        isAuthenticated: true,
        isAdmin: false,
      })
    })

    it('calls navigate with "/"', () => {
      renderProtected(true)
      expect(mockNavigate).toHaveBeenCalledWith('/')
    })

    it('does not render the child route content', () => {
      renderProtected(true)
      expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument()
    })
  })

  describe('when authenticated and admin with requireAdmin', () => {
    beforeEach(() => {
      mockUseAuthStore.mockReturnValue({
        user: {
          id: '1',
          username: 'admin',
          email: 'admin@example.com',
          role: 'admin',
          created_at: '2025-01-01T00:00:00Z',
        },
        isAuthenticated: true,
        isAdmin: true,
      })
    })

    it('does not call navigate', () => {
      renderProtected(true)
      expect(mockNavigate).not.toHaveBeenCalled()
    })

    it('renders the child route content via Outlet', () => {
      renderProtected(true)
      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
      expect(screen.getByText('Protected Content')).toBeInTheDocument()
    })
  })

  describe('when requireAdmin is false but user is admin', () => {
    beforeEach(() => {
      mockUseAuthStore.mockReturnValue({
        user: {
          id: '1',
          username: 'admin',
          email: 'admin@example.com',
          role: 'admin',
          created_at: '2025-01-01T00:00:00Z',
        },
        isAuthenticated: true,
        isAdmin: true,
      })
    })

    it('does not call navigate', () => {
      renderProtected(false)
      expect(mockNavigate).not.toHaveBeenCalled()
    })

    it('renders the child route content', () => {
      renderProtected(false)
      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
    })
  })
})
