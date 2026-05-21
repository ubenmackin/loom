// Package models defines the domain types for the Loom Kanban board.
package models

import "time"

// Status constants for stories and tasks.
const (
	StatusNew        = "new"
	StatusReady      = "ready"
	StatusInProgress = "in_progress"
	StatusBlocked    = "blocked"
	StatusDone       = "done"
)

// Session status constants.
const (
	SessionStatusActive       = "active"
	SessionStatusStale        = "stale"
	SessionStatusDisconnected = "disconnected"
)

// TaskType constants.
const (
	TaskTypeCode   = "code"
	TaskTypeBuild  = "build"
	TaskTypeReview = "review"
)

// AssigneeType constants.
const (
	AssigneeTypeHuman   = "human"
	AssigneeTypeSession = "session"
)

// WorkItemType constants.
const (
	WorkItemTypeStory = "story"
	WorkItemTypeTask  = "task"
)

// User represents a human user of the board.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session represents an agent session connected to the board.
type Session struct {
	ID           string    `json:"id"`
	HarnessType  string    `json:"harness_type"`
	Capabilities string    `json:"capabilities,omitempty"` // JSON string
	Metadata     string    `json:"metadata,omitempty"`     // JSON string
	LastSeenAt   time.Time `json:"last_seen_at"`
	Status       string    `json:"status"` // active | stale | disconnected
	CreatedAt    time.Time `json:"created_at"`
}

// Story represents a user story on the Kanban board.
type Story struct {
	ID             string    `json:"id"`
	NumericID      int       `json:"numeric_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	Status         string    `json:"status"`
	Priority       int       `json:"priority"`
	RequiresBuild  bool      `json:"requires_build"`
	RequiresReview bool      `json:"requires_review"`
	AssignedTo     string    `json:"assigned_to,omitempty"`
	AssigneeType   string    `json:"assignee_type,omitempty"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Task represents a task (child of a story) on the Kanban board.
type Task struct {
	ID           string    `json:"id"`
	NumericID    int       `json:"numeric_id"`
	StoryID      string    `json:"story_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	Status       string    `json:"status"`
	Priority     int       `json:"priority"`
	TaskType     string    `json:"task_type"`
	Estimate     *int      `json:"estimate,omitempty"`
	AssignedTo   string    `json:"assigned_to,omitempty"`
	AssigneeType string    `json:"assignee_type,omitempty"`
	SortOrder    int       `json:"sort_order"`
	Context      string    `json:"context,omitempty"` // JSON stored as text
	Instructions string    `json:"instructions,omitempty"`
	IsStale      bool      `json:"is_stale"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TaskDependency represents a finish-to-start dependency between tasks.
type TaskDependency struct {
	TaskID          string `json:"task_id"`
	DependsOnTaskID string `json:"depends_on_task_id"`
}

// Comment represents a comment by a human or agent on a work item.
type Comment struct {
	ID           string    `json:"id"`
	WorkItemID   string    `json:"work_item_id"`
	WorkItemType string    `json:"work_item_type"`
	AuthorID     string    `json:"author_id"`
	AuthorType   string    `json:"author_type"`
	Body         string    `json:"body,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ActivityLogEntry represents an immutable entry in the system activity log.
type ActivityLogEntry struct {
	ID           string    `json:"id"`
	WorkItemID   string    `json:"work_item_id"`
	WorkItemType string    `json:"work_item_type"`
	Action       string    `json:"action"`
	Details      string    `json:"details,omitempty"` // JSON as text
	CreatedAt    time.Time `json:"created_at"`
}

// PromptTemplate represents a template for generating agent instructions.
type PromptTemplate struct {
	ID        string    `json:"id"`
	TaskType  string    `json:"task_type"`
	Template  string    `json:"template"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UnreadComment tracks which comments a session has not yet read.
type UnreadComment struct {
	SessionID string `json:"session_id"`
	CommentID string `json:"comment_id"`
}
