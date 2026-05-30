import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { CellDropZone } from './Column'

// Mock useDroppable from @dnd-kit/core so we can inspect the config passed to it.
// vi.hoisted ensures the variable is available before the hoisted vi.mock factory runs.
const mockSetNodeRef = vi.hoisted(() => vi.fn())
vi.mock('@dnd-kit/core', () => ({
  useDroppable: vi.fn().mockReturnValue({ setNodeRef: mockSetNodeRef }),
}))

import { useDroppable } from '@dnd-kit/core'

describe('CellDropZone', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders children inside the wrapper div', () => {
    render(
      <CellDropZone id="cell-1" storyId="story-1" status="new">
        <span>Child Content</span>
      </CellDropZone>,
    )
    expect(screen.getByText('Child Content')).toBeInTheDocument()
  })

  it('renders multiple children', () => {
    render(
      <CellDropZone id="cell-2" storyId="story-2" status="in_progress">
        <span>First</span>
        <span>Second</span>
      </CellDropZone>,
    )
    expect(screen.getByText('First')).toBeInTheDocument()
    expect(screen.getByText('Second')).toBeInTheDocument()
  })

  it('calls useDroppable with the correct id', () => {
    render(
      <CellDropZone id="cell-test-42" storyId="story-abc" status="done">
        <span>test</span>
      </CellDropZone>,
    )
    expect(useDroppable).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'cell-test-42' }),
    )
  })

  it('calls useDroppable with correct data attributes', () => {
    render(
      <CellDropZone id="cell-99" storyId="story-xyz" status="blocked">
        <span>test</span>
      </CellDropZone>,
    )
    expect(useDroppable).toHaveBeenCalledWith(
      expect.objectContaining({
        data: {
          type: 'cell',
          storyId: 'story-xyz',
          status: 'blocked',
        },
      }),
    )
  })

  it('renders wrapper div with ref for droppable', () => {
    render(
      <CellDropZone id="cell-1" storyId="story-1" status="new">
        <span>test</span>
      </CellDropZone>,
    )
    const child = screen.getByText('test')
    const parentDiv = child.closest('div')
    expect(parentDiv).toBeInTheDocument()
  })

  it('renders without crashing when children is empty', () => {
    render(
      <CellDropZone id="cell-empty" storyId="story-empty" status="ready">
        <></>
      </CellDropZone>,
    )
    // Should not throw — the wrapper div should be present
    expect(mockSetNodeRef).toHaveBeenCalledTimes(1)
    expect(mockSetNodeRef.mock.calls[0][0]).toBeInstanceOf(HTMLElement)
  })

  it('passes setNodeRef to the wrapper div', () => {
    render(
      <CellDropZone id="cell-ref" storyId="story-ref" status="new">
        <span>test-ref</span>
      </CellDropZone>,
    )
    // The mocked setNodeRef should have been called once with the wrapper div element
    expect(mockSetNodeRef).toHaveBeenCalledTimes(1)
    const el = mockSetNodeRef.mock.calls[0][0]
    expect(el).toBeInstanceOf(HTMLElement)
    expect(el.tagName).toBe('DIV')
  })
})
