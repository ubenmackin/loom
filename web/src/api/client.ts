import type {
  Story,
  StoryWithTasks,
  Task,
  Session,
  Comment,
  ActivityLogEntry,
  PromptTemplate,
  StoryFilter,
  TaskFilter,
  WorkComplete,
  WorkBlock,
  BoardState,
  User,
  AuthResponse,
} from '../types'
import { useAuthStore } from '../stores/auth'

const BASE_URL = import.meta.env.VITE_API_URL || '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem('loom_auth_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: { ...headers, ...options?.headers },
  })

  if (res.status === 401) {
    useAuthStore.getState().logout()
    window.location.href = '/login'
    throw new Error('Unauthorized')
  }

  if (res.status === 403) {
    console.error('Forbidden')
    window.location.href = '/'
    throw new Error('Forbidden')
  }

  if (res.status === 204) return undefined as T;

  if (!res.ok) {
    const body = await res.text().catch(() => res.statusText)
    throw new Error(`API ${res.status}: ${body}`)
  }

  return res.json()
}

// ── Board ───────────────────────────────────────────────────────────────

export async function fetchBoard(): Promise<BoardState> {
  const data = await request<BoardState>('/board')
  return data
}

// ── Stories ─────────────────────────────────────────────────────────────

export async function fetchStories(filter?: StoryFilter): Promise<Story[]> {
  const params = new URLSearchParams()
  if (filter?.status) params.set('status', filter.status)
  if (filter?.assigned_to) params.set('assigned_to', filter.assigned_to)
  const qs = params.toString()
  return request(`/stories${qs ? `?${qs}` : ''}`)
}

export async function fetchStory(id: string): Promise<StoryWithTasks> {
  return request(`/stories/${id}`)
}

export async function createStory(data: Partial<Story>): Promise<Story> {
  return request('/stories', { method: 'POST', body: JSON.stringify(data) })
}

export async function updateStory(id: string, data: Partial<Story>): Promise<Story> {
  return request(`/stories/${id}`, { method: 'PUT', body: JSON.stringify(data) })
}

export async function updateStoryStatus(id: string, status: string): Promise<Story> {
  return request(`/stories/${id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) })
}

export async function deleteStory(id: string): Promise<void> {
  await request(`/stories/${id}`, { method: 'DELETE' })
}

// ── Tasks ───────────────────────────────────────────────────────────────

export async function fetchTasks(filter?: TaskFilter): Promise<Task[]> {
  const params = new URLSearchParams()
  if (filter?.status) params.set('status', filter.status)
  if (filter?.story_id) params.set('story_id', filter.story_id)
  if (filter?.task_type) params.set('task_type', filter.task_type)
  if (filter?.assigned_to) params.set('assigned_to', filter.assigned_to)
  const qs = params.toString()
  return request(`/tasks${qs ? `?${qs}` : ''}`)
}

/** Response shape of GET /api/tasks/{id} */
export interface TaskDetailResponse {
  task: Task
  dependencies: string[]
  dependents: Task[]
}

export async function fetchTask(id: string): Promise<TaskDetailResponse> {
  return request(`/tasks/${id}`)
}

export async function createTask(storyId: string, data: Partial<Task>): Promise<Task> {
  return request(`/stories/${storyId}/tasks`, { method: 'POST', body: JSON.stringify(data) })
}

export async function updateTask(id: string, data: Partial<Task>): Promise<Task> {
  return request(`/tasks/${id}`, { method: 'PUT', body: JSON.stringify(data) })
}

export async function updateTaskStatus(id: string, status: string): Promise<Task> {
  return request(`/tasks/${id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) })
}

export async function fetchBlockers(id: string): Promise<Task[]> {
  return request(`/tasks/${id}/blockers`)
}

export async function addDependency(id: string, depId: string): Promise<void> {
  await request(`/tasks/${id}/dependencies`, {
    method: 'POST',
    body: JSON.stringify({ depends_on_task_id: depId }),
  })
}

export async function removeDependency(id: string, depId: string): Promise<void> {
  await request(`/tasks/${id}/dependencies/${depId}`, { method: 'DELETE' })
}

export async function deleteTask(id: string): Promise<void> {
  await request(`/tasks/${id}`, { method: 'DELETE' })
}

export async function fetchActivity(id: string): Promise<ActivityLogEntry[]> {
  return request(`/tasks/${id}/activity`)
}

// ── Global Activity ───────────────────────────────────────────────────

export async function fetchActivityLog(limit?: number): Promise<ActivityLogEntry[]> {
  const params = new URLSearchParams()
  if (limit !== undefined) params.set('limit', String(limit))
  const qs = params.toString()
  return request(`/activity${qs ? `?${qs}` : ''}`)
}

