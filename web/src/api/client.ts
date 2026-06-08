import type {
  Story,
  StoryWithTasks,
  Task,
  Project,
  Session,
  Comment,
  ActivityLogEntry,
  PromptTemplate,
  StoryFilter,
  TaskFilter,
  BoardState,
  DispatcherStatus,
  User,
  AuthResponse,
  TaskDetailResponse,
  StatusType,
  WorkItemTypeType,
  TaskTypeType,
  UserRoleType,
} from '../types'
import { useAuthStore } from '../stores/auth'

const BASE_URL = import.meta.env.VITE_API_URL || '/api'

/** Shared request helper with auth header, error handling, and AbortSignal support. */
async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = useAuthStore.getState().token
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

// ── DRY Helpers ─────────────────────────────────────────────────────────

async function updateResource<T>(path: string, data: Partial<T>): Promise<T> {
  return request<T>(path, { method: 'PUT', body: JSON.stringify(data) })
}

async function deleteResource(path: string): Promise<void> {
  await request(path, { method: 'DELETE' })
}

async function patchResourceStatus<T>(path: string, status: StatusType): Promise<T> {
  return request<T>(path, { method: 'PATCH', body: JSON.stringify({ status }) })
}

// ── Board ───────────────────────────────────────────────────────────────

export async function fetchBoard(projectId?: string): Promise<BoardState> {
  const path = projectId ? `/board?project_id=${encodeURIComponent(projectId)}` : '/board'
  return request<BoardState>(path)
}

// ── Projects ────────────────────────────────────────────────────────────

export async function fetchProjects(): Promise<Project[]> {
  return request<Project[]>('/projects')
}

export async function fetchProject(id: string): Promise<Project> {
  return request<Project>(`/projects/${id}`)
}

export async function createProject(data: { name: string; description?: string; repo_path?: string; language?: string; build_command?: string }): Promise<Project> {
  return request<Project>('/projects', { method: 'POST', body: JSON.stringify(data) })
}

export async function updateProject(id: string, data: Partial<Project>): Promise<Project> {
  return updateResource<Project>(`/projects/${id}`, data)
}

export async function deleteProject(id: string): Promise<void> {
  await deleteResource(`/projects/${id}`)
}

// ── Stories ─────────────────────────────────────────────────────────────

export async function fetchStories(filter?: StoryFilter): Promise<Story[]> {
  const params = new URLSearchParams()
  if (filter?.status) params.set('status', filter.status)
  if (filter?.assigned_to) params.set('assigned_to', filter.assigned_to)
  if (filter?.project_id) params.set('project_id', filter.project_id)
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
  return updateResource<Story>(`/stories/${id}`, data)
}

export async function batchReorderStories(stories: { id: string; sort_order: number }[]): Promise<{ updated: number }> {
  return request('/stories/reorder', { method: 'PATCH', body: JSON.stringify({ stories }) })
}

export async function updateStoryStatus(id: string, status: StatusType): Promise<Story> {
  return patchResourceStatus<Story>(`/stories/${id}/status`, status)
}

