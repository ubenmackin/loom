import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

// ── Mocks ───────────────────────────────────────────────────────────────────

vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(),
}))

vi.mock('../stores/project', () => ({
  useProjectFilterStore: vi.fn(() => ({
    selectedProjectId: null,
    setSelectedProjectId: vi.fn(),
    clearProjectFilter: vi.fn(),
  })),
}))

vi.mock('./ProjectPicker', () => ({
  default: () => <div data-testid="project-picker">ProjectPicker</div>,
}))

// ── Imports (after mocks) ──────────────────────────────────────────────────

import SubNav from './SubNav'
import { useAuthStore } from '../stores/auth'
import type { User } from '../types'

const mockUseAuthStore = vi.mocked(useAuthStore)

// ── Types ───────────────────────────────────────────────────────────────────

type AuthState = {
  user: User | null
  token: string | null
  isAuthenticated: boolean
}

// ── Fixtures ────────────────────────────────────────────────────────────────

const normalUser: User = {
  id: '1',
  username: 'test',
  role: 'normal',
  email: 'a@b.com',
  created_at: '',
}

// ── Helpers ─────────────────────────────────────────────────────────────────

function setAuthState(user: User | null, isAuthenticated: boolean) {
  const state: AuthState = {
    user,
    token: user ? 'token' : null,
    isAuthenticated,
  }
  type MockSelectorFn = (selector?: (state: AuthState) => unknown) => unknown

  ;(mockUseAuthStore as unknown as { mockImplementation: (fn: MockSelectorFn) => void }).mockImplementation((selector?) => {
    if (selector) return selector(state)
    return state
  })
}

// ── Tests ──────────────────────────────────────────────────────────────────

describe('SubNav', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setAuthState(null, false)
  })

  it('returns null when not authenticated', () => {
    const { container } = render(
      <MemoryRouter>
        <SubNav />
      </MemoryRouter>,
    )
    expect(container.firstChild).toBeNull()
  })

  it('renders ProjectPicker when authenticated', () => {
    setAuthState(normalUser, true)

    render(
      <MemoryRouter>
        <SubNav />
      </MemoryRouter>,
    )
    expect(screen.getByTestId('project-picker')).toBeInTheDocument()
  })

  it('shows breadcrumb for board page (/)', () => {
    setAuthState(normalUser, true)

    render(
      <MemoryRouter initialEntries={['/']}>
        <SubNav />
      </MemoryRouter>,
    )
    expect(screen.getByText('Board')).toBeInTheDocument()
  })

  it('shows correct breadcrumb for /activity', () => {
    setAuthState(normalUser, true)

    render(
      <MemoryRouter initialEntries={['/activity']}>
        <SubNav />
      </MemoryRouter>,
    )
    expect(screen.getByText('Activity')).toBeInTheDocument()
  })
})