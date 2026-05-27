import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import EditableTitle from './EditableTitle'

describe('EditableTitle', () => {
  it('shows value as a button initially', () => {
    render(<EditableTitle value="My Title" onSave={vi.fn()} />)

    const button = screen.getByRole('button', { name: 'My Title' })
    expect(button).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  it('clicking the button switches to edit mode with an input field', async () => {
    const user = userEvent.setup()
    render(<EditableTitle value="My Title" onSave={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: 'My Title' }))

    const input = screen.getByRole('textbox')
    expect(input).toBeInTheDocument()
    expect(input).toHaveValue('My Title')
  })

  it('typing in the input updates the value', async () => {
    const user = userEvent.setup()
    render(<EditableTitle value="Original" onSave={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: 'Original' }))

    const input = screen.getByRole('textbox')
    await user.clear(input)
    await user.type(input, 'Updated Title')

    expect(input).toHaveValue('Updated Title')
  })

  it('Enter key saves and exits edit mode', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn()
    render(<EditableTitle value="Original" onSave={onSave} />)

    await user.click(screen.getByRole('button', { name: 'Original' }))

    const input = screen.getByRole('textbox')
    await user.clear(input)
    await user.type(input, 'New Value{Enter}')

    expect(onSave).toHaveBeenCalledWith('New Value')
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    // Button shows the original prop value since the parent hasn't re-rendered with a new value
    expect(screen.getByRole('button')).toBeInTheDocument()
  })

  it('Escape key cancels and exits edit mode', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn()
    render(<EditableTitle value="Original" onSave={onSave} />)

    await user.click(screen.getByRole('button', { name: 'Original' }))

    const input = screen.getByRole('textbox')
    await user.clear(input)
    await user.type(input, 'New Value{Escape}')

    expect(onSave).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Original' })).toBeInTheDocument()
  })

  it('empty trimmed value does not call onSave', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn()
    render(<EditableTitle value="Original" onSave={onSave} />)

    await user.click(screen.getByRole('button', { name: 'Original' }))

    const input = screen.getByRole('textbox')
    await user.clear(input)
    await user.type(input, '  {Enter}')

    expect(onSave).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Original' })).toBeInTheDocument()
  })
})