export async function deleteStory(id: string): Promise<void> {
  await deleteResource(`/stories/${id}`)
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

/**
 * Response shape of GET /api/tasks/{id}
 */
export async function fetchTask(id: string): Promise<TaskDetailResponse> {
  return request(`/tasks/${id}`)
}

export async function createTask(storyId: string, data: Partial<Task>): Promise<Task> {
  return request(`/stories/${storyId}/tasks`, { method: 'POST', body: JSON.stringify(data) })
}

export async function updateTask(id: string, data: Partial<Task>): Promise<Task> {
  return updateResource<Task>(`/tasks/${id}`, data)
}

export async function updateTaskStatus(id: string, status: StatusType): Promise<Task> {
  return patchResourceStatus<Task>(`/tasks/${id}/status`, status)
}

export async function batchReorderTasks(tasks: { id: string; sort_order: number }[]): Promise<{ updated: number }> {
  return request('/tasks/reorder', { method: 'PATCH', body: JSON.stringify({ tasks }) })
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
  await deleteResource(`/tasks/${id}`)
}

export interface GenerateTaskItem {
  title: string
  description?: string
  task_type?: string
  depends_on?: string[]
}

export async function generateTasks(storyId: string, tasks: GenerateTaskItem[]): Promise<{ tasks: Task[] }> {
  return request(`/stories/${storyId}/generate-tasks`, {
    method: 'POST',
    body: JSON.stringify({ tasks }),
  })
}

export async function fetchActivity(id: string, workItemType: WorkItemTypeType = 'task'): Promise<ActivityLogEntry[]> {
  const endpoint = workItemType === 'story' ? `/stories/${id}/activity` : `/tasks/${id}/activity`
  return request(endpoint)
}

// ── Global Activity ───────────────────────────────────────────────────

export async function fetchActivityLog(limit?: number): Promise<ActivityLogEntry[]> {
  const params = new URLSearchParams()
  if (limit !== undefined) params.set('limit', String(limit))
  const qs = params.toString()
  return request(`/activity${qs ? `?${qs}` : ''}`)
}

// ── Dispatcher ─────────────────────────────────────────────────────────

export async function fetchDispatcherStatus(): Promise<DispatcherStatus> {
  return request<DispatcherStatus>('/dispatcher/status')
}

// ── Gateway ────────────────────────────────────────────────────────────

export interface GatewayStatus {
  running: boolean
  active_sessions: number
  queue_depth: number
  events_processed: number
  uptime_seconds: number
  sessions_by_project: Record<string, number>
  sessions_by_agent: Record<string, number>
}

export interface GatewayJob {
  id: string
  project_id: string
  agent_type: string
  task_id: string
  event_ref: string
  created_at: string
}

export interface GatewayQueueResponse {
  total: number
  jobs: GatewayJob[]
}

export interface GatewayTriggerRequest {
  event_type: string
  project_id: string
  agent_type: string
  task_id: string
}

export async function fetchGatewayStatus(): Promise<GatewayStatus> {
  return request<GatewayStatus>('/gateway/status')
}

export async function fetchGatewayQueue(): Promise<GatewayQueueResponse> {
  return request<GatewayQueueResponse>('/gateway/queue')
}

export async function triggerGatewayAction(data: GatewayTriggerRequest): Promise<void> {
  await request('/gateway/trigger', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

// ── Agent Profiles ─────────────────────────────────────────────────────────

export interface AgentProfile {
  id: string
  name: string
  description?: string
  capabilities?: string // JSON string array
  max_concurrency: number
  task_types?: string[]
  created_at: string
  updated_at: string
}

export interface TriggerRule {
  id: string
  agent_profile_id: string
  event_type: string
  action: string
  priority: number
  enabled: boolean
  created_at: string
}

export async function fetchProfiles(): Promise<AgentProfile[]> {
  return request<AgentProfile[]>('/profiles')
}

export async function fetchProfile(id: string): Promise<AgentProfile> {
  return request<AgentProfile>(`/profiles/${id}`)
}

export async function createProfile(data: { name: string; description?: string; capabilities?: string; max_concurrency?: number; task_types?: string[] }): Promise<AgentProfile> {
  return request<AgentProfile>('/profiles', { method: 'POST', body: JSON.stringify(data) })
}

export async function updateProfile(id: string, data: Partial<AgentProfile>): Promise<AgentProfile> {
  return request<AgentProfile>(`/profiles/${id}`, { method: 'PUT', body: JSON.stringify(data) })
}

export async function deleteProfile(id: string): Promise<void> {
  await request(`/profiles/${id}`, { method: 'DELETE' })
}

export async function importProfiles(projectId?: string): Promise<AgentProfile[]> {
  const url = projectId ? `/profiles/import?project_id=${encodeURIComponent(projectId)}` : '/profiles/import'
  return request<AgentProfile[]>(url, { method: 'POST' })
}

export async function fetchRulesByProfile(profileId: string): Promise<TriggerRule[]> {
  return request<TriggerRule[]>(`/profiles/${profileId}/rules`)
}

export async function createRule(profileId: string, data: { event_type: string; action: string; priority?: number; enabled?: boolean }): Promise<TriggerRule> {
  return request<TriggerRule>(`/profiles/${profileId}/rules`, { method: 'POST', body: JSON.stringify(data) })
}

export async function updateRule(profileId: string, ruleId: string, data: Partial<TriggerRule>): Promise<TriggerRule> {
  return request<TriggerRule>(`/profiles/${profileId}/rules/${ruleId}`, { method: 'PUT', body: JSON.stringify(data) })
}

export async function deleteRule(profileId: string, ruleId: string): Promise<void> {
  await request(`/profiles/${profileId}/rules/${ruleId}`, { method: 'DELETE' })
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
  result: string,
): Promise<void> {
  await request('/work/complete', {
    method: 'POST',
    body: JSON.stringify({ session_id: sessionId, task_id: taskId, result }),
  })
}

export async function blockWork(
  sessionId: string,
  taskId: string,
  reason: string,
): Promise<void> {
  await request('/work/block', {
    method: 'POST',
    body: JSON.stringify({ session_id: sessionId, task_id: taskId, reason }),
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
  type: WorkItemTypeType,
): Promise<Comment[]> {
  return request(`/work-items/${workItemId}/comments?type=${type}`)
}

export async function addComment(
  workItemId: string,
  type: WorkItemTypeType,
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

export async function fetchTemplate(taskType: TaskTypeType): Promise<PromptTemplate> {
  return request(`/templates/${taskType}`)
}

export async function upsertTemplate(
  taskType: TaskTypeType,
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

export async function postUser(data: { username: string; email: string; display_name: string; password: string; role: UserRoleType }): Promise<User> {
  return request<User>('/users', { method: 'POST', body: JSON.stringify(data) })
}

export async function deleteUser(id: string): Promise<void> {
  await deleteResource(`/users/${id}`)
}