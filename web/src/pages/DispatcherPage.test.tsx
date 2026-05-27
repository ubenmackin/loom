import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import DispatcherPage from './DispatcherPage'
import type { DispatcherStatus } from '../types'

vi.mock('../hooks/useWebSocket', () => ({
  useWebSocket: vi.fn(),
}))

vi.mock('../hooks/useDispatcher', () => ({
  useDispatcher: vi.fn(),
}))

import { useWebSocket } from '../hooks/useWebSocket'
import { useDispatcher } from '../hooks/useDispatcher'

const mockedUseWebSocket = useWebSocket as ReturnType<typeof vi.fn>
const mockedUseDispatcher = useDispatcher as ReturnType<typeof vi.fn>

const mockRunningStatus: DispatcherStatus = {
  running: true,
  uptime_seconds: 3661,
  event_queue_depth: 5,
  events_processed: {
    assignment_pass_started: 10,
    assignment_pass_finished: 8,
    gate_check: 3,
    staleness_check: 1,
  },
  started_at: '2025-05-26T09:00:00Z',
}

const mockStoppedStatus: DispatcherStatus = {
  running: false,
  uptime_seconds: 0,
  event_queue_depth: 0,
  events_processed: {},
  started_at: '2025-05-26T09:00:00Z',
}

function setupRunning(isConnected = true) {
  mockedUseWebSocket.mockReturnValue({
    lastEvent: null,
    isConnected,
  })
  mockedUseDispatcher.mockReturnValue({
    status: mockRunningStatus,
    dispatcherEvents: [],
    isConnected,
  })
}

function setupStopped() {
  mockedUseWebSocket.mockReturnValue({
    lastEvent: null,
    isConnected: false,
  })
  mockedUseDispatcher.mockReturnValue({
    status: mockStoppedStatus,
    dispatcherEvents: [],
    isConnected: false,
  })
}

function setupWithEvents() {
  mockedUseWebSocket.mockReturnValue({
    lastEvent: null,
    isConnected: true,
  })
  mockedUseDispatcher.mockReturnValue({
    status: mockRunningStatus,
    dispatcherEvents: [
      { type: 'assignment_pass_started', timestamp: '2025-05-26T10:00:00Z', story_id: 'story-abc' },
      { type: 'assignment_pass_finished', timestamp: '2025-05-26T10:01:00Z', story_id: 'story-def' },
      { type: 'gate_check', timestamp: '2025-05-26T10:02:00Z' },
    ],
    isConnected: true,
  })
}

