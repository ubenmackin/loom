import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'

// ── Mock TopNav so Layout tests don't depend on auth/theme state ─────────

vi.mock('./TopNav', () => ({
  default: () => <nav data-testid="top-nav">Mock TopNav</nav>,
}))

vi.mock('./SubNav', () => ({
  default: () => <div data-testid="sub-nav">Mock SubNav</div>,
}))

// ── Imports (after mocks) ─────────────────────────────────────────────────

import { MemoryRouter, Routes, Route } from 'react-router-dom'
import Layout from './Layout'

// ── Fixtures ──────────────────────────────────────────────────────────────

function MockChild() {
  return <div data-testid="outlet-child">Outlet Content</div>
}

// ── Helpers ───────────────────────────────────────────────────────────────

function renderLayout() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<MockChild />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────

describe('Layout', () => {
  it('renders the TopNav component', () => {
    renderLayout()
    expect(screen.getByTestId('top-nav')).toBeInTheDocument()
  })

  it('renders the SubNav component', () => {
    renderLayout()
    expect(screen.getByTestId('sub-nav')).toBeInTheDocument()
  })

  it('renders the TopNav text content', () => {
    renderLayout()
    expect(screen.getByText('Mock TopNav')).toBeInTheDocument()
  })

  it('renders the Outlet content (child route)', () => {
    renderLayout()
    expect(screen.getByTestId('outlet-child')).toBeInTheDocument()
    expect(screen.getByText('Outlet Content')).toBeInTheDocument()
  })

  it('contains a <main> element', () => {
    renderLayout()
    const main = document.querySelector('main')
    expect(main).toBeInTheDocument()
  })

  it('renders TopNav, SubNav, then Outlet content in DOM order', () => {
    renderLayout()
    const topNav = screen.getByTestId('top-nav')
    const subNav = screen.getByTestId('sub-nav')
    const outlet = screen.getByTestId('outlet-child')

    // TopNav before SubNav before Outlet
    expect(topNav.compareDocumentPosition(subNav)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    )
    expect(subNav.compareDocumentPosition(outlet)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    )
  })
})
