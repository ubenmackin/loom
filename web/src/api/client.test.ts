import { describe, it, expect, beforeAll, afterAll, afterEach, vi } from 'vitest'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import {
  login,
  signup,
  postLogout,
  getMe,
  getOnboardingCheck,
  fetchBoard,
  fetchStories,
  fetchStory,
  createStory,
  updateStory,
  batchReorderStories,
  updateStoryStatus,
  deleteStory,
  fetchTasks,
  fetchTask,
  createTask,
  updateTask,
  updateTaskStatus,
  batchReorderTasks,
  fetchBlockers,
  addDependency,
  removeDependency,
  deleteTask,
  fetchActivity,
  fetchActivityLog,
  registerSession,
  fetchSession,
  disconnectSession,
  fetchSessionTasks,
  fetchUnreadComments,
  fetchSessions,
  requestWork,
  startWork,
  completeWork,
  blockWork,
  keepAlive,
  fetchComments,
  addComment,
  updateComment,
  deleteComment,
  fetchTemplates,
  fetchTemplate,
  upsertTemplate,
  getUsers,
  postUser,
  deleteUser,
  fetchProfiles,
  fetchProfile,
  createProfile,
  updateProfile,
  deleteProfile,
  fetchRulesByProfile,
  createRule,
  updateRule,
  deleteRule,
} from './client'
import { useAuthStore } from '../stores/auth'
import type { User, AuthResponse, Story, StoryWithTasks, Task, BoardState, TaskDetailResponse, Session, Comment, ActivityLogEntry, PromptTemplate, UserRoleType } from '../types'
import type { AgentProfile, TriggerRule } from './client'

// ── Fixtures ─────────────────────────────────────────────────────────────────

const normalUser: User = {
  id: 'user-1',
  username: 'alice',
  email: 'alice@example.com',
  display_name: 'Alice',
  role: 'normal',
  created_at: '2025-01-01T00:00:00Z',
}

const adminUser: User = {
  id: 'admin-1',
  username: 'bob',
  email: 'bob@example.com',
  display_name: 'Bob',
  role: 'admin',
  created_at: '2025-01-01T00:00:00Z',
}

