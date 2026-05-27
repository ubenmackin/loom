import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import StatCard from './StatCard'

describe('StatCard', () => {
  it('renders the label text', () => {
    render(<StatCard label="Total Stories" value={42} />)
    expect(screen.getByText('Total Stories')).toBeInTheDocument()
  })

  it('renders a numeric value', () => {
    render(<StatCard label="Count" value={99} />)
    expect(screen.getByText('99')).toBeInTheDocument()
  })

  it('renders a string value', () => {
    render(<StatCard label="Status" value="Active" />)
    expect(screen.getByText('Active')).toBeInTheDocument()
  })
})
