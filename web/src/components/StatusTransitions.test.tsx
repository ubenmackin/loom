import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Status } from '../types'
import StatusTransitions from './StatusTransitions'

describe('StatusTransitions', () => {
  const defaultProps = {
    currentStatus: Status.New,
    transitions: [Status.Ready, Status.InProgress, Status.Canceled],
    onTransition: vi.fn(),
    isPending: false,
  }

  it('renders the current status as a SharpTag formatted in uppercase', () => {
    render(<StatusTransitions {...defaultProps} />)
    expect(screen.getByText('[NEW]')).toBeInTheDocument()
  })

  it('renders a SharpTag for a different status', () => {
    render(
      <StatusTransitions
        {...defaultProps}
        currentStatus={Status.Done}
        transitions={[Status.Archived, Status.Canceled]}
      />,
    )
    expect(screen.getByText('[DONE]')).toBeInTheDocument()
  })

  it('renders all transition buttons for each available transition', () => {
    render(<StatusTransitions {...defaultProps} />)
    expect(screen.getByRole('button', { name: /Transition to READY status/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Transition to IN_PROGRESS status/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Transition to CANCELED status/i })).toBeInTheDocument()
    expect(screen.getAllByRole('button')).toHaveLength(3)
  })

  it('each button has an aria-label in the correct format', () => {
    render(<StatusTransitions {...defaultProps} />)
    const buttons = screen.getAllByRole('button')
    expect(buttons[0]).toHaveAttribute('aria-label', 'Transition to READY status')
    expect(buttons[1]).toHaveAttribute('aria-label', 'Transition to IN_PROGRESS status')
    expect(buttons[2]).toHaveAttribute('aria-label', 'Transition to CANCELED status')
  })

  it('calls onTransition with the status string when a transition button is clicked', async () => {
    const onTransition = vi.fn()
    const user = userEvent.setup()
    render(
      <StatusTransitions
        {...defaultProps}
        onTransition={onTransition}
      />,
    )

    await user.click(screen.getByRole('button', { name: /Transition to READY status/i }))
    expect(onTransition).toHaveBeenCalledWith(Status.Ready)

    await user.click(screen.getByRole('button', { name: /Transition to CANCELED status/i }))
    expect(onTransition).toHaveBeenCalledWith(Status.Canceled)
  })

  it('disables all transition buttons when isPending is true', () => {
    render(<StatusTransitions {...defaultProps} isPending={true} />)
    const buttons = screen.getAllByRole('button')
    buttons.forEach((button) => {
      expect(button).toBeDisabled()
    })
  })

  it('buttons are enabled when isPending is false', () => {
    render(<StatusTransitions {...defaultProps} />)
    const buttons = screen.getAllByRole('button')
    buttons.forEach((button) => {
      expect(button).toBeEnabled()
    })
  })

  it('does not call onTransition when a disabled button is clicked', async () => {
    const onTransition = vi.fn()
    const user = userEvent.setup()
    render(
      <StatusTransitions
        {...defaultProps}
        onTransition={onTransition}
        isPending={true}
      />,
    )

    await user.click(screen.getAllByRole('button')[0])
    expect(onTransition).not.toHaveBeenCalled()
  })

  it('renders the done button with glow class when transition target is "done"', () => {
    render(
      <StatusTransitions
        {...defaultProps}
        currentStatus={Status.InProgress}
        transitions={[Status.Done, Status.Blocked, Status.Canceled]}
      />,
    )
    const doneButton = screen.getByRole('button', { name: /Transition to DONE status/i })
    expect(doneButton.className).toContain('glow-button')
  })
})