describe('DispatcherPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the "Dispatcher Dashboard" header', () => {
    setupRunning()
    render(<DispatcherPage />)

    expect(screen.getByText('Dispatcher Dashboard')).toBeInTheDocument()
  })

  describe('status bar', () => {
    it('shows Running status when dispatcher is running', () => {
      setupRunning()
      render(<DispatcherPage />)

      expect(screen.getByText('Running')).toBeInTheDocument()
    })

    it('shows Stopped status when dispatcher is stopped', () => {
      setupStopped()
      render(<DispatcherPage />)

      expect(screen.getByText('Stopped')).toBeInTheDocument()
    })

    it('shows uptime in human-readable format', () => {
      setupRunning()
      render(<DispatcherPage />)

      // 3661 seconds = 1h 1m 1s
      expect(screen.getByText('1h 1m 1s')).toBeInTheDocument()
    })

    it('shows queue depth', () => {
      setupRunning()
      render(<DispatcherPage />)

      expect(screen.getByText('5')).toBeInTheDocument()
    })

    it('shows queue dash em dash when status is null', () => {
      mockedUseWebSocket.mockReturnValue({
        lastEvent: null,
        isConnected: false,
      })
      mockedUseDispatcher.mockReturnValue({
        status: null,
        dispatcherEvents: [],
        isConnected: false,
      })

      render(<DispatcherPage />)

      // The queue depth — appears in the Queue Depth card (first em-dash in the document)
      const emDashes = screen.getAllByText('—')
      expect(emDashes.length).toBeGreaterThanOrEqual(1)
    })
  })

  describe('WebSocket indicator', () => {
    it('shows Connected when isConnected is true', () => {
      setupRunning(true)
      render(<DispatcherPage />)

      expect(screen.getByText('Connected')).toBeInTheDocument()
    })

    it('shows Disconnected when isConnected is false', () => {
      setupStopped()
      render(<DispatcherPage />)

      expect(screen.getByText('Disconnected')).toBeInTheDocument()
    })
  })

  describe('Events Processed', () => {
    it('renders the Events Processed section with counts', () => {
      setupRunning()
      render(<DispatcherPage />)

      expect(screen.getByText('Events Processed')).toBeInTheDocument()
      expect(screen.getByText('10')).toBeInTheDocument()
      expect(screen.getByText('8')).toBeInTheDocument()
      expect(screen.getByText('3')).toBeInTheDocument()
      expect(screen.getByText('1')).toBeInTheDocument()
    })

    it('does not render Events Processed section when events_processed is absent from status', () => {
      mockedUseWebSocket.mockReturnValue({
        lastEvent: null,
        isConnected: false,
      })
      mockedUseDispatcher.mockReturnValue({
        status: {
          running: true,
          uptime_seconds: 100,
          event_queue_depth: 0,
          started_at: '2025-05-26T09:00:00Z',
        } as DispatcherStatus,
        dispatcherEvents: [],
        isConnected: false,
      })

      render(<DispatcherPage />)

      // events_processed is undefined, so the condition is falsy
      expect(screen.queryByText('Events Processed')).not.toBeInTheDocument()
    })
  })

  describe('Live Event Feed', () => {
    it('renders Live Event Feed header', () => {
      setupRunning()
      render(<DispatcherPage />)

      expect(screen.getByText('Live Event Feed')).toBeInTheDocument()
    })

    it('shows "Waiting for dispatcher events..." when feed is empty', () => {
      setupRunning()
      render(<DispatcherPage />)

      expect(screen.getByText('Waiting for dispatcher events...')).toBeInTheDocument()
    })

    it('renders events when present', () => {
      setupWithEvents()
      render(<DispatcherPage />)

      // The event type text appears in both Events Processed labels and event feed badges
      // Use getAllByText and verify count — each event appears once in the feed badge
      // plus once in the events_processed grid for assignment_pass_started/finished
      expect(screen.getAllByText('assignment_pass_started').length).toBeGreaterThanOrEqual(1)
      expect(screen.getAllByText('assignment_pass_finished').length).toBeGreaterThanOrEqual(1)
      expect(screen.getAllByText('gate_check').length).toBeGreaterThanOrEqual(1)
    })

    it('shows event count in header when events are present', () => {
      setupWithEvents()
      render(<DispatcherPage />)

      expect(screen.getByText('(3)')).toBeInTheDocument()
    })

    it('does not show event count when feed is empty', () => {
      setupRunning()
      render(<DispatcherPage />)

      expect(screen.queryByText('(0)')).not.toBeInTheDocument()
    })
  })

  describe('Pipeline Panels', () => {
    it('renders the Assignment Pipeline panel', () => {
      setupRunning()
      render(<DispatcherPage />)

      expect(screen.getByText('Assignment Pipeline')).toBeInTheDocument()
      expect(screen.getByText('Ready Tasks')).toBeInTheDocument()
      expect(screen.getByText('Active Sessions')).toBeInTheDocument()
      expect(screen.getByText('Last Pass')).toBeInTheDocument()
    })

    it('renders the Gate Pipeline panel', () => {
      setupRunning()
      render(<DispatcherPage />)

      expect(screen.getByText('Gate Pipeline')).toBeInTheDocument()
      expect(screen.getByText('Pending Build Gates')).toBeInTheDocument()
      expect(screen.getByText('Pending Review Gates')).toBeInTheDocument()
    })

    it('renders the Staleness Monitor panel', () => {
      setupRunning()
      render(<DispatcherPage />)

      expect(screen.getByText('Staleness Monitor')).toBeInTheDocument()
      expect(screen.getByText('Stale Sessions')).toBeInTheDocument()
      expect(screen.getByText('Last Check')).toBeInTheDocument()
    })
  })
})