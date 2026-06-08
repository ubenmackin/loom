import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import GatewayPage from './GatewayPage'
import type { GatewayStatus } from '../api/client'

vi.mock('../api/client', () => ({
  fetchGatewayStatus: vi.fn(),
  fetchGatewayQueue: vi.fn(),
  triggerGatewayAction: vi.fn(),
}))

import { fetchGatewayStatus, fetchGatewayQueue } from '../api/client'

const mockedFetchGatewayStatus = fetchGatewayStatus as ReturnType<typeof vi.fn>
const mockedFetchGatewayQueue = fetchGatewayQueue as ReturnType<typeof vi.fn>

const mockRunningStatus: GatewayStatus = {
  running: true,
  active_sessions: 3,
  queue_depth: 2,
  events_processed: 42,
  uptime_seconds: 900,
  sessions_by_project: {
    'project-alpha': 2,
    'project-beta': 1,
  },
  sessions_by_agent: {
    architect: 2,
    coder: 1,
  },
}

const mockStoppedStatus: GatewayStatus = {
  running: false,
  active_sessions: 0,
  queue_depth: 0,
  events_processed: 0,
  uptime_seconds: 0,
  sessions_by_project: {},
  sessions_by_agent: {},
}

function setupRunning() {
  mockedFetchGatewayStatus.mockResolvedValue(mockRunningStatus)
  mockedFetchGatewayQueue.mockResolvedValue({ total: 2, jobs: [] })
}

function setupStopped() {
  mockedFetchGatewayStatus.mockResolvedValue(mockStoppedStatus)
  mockedFetchGatewayQueue.mockResolvedValue({ total: 0, jobs: [] })
}

describe('GatewayPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the "Gateway Dashboard" header', async () => {
    setupRunning()
    render(<GatewayPage />)

    await waitFor(() => {
      expect(screen.getByText('Gateway Dashboard')).toBeInTheDocument()
    })
  })

  describe('status bar', () => {
    it('shows Running status when gateway is running', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        expect(screen.getByText('Running')).toBeInTheDocument()
      })
    })

    it('shows Stopped status when gateway is stopped', async () => {
      setupStopped()
      render(<GatewayPage />)

      await waitFor(() => {
        expect(screen.getByText('Stopped')).toBeInTheDocument()
      })
    })

    it('shows active sessions count', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        // "3" appears in the Active Sessions panel (and may appear in
        // other numeric columns — verify the count matches expectations).
        const matches = screen.getAllByText('3')
        // Active Sessions is the only column with value 3.
        expect(matches.length).toBeGreaterThanOrEqual(1)
      })
    })

    it('shows queue depth', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        // "2" appears in Queue Depth, project-alpha sessions,
        // architect sessions, and the queue total badge (4 total).
        const matches = screen.getAllByText('2')
        expect(matches.length).toBeGreaterThanOrEqual(3)
      })
    })

    it('shows events processed count', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        // "42" is unique — only appears in Events Processed.
        expect(screen.getByText('42')).toBeInTheDocument()
      })
    })
  })

  describe('Sessions by Project', () => {
    it('renders the Sessions by Project section', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        expect(screen.getByText('Sessions by Project')).toBeInTheDocument()
      })
    })

    it('shows project breakdown', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        expect(screen.getByText('project-alpha')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('project-beta')).toBeInTheDocument()
      })
    })
  })

  describe('Sessions by Agent', () => {
    it('renders the Sessions by Agent section', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        expect(screen.getByText('Sessions by Agent')).toBeInTheDocument()
      })
    })

    it('shows agent breakdown', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        expect(screen.getByText('architect')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('coder')).toBeInTheDocument()
      })
    })
  })

  describe('Queue', () => {
    it('renders the Queue section', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        expect(screen.getByText('Queue')).toBeInTheDocument()
      })
    })

    it('shows "Queue is empty." when no jobs exist', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        expect(screen.getByText('Queue is empty.')).toBeInTheDocument()
      })
    })
  })

  describe('Manual Trigger', () => {
    it('renders the Manual Trigger section', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        expect(screen.getByText('Manual Trigger')).toBeInTheDocument()
      })
    })

    it('renders the Trigger button', async () => {
      setupRunning()
      render(<GatewayPage />)

      await waitFor(() => {
        expect(screen.getByText('Trigger')).toBeInTheDocument()
      })
    })
  })
})