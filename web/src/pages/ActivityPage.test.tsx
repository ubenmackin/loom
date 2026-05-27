import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ActivityPage from './ActivityPage'
import type { ActivityLogEntry } from '../types'

vi.mock('../hooks/useActivity', () => ({
  useActivity: vi.fn(),
}))

vi.mock('../utils/relativeTime', () => ({
  relativeTime: vi.fn().mockReturnValue('5m ago'),
}))

vi.mock('../components/StoryDetail', () => ({
  default: () => <div data-testid="story-detail">Story Detail</div>,
}))

import { useActivity } from '../hooks/useActivity'

const mockedUseActivity = useActivity as ReturnType<typeof vi.fn>

const mockEntries: ActivityLogEntry[] = [
  {
    id: 'entry-1',
    work_item_id: 'story-abc',
    work_item_type: 'story',
    action: 'created',
    details: 'Created a new story',
    created_at: '2025-05-26T10:00:00Z',
  },
  {
    id: 'entry-2',
    work_item_id: 'task-xyz',
    work_item_type: 'task',
    action: 'updated',
    details: 'Updated the task',
    created_at: '2025-05-26T10:05:00Z',
  },
  {
    id: 'entry-3',
    work_item_id: 'story-def',
    work_item_type: 'story',
    action: 'status_changed',
    created_at: '2025-05-26T10:10:00Z',
  },
]

describe('ActivityPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('loading state', () => {
    it('renders "Loading activity..." when isLoading is true', () => {
      mockedUseActivity.mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
        refetch: vi.fn(),
      })

      render(<ActivityPage />)

      expect(screen.getByText('Loading activity...')).toBeInTheDocument()
    })
  })

  describe('error state', () => {
    it('renders error message and Retry button', async () => {
      const refetch = vi.fn()
      mockedUseActivity.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Network failure'),
        refetch,
      })

      render(<ActivityPage />)

      expect(screen.getByText('Error loading activity: Network failure')).toBeInTheDocument()

      const retryButton = screen.getByText('Retry')
      expect(retryButton).toBeInTheDocument()

      const user = userEvent.setup()
      await user.click(retryButton)
      expect(refetch).toHaveBeenCalledOnce()
    })
  })

  describe('empty state', () => {
    it('renders "No activity yet" when entries array is empty', () => {
      mockedUseActivity.mockReturnValue({
        data: [],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<ActivityPage />)

      expect(screen.getByText('No activity yet')).toBeInTheDocument()
    })
  })

  describe('data state', () => {
    it('renders activity entries with relative time and action badges', () => {
      mockedUseActivity.mockReturnValue({
        data: mockEntries,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<ActivityPage />)

      // relativeTime mock returns '5m ago' for all entries
      const timeLabels = screen.getAllByText('5m ago')
      expect(timeLabels).toHaveLength(3)

      // Action badges — the component replaces '_' with ' '
      expect(screen.getByText('created')).toBeInTheDocument()
      expect(screen.getByText('updated')).toBeInTheDocument()
      expect(screen.getByText('status changed')).toBeInTheDocument()
    })

    it('renders work item references (clickable for stories, plain for tasks)', () => {
      mockedUseActivity.mockReturnValue({
        data: mockEntries,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<ActivityPage />)

      // Story work_item_ids are rendered as buttons (clickable)
      const storyButtons = screen.getAllByText('story-abc')
      expect(storyButtons.length).toBeGreaterThanOrEqual(1)
      storyButtons.forEach((btn) => {
        expect(btn.tagName).toBe('BUTTON')
      })

      // Task work_item_id is rendered as plain text (not a button)
      const taskId = screen.getByText('task-xyz')
      expect(taskId.tagName).not.toBe('BUTTON')
    })

    it('renders details text when present', () => {
      mockedUseActivity.mockReturnValue({
        data: mockEntries,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<ActivityPage />)

      expect(screen.getByText('Created a new story')).toBeInTheDocument()
      expect(screen.getByText('Updated the task')).toBeInTheDocument()
    })

    it('shows entry count in the header', () => {
      mockedUseActivity.mockReturnValue({
        data: mockEntries,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<ActivityPage />)

      expect(screen.getByText('[3]')).toBeInTheDocument()
    })
  })

  describe('story detail panel', () => {
    it('opens StoryDetail when a story entry is clicked', async () => {
      const user = userEvent.setup()
      mockedUseActivity.mockReturnValue({
        data: mockEntries,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      render(<ActivityPage />)

      // Click the story-abc button
      const storyButton = screen.getByText('story-abc')
      await user.click(storyButton)

      expect(screen.getByTestId('story-detail')).toBeInTheDocument()
    })
  })
})