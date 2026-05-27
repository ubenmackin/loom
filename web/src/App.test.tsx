import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Outlet } from 'react-router-dom'

// ── Mocks ─────────────────────────────────────────────────────────────────

vi.mock('./components/Layout', () => ({
  default: () => (
    <div data-testid="layout">
      <Outlet />
    </div>
  ),
}))

vi.mock('./components/ProtectedRoute', () => ({
  default: ({ requireAdmin }: { requireAdmin?: boolean }) =>
    requireAdmin ? (
      <div data-testid="protected-route-admin">
        <Outlet />
      </div>
    ) : (
      <div data-testid="protected-route">
        <Outlet />
      </div>
    ),
}))

vi.mock('./components/Board', () => ({
  default: () => <div data-testid="board-page">Board</div>,
}))

vi.mock('./pages/LoginPage', () => ({
  default: () => <div data-testid="login-page">Login</div>,
}))

vi.mock('./pages/OnboardingPage', () => ({
  default: () => <div data-testid="onboarding-page">Onboarding</div>,
}))

vi.mock('./pages/ActivityPage', () => ({
  default: () => <div data-testid="activity-page">Activity</div>,
}))

vi.mock('./pages/AgentsPage', () => ({
  default: () => <div data-testid="agents-page">Agents</div>,
}))

vi.mock('./pages/DispatcherPage', () => ({
  default: () => <div data-testid="dispatcher-page">Dispatcher</div>,
}))

vi.mock('./pages/UsersPage', () => ({
  default: () => <div data-testid="users-page">Users</div>,
}))

vi.mock('./hooks/useWebSocket', () => ({
  useWebSocket: vi.fn(),
}))

// ── Imports (after mocks) ─────────────────────────────────────────────────

import App from './App'

// ── Tests ─────────────────────────────────────────────────────────────────

describe('App', () => {
  it('/login renders LoginPage', () => {
    render(
      <MemoryRouter initialEntries={['/login']}>
        <App />
      </MemoryRouter>,
    )
    expect(screen.getByTestId('login-page')).toBeInTheDocument()
    expect(screen.getByText('Login')).toBeInTheDocument()
  })

  it('/onboarding renders OnboardingPage', () => {
    render(
      <MemoryRouter initialEntries={['/onboarding']}>
        <App />
      </MemoryRouter>,
    )
    expect(screen.getByTestId('onboarding-page')).toBeInTheDocument()
    expect(screen.getByText('Onboarding')).toBeInTheDocument()
  })

  it('/ renders Board (inside Layout and ProtectedRoute)', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )
    expect(screen.getByTestId('protected-route')).toBeInTheDocument()
    expect(screen.getByTestId('layout')).toBeInTheDocument()
    expect(screen.getByTestId('board-page')).toBeInTheDocument()
    expect(screen.getByText('Board')).toBeInTheDocument()
  })

  it('/activity renders ActivityPage', () => {
    render(
      <MemoryRouter initialEntries={['/activity']}>
        <App />
      </MemoryRouter>,
    )
    expect(screen.getByTestId('protected-route')).toBeInTheDocument()
    expect(screen.getByTestId('layout')).toBeInTheDocument()
    expect(screen.getByTestId('activity-page')).toBeInTheDocument()
    expect(screen.getByText('Activity')).toBeInTheDocument()
  })

  it('/agents renders AgentsPage', () => {
    render(
      <MemoryRouter initialEntries={['/agents']}>
        <App />
      </MemoryRouter>,
    )
    expect(screen.getByTestId('protected-route')).toBeInTheDocument()
    expect(screen.getByTestId('layout')).toBeInTheDocument()
    expect(screen.getByTestId('agents-page')).toBeInTheDocument()
    expect(screen.getByText('Agents')).toBeInTheDocument()
  })

  it('/dispatcher renders DispatcherPage', () => {
    render(
      <MemoryRouter initialEntries={['/dispatcher']}>
        <App />
      </MemoryRouter>,
    )
    expect(screen.getByTestId('protected-route')).toBeInTheDocument()
    expect(screen.getByTestId('layout')).toBeInTheDocument()
    expect(screen.getByTestId('dispatcher-page')).toBeInTheDocument()
    expect(screen.getByText('Dispatcher')).toBeInTheDocument()
  })

  it('/users renders UsersPage (inside admin-protected route)', () => {
    render(
      <MemoryRouter initialEntries={['/users']}>
        <App />
      </MemoryRouter>,
    )
    // The outer ProtectedRoute renders, plus the admin one
    expect(screen.getByTestId('protected-route')).toBeInTheDocument()
    expect(screen.getByTestId('protected-route-admin')).toBeInTheDocument()
    expect(screen.getByTestId('layout')).toBeInTheDocument()
    expect(screen.getByTestId('users-page')).toBeInTheDocument()
    expect(screen.getByText('Users')).toBeInTheDocument()
  })
})