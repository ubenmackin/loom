// ── Status Constants ────────────────────────────────────────────────────

export const Status = {
  New: 'new',
  Draft: 'draft',
  Planning: 'planning',
  Ready: 'ready',
  InProgress: 'in_progress',
  Blocked: 'blocked',
  Done: 'done',
  Completed: 'completed',
  Canceled: 'canceled',
  Archived: 'archived',
  Failed: 'failed',
} as const

export type StatusType = (typeof Status)[keyof typeof Status]

export const SessionStatus = {
  Active: 'active',
  Stale: 'stale',
  Disconnected: 'disconnected',
} as const

export type SessionStatusType = (typeof SessionStatus)[keyof typeof SessionStatus]

export const TaskType = {
  Code: 'code',
  Build: 'build',
  Review: 'review',
  Planning: 'planning',
  Security: 'security',
  Release: 'release',
  WorkspaceSetup: 'workspace_setup',
} as const

export type TaskTypeType = (typeof TaskType)[keyof typeof TaskType]

export const AssigneeType = {
  Human: 'human',
  Session: 'session',
} as const

export type AssigneeTypeType = (typeof AssigneeType)[keyof typeof AssigneeType]

export const WorkItemType = {
  Story: 'story',
  Task: 'task',
} as const

export type WorkItemTypeType = (typeof WorkItemType)[keyof typeof WorkItemType]

// TaskDetailResponse — detail response for a single task
export interface TaskDetailResponse {
  task: Task
  dependencies: string[]
  dependents: Task[]
}

// ── Domain Models ───────────────────────────────────────────────────────

export interface Project {
  id: string
  name: string
  description?: string
  repo_path?: string
  language?: string
  build_command?: string
  created_at: string
  updated_at: string
}

export interface Story {
  id: string
  numeric_id?: number
  title: string
  description?: string
  status: StatusType
  requires_build: boolean
  requires_review: boolean
  requires_security: boolean
  failure_count?: number
  branch_name?: string
  assigned_to?: string
  assignee_type?: AssigneeTypeType
  project_id?: string
  agent_session_id?: string
  agent_type?: string
  sort_order: number
  created_at: string
  updated_at: string
}

export interface Task {
  id: string
  numeric_id?: number
  story_id: string
  title: string
  description?: string
  status: StatusType
  task_type: TaskTypeType
  assigned_to?: string
  assignee_type?: AssigneeTypeType
  agent_session_id?: string
  agent_type?: string
  sort_order: number
  instructions?: string
  is_stale: boolean
  has_unread_comments?: boolean
  created_at: string
  updated_at: string
}

export interface TaskDependency {
  task_id: string
  depends_on_task_id: string
}

export interface Session {
  id: string
  harness_type: string
  capabilities?: string
  metadata?: string
  last_seen_at: string
  status: SessionStatusType
  created_at: string
}

export interface Comment {
  id: string
  work_item_id: string
  work_item_type: WorkItemTypeType
  author_id: string
  author_type: string
  body?: string
  created_at: string
  updated_at: string
}

export interface ActivityLogEntry {
  id: string
  work_item_id: string
  work_item_type: WorkItemTypeType
  action: string
  details?: string
  project_id: string
  work_item_title: string
  project_name: string
  created_at: string
}

export interface PromptTemplate {
  id: string
  task_type: string
  template: string
  created_at: string
  updated_at: string
}

// ── Board State ─────────────────────────────────────────────────────────

export interface StoryWithTasks {
  story: Story
  tasks: Task[]
}

export interface BoardStats {
  total_stories: number
  total_tasks: number
  ready_tasks: number
  in_progress_tasks: number
  blocked_tasks: number
  done_tasks: number
  canceled_tasks: number
  archived_tasks: number
  stale_tasks: number
}

export interface BoardState {
  stories: Story[]
  tasks_by_status: Record<string, Task[]>
  tasks_by_story_and_status?: Record<string, Record<string, Task[]>>
  stats: BoardStats
}

// ── Dispatcher Status ───────────────────────────────────────────────────

export interface DispatcherStatus {
  running: boolean
  uptime_seconds: number
  event_queue_depth: number
  events_processed: Record<string, number>
  started_at: string
  ready_tasks: number
  active_sessions: number
  pending_build_gates: number
  pending_review_gates: number
  stale_sessions: number
  last_assign_pass: string | null
  last_staleness_check: string | null
}

// ── WebSocket Events ────────────────────────────────────────────────────

export interface WebSocketEvent {
  type: string
  data?: unknown
}

// ── Work Protocol ───────────────────────────────────────────────────────

export interface WorkRequest {
  session_id: string
}

export interface WorkComplete {
  task_id: string
  result: string
}

export interface WorkBlock {
  task_id: string
  reason: string
}

// ── Filters ─────────────────────────────────────────────────────────────

export interface StoryFilter {
  status?: StatusType
  assigned_to?: string
  project_id?: string
}

export interface TaskFilter {
  story_id?: string
  task_type?: TaskTypeType
  assigned_to?: string
  status?: StatusType
}

export const UserRole = { Admin: 'admin', Normal: 'normal' } as const
export type UserRoleType = (typeof UserRole)[keyof typeof UserRole]

export interface User {
  id: string
  username: string
  email: string
  display_name?: string
  role: UserRoleType
  created_at: string
}

export interface AuthResponse {
  user: User
  token: string
}
