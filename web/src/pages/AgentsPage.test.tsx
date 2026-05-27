import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AgentsPage from './AgentsPage'
import type { Session } from '../types'
import { SessionStatus } from '../types'

vi.mock('../hooks/useSessions', () => ({
  useSessions: vi.fn(),
}))

vi.mock('../utils/relativeTime', () => ({
  relativeTime: vi.fn().mockReturnValue('2m ago'),
}))

import { useSessions } from '../hooks/useSessions'

const mockedUseSessions = useSessions as ReturnType<typeof vi.fn>

const activeSession: Session = {
  id: 'session-active-123456',
  harness_type: 'openai',
  capabilities: JSON.stringify(['code', 'review']),
  last_seen_at: '2025-05-26T10:00:00Z',
  status: SessionStatus.Active,
  created_at: '2025-05-25T10:00:00Z',
}

const staleSession: Session = {
  id: 'session-stale-789012',
  harness_type: 'anthropic',
  capabilities: JSON.stringify(['code']),
  last_seen_at: '2025-05-26T08:00:00Z',
  status: SessionStatus.Stale,
  created_at: '2025-05-24T10:00:00Z',
}

const disconnectedSession: Session = {
  id: 'session-disc-345678',
  harness_type: 'custom',
  capabilities: undefined,
  last_seen_at: '2025-05-25T10:00:00Z',
  status: SessionStatus.Disconnected,
  created_at: '2025-05-23T10:00:00Z',
}

describe('AgentsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('loading state', () => {
    it('renders "Loading agents..." when isLoading is true', () => {
      mockedUseSessions.mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
        refetch: vi.fn(),
      })

      render(<AgentsPage />)

      expect(screen.getByText('Loading agents...')).toBeInTheDocument()
    })
  })

  describe('error state', () => {
    it('renders error message with Retry button', async () => {
      const refetch = vi.fn()
      mockedUseSessions.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Server error'),
        refetch,
      })

      render(<AgentsPage />)

      expect(screen.getByText('Error loading agents: Server error')).toBeInTheDocument()

      const retryButton = screen.getByText('Retry')
      expect(retryButton).toBeInTheDocument()

      const user = userEvent.setup()
      await user.click(retryButton)
      expect(refetch).toHaveBeenCalledOnce()
    })
  })

  describe('empty state', () => {
    it('renders "No agents connected" when sessions array is empty', () => {
      mockedUseSessions.mockReturnValue({
        data: [],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<AgentsPage />)

      expect(screen.getByText('No agents connected')).toBeInTheDocument()
    })
  })

  describe('data state', () => {
    it('renders sessions grouped by status', () => {
      mockedUseSessions.mockReturnValue({
        data: [activeSession, staleSession, disconnectedSession],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<AgentsPage />)

      // Status group headers
      expect(screen.getByText('active')).toBeInTheDocument()
      expect(screen.getByText('stale')).toBeInTheDocument()
      expect(screen.getByText('disconnected')).toBeInTheDocument()

      // Count badges in header
      expect(screen.getByText('active: [1]')).toBeInTheDocument()
      expect(screen.getByText('stale: [1]')).toBeInTheDocument()
      expect(screen.getByText('disconnected: [1]')).toBeInTheDocument()
    })

    it('renders session card with truncated ID and harness_type', () => {
      mockedUseSessions.mockReturnValue({
        data: [activeSession],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<AgentsPage />)

      // ID truncated to 12 chars; the ellipsis is a separate text node
      const idSpan = screen.getByTitle('session-active-123456')
      expect(idSpan).toBeInTheDocument()
      expect(idSpan.textContent).toContain('session-acti')

      // Harness type badge (rendered as a span with sharp-tag classes, not SharpTag component)
      expect(screen.getByText('openai')).toBeInTheDocument()
    })

    it('renders capabilities as SharpTags', () => {
      mockedUseSessions.mockReturnValue({
        data: [activeSession],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<AgentsPage />)

      // Capabilities are rendered as SharpTag with variant="amber" = [CODE], [REVIEW]
      expect(screen.getByText('[code]')).toBeInTheDocument()
      expect(screen.getByText('[review]')).toBeInTheDocument()
    })

    it('does not render capabilities section when capabilities is undefined', () => {
      mockedUseSessions.mockReturnValue({
        data: [disconnectedSession],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<AgentsPage />)

      // No SharpTag for capabilities
      expect(screen.queryByText('[code]')).not.toBeInTheDocument()
      expect(screen.queryByText('[review]')).not.toBeInTheDocument()
    })

    it('renders timestamps using relativeTime', () => {
      mockedUseSessions.mockReturnValue({
        data: [activeSession],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<AgentsPage />)

      // relativeTime mock returns '2m ago' for all
      const timeLabels = screen.getAllByText(/2m ago/)
      expect(timeLabels.length).toBeGreaterThanOrEqual(2)
    })
  })

  describe('parseCapabilities', () => {
    // parseCapabilities is a module-private function; we test it indirectly
    // through the component rendering, but we can also test the logic here
    // by importing it via extracting the test cases
    it('parses valid JSON array string into array', () => {
      mockedUseSessions.mockReturnValue({
        data: [{
          ...activeSession,
          capabilities: JSON.stringify(['code', 'build', 'review']),
        }],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<AgentsPage />)

      expect(screen.getByText('[code]')).toBeInTheDocument()
      expect(screen.getByText('[build]')).toBeInTheDocument()
      expect(screen.getByText('[review]')).toBeInTheDocument()
    })

    it('returns empty array for invalid JSON string', () => {
      mockedUseSessions.mockReturnValue({
        data: [{
          ...activeSession,
          capabilities: 'not-valid-json',
        }],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<AgentsPage />)

      // No capability tags should be rendered
      expect(screen.queryByText('[code]')).not.toBeInTheDocument()
      expect(screen.queryByText('[build]')).not.toBeInTheDocument()
      expect(screen.queryByText('[review]')).not.toBeInTheDocument()
    })

    it('returns empty array for empty string', () => {
      mockedUseSessions.mockReturnValue({
        data: [{
          ...activeSession,
          capabilities: '',
        }],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<AgentsPage />)

      // No capability tags should be rendered
      expect(screen.queryByText('[code]')).not.toBeInTheDocument()
      expect(screen.queryByText('[build]')).not.toBeInTheDocument()
      expect(screen.queryByText('[review]')).not.toBeInTheDocument()
    })
  })
})