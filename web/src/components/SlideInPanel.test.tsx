import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import SlideInPanel, { PanelLoading, PanelNotFound } from './SlideInPanel'

describe('SlideInPanel', () => {
  it('renders children inside the panel wrapper', () => {
    render(
      <SlideInPanel>
        <span>child content</span>
      </SlideInPanel>,
    )
    expect(screen.getByText('child content')).toBeInTheDocument()
  })
})

describe('PanelLoading', () => {
  it('renders "Loading..." message by default', () => {
    render(<PanelLoading />)
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  it('renders a custom message when provided', () => {
    render(<PanelLoading message="Custom loading message" />)
    expect(screen.getByText('Custom loading message')).toBeInTheDocument()
  })

  it('renders inside the slide-in wrapper', () => {
    render(<PanelLoading />)
    const container = screen.getByText('Loading...').closest('div')
    expect(container).toBeInTheDocument()
  })
})

describe('PanelNotFound', () => {
  it('renders "Not found" message by default', () => {
    render(<PanelNotFound />)
    expect(screen.getByText('Not found')).toBeInTheDocument()
  })

  it('renders a custom message when provided', () => {
    render(<PanelNotFound message="Custom not found" />)
    expect(screen.getByText('Custom not found')).toBeInTheDocument()
  })

  it('renders inside the slide-in wrapper', () => {
    render(<PanelNotFound />)
    const container = screen.getByText('Not found').closest('div')
    expect(container).toBeInTheDocument()
  })
})
