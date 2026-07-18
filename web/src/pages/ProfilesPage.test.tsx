import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ProfilesPage from './ProfilesPage'
import type { AgentProfile } from '../api/client'

vi.mock('../api/client', () => ({
  fetchProfiles: vi.fn(),
  createProfile: vi.fn(),
  updateProfile: vi.fn(),
  deleteProfile: vi.fn(),
}))

import { fetchProfiles, createProfile, updateProfile } from '../api/client'

const mockedFetchProfiles = fetchProfiles as ReturnType<typeof vi.fn>
const mockedCreateProfile = createProfile as ReturnType<typeof vi.fn>
const mockedUpdateProfile = updateProfile as ReturnType<typeof vi.fn>

const mockProfiles: AgentProfile[] = [
  {
    id: 'profile-1',
    name: 'Planner',
    description: 'Handles story planning and estimation',
    max_concurrency: 3,
    agent_role: '',
    task_types: ['planning'],
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
  {
    id: 'profile-2',
    name: 'Coder',
    description: 'Implements features and fixes bugs',
    max_concurrency: 5,
    agent_role: '',
    task_types: ['code', 'review'],
    created_at: '2025-01-02T00:00:00Z',
    updated_at: '2025-01-02T00:00:00Z',
  },
]

function setupWithProfiles() {
  mockedFetchProfiles.mockResolvedValue(mockProfiles)
}

function setupEmpty() {
  mockedFetchProfiles.mockResolvedValue([])
}

describe('ProfilesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders profile cards with names', async () => {
    setupWithProfiles()
    render(<ProfilesPage />)

    await waitFor(() => {
      expect(screen.getByText('Planner')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('Coder')).toBeInTheDocument()
    })
  })

  it('renders capability tags', async () => {
    setupWithProfiles()
    render(<ProfilesPage />)

    await waitFor(() => {
      expect(screen.getByText('planning')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('code')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('review')).toBeInTheDocument()
    })
  })

  it('shows max concurrency values', async () => {
    setupWithProfiles()
    render(<ProfilesPage />)

    await waitFor(() => {
      const concurrencyValues = screen.getAllByText('3')
      expect(concurrencyValues.length).toBeGreaterThanOrEqual(1)
    })
    await waitFor(() => {
      const concurrencyValues = screen.getAllByText('5')
      expect(concurrencyValues.length).toBeGreaterThanOrEqual(1)
    })
  })

  it('shows create profile button', async () => {
    setupWithProfiles()
    render(<ProfilesPage />)

    await waitFor(() => {
      expect(screen.getByText('+ Create Profile')).toBeInTheDocument()
    })
  })

  it('shows loading state initially', () => {
    setupWithProfiles()
    mockedFetchProfiles.mockImplementationOnce(() => new Promise(() => {})) // never resolves
    render(<ProfilesPage />)

    expect(screen.getByText('Loading profiles...')).toBeInTheDocument()
  })

  it('shows empty state when no profiles exist', async () => {
    setupEmpty()
    render(<ProfilesPage />)

    await waitFor(() => {
      expect(screen.getByText('No agent profiles found. Create one to get started.')).toBeInTheDocument()
    })
  })

  it('renders all 7 task type checkboxes in the edit form', async () => {
    setupWithProfiles()
    render(<ProfilesPage />)

    // Wait for profile cards to render, then open the edit form for the first profile.
    await waitFor(() => {
      expect(screen.getByText('Planner')).toBeInTheDocument()
    })
    const user = userEvent.setup()
    // Each profile card renders its own "Edit" button; open the first profile's edit form.
    await user.click(screen.getAllByText('Edit')[0])

    // The render order in ProfileForm matches the 7-entry taskTypeOptions array.
    const expectedLabels = [
      'Code',
      'Build',
      'Review',
      'Planning',
      'Security',
      'Release',
      'Workspace Setup',
    ]
    for (const label of expectedLabels) {
      expect(screen.getByRole('checkbox', { name: label })).toBeInTheDocument()
    }
  })

  it('renders the Agent Role select with all 8 options in the edit form', async () => {
    setupWithProfiles()
    render(<ProfilesPage />)

    await waitFor(() => {
      expect(screen.getByText('Planner')).toBeInTheDocument()
    })
    const user = userEvent.setup()
    await user.click(screen.getAllByText('Edit')[0])

    const select = screen.getByLabelText('Agent Role')
    expect(select).toBeInTheDocument()

    // The select offers Default + the 7 known roles.
    const expectedOptions = [
      'Default (use profile name)',
      'Planner',
      'Executor',
      'Builder',
      'Reviewer',
      'Security Auditor',
      'Release Manager',
      'Workspace Setup',
    ]
    for (const label of expectedOptions) {
      expect(
        screen.getByRole('option', { name: label }),
      ).toBeInTheDocument()
    }
    expect(screen.getAllByRole('option')).toHaveLength(8)
  })

  it('selecting an agent role propagates through to the updateProfile call', async () => {
    setupWithProfiles()
    // updateProfile must resolve so handleSave can continue to loadProfiles.
    mockedUpdateProfile.mockResolvedValue(mockProfiles[0])
    mockedFetchProfiles.mockResolvedValue(mockProfiles)

    render(<ProfilesPage />)

    await waitFor(() => {
      expect(screen.getByText('Planner')).toBeInTheDocument()
    })
    const user = userEvent.setup()
    await user.click(screen.getAllByText('Edit')[0])

    const select = screen.getByLabelText('Agent Role')
    await user.selectOptions(select, 'executor')
    expect((select as HTMLSelectElement).value).toBe('executor')

    // Save the edit.
    await user.click(screen.getAllByText('Save')[0])

    await waitFor(() => {
      expect(mockedUpdateProfile).toHaveBeenCalledTimes(1)
    })
    const payload = mockedUpdateProfile.mock.calls[0][1] as Partial<AgentProfile>
    expect(payload.agent_role).toBe('executor')
  })

  it('selecting an agent role on a new profile propagates through to the createProfile call', async () => {
    setupWithProfiles()
    mockedCreateProfile.mockResolvedValue(mockProfiles[0])

    render(<ProfilesPage />)

    await waitFor(() => {
      expect(screen.getByText('+ Create Profile')).toBeInTheDocument()
    })
    const user = userEvent.setup()
    await user.click(screen.getByText('+ Create Profile'))

    // Name is required to reach the createProfile call.
    await user.type(screen.getByPlaceholderText('e.g., Planner'), 'New Role Agent')

    const select = screen.getByLabelText('Agent Role')
    await user.selectOptions(select, 'executor')

    await user.click(screen.getAllByText('Save')[0])

    await waitFor(() => {
      expect(mockedCreateProfile).toHaveBeenCalledTimes(1)
    })
    const payload = mockedCreateProfile.mock.calls[0][0] as { name: string; agent_role?: string }
    expect(payload.name).toBe('New Role Agent')
    expect(payload.agent_role).toBe('executor')
  })
})