// ── Work Protocol ───────────────────────────────────────────────────────

export async function requestWork(sessionId: string): Promise<Task> {
  return request('/work/request', {
    method: 'POST',
    body: JSON.stringify({ session_id: sessionId }),
  })
}

export async function startWork(sessionId: string, taskId: string): Promise<Task> {
  return request('/work/start', {
    method: 'POST',
    body: JSON.stringify({ session_id: sessionId, task_id: taskId }),
  })
}

export async function completeWork(
  sessionId: string,
  taskId: string,
  result: WorkComplete,
): Promise<void> {
  await request('/work/complete', {
    method: 'POST',
    body: JSON.stringify({ session_id: sessionId, task_id: taskId, result: result.result }),
  })
}

export async function blockWork(
  sessionId: string,
  taskId: string,
  reason: WorkBlock,
): Promise<void> {
  await request('/work/block', {
    method: 'POST',
    body: JSON.stringify({ session_id: sessionId, task_id: taskId, reason: reason.reason }),
  })
}

export async function keepAlive(sessionId: string): Promise<void> {
  await request('/work/keepalive', {
    method: 'POST',
    body: JSON.stringify({ session_id: sessionId }),
  })
}

// ── Sessions ────────────────────────────────────────────────────────────

export async function registerSession(data: {
  id: string
  harness_type: string
  capabilities?: string
  metadata?: string
}): Promise<Session> {
  return request('/sessions/register', { method: 'POST', body: JSON.stringify(data) })
}

export async function fetchSession(id: string): Promise<Session> {
  return request(`/sessions/${id}`)
}

export async function disconnectSession(id: string): Promise<void> {
  await request(`/sessions/${id}`, { method: 'DELETE' })
}

export async function fetchSessionTasks(id: string): Promise<Task[]> {
  return request(`/sessions/${id}/tasks`)
}

export async function fetchUnreadComments(id: string): Promise<Comment[]> {
  return request(`/sessions/${id}/unread-comments`)
}

export async function fetchSessions(): Promise<Session[]> {
  return request('/sessions')
}

// ── Comments ────────────────────────────────────────────────────────────

export async function fetchComments(
  workItemId: string,
  type: string,
): Promise<Comment[]> {
  return request(`/work-items/${workItemId}/comments?type=${type}`)
}

export async function addComment(
  workItemId: string,
  type: string,
  data: { body: string; author_id: string; author_type: string },
): Promise<Comment> {
  return request(`/work-items/${workItemId}/comments`, {
    method: 'POST',
    body: JSON.stringify({ ...data, work_item_type: type }),
  })
}

export async function updateComment(
  id: string,
  data: Partial<Pick<Comment, 'body'>>,
): Promise<Comment> {
  return request(`/work-items/comments/${id}`, { method: 'PUT', body: JSON.stringify(data) })
}

export async function deleteComment(id: string): Promise<void> {
  await request(`/work-items/comments/${id}`, { method: 'DELETE' })
}

// ── Templates ───────────────────────────────────────────────────────────

export async function fetchTemplates(): Promise<PromptTemplate[]> {
  return request('/templates')
}

export async function fetchTemplate(taskType: string): Promise<PromptTemplate> {
  return request(`/templates/${taskType}`)
}

export async function upsertTemplate(
  taskType: string,
  data: { template: string },
): Promise<PromptTemplate> {
  return request(`/templates/${taskType}`, { method: 'PUT', body: JSON.stringify(data) })
}

// ── Auth ────────────────────────────────────────────────────────────────

export async function login(data: { username_or_email: string; password: string }): Promise<AuthResponse> {
  return request('/auth/login', { method: 'POST', body: JSON.stringify(data) })
}

export async function signup(data: { username: string; email: string; password: string; display_name?: string }): Promise<AuthResponse> {
  return request('/auth/signup', { method: 'POST', body: JSON.stringify(data) })
}

export async function postLogout(): Promise<void> {
  await request('/auth/logout', { method: 'POST' })
  useAuthStore.getState().logout()
}

export async function getMe(): Promise<{ user: User }> {
  return request('/auth/me')
}

export async function getOnboardingCheck(): Promise<{ onboarding_required: boolean }> {
  return request('/auth/onboarding-check')
}

// ── Users ────────────────────────────────────────────────────────────────

export async function getUsers(): Promise<User[]> {
  return request<User[]>('/users')
}

export async function postUser(data: { username: string; email: string; display_name: string; password: string; role: 'admin' | 'normal' }): Promise<User> {
  return request<User>('/users', { method: 'POST', body: JSON.stringify(data) })
}

export async function deleteUser(id: string): Promise<void> {
  await request(`/users/${id}`, { method: 'DELETE' })
}