import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ProjectPicker from './ProjectPicker'

const mockProjects = [
  { id: 'proj-1', name: 'Frontend', created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' },
  { id: 'proj-2', name: 'Backend', created_at: '2025-01-02T00:00:00Z', updated_at: '2025-01-02T00:00:00Z' },
  { id: 'proj-3', name: 'DevOps', created_at: '2025-01-03T00:00:00Z', updated_at: '2025-01-03T00:00:00Z' },
]

// Mock functions
const storeSetSelectedProjectId = vi.fn()

// Mock the project store
vi.mock('../stores/project', () => ({
  useProjectFilterStore: vi.fn(),
}))

// Mock @tanstack/react-query
const mockUseQuery = vi.fn()
vi.mock('@tanstack/react-query', () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}))

// Mock the api client
vi.mock('../api/client', () => ({
  fetchProjects: vi.fn(),
}))

import { useProjectFilterStore } from '../stores/project'

function setupStore(selectedProjectId: string | null = null) {
  const mockStore = useProjectFilterStore as unknown as ReturnType<typeof vi.fn>
  mockStore.mockReturnValue({
    selectedProjectId,
    setSelectedProjectId: storeSetSelectedProjectId,
    clearProjectFilter: vi.fn(),
  })
}

function setupQuery(overrides: { data?: typeof mockProjects; isLoading?: boolean } = {}) {
  mockUseQuery.mockReturnValue({
    data: overrides.data ?? mockProjects,
    isLoading: overrides.isLoading ?? false,
  })
}

describe('ProjectPicker', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setupStore(null)
    setupQuery()
  })

  it('renders "Select project..."" placeholder when no project selected', () => {
    render(<ProjectPicker />)
    expect(screen.getByRole('combobox')).toHaveTextContent('Select project...')
  })

  it('renders loading placeholder when data is loading', () => {
    setupQuery({ isLoading: true, data: [] })
    render(<ProjectPicker />)
    expect(screen.getByRole('combobox')).toHaveTextContent('Loading...')
  })

  it('renders project name when a project is selected', () => {
    setupStore('proj-1')
    render(<ProjectPicker />)
    expect(screen.getByRole('combobox')).toHaveTextContent('Frontend')
  })

  it('opens dropdown on button click', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    expect(screen.getByRole('listbox')).toBeInTheDocument()
  })

  it('closes dropdown when clicking outside', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    expect(screen.getByRole('listbox')).toBeInTheDocument()
    await user.click(document.body)
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('toggles dropdown closed on second button click', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    expect(screen.getByRole('listbox')).toBeInTheDocument()
    await user.click(screen.getByRole('combobox'))
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('renders search input inside dropdown', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    expect(screen.getByPlaceholderText('Search...')).toBeInTheDocument()
  })

  it('filters projects by search term', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    expect(screen.getAllByRole('option')).toHaveLength(3)
    await user.type(screen.getByPlaceholderText('Search...'), 'Front')
    expect(screen.getAllByRole('option')).toHaveLength(1)
    expect(screen.getByRole('option')).toHaveTextContent('Frontend')
  })

  it('shows "No projects found" when filter matches nothing', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    await user.type(screen.getByPlaceholderText('Search...'), 'zzz')
    expect(screen.getByText('No projects found')).toBeInTheDocument()
  })

  it('calls setSelectedProjectId when a project is selected', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    await user.click(screen.getByText('Backend'))
    expect(storeSetSelectedProjectId).toHaveBeenCalledWith('proj-2')
  })

  it('closes dropdown after selecting a project', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    await user.click(screen.getByText('Backend'))
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('has correct ARIA attributes on trigger button when closed', () => {
    render(<ProjectPicker />)
    const button = screen.getByRole('combobox')
    expect(button).toHaveAttribute('aria-expanded', 'false')
    expect(button).toHaveAttribute('aria-haspopup', 'listbox')
    expect(button).toHaveAttribute('aria-controls', 'project-listbox')
  })

  it('has correct ARIA attributes on trigger button when open', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    expect(screen.getByRole('combobox')).toHaveAttribute('aria-expanded', 'true')
  })

  it('sets role="option" and aria-selected on project items', async () => {
    setupStore('proj-1')
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    const options = screen.getAllByRole('option')
    expect(options).toHaveLength(3)
    expect(options[0]).toHaveAttribute('aria-selected', 'true')
    expect(options[1]).toHaveAttribute('aria-selected', 'false')
    expect(options[2]).toHaveAttribute('aria-selected', 'false')
  })

  it('navigates with ArrowDown through options', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    await user.keyboard('{ArrowDown}')
    const options = screen.getAllByRole('option')
    expect(options[0].className).toContain('bg-gray-50')
    await user.keyboard('{ArrowDown}')
    expect(options[1].className).toContain('bg-gray-50')
    expect(options[0].className).not.toContain('bg-gray-50')
  })

  it('navigates with ArrowUp through options', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    await user.keyboard('{ArrowUp}')
    const options = screen.getAllByRole('option')
    expect(options[2].className).toContain('bg-gray-50')
    await user.keyboard('{ArrowUp}')
    expect(options[1].className).toContain('bg-gray-50')
    expect(options[2].className).not.toContain('bg-gray-50')
  })

  it('selects highlighted option on Enter', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    await user.keyboard('{ArrowDown}{ArrowDown}')
    await user.keyboard('{Enter}')
    expect(storeSetSelectedProjectId).toHaveBeenCalledWith('proj-2')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('closes dropdown on Escape key', async () => {
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    expect(screen.getByRole('listbox')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('renders no options and shows "No projects found" when project list is empty', async () => {
    setupQuery({ data: [] })
    const user = userEvent.setup()
    render(<ProjectPicker />)
    await user.click(screen.getByRole('combobox'))
    expect(screen.getByText('No projects found')).toBeInTheDocument()
  })
})