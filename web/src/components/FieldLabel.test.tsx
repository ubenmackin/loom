import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import FieldLabel from './FieldLabel'

describe('FieldLabel', () => {
  it('renders children text content', () => {
    render(<FieldLabel>Hello World</FieldLabel>)
    expect(screen.getByText('Hello World')).toBeInTheDocument()
  })

  it('renders children elements', () => {
    render(
      <FieldLabel>
        <span>Nested Element</span>
      </FieldLabel>,
    )
    expect(screen.getByText('Nested Element')).toBeInTheDocument()
  })

  it('applies the default mb-1 margin class', () => {
    render(<FieldLabel>Test</FieldLabel>)
    const label = screen.getByText('Test')
    expect(label).toHaveClass('mb-1')
  })

  it('applies a custom margin class when margin prop is provided', () => {
    render(<FieldLabel margin="mb-2">Test</FieldLabel>)
    const label = screen.getByText('Test')
    expect(label).toHaveClass('mb-2')
    expect(label).not.toHaveClass('mb-1')
  })

  it('renders as a label element', () => {
    render(<FieldLabel>Label Element</FieldLabel>)
    const label = screen.getByText('Label Element')
    expect(label.tagName).toBe('LABEL')
  })
})
