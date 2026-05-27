import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import CreateStoryForm from './CreateStoryForm'

describe('CreateStoryForm', () => {
  it('returns null when closed', () => {
    const { container } = render(
      <CreateStoryForm open={false} onSubmit={vi.fn()} onCancel={vi.fn()} />,
    )

    expect(container).toBeEmptyDOMElement()
  })

  it('renders form elements when open', () => {
    render(<CreateStoryForm open={true} onSubmit={vi.fn()} onCancel={vi.fn()} />)

    expect(screen.getByRole('heading', { name: /create story/i })).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Story title...')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Markdown description...')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /create story/i })).toBeInTheDocument()
  })

  it('renders build and review checkboxes when open', () => {
    render(<CreateStoryForm open={true} onSubmit={vi.fn()} onCancel={vi.fn()} />)

    const checkboxes = screen.getAllByRole('checkbox')
    expect(checkboxes).toHaveLength(2)
    expect(screen.getByText(/requires build/i)).toBeInTheDocument()
    expect(screen.getByText(/requires review/i)).toBeInTheDocument()
  })

  it('calls onCancel when Escape key is pressed', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()
    render(<CreateStoryForm open={true} onSubmit={vi.fn()} onCancel={onCancel} />)

    await user.keyboard('{Escape}')

    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('shows error when submitting with empty title', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<CreateStoryForm open={true} onSubmit={onSubmit} onCancel={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: /create story/i }))

    expect(screen.getByText('Title is required')).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('submits valid data with correct shape', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<CreateStoryForm open={true} onSubmit={onSubmit} onCancel={vi.fn()} />)

    await user.type(screen.getByPlaceholderText('Story title...'), 'My New Story')
    await user.type(
      screen.getByPlaceholderText('Markdown description...'),
      'This is a description',
    )

    await user.click(screen.getByRole('button', { name: /create story/i }))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit).toHaveBeenCalledWith({
      title: 'My New Story',
      description: 'This is a description',
      requires_build: false,
      requires_review: false,
    })
  })

  it('submits with build and review checkboxes checked', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<CreateStoryForm open={true} onSubmit={onSubmit} onCancel={vi.fn()} />)

    await user.type(screen.getByPlaceholderText('Story title...'), 'Story with Flags')

    const checkboxes = screen.getAllByRole('checkbox')
    await user.click(checkboxes[0]) // requires_build
    await user.click(checkboxes[1]) // requires_review

    await user.click(screen.getByRole('button', { name: /create story/i }))

    expect(onSubmit).toHaveBeenCalledWith({
      title: 'Story with Flags',
      description: '',
      requires_build: true,
      requires_review: true,
    })
  })

  it('Cancel button resets state and calls onCancel', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()
    const onSubmit = vi.fn()
    render(<CreateStoryForm open={true} onSubmit={onSubmit} onCancel={onCancel} />)

    // Fill in form fields
    await user.type(screen.getByPlaceholderText('Story title...'), 'Cancel Story')
    await user.type(
      screen.getByPlaceholderText('Markdown description...'),
      'Will be cancelled',
    )

    const checkboxes = screen.getAllByRole('checkbox')
    await user.click(checkboxes[0]) // requires_build

    await user.click(screen.getByRole('button', { name: /cancel/i }))

    expect(onCancel).toHaveBeenCalledTimes(1)
    // onSubmit should not have been called
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
