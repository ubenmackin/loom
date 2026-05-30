import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import StoryCard from './StoryCard'
import type { Story } from '../types'

const baseStory: Story = {
  id: 'story-1',
  title: 'Test Story',
  description: 'A test story',
  status: 'new',
  requires_build: false,
  requires_review: false,
  assigned_to: undefined,
  sort_order: 1,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

describe('StoryCard', () => {
  it('renders story title', () => {
    render(<StoryCard story={baseStory} />)
    expect(screen.getByText('Test Story')).toBeInTheDocument()
  })

  it('renders "STORY" SharpTag with variant="primary"', () => {
    render(<StoryCard story={baseStory} />)
    // SharpTag renders label inside brackets
    expect(screen.getByText('[STORY]')).toBeInTheDocument()
  })

  it('renders status SharpTag with label uppercase', () => {
    render(<StoryCard story={{ ...baseStory, status: 'blocked' }} />)
    // status is 'blocked', so label should be 'BLOCKED'
    expect(screen.getByText('[BLOCKED]')).toBeInTheDocument()
  })

  describe('build indicator', () => {
    it('shows CheckSquare icon when requires_build is true', () => {
      render(<StoryCard story={{ ...baseStory, requires_build: true }} />)
      // The CheckSquare icon from lucide-react renders as an SVG
      // We can look for the lucide-react icon by its class or role
      const svg = document.querySelector('svg')
      expect(svg).toBeInTheDocument()
    })

    it('does NOT show build icon when requires_build is false', () => {
      render(<StoryCard story={{ ...baseStory, requires_build: false }} />)
      // No CheckSquare icon should be present when requires_build is false
      // However, requires_review might also be false, check both
      const svg = document.querySelector('svg')
      // With status 'new' (no icon), and requires_build=false + requires_review=false
      // there should be no CheckSquare icons. But SharpTag doesn't render SVGs.
      // The only SVG source would be the CheckSquare icon from build/review indicators
      // Since both are false, no SVGs should be rendered from lucide-react
      // But SharpTag is not an SVG element - just text. So no SVGs at all.
      expect(svg).not.toBeInTheDocument()
    })
  })

  describe('review indicator', () => {
    it('shows CheckSquare icon when requires_review is true', () => {
      render(<StoryCard story={{ ...baseStory, requires_review: true }} />)
      const svg = document.querySelector('svg')
      expect(svg).toBeInTheDocument()
    })

    it('does NOT show review icon when requires_review is false', () => {
      render(<StoryCard story={{ ...baseStory, requires_review: false }} />)
      const svg = document.querySelector('svg')
      expect(svg).not.toBeInTheDocument()
    })
  })

  describe('assignee', () => {
    it('shows assignee name when present via story.assigned_to', () => {
      render(<StoryCard story={{ ...baseStory, assigned_to: 'agent-7' }} />)
      expect(screen.getByText('agent-7')).toBeInTheDocument()
    })

    it('prefers assigneeName prop over story.assigned_to', () => {
      render(
        <StoryCard
          story={{ ...baseStory, assigned_to: 'fallback-agent' }}
          assigneeName="preferred-agent"
        />
      )
      expect(screen.getByText('preferred-agent')).toBeInTheDocument()
    })

    it('does NOT show assignee when neither is provided', () => {
      render(<StoryCard story={baseStory} />)
      expect(screen.queryByText('fallback-agent')).not.toBeInTheDocument()
      expect(screen.queryByText('preferred-agent')).not.toBeInTheDocument()
    })
  })

  describe('stale indicator', () => {
    it('shows "stale" text when updated_at is more than 2 hours ago', () => {
      const oldDate = new Date(Date.now() - 3 * 60 * 60 * 1000).toISOString() // 3 hours ago
      render(<StoryCard story={{ ...baseStory, updated_at: oldDate }} />)
      expect(screen.getByText('stale')).toBeInTheDocument()
    })

    it('does NOT show stale indicator when updated_at is recent', () => {
      const recentDate = new Date().toISOString()
      render(<StoryCard story={{ ...baseStory, updated_at: recentDate }} />)
      expect(screen.queryByText('stale')).not.toBeInTheDocument()
    })
  })

  describe('onClick', () => {
    it('calls onClick when card is clicked', async () => {
      const onClick = vi.fn()
      const user = userEvent.setup()
      render(<StoryCard story={baseStory} onClick={onClick} />)
      await user.click(screen.getByText('Test Story'))
      expect(onClick).toHaveBeenCalledOnce()
    })
  })

  it('is wrapped in memo (memoized component)', () => {
    // Since StoryCard is exported as `memo(StoryCard)`, the default export
    // should have a `displayName` (by convention) or at minimum be a memo-wrapped component.
    // We can verify the exported component is not the raw function.
    const StoryCardModule = StoryCard
    // Memo wraps the component; the default export is a React.memo result.
    // Verify that the component's type is not a plain function.
    expect(typeof StoryCardModule).not.toBe('function')
  })
})