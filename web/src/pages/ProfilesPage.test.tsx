import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import ProfilesPage from './ProfilesPage'
import type { AgentProfile } from '../api/client'

vi.mock('../api/client', () => ({
  fetchProfiles: vi.fn(),
  createProfile: vi.fn(),
  updateProfile: vi.fn(),
  deleteProfile: vi.fn(),
}))

import { fetchProfiles } from '../api/client'

const mockedFetchProfiles = fetchProfiles as ReturnType<typeof vi.fn>

const mockProfiles: AgentProfile[] = [
  {
    id: 'profile-1',
    name: 'Planner',
    description: 'Handles story planning and estimation',
    capabilities: '["story_planning","estimation"]',
    max_concurrency: 3,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
  {
    id: 'profile-2',
    name: 'Coder',
    description: 'Implements features and fixes bugs',
    capabilities: '["code","review","refactor"]',
    max_concurrency: 5,
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

  it('renders the "Agent Profiles" header', async () => {
    setupWithProfiles()
    render(<ProfilesPage />)

    await waitFor(() => {
      expect(screen.getByText('Agent Profiles')).toBeInTheDocument()
    })
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
      expect(screen.getByText('story_planning')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('code')).toBeInTheDocument()
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
})