const sampleStory: Story = {
  id: 'story-1',
  title: 'Test Story',
  status: 'new',
  requires_build: false,
  requires_review: false,
  sort_order: 1,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const sampleStoryWithTasks: StoryWithTasks = {
  story: sampleStory,
  tasks: [],
}

const sampleTask: Task = {
  id: 'task-1',
  story_id: 'story-1',
  title: 'Test Task',
  status: 'new',
  task_type: 'code',
  sort_order: 1,
  is_stale: false,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const sampleBoardState: BoardState = {
  stories: [sampleStory],
  tasks_by_status: { new: [sampleTask] },
  stats: {
    total_stories: 1,
    total_tasks: 1,
    ready_tasks: 0,
    in_progress_tasks: 0,
    blocked_tasks: 0,
    done_tasks: 0,
    canceled_tasks: 0,
    archived_tasks: 0,
    stale_tasks: 0,
  },
}

const sampleProfile: AgentProfile = {
  id: 'profile-1',
  name: 'Default Agent',
  description: 'Default agent profile',
  capabilities: '["code","review"]',
  max_concurrency: 3,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const sampleProfile2: AgentProfile = {
  id: 'profile-2',
  name: 'Build Agent',
  description: 'Handles build tasks',
  capabilities: '["build","test"]',
  max_concurrency: 2,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const sampleRule: TriggerRule = {
  id: 'rule-1',
  agent_profile_id: 'profile-1',
  event_type: 'story.created',
  action: 'assign',
  priority: 10,
  enabled: true,
  created_at: '2025-01-01T00:00:00Z',
}

// ── MSW Server ──────────────────────────────────────────────────────────────

const handlers = [
  http.get('/api/auth/me', () => {
    return HttpResponse.json({ user: normalUser })
  }),

  http.post('/api/auth/login', async ({ request }) => {
    await request.json() as {
      username_or_email: string
      password: string
    }
    return HttpResponse.json({
      user: normalUser,
      token: 'login-token-abc',
    } satisfies AuthResponse)
  }),

  http.post('/api/auth/signup', async ({ request }) => {
    const body = (await request.json()) as {
      username: string
      email: string
      password: string
      display_name?: string
    }
    return HttpResponse.json({
      user: {
        ...normalUser,
        username: body.username,
        email: body.email,
      },
      token: 'signup-token-xyz',
    } satisfies AuthResponse)
  }),

  http.post('/api/auth/logout', () => {
    return new HttpResponse(null, { status: 204 })
  }),

  http.get('/api/auth/onboarding-check', () => {
    return HttpResponse.json({ onboarding_required: true })
  }),

  // ── Board ──────────────────────────────────────────────────────────────
  http.get('/api/board', () => {
    return HttpResponse.json(sampleBoardState)
  }),

  // ── Stories ────────────────────────────────────────────────────────────
  http.get('/api/stories', ({ request }) => {
    const url = new URL(request.url)
    const status = url.searchParams.get('status')
    const assignedTo = url.searchParams.get('assigned_to')
    let stories = [sampleStory]
    if (status) {
      stories = stories.filter((s) => s.status === status)
    }
    if (assignedTo) {
      stories = stories.filter((s) => s.assigned_to === assignedTo)
    }
    return HttpResponse.json(stories)
  }),

  http.get('/api/stories/:id', () => {
    return HttpResponse.json(sampleStoryWithTasks)
  }),

  http.post('/api/stories', async ({ request }) => {
    const body = (await request.json()) as Partial<Story>
    return HttpResponse.json({ ...sampleStory, ...body, id: 'story-new' })
  }),

  http.put('/api/stories/:id', async ({ request }) => {
    const body = (await request.json()) as Partial<Story>
    return HttpResponse.json({ ...sampleStory, ...body })
  }),

  http.patch('/api/stories/reorder', async ({ request }) => {
    const body = (await request.json()) as { stories: { id: string; sort_order: number }[] }
    return HttpResponse.json({ updated: body.stories.length })
  }),

  http.patch('/api/stories/:id/status', async ({ request }) => {
    const body = (await request.json()) as { status: string }
    return HttpResponse.json({ ...sampleStory, status: body.status })
  }),

  http.delete('/api/stories/:id', () => {
    return new HttpResponse(null, { status: 204 })
  }),

  // ── Tasks ──────────────────────────────────────────────────────────────
  http.get('/api/tasks', ({ request }) => {
    const url = new URL(request.url)
    const status = url.searchParams.get('status')
    const storyId = url.searchParams.get('story_id')
    const taskType = url.searchParams.get('task_type')
    const assignedTo = url.searchParams.get('assigned_to')
    let tasks = [sampleTask]
    if (status) tasks = tasks.filter((t) => t.status === status)
    if (storyId) tasks = tasks.filter((t) => t.story_id === storyId)
    if (taskType) tasks = tasks.filter((t) => t.task_type === taskType)
    if (assignedTo) tasks = tasks.filter((t) => t.assigned_to === assignedTo)
    return HttpResponse.json(tasks)
  }),

  http.get('/api/tasks/:id', () => {
    return HttpResponse.json({
      task: sampleTask,
      dependencies: [],
      dependents: [],
    } satisfies TaskDetailResponse)
  }),

  http.post('/api/stories/:storyId/tasks', async ({ request }) => {
    const body = (await request.json()) as Partial<Task>
    return HttpResponse.json({ ...sampleTask, ...body, id: 'task-new' })
  }),

  http.put('/api/tasks/:id', async ({ request }) => {
    const body = (await request.json()) as Partial<Task>
    return HttpResponse.json({ ...sampleTask, ...body })
  }),

  http.patch('/api/tasks/:id/status', async ({ request }) => {
    const body = (await request.json()) as { status: string }
    return HttpResponse.json({ ...sampleTask, status: body.status })
  }),

  http.patch('/api/tasks/reorder', async ({ request }) => {
    const body = (await request.json()) as { tasks: { id: string; sort_order: number }[] }
    return HttpResponse.json({ updated: body.tasks.length })
  }),

  // ── Dependencies ─────────────────────────────────────────────────────────
  http.get('/api/tasks/:id/blockers', () => {
    return HttpResponse.json([sampleTask])
  }),

  http.post('/api/tasks/:id/dependencies', async ({ request }) => {
    const body = (await request.json()) as { depends_on_task_id: string }
    return HttpResponse.json({ success: true, depends_on_task_id: body.depends_on_task_id })
  }),

  http.delete('/api/tasks/:id/dependencies/:depId', () => {
    return new HttpResponse(null, { status: 204 })
  }),

  http.delete('/api/tasks/:id', () => {
    return new HttpResponse(null, { status: 204 })
  }),

  // ── Activity ─────────────────────────────────────────────────────────────
  http.get('/api/tasks/:id/activity', () => {
    const entry: ActivityLogEntry = {
      id: 'act-1',
      work_item_id: 'task-1',
      work_item_type: 'task',
      action: 'status_change',
      details: 'changed from new to in_progress',
      created_at: '2025-01-01T00:00:00Z',
    }
    return HttpResponse.json([entry])
  }),

  http.get('/api/stories/:id/activity', () => {
    const entry: ActivityLogEntry = {
      id: 'act-2',
      work_item_id: 'story-1',
      work_item_type: 'story',
      action: 'created',
      created_at: '2025-01-01T00:00:00Z',
    }
    return HttpResponse.json([entry])
  }),

  http.get('/api/activity', ({ request }) => {
    const url = new URL(request.url)
    const limit = url.searchParams.get('limit')
    const entries: ActivityLogEntry[] = [
      {
        id: 'act-1',
        work_item_id: 'task-1',
        work_item_type: 'task',
        action: 'status_change',
        created_at: '2025-01-01T00:00:00Z',
      },
    ]
    if (limit) {
      return HttpResponse.json(entries.slice(0, Number(limit)))
    }
    return HttpResponse.json(entries)
  }),

  // ── Sessions ─────────────────────────────────────────────────────────────
  http.post('/api/sessions/register', async ({ request }) => {
    const body = (await request.json()) as { id: string; harness_type: string }
    return HttpResponse.json({
      id: body.id,
      harness_type: body.harness_type,
      last_seen_at: '2025-01-01T00:00:00Z',
      status: 'active',
      created_at: '2025-01-01T00:00:00Z',
    } satisfies Session)
  }),

  http.get('/api/sessions/:id', () => {
    return HttpResponse.json({
      id: 'session-1',
      harness_type: 'terminal',
      last_seen_at: '2025-01-01T00:00:00Z',
      status: 'active',
      created_at: '2025-01-01T00:00:00Z',
    } satisfies Session)
  }),

  http.delete('/api/sessions/:id', () => {
    return new HttpResponse(null, { status: 204 })
  }),

  http.get('/api/sessions/:id/tasks', () => {
    return HttpResponse.json([sampleTask])
  }),

  http.get('/api/sessions/:id/unread-comments', () => {
    const comment: Comment = {
      id: 'comment-1',
      work_item_id: 'task-1',
      work_item_type: 'task',
      author_id: 'user-1',
      author_type: 'human',
      body: 'Unread comment',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    }
    return HttpResponse.json([comment])
  }),

  http.get('/api/sessions', () => {
    return HttpResponse.json([
      {
        id: 'session-1',
        harness_type: 'terminal',
        last_seen_at: '2025-01-01T00:00:00Z',
        status: 'active',
        created_at: '2025-01-01T00:00:00Z',
      } satisfies Session,
    ])
  }),

  // ── Work Protocol ────────────────────────────────────────────────────────
  http.post('/api/work/request', async ({ request }) => {
    const body = (await request.json()) as { session_id: string }
    return HttpResponse.json({ ...sampleTask, assigned_to: body.session_id })
  }),

  http.post('/api/work/start', async ({ request }) => {
    const body = (await request.json()) as { session_id: string; task_id: string }
    return HttpResponse.json({ ...sampleTask, id: body.task_id, assigned_to: body.session_id, status: 'in_progress' })
  }),

  http.post('/api/work/complete', async ({ request }) => {
    await request.json() as { session_id: string; task_id: string; result: string }
    return new HttpResponse(null, { status: 204 })
  }),

  http.post('/api/work/block', async ({ request }) => {
    await request.json() as { session_id: string; task_id: string; reason: string }
    return new HttpResponse(null, { status: 204 })
  }),

  http.post('/api/work/keepalive', async ({ request }) => {
    await request.json() as { session_id: string }
    return new HttpResponse(null, { status: 204 })
  }),

  // ── Comments ─────────────────────────────────────────────────────────────
  http.get('/api/work-items/:workItemId/comments', ({ request }) => {
    const url = new URL(request.url)
    const type = url.searchParams.get('type')
    const comment: Comment = {
      id: 'comment-1',
      work_item_id: 'work-item-1',
      work_item_type: (type as 'task' | 'story') ?? 'task',
      author_id: 'user-1',
      author_type: 'human',
      body: 'Test comment',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    }
    return HttpResponse.json([comment])
  }),

  http.post('/api/work-items/:workItemId/comments', async ({ request }) => {
    const body = (await request.json()) as { body: string; author_id: string; author_type: string; work_item_type: string }
    return HttpResponse.json({
      id: 'comment-new',
      work_item_id: 'work-item-1',
      work_item_type: body.work_item_type as 'story' | 'task',
      author_id: body.author_id,
      author_type: body.author_type,
      body: body.body,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    } satisfies Comment)
  }),

  http.put('/api/work-items/comments/:id', async ({ request }) => {
    const body = (await request.json()) as { body: string }
    return HttpResponse.json({
      id: 'comment-1',
      work_item_id: 'work-item-1',
      work_item_type: 'task',
      author_id: 'user-1',
      author_type: 'human',
      body: body.body,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    } satisfies Comment)
  }),

  http.delete('/api/work-items/comments/:id', () => {
    return new HttpResponse(null, { status: 204 })
  }),

  // ── Templates ────────────────────────────────────────────────────────────
  http.get('/api/templates', () => {
    return HttpResponse.json([
      {
        id: 'tmpl-1',
        task_type: 'code',
        template: 'Write code for: {{title}}',
        created_at: '2025-01-01T00:00:00Z',
        updated_at: '2025-01-01T00:00:00Z',
      } satisfies PromptTemplate,
    ])
  }),

  http.get('/api/templates/:taskType', () => {
    return HttpResponse.json({
      id: 'tmpl-1',
      task_type: 'code',
      template: 'Write code for: {{title}}',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    } satisfies PromptTemplate)
  }),

  http.put('/api/templates/:taskType', async ({ request }) => {
    const body = (await request.json()) as { template: string }
    return HttpResponse.json({
      id: 'tmpl-1',
      task_type: 'code',
      template: body.template,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    } satisfies PromptTemplate)
  }),

  // ── Users ────────────────────────────────────────────────────────────────
  http.get('/api/users', () => {
    return HttpResponse.json([normalUser, adminUser])
  }),

  http.post('/api/users', async ({ request }) => {
    const body = (await request.json()) as { username: string; email: string; display_name: string; password: string; role: UserRoleType }
    return HttpResponse.json({
      id: 'user-new',
      username: body.username,
      email: body.email,
      display_name: body.display_name,
      role: body.role,
      created_at: '2025-01-01T00:00:00Z',
    } satisfies User)
  }),

  http.delete('/api/users/:id', () => {
    return new HttpResponse(null, { status: 204 })
  }),

  // ── Agent Profiles ──────────────────────────────────────────────────────
  http.get('/api/profiles', () => {
    return HttpResponse.json([sampleProfile, sampleProfile2])
  }),

  http.get('/api/profiles/:id', ({ params }) => {
    return HttpResponse.json(
      params.id === sampleProfile.id ? sampleProfile : sampleProfile2,
    )
  }),

  http.post('/api/profiles', async ({ request }) => {
    const body = (await request.json()) as {
      name: string
      description?: string
      capabilities?: string
      max_concurrency?: number
    }
    return HttpResponse.json({
      id: 'profile-new',
      name: body.name,
      description: body.description,
      capabilities: body.capabilities,
      max_concurrency: body.max_concurrency ?? 1,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    } satisfies AgentProfile)
  }),

  http.put('/api/profiles/:id', async ({ request }) => {
    const body = (await request.json()) as Partial<AgentProfile>
    return HttpResponse.json({ ...sampleProfile, ...body })
  }),

  http.delete('/api/profiles/:id', () => {
    return new HttpResponse(null, { status: 204 })
  }),

  // ── Trigger Rules ──────────────────────────────────────────────────────
  http.get('/api/profiles/:id/rules', () => {
    return HttpResponse.json([sampleRule])
  }),

  http.post('/api/profiles/:id/rules', async ({ request }) => {
    const body = (await request.json()) as {
      event_type: string
      action: string
      priority?: number
      enabled?: boolean
    }
    return HttpResponse.json({
      id: 'rule-new',
      agent_profile_id: 'profile-1',
      event_type: body.event_type,
      action: body.action,
      priority: body.priority ?? 5,
      enabled: body.enabled ?? true,
      created_at: '2025-01-01T00:00:00Z',
    } satisfies TriggerRule)
  }),

  http.put('/api/profiles/:id/rules/:ruleId', async ({ request }) => {
    const body = (await request.json()) as Partial<TriggerRule>
    return HttpResponse.json({ ...sampleRule, ...body })
  }),

  http.delete('/api/profiles/:id/rules/:ruleId', () => {
    return new HttpResponse(null, { status: 204 })
  }),
]

const server = setupServer(...handlers)

// ── Tests ───────────────────────────────────────────────────────────────────

describe('API Client', () => {
  const originalLocation = window.location

  beforeAll(() => {
    server.listen({ onUnhandledRequest: 'error' })
  })

  afterAll(() => {
    server.close()
  })

  afterEach(() => {
    server.resetHandlers()
    vi.restoreAllMocks()
    localStorage.clear()
    useAuthStore.setState({
      user: null,
      token: null,
      isAuthenticated: false,
    })

    // Restore window.location in case a test replaced it
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: originalLocation,
      writable: true,
    })
  })

  // ── request() ─────────────────────────────────────────────────────────────

  describe('request()', () => {
    it('successful GET returns parsed JSON', async () => {
      const result = await getMe()
      expect(result).toEqual({ user: normalUser })
    })

    it('POST with JSON body works', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/auth/login', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            user: normalUser,
            token: 'login-token-abc',
          })
        }),
      )

      const credentials = { username_or_email: 'alice', password: 'secret' }
      const result = await login(credentials)

      expect(capturedBody).toEqual(credentials)
      expect(result.token).toBe('login-token-abc')
    })

    it('Authorization header is attached when store has token', async () => {
      useAuthStore.setState({ token: 'my-secret-token' })

      let authHeader: string | null = null
      server.use(
        http.get('/api/auth/me', ({ request }) => {
          authHeader = request.headers.get('Authorization')
          return HttpResponse.json({ user: normalUser })
        }),
      )

      await getMe()
      expect(authHeader).toBe('Bearer my-secret-token')
    })

    it('401 triggers logout + redirect to /login and throws Error', async () => {
      const logoutSpy = vi.spyOn(useAuthStore.getState(), 'logout')

      // Replace window.location with a mock that has a valid base URL
      // so msw's fetch interceptor can resolve relative URLs
      Object.defineProperty(window, 'location', {
        configurable: true,
        value: { href: 'http://localhost' },
        writable: true,
      })

      server.use(
        http.get('/api/auth/me', () => new HttpResponse(null, { status: 401 })),
      )

      await expect(getMe()).rejects.toThrow('Unauthorized')
      expect(logoutSpy).toHaveBeenCalled()
      expect(window.location.href).toBe('/login')
    })

    it('403 triggers redirect to / and throws Error', async () => {
      Object.defineProperty(window, 'location', {
        configurable: true,
        value: { href: 'http://localhost' },
        writable: true,
      })

      server.use(
        http.get('/api/auth/me', () => new HttpResponse(null, { status: 403 })),
      )

      await expect(getMe()).rejects.toThrow('Forbidden')
      expect(window.location.href).toBe('/')
    })

    it('204 returns undefined', async () => {
      const result = await postLogout()
      expect(result).toBeUndefined()
    })

    it('Non-ok response throws Error with status text', async () => {
      server.use(
        http.get('/api/auth/me', () =>
          new HttpResponse('Not found', { status: 404 }),
        ),
      )

      await expect(getMe()).rejects.toThrow('API 404: Not found')
    })

    it('Network error rejects', async () => {
      server.use(
        http.get('/api/auth/me', () => HttpResponse.error()),
      )

      await expect(getMe()).rejects.toThrow()
    })
  })

  // ── Auth endpoints ────────────────────────────────────────────────────────

  describe('Auth endpoints', () => {
    it('login() posts to /auth/login and returns AuthResponse', async () => {
      const result = await login({
        username_or_email: 'alice',
        password: 'secret',
      })

      expect(result).toHaveProperty('user')
      expect(result).toHaveProperty('token')
      expect(result.token).toBe('login-token-abc')
      expect(result.user).toMatchObject({
        username: 'alice',
        email: 'alice@example.com',
      })
    })

    it('signup() posts to /auth/signup with user data', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/auth/signup', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            user: {
              ...normalUser,
              username: 'charlie',
              email: 'charlie@test.com',
            },
            token: 'signup-token-xyz',
          })
        }),
      )

      const data = {
        username: 'charlie',
        email: 'charlie@test.com',
        password: 'strong-pass',
        display_name: 'Charlie',
      }
      const result = await signup(data)

      expect(capturedBody).toEqual(data)
      expect(result.token).toBe('signup-token-xyz')
      expect(result.user.username).toBe('charlie')
    })

    it('postLogout() posts to /auth/logout and calls logout()', async () => {
      const logoutSpy = vi.spyOn(useAuthStore.getState(), 'logout')

      await postLogout()

      expect(logoutSpy).toHaveBeenCalled()
    })

    it('getMe() fetches /auth/me and returns user', async () => {
      const result = await getMe()

      expect(result).toEqual({ user: normalUser })
    })

    it('getOnboardingCheck() fetches /auth/onboarding-check', async () => {
      const result = await getOnboardingCheck()

      expect(result).toEqual({ onboarding_required: true })
    })
  })

  // ── Board ─────────────────────────────────────────────────────────────

  describe('Board endpoints', () => {
    it('fetchBoard() fetches /board and returns BoardState', async () => {
      const result = await fetchBoard()

      expect(result).toHaveProperty('stories')
      expect(result).toHaveProperty('tasks_by_status')
      expect(result).toHaveProperty('stats')
      expect(result.stories).toHaveLength(1)
      expect(result.stories[0].id).toBe('story-1')
    })
  })

  // ── Stories ───────────────────────────────────────────────────────────

  describe('Story endpoints', () => {
    it('fetchStories() fetches /stories with no filters', async () => {
      const result = await fetchStories()

      expect(result).toHaveLength(1)
      expect(result[0].title).toBe('Test Story')
    })

    it('fetchStories() passes filter query params', async () => {
      let capturedUrl: string | null = null
      server.use(
        http.get('/api/stories', ({ request }) => {
          capturedUrl = request.url
          return HttpResponse.json([sampleStory])
        }),
      )

      await fetchStories({ status: 'in_progress', assigned_to: 'user-1' })

      expect(capturedUrl).toContain('status=in_progress')
      expect(capturedUrl).toContain('assigned_to=user-1')
    })

    it('fetchStory(id) fetches /stories/{id} and returns StoryWithTasks', async () => {
      const result = await fetchStory('story-1')

      expect(result).toHaveProperty('story')
      expect(result).toHaveProperty('tasks')
      expect(result.story.id).toBe('story-1')
    })

    it('createStory(data) posts to /stories with JSON body', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/stories', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({ ...sampleStory, ...(capturedBody as Partial<Story>), id: 'story-new' })
        }),
      )

      const data = { title: 'New Story', status: 'new' as const }
      const result = await createStory(data)

      expect(capturedBody).toEqual(data)
      expect(result.id).toBe('story-new')
      expect(result.title).toBe('New Story')
    })

    it('updateStory(id, data) puts to /stories/{id} with JSON body', async () => {
      let capturedBody: unknown
      server.use(
        http.put('/api/stories/:id', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({ ...sampleStory, ...(capturedBody as Partial<Story>) })
        }),
      )

      const data = { title: 'Updated Title' }
      const result = await updateStory('story-1', data)

      expect(capturedBody).toEqual(data)
      expect(result.title).toBe('Updated Title')
    })

    it('batchReorderStories(stories) patches /stories/reorder', async () => {
      let capturedBody: unknown
      server.use(
        http.patch('/api/stories/reorder', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({ updated: 2 })
        }),
      )

      const reorder = [
        { id: 'story-1', sort_order: 1 },
        { id: 'story-2', sort_order: 2 },
      ]
      const result = await batchReorderStories(reorder)

      expect(capturedBody).toEqual({ stories: reorder })
      expect(result.updated).toBe(2)
    })

    it('updateStoryStatus(id, status) patches /stories/{id}/status', async () => {
      let capturedBody: unknown
      server.use(
        http.patch('/api/stories/:id/status', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({ ...sampleStory, status: (capturedBody as { status: string }).status })
        }),
      )

      const result = await updateStoryStatus('story-1', 'in_progress')

      expect(capturedBody).toEqual({ status: 'in_progress' })
      expect(result.status).toBe('in_progress')
    })

    it('deleteStory(id) deletes /stories/{id}', async () => {
      let methodSeen: string | undefined
      server.use(
        http.delete('/api/stories/:id', ({ request }) => {
          methodSeen = request.method
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await deleteStory('story-1')

      expect(methodSeen).toBe('DELETE')
    })
  })

  // ── Tasks ─────────────────────────────────────────────────────────────

  describe('Task endpoints', () => {
    it('fetchTasks() fetches /tasks with no filters', async () => {
      const result = await fetchTasks()

      expect(result).toHaveLength(1)
      expect(result[0].id).toBe('task-1')
    })

    it('fetchTasks() passes filter query params', async () => {
      let capturedUrl: string | null = null
      server.use(
        http.get('/api/tasks', ({ request }) => {
          capturedUrl = request.url
          return HttpResponse.json([sampleTask])
        }),
      )

      await fetchTasks({
        status: 'in_progress',
        story_id: 'story-1',
        task_type: 'code',
        assigned_to: 'user-1',
      })

      expect(capturedUrl).toContain('status=in_progress')
      expect(capturedUrl).toContain('story_id=story-1')
      expect(capturedUrl).toContain('task_type=code')
      expect(capturedUrl).toContain('assigned_to=user-1')
    })

    it('fetchTask(id) fetches /tasks/{id} and returns TaskDetailResponse', async () => {
      const result = await fetchTask('task-1')

      expect(result).toHaveProperty('task')
      expect(result).toHaveProperty('dependencies')
      expect(result).toHaveProperty('dependents')
      expect(result.task.id).toBe('task-1')
    })

    it('createTask(storyId, data) posts to /stories/{storyId}/tasks', async () => {
      let capturedBody: unknown
      let capturedStoryId: string | undefined
      server.use(
        http.post('/api/stories/:storyId/tasks', async ({ request, params }) => {
          capturedBody = await request.json()
          capturedStoryId = params.storyId as string
          return HttpResponse.json({ ...sampleTask, ...(capturedBody as Partial<Task>), id: 'task-new' })
        }),
      )

      const data = { title: 'New Task', task_type: 'code' as const }
      const result = await createTask('story-1', data)

      expect(capturedBody).toEqual(data)
      expect(capturedStoryId).toBe('story-1')
      expect(result.id).toBe('task-new')
      expect(result.title).toBe('New Task')
    })

    it('updateTask(id, data) puts to /tasks/{id} with JSON body', async () => {
      let capturedBody: unknown
      server.use(
        http.put('/api/tasks/:id', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({ ...sampleTask, ...(capturedBody as Partial<Task>) })
        }),
      )

      const data = { title: 'Updated Task' }
      const result = await updateTask('task-1', data)

      expect(capturedBody).toEqual(data)
      expect(result.title).toBe('Updated Task')
    })

    it('updateTaskStatus(id, status) patches /tasks/{id}/status', async () => {
      let capturedBody: unknown
      server.use(
        http.patch('/api/tasks/:id/status', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({ ...sampleTask, status: (capturedBody as { status: string }).status })
        }),
      )

      const result = await updateTaskStatus('task-1', 'done')

      expect(capturedBody).toEqual({ status: 'done' })
      expect(result.status).toBe('done')
    })

    it('batchReorderTasks(tasks) patches /tasks/reorder', async () => {
      let capturedBody: unknown
      server.use(
        http.patch('/api/tasks/reorder', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({ updated: 3 })
        }),
      )

      const reorder = [
        { id: 'task-1', sort_order: 1 },
        { id: 'task-2', sort_order: 2 },
        { id: 'task-3', sort_order: 3 },
      ]
      const result = await batchReorderTasks(reorder)

      expect(capturedBody).toEqual({ tasks: reorder })
      expect(result.updated).toBe(3)
    })
  })

  // ── Dependencies ──────────────────────────────────────────────────────

  describe('Dependency endpoints', () => {
    it('fetchBlockers(id) fetches /tasks/{id}/blockers', async () => {
      const result = await fetchBlockers('task-1')

      expect(result).toHaveLength(1)
      expect(result[0].id).toBe('task-1')
    })

    it('addDependency(id, depId) posts to /tasks/{id}/dependencies', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/tasks/:id/dependencies', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({ success: true })
        }),
      )

      await addDependency('task-1', 'task-2')

      expect(capturedBody).toEqual({ depends_on_task_id: 'task-2' })
    })

    it('removeDependency(id, depId) deletes /tasks/{id}/dependencies/{depId}', async () => {
      let methodSeen: string | undefined
      let capturedDepId: string | undefined
      server.use(
        http.delete('/api/tasks/:id/dependencies/:depId', ({ request, params }) => {
          methodSeen = request.method
          capturedDepId = params.depId as string
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await removeDependency('task-1', 'dep-2')

      expect(methodSeen).toBe('DELETE')
      expect(capturedDepId).toBe('dep-2')
    })

    it('deleteTask(id) deletes /tasks/{id}', async () => {
      let methodSeen: string | undefined
      server.use(
        http.delete('/api/tasks/:id', ({ request }) => {
          methodSeen = request.method
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await deleteTask('task-1')

      expect(methodSeen).toBe('DELETE')
    })
  })

  // ── Activity ─────────────────────────────────────────────────────────

  describe('Activity endpoints', () => {
    it('fetchActivity(id, "task") fetches /tasks/{id}/activity', async () => {
      const result = await fetchActivity('task-1', 'task')

      expect(result).toHaveLength(1)
      expect(result[0].work_item_id).toBe('task-1')
      expect(result[0].work_item_type).toBe('task')
    })

    it('fetchActivity(id, "story") fetches /stories/{id}/activity', async () => {
      const result = await fetchActivity('story-1', 'story')

      expect(result).toHaveLength(1)
      expect(result[0].work_item_id).toBe('story-1')
      expect(result[0].work_item_type).toBe('story')
    })

    it('fetchActivityLog() fetches /activity with no limit', async () => {
      const result = await fetchActivityLog()

      expect(result).toHaveLength(1)
      expect(result[0].action).toBe('status_change')
    })

    it('fetchActivityLog(limit) passes limit query param', async () => {
      let capturedUrl: string | null = null
      server.use(
        http.get('/api/activity', ({ request }) => {
          capturedUrl = request.url
          return HttpResponse.json([])
        }),
      )

      await fetchActivityLog(5)

      expect(capturedUrl).toContain('limit=5')
    })
  })

  // ── Sessions ─────────────────────────────────────────────────────────

  describe('Session endpoints', () => {
    it('registerSession(data) posts to /sessions/register', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/sessions/register', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            id: 'session-new',
            harness_type: 'terminal',
            last_seen_at: '2025-01-01T00:00:00Z',
            status: 'active',
            created_at: '2025-01-01T00:00:00Z',
          } satisfies Session)
        }),
      )

      const data = { id: 'session-new', harness_type: 'terminal' }
      const result = await registerSession(data)

      expect(capturedBody).toEqual(data)
      expect(result.id).toBe('session-new')
      expect(result.status).toBe('active')
    })

    it('fetchSession(id) fetches /sessions/{id}', async () => {
      const result = await fetchSession('session-1')

      expect(result.id).toBe('session-1')
      expect(result.harness_type).toBe('terminal')
      expect(result.status).toBe('active')
    })

    it('disconnectSession(id) deletes /sessions/{id}', async () => {
      let methodSeen: string | undefined
      server.use(
        http.delete('/api/sessions/:id', ({ request }) => {
          methodSeen = request.method
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await disconnectSession('session-1')

      expect(methodSeen).toBe('DELETE')
    })

    it('fetchSessionTasks(id) fetches /sessions/{id}/tasks', async () => {
      const result = await fetchSessionTasks('session-1')

      expect(result).toHaveLength(1)
      expect(result[0].id).toBe('task-1')
    })

    it('fetchUnreadComments(id) fetches /sessions/{id}/unread-comments', async () => {
      const result = await fetchUnreadComments('session-1')

      expect(result).toHaveLength(1)
      expect(result[0].id).toBe('comment-1')
      expect(result[0].body).toBe('Unread comment')
    })

    it('fetchSessions() fetches /sessions', async () => {
      const result = await fetchSessions()

      expect(result).toHaveLength(1)
      expect(result[0].id).toBe('session-1')
    })
  })

  // ── Work Protocol ────────────────────────────────────────────────────

  describe('Work protocol endpoints', () => {
    it('requestWork(sessionId) posts to /work/request', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/work/request', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({ ...sampleTask, assigned_to: (capturedBody as { session_id: string }).session_id })
        }),
      )

      const result = await requestWork('session-1')

      expect(capturedBody).toEqual({ session_id: 'session-1' })
      expect(result.assigned_to).toBe('session-1')
    })

    it('startWork(sessionId, taskId) posts to /work/start', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/work/start', async ({ request }) => {
          capturedBody = await request.json()
          const b = capturedBody as { session_id: string; task_id: string }
          return HttpResponse.json({ ...sampleTask, id: b.task_id, assigned_to: b.session_id, status: 'in_progress' })
        }),
      )

      const result = await startWork('session-1', 'task-2')

      expect(capturedBody).toEqual({ session_id: 'session-1', task_id: 'task-2' })
      expect(result.id).toBe('task-2')
      expect(result.status).toBe('in_progress')
    })

    it('completeWork(sessionId, taskId, result) posts to /work/complete', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/work/complete', async ({ request }) => {
          capturedBody = await request.json()
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await completeWork('session-1', 'task-2', 'Done')

      expect(capturedBody).toEqual({ session_id: 'session-1', task_id: 'task-2', result: 'Done' })
    })

    it('blockWork(sessionId, taskId, reason) posts to /work/block', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/work/block', async ({ request }) => {
          capturedBody = await request.json()
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await blockWork('session-1', 'task-2', 'Waiting for review')

      expect(capturedBody).toEqual({ session_id: 'session-1', task_id: 'task-2', reason: 'Waiting for review' })
    })

    it('keepAlive(sessionId) posts to /work/keepalive', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/work/keepalive', async ({ request }) => {
          capturedBody = await request.json()
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await keepAlive('session-1')

      expect(capturedBody).toEqual({ session_id: 'session-1' })
    })
  })

  // ── Comments ─────────────────────────────────────────────────────────

  describe('Comment endpoints', () => {
    it('fetchComments(workItemId, type) fetches /work-items/{id}/comments', async () => {
      let capturedUrl: string | null = null
      server.use(
        http.get('/api/work-items/:workItemId/comments', ({ request }) => {
          capturedUrl = request.url
          return HttpResponse.json([])
        }),
      )

      await fetchComments('work-item-1', 'task')

      expect(capturedUrl).toContain('type=task')
    })

    it('addComment(workItemId, type, data) posts to /work-items/{id}/comments', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/work-items/:workItemId/comments', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            id: 'comment-new',
            work_item_id: 'work-item-1',
            work_item_type: 'task',
            author_id: 'user-1',
            author_type: 'human',
            body: 'New comment',
            created_at: '2025-01-01T00:00:00Z',
            updated_at: '2025-01-01T00:00:00Z',
          } satisfies Comment)
        }),
      )

      const data = { body: 'New comment', author_id: 'user-1', author_type: 'human' }
      const result = await addComment('work-item-1', 'task', data)

      expect(capturedBody).toEqual({ ...data, work_item_type: 'task' })
      expect(result.body).toBe('New comment')
    })

    it('updateComment(id, data) puts to /work-items/comments/{id}', async () => {
      let capturedBody: unknown
      server.use(
        http.put('/api/work-items/comments/:id', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            id: 'comment-1',
            work_item_id: 'work-item-1',
            work_item_type: 'task',
            author_id: 'user-1',
            author_type: 'human',
            body: (capturedBody as { body: string }).body,
            created_at: '2025-01-01T00:00:00Z',
            updated_at: '2025-01-01T00:00:00Z',
          } satisfies Comment)
        }),
      )

      const result = await updateComment('comment-1', { body: 'Updated body' })

      expect(capturedBody).toEqual({ body: 'Updated body' })
      expect(result.body).toBe('Updated body')
    })

    it('deleteComment(id) deletes /work-items/comments/{id}', async () => {
      let methodSeen: string | undefined
      server.use(
        http.delete('/api/work-items/comments/:id', ({ request }) => {
          methodSeen = request.method
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await deleteComment('comment-1')

      expect(methodSeen).toBe('DELETE')
    })
  })

  // ── Templates ────────────────────────────────────────────────────────

  describe('Template endpoints', () => {
    it('fetchTemplates() fetches /templates', async () => {
      const result = await fetchTemplates()

      expect(result).toHaveLength(1)
      expect(result[0].task_type).toBe('code')
      expect(result[0].template).toContain('{{title}}')
    })

    it('fetchTemplate(taskType) fetches /templates/{taskType}', async () => {
      const result = await fetchTemplate('code')

      expect(result.task_type).toBe('code')
      expect(result.template).toContain('{{title}}')
    })

    it('upsertTemplate(taskType, data) puts to /templates/{taskType}', async () => {
      let capturedBody: unknown
      server.use(
        http.put('/api/templates/:taskType', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            id: 'tmpl-1',
            task_type: 'build',
            template: (capturedBody as { template: string }).template,
            created_at: '2025-01-01T00:00:00Z',
            updated_at: '2025-01-01T00:00:00Z',
          } satisfies PromptTemplate)
        }),
      )

      const data = { template: 'Build step: {{title}}' }
      const result = await upsertTemplate('build', data)

      expect(capturedBody).toEqual(data)
      expect(result.task_type).toBe('build')
      expect(result.template).toBe('Build step: {{title}}')
    })
  })

  // ── Users (admin) ────────────────────────────────────────────────────

  describe('User endpoints', () => {
    it('getUsers() fetches /users', async () => {
      const result = await getUsers()

      expect(result).toHaveLength(2)
      expect(result[0].username).toBe('alice')
      expect(result[1].username).toBe('bob')
    })

    it('postUser(data) posts to /users', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/users', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            id: 'user-new',
            username: (capturedBody as { username: string }).username,
            email: (capturedBody as { email: string }).email,
            display_name: (capturedBody as { display_name: string }).display_name,
            role: (capturedBody as { role: UserRoleType }).role,
            created_at: '2025-01-01T00:00:00Z',
          } satisfies User)
        }),
      )

      const data = {
        username: 'charlie',
        email: 'charlie@test.com',
        display_name: 'Charlie',
        password: 'secret',
        role: 'normal' as const,
      }
      const result = await postUser(data)

      expect(capturedBody).toEqual(data)
      expect(result.username).toBe('charlie')
      expect(result.role).toBe('normal')
    })

    it('deleteUser(id) deletes /users/{id}', async () => {
      let methodSeen: string | undefined
      server.use(
        http.delete('/api/users/:id', ({ request }) => {
          methodSeen = request.method
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await deleteUser('user-1')

      expect(methodSeen).toBe('DELETE')
    })
  })

  // ── Agent Profiles ──────────────────────────────────────────────────

  describe('Agent Profile endpoints', () => {
    it('fetchProfiles() fetches /profiles and returns all profiles', async () => {
      const result = await fetchProfiles()

      expect(result).toHaveLength(2)
      expect(result[0].id).toBe('profile-1')
      expect(result[0].name).toBe('Default Agent')
      expect(result[1].id).toBe('profile-2')
      expect(result[1].name).toBe('Build Agent')
    })

    it('fetchProfile(id) fetches /profiles/{id} and returns a profile', async () => {
      const result = await fetchProfile('profile-1')

      expect(result.id).toBe('profile-1')
      expect(result.name).toBe('Default Agent')
      expect(result.capabilities).toBe('["code","review"]')
      expect(result.max_concurrency).toBe(3)
    })

    it('fetchProfile(id) returns the correct profile for a different id', async () => {
      const result = await fetchProfile('profile-2')

      expect(result.id).toBe('profile-2')
      expect(result.name).toBe('Build Agent')
      expect(result.max_concurrency).toBe(2)
    })

    it('createProfile(data) posts to /profiles with JSON body', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/profiles', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            id: 'profile-new',
            name: (capturedBody as { name: string }).name,
            description: (capturedBody as { description?: string }).description,
            capabilities: (capturedBody as { capabilities?: string }).capabilities,
            max_concurrency: (capturedBody as { max_concurrency?: number }).max_concurrency ?? 1,
            created_at: '2025-01-01T00:00:00Z',
            updated_at: '2025-01-01T00:00:00Z',
          } satisfies AgentProfile)
        }),
      )

      const data = { name: 'New Agent', description: 'A new agent', capabilities: '["code"]', max_concurrency: 5 }
      const result = await createProfile(data)

      expect(capturedBody).toEqual(data)
      expect(result.id).toBe('profile-new')
      expect(result.name).toBe('New Agent')
      expect(result.max_concurrency).toBe(5)
    })

    it('createProfile(data) defaults max_concurrency when not provided', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/profiles', async ({ request }) => {
          capturedBody = await request.json()
          const body = capturedBody as { name: string; max_concurrency?: number }
          return HttpResponse.json({
            id: 'profile-min',
            name: body.name,
            max_concurrency: body.max_concurrency ?? 1,
            created_at: '2025-01-01T00:00:00Z',
            updated_at: '2025-01-01T00:00:00Z',
          } satisfies AgentProfile)
        }),
      )

      const result = await createProfile({ name: 'Minimal Agent' })

      expect(capturedBody).toEqual({ name: 'Minimal Agent' })
      expect(result.max_concurrency).toBe(1)
    })

    it('updateProfile(id, data) puts to /profiles/{id} with JSON body', async () => {
      let capturedBody: unknown
      server.use(
        http.put('/api/profiles/:id', async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({ ...sampleProfile, ...(capturedBody as Partial<AgentProfile>) })
        }),
      )

      const data = { name: 'Updated Agent', max_concurrency: 10 }
      const result = await updateProfile('profile-1', data)

      expect(capturedBody).toEqual(data)
      expect(result.name).toBe('Updated Agent')
      expect(result.max_concurrency).toBe(10)
      expect(result.id).toBe('profile-1')
    })

    it('deleteProfile(id) deletes /profiles/{id}', async () => {
      let methodSeen: string | undefined
      server.use(
        http.delete('/api/profiles/:id', ({ request }) => {
          methodSeen = request.method
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await deleteProfile('profile-1')

      expect(methodSeen).toBe('DELETE')
    })
  })

  // ── Trigger Rules ───────────────────────────────────────────────────

  describe('Trigger Rule endpoints', () => {
    it('fetchRulesByProfile(profileId) fetches /profiles/{id}/rules', async () => {
      const result = await fetchRulesByProfile('profile-1')

      expect(result).toHaveLength(1)
      expect(result[0].id).toBe('rule-1')
      expect(result[0].event_type).toBe('story.created')
      expect(result[0].action).toBe('assign')
      expect(result[0].priority).toBe(10)
      expect(result[0].enabled).toBe(true)
    })

    it('createRule(profileId, data) posts to /profiles/{id}/rules', async () => {
      let capturedBody: unknown
      let capturedProfileId: string | undefined
      server.use(
        http.post('/api/profiles/:id/rules', async ({ request, params }) => {
          capturedBody = await request.json()
          capturedProfileId = params.id as string
          const body = capturedBody as { event_type: string; action: string; priority?: number; enabled?: boolean }
          return HttpResponse.json({
            id: 'rule-new',
            agent_profile_id: capturedProfileId,
            event_type: body.event_type,
            action: body.action,
            priority: body.priority ?? 5,
            enabled: body.enabled ?? true,
            created_at: '2025-01-01T00:00:00Z',
          } satisfies TriggerRule)
        }),
      )

      const data = { event_type: 'task.created', action: 'notify', priority: 8, enabled: true }
      const result = await createRule('profile-1', data)

      expect(capturedBody).toEqual(data)
      expect(capturedProfileId).toBe('profile-1')
      expect(result.id).toBe('rule-new')
      expect(result.event_type).toBe('task.created')
      expect(result.action).toBe('notify')
      expect(result.priority).toBe(8)
      expect(result.enabled).toBe(true)
    })

    it('createRule(profileId, data) defaults priority and enabled', async () => {
      let capturedBody: unknown
      server.use(
        http.post('/api/profiles/:id/rules', async ({ request }) => {
          capturedBody = await request.json()
          const body = capturedBody as { event_type: string; action: string; priority?: number; enabled?: boolean }
          return HttpResponse.json({
            id: 'rule-default',
            agent_profile_id: 'profile-1',
            event_type: body.event_type,
            action: body.action,
            priority: body.priority ?? 5,
            enabled: body.enabled ?? true,
            created_at: '2025-01-01T00:00:00Z',
          } satisfies TriggerRule)
        }),
      )

      const result = await createRule('profile-1', { event_type: 'build.complete', action: 'deploy' })

      expect(capturedBody).toEqual({ event_type: 'build.complete', action: 'deploy' })
      expect(result.priority).toBe(5)
      expect(result.enabled).toBe(true)
    })

    it('updateRule(profileId, ruleId, data) puts to /profiles/{id}/rules/{ruleId}', async () => {
      let capturedBody: unknown
      let capturedProfileId: string | undefined
      let capturedRuleId: string | undefined
      server.use(
        http.put('/api/profiles/:id/rules/:ruleId', async ({ request, params }) => {
          capturedBody = await request.json()
          capturedProfileId = params.id as string
          capturedRuleId = params.ruleId as string
          return HttpResponse.json({ ...sampleRule, ...(capturedBody as Partial<TriggerRule>) })
        }),
      )

      const data = { priority: 20, enabled: false }
      const result = await updateRule('profile-1', 'rule-1', data)

      expect(capturedBody).toEqual(data)
      expect(capturedProfileId).toBe('profile-1')
      expect(capturedRuleId).toBe('rule-1')
      expect(result.priority).toBe(20)
      expect(result.enabled).toBe(false)
    })

    it('deleteRule(profileId, ruleId) deletes /profiles/{id}/rules/{ruleId}', async () => {
      let methodSeen: string | undefined
      let capturedProfileId: string | undefined
      let capturedRuleId: string | undefined
      server.use(
        http.delete('/api/profiles/:id/rules/:ruleId', ({ request, params }) => {
          methodSeen = request.method
          capturedProfileId = params.id as string
          capturedRuleId = params.ruleId as string
          return new HttpResponse(null, { status: 204 })
        }),
      )

      await deleteRule('profile-1', 'rule-1')

      expect(methodSeen).toBe('DELETE')
      expect(capturedProfileId).toBe('profile-1')
      expect(capturedRuleId).toBe('rule-1')
    })
  })
})
