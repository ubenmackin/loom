import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import SharpTag from './SharpTag'

describe('SharpTag', () => {
  it('renders label text inside brackets', () => {
    render(<SharpTag label="test-label" />)
    const element = screen.getByText('[test-label]')
    expect(element).toBeInTheDocument()
  })

  it('applies the default variant class when no variant is specified', () => {
    render(<SharpTag label="hello" />)
    const element = screen.getByText('[hello]')
    expect(element).toHaveClass('sharp-tag')
    // default variant has an empty string class, so no extra variant class
    expect(element.className).toBe('sharp-tag ')
  })

  it('applies the primary variant class', () => {
    render(<SharpTag label="primary" variant="primary" />)
    const element = screen.getByText('[primary]')
    expect(element).toHaveClass('sharp-tag', 'sharp-tag-primary')
  })

  it('applies the amber variant class', () => {
    render(<SharpTag label="amber" variant="amber" />)
    const element = screen.getByText('[amber]')
    expect(element).toHaveClass('sharp-tag', 'sharp-tag-amber')
  })

  it('applies the success variant class', () => {
    render(<SharpTag label="success" variant="success" />)
    const element = screen.getByText('[success]')
    expect(element).toHaveClass('sharp-tag', 'sharp-tag-success')
  })

  it('applies the error variant class', () => {
    render(<SharpTag label="error" variant="error" />)
    const element = screen.getByText('[error]')
    expect(element).toHaveClass('sharp-tag', 'sharp-tag-error')
  })
})
