import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ConfirmModal from './ConfirmModal'

describe('ConfirmModal', () => {
  const defaultProps = {
    open: true,
    title: 'Delete Board',
    message: 'Are you sure you want to delete this board?',
    onConfirm: vi.fn(),
    onCancel: vi.fn(),
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns null when open is false', () => {
    const { container } = render(
      <ConfirmModal {...defaultProps} open={false} />,
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders the title when open is true', () => {
    render(<ConfirmModal {...defaultProps} />)
    expect(screen.getByText('Delete Board')).toBeInTheDocument()
  })

  it('renders the message when open is true', () => {
    render(<ConfirmModal {...defaultProps} />)
    expect(
      screen.getByText('Are you sure you want to delete this board?'),
    ).toBeInTheDocument()
  })

  it('renders a Confirm button', () => {
    render(<ConfirmModal {...defaultProps} />)
    expect(
      screen.getByRole('button', { name: /confirm/i }),
    ).toBeInTheDocument()
  })

  it('renders a Cancel button', () => {
    render(<ConfirmModal {...defaultProps} />)
    expect(
      screen.getByRole('button', { name: /cancel/i }),
    ).toBeInTheDocument()
  })

  it('calls onConfirm when the Confirm button is clicked', async () => {
    const onConfirm = vi.fn()
    render(<ConfirmModal {...defaultProps} onConfirm={onConfirm} />)

    await userEvent.click(screen.getByRole('button', { name: /confirm/i }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('calls onCancel when the Cancel button is clicked', async () => {
    const onCancel = vi.fn()
    render(<ConfirmModal {...defaultProps} onCancel={onCancel} />)

    await userEvent.click(screen.getByRole('button', { name: /cancel/i }))
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('does not render title/message/buttons when open is false', () => {
    render(<ConfirmModal {...defaultProps} open={false} />)

    expect(screen.queryByText('Delete Board')).not.toBeInTheDocument()
    expect(
      screen.queryByText('Are you sure you want to delete this board?'),
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /confirm/i }),
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /cancel/i }),
    ).not.toBeInTheDocument()
  })
})
