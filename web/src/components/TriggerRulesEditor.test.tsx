import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import TriggerRulesEditor from './TriggerRulesEditor'
import * as api from '../api/client'

vi.mock('../api/client')

const mockRules = [
  {
    id: 'rule-1',
    agent_profile_id: 'prof-1',
    event_type: 'task_completed',
    action: 'create_session',
    priority: 0,
    enabled: true,
    created_at: '2025-01-01T00:00:00Z',
  },
]

describe('TriggerRulesEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.fetchRulesByProfile).mockResolvedValue(mockRules)
    vi.mocked(api.createRule).mockResolvedValue(mockRules[0])
    vi.mocked(api.updateRule).mockResolvedValue(mockRules[0])
    vi.mocked(api.deleteRule).mockResolvedValue(undefined)
  })

  it('renders the component and loads rules', async () => {
    render(<TriggerRulesEditor profileId="prof-1" />)
    
    await waitFor(() => {
      expect(screen.getByText('Trigger Rules')).toBeInTheDocument()
    })
    
    await waitFor(() => {
      expect(screen.getByText('task_completed')).toBeInTheDocument()
    })
  })

  it('shows add form elements', async () => {
    render(<TriggerRulesEditor profileId="prof-1" />)
    
    await waitFor(() => {
      expect(screen.getByPlaceholderText(/event_type/)).toBeInTheDocument()
      expect(screen.getByText('Add')).toBeInTheDocument()
    })
  })

  it('shows empty state when no rules exist', async () => {
    vi.mocked(api.fetchRulesByProfile).mockResolvedValue([])
    render(<TriggerRulesEditor profileId="prof-1" />)
    
    await waitFor(() => {
      expect(screen.getByText('No trigger rules configured. Add one below.')).toBeInTheDocument()
    })
  })

  it('calls createRule when Add button is clicked', async () => {
    const user = userEvent.setup()
    vi.mocked(api.fetchRulesByProfile).mockResolvedValue([])
    render(<TriggerRulesEditor profileId="prof-1" />)
    
    await waitFor(() => {
      expect(screen.getByPlaceholderText(/event_type/)).toBeInTheDocument()
    })

    await user.type(screen.getByPlaceholderText(/event_type/), 'task_failed')
    await user.click(screen.getByText('Add'))

    await waitFor(() => {
      expect(api.createRule).toHaveBeenCalledWith('prof-1', {
        event_type: 'task_failed',
        action: 'create_session',
        priority: 0,
        enabled: true,
      })
    })
  })

  it('calls deleteRule when Delete is clicked and confirmed', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<TriggerRulesEditor profileId="prof-1" />)
    
    await waitFor(() => {
      expect(screen.getByText('task_completed')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Delete'))

    await waitFor(() => {
      expect(api.deleteRule).toHaveBeenCalledWith('prof-1', 'rule-1')
    })
  })

  it('does not call deleteRule when confirmation is denied', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<TriggerRulesEditor profileId="prof-1" />)
    
    await waitFor(() => {
      expect(screen.getByText('task_completed')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Delete'))

    expect(api.deleteRule).not.toHaveBeenCalled()
  })

  it('shows loading state initially', () => {
    vi.mocked(api.fetchRulesByProfile).mockImplementation(
      () => new Promise(() => {}) // never resolves
    )
    render(<TriggerRulesEditor profileId="prof-1" />)
    expect(screen.getByText('Loading trigger rules...')).toBeInTheDocument()
  })

  it('shows error when fetch fails', async () => {
    vi.mocked(api.fetchRulesByProfile).mockRejectedValue(new Error('Network error'))
    render(<TriggerRulesEditor profileId="prof-1" />)
    
    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })
})
