import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// ── Mocks ─────────────────────────────────────────────────────────────────

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

const mockLogout = vi.fn()
vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(),
}))

vi.mock('../hooks/useTheme', () => ({
  useTheme: vi.fn(),
}))

// ── Imports (after mocks) ─────────────────────────────────────────────────

import { MemoryRouter } from 'react-router-dom'
import TopNav from './TopNav'
import { useAuthStore } from '../stores/auth'
import { useTheme } from '../hooks/useTheme'

const mockUseAuthStore = vi.mocked(useAuthStore)
const mockUseTheme = vi.mocked(useTheme)

// ── Fixtures ──────────────────────────────────────────────────────────────

const defaultUser = {
  id: '1',
  username: 'testuser',
  email: 'test@example.com',
  display_name: 'Test User',
  role: 'normal' as const,
  created_at: '2025-01-01T00:00:00Z',
}

const adminUser = {
  ...defaultUser,
  username: 'admin',
  display_name: 'Admin',
  role: 'admin' as const,
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
let authState: any = {
  user: null,
  isAuthenticated: false,
  logout: mockLogout,
}

// ── Helpers ───────────────────────────────────────────────────────────────

function renderTopNav() {
  return render(
    <MemoryRouter>
      <TopNav />
    </MemoryRouter>,
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────

describe('TopNav', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockLogout.mockClear()
    authState = {
      user: null,
      isAuthenticated: false,
      logout: mockLogout,
    }
    mockUseAuthStore.mockImplementation(
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      ((selector?: any) => (selector ? selector(authState) : authState)) as any,
    )
    mockUseTheme.mockReturnValue({
      isDark: false,
      toggle: vi.fn(),
      setDark: vi.fn(),
    })
  })

  // ── Brand / Logo ──────────────────────────────────────────────────────

  it('renders the Loom brand text', () => {
    renderTopNav()
    expect(screen.getByText('LOOM')).toBeInTheDocument()
  })

  it('renders the Layers icon', () => {
    renderTopNav()
    expect(document.querySelector('.lucide-layers')).toBeInTheDocument()
  })

  it('renders the JIT Kanban tagline', () => {
    renderTopNav()
    expect(screen.getByText('JIT Kanban')).toBeInTheDocument()
  })

  // ── Nav Links ─────────────────────────────────────────────────────────

  it('renders all four nav links: Board, Activity, Dispatcher, Agents', () => {
    renderTopNav()
    expect(screen.getByText('Board')).toBeInTheDocument()
    expect(screen.getByText('Activity')).toBeInTheDocument()
    expect(screen.getByText('Dispatcher')).toBeInTheDocument()
    expect(screen.getByText('Agents')).toBeInTheDocument()
  })

  // ── Unauthenticated State ─────────────────────────────────────────────

  describe('when not authenticated', () => {
    it('shows the Sign In link', () => {
      renderTopNav()
      expect(screen.getByText('Sign In')).toBeInTheDocument()
    })

    it('does not render a user dropdown', () => {
      renderTopNav()
      expect(screen.queryByLabelText('User menu')).not.toBeInTheDocument()
    })

    it('still renders the theme toggle', () => {
      renderTopNav()
      // Theme toggle is always rendered regardless of auth state
      const toggles = screen.getAllByRole('button')
      expect(toggles.length).toBeGreaterThanOrEqual(1)
    })
  })

  // ── Authenticated State ───────────────────────────────────────────────

  describe('when authenticated', () => {
    beforeEach(() => {
      authState = {
        user: defaultUser,
        isAuthenticated: true,
        logout: mockLogout,
      }
    })

    it('shows the user display_name', () => {
      renderTopNav()
      expect(screen.getByText('Test User')).toBeInTheDocument()
    })

    it('hides the Sign In link', () => {
      renderTopNav()
      expect(screen.queryByText('Sign In')).not.toBeInTheDocument()
    })

    it('renders the user menu button', () => {
      renderTopNav()
      expect(screen.getByLabelText('User menu')).toBeInTheDocument()
    })

    it('falls back to username when display_name is not set', () => {
      authState = {
        user: { ...defaultUser, display_name: undefined },
        isAuthenticated: true,
        logout: mockLogout,
      }
      renderTopNav()
      expect(screen.getByText('testuser')).toBeInTheDocument()
    })

    it('opens the dropdown when the user menu button is clicked', async () => {
      const user = userEvent.setup()
      renderTopNav()
      await user.click(screen.getByLabelText('User menu'))
      expect(screen.getByText('Profile')).toBeInTheDocument()
      expect(screen.getByText('Logout')).toBeInTheDocument()
    })

    it('closes the dropdown when clicking outside', async () => {
      const user = userEvent.setup()
      renderTopNav()
      await user.click(screen.getByLabelText('User menu'))
      expect(screen.getByText('Profile')).toBeInTheDocument()

      // Click on the brand text (outside the dropdown)
      await user.click(screen.getByText('LOOM'))
      expect(screen.queryByText('Profile')).not.toBeInTheDocument()
    })

    it('calls logout and navigates to /login on Logout click', async () => {
      const user = userEvent.setup()
      renderTopNav()
      await user.click(screen.getByLabelText('User menu'))
      await user.click(screen.getByText('Logout'))

      expect(mockLogout).toHaveBeenCalledTimes(1)
      expect(mockNavigate).toHaveBeenCalledWith('/login')
    })
  })

  // ── Admin State ───────────────────────────────────────────────────────

  describe('when admin', () => {
    beforeEach(() => {
      authState = {
        user: adminUser,
        isAuthenticated: true,
        logout: mockLogout,
      }
    })

    it('shows the Users link in the dropdown', async () => {
      const user = userEvent.setup()
      renderTopNav()
      await user.click(screen.getByLabelText('User menu'))

      expect(screen.getByText('Users')).toBeInTheDocument()
      // Users icon should be present too
      expect(document.querySelector('.lucide-users')).toBeInTheDocument()
    })

    it('shows Profile and Logout alongside Users', async () => {
      const user = userEvent.setup()
      renderTopNav()
      await user.click(screen.getByLabelText('User menu'))

      expect(screen.getByText('Profile')).toBeInTheDocument()
      expect(screen.getByText('Users')).toBeInTheDocument()
      expect(screen.getByText('Logout')).toBeInTheDocument()
    })
  })

  describe('when not admin', () => {
    it('does not show the Users link in the dropdown', async () => {
      const user = userEvent.setup()
      authState = {
        user: defaultUser,
        isAuthenticated: true,
        logout: mockLogout,
      }
      renderTopNav()
      await user.click(screen.getByLabelText('User menu'))

      expect(screen.queryByText('Users')).not.toBeInTheDocument()
      expect(screen.getByText('Profile')).toBeInTheDocument()
      expect(screen.getByText('Logout')).toBeInTheDocument()
    })
  })

  // ── Theme Toggle ──────────────────────────────────────────────────────

  describe('theme toggle', () => {
    it('shows "Switch to dark mode" aria-label in light mode', () => {
      mockUseTheme.mockReturnValue({ isDark: false, toggle: vi.fn(), setDark: vi.fn() })
      renderTopNav()
      expect(screen.getByLabelText('Switch to dark mode')).toBeInTheDocument()
    })

    it('shows "Switch to light mode" aria-label in dark mode', () => {
      mockUseTheme.mockReturnValue({ isDark: true, toggle: vi.fn(), setDark: vi.fn() })
      renderTopNav()
      expect(screen.getByLabelText('Switch to light mode')).toBeInTheDocument()
    })
  })
})
