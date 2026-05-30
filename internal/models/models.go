// Package models defines the domain types for the Loom Kanban board.
package models

import (
	"encoding/json"
	"time"
)

// UserRole represents the access level of a human user.
type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleNormal UserRole = "normal"
)

func (r UserRole) String() string { return string(r) }

// Status represents the lifecycle state of a story or task.
type Status string

const (
	StatusNew        Status = "new"
	StatusReady      Status = "ready"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusDone       Status = "done"
	StatusCancelled  Status = "canceled"
	StatusArchived   Status = "archived"
)

func (s Status) String() string { return string(s) }

// AllStatuses returns all known status values in a canonical order.
func AllStatuses() []Status {
	return []Status{StatusNew, StatusReady, StatusInProgress, StatusBlocked, StatusDone, StatusCancelled, StatusArchived}
}

// SessionStatus represents the connection state of an agent session.
type SessionStatus string

const (
	SessionStatusActive       SessionStatus = "active"
	SessionStatusStale        SessionStatus = "stale"
	SessionStatusDisconnected SessionStatus = "disconnected"
)

func (s SessionStatus) String() string { return string(s) }

// TaskType represents the category of work a task performs.
type TaskType string

const (
	TaskTypeCode   TaskType = "code"
	TaskTypeBuild  TaskType = "build"
	TaskTypeReview TaskType = "review"
)

func (t TaskType) String() string { return string(t) }

// AssigneeType identifies whether a work item is assigned to a session or a human user.
type AssigneeType string

const (
	AssigneeTypeHuman   AssigneeType = "human"
	AssigneeTypeSession AssigneeType = "session"
)

func (a AssigneeType) String() string { return string(a) }

// WorkItemType distinguishes between stories and tasks.
type WorkItemType string

const (
	WorkItemTypeStory WorkItemType = "story"
	WorkItemTypeTask  WorkItemType = "task"
)

func (w WorkItemType) String() string { return string(w) }

// User represents a human user of the board.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name,omitempty"`
	Role         UserRole  `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session represents an agent session connected to the board.
type Session struct {
	ID           string        `json:"id"`
	HarnessType  string        `json:"harness_type"`
	Capabilities string        `json:"capabilities,omitempty"` // JSON string
	Metadata     string        `json:"metadata,omitempty"`     // JSON string
	LastSeenAt   time.Time     `json:"last_seen_at"`
	Status       SessionStatus `json:"status"` // active | stale | disconnected
	CreatedAt    time.Time     `json:"created_at"`
}

// CapabilitiesSlice parses the JSON-encoded Capabilities string into a slice.
// Returns an empty slice if Capabilities is empty or if parsing fails.
func (s *Session) CapabilitiesSlice() ([]string, error) {
	var caps []string
	if s.Capabilities == "" {
		return caps, nil
	}
	err := json.Unmarshal([]byte(s.Capabilities), &caps)
	return caps, err
}

// Story represents a user story on the Kanban board.
type Story struct {
	ID             string       `json:"id"`
	NumericID      int          `json:"numeric_id"`
	Title          string       `json:"title"`
	Description    string       `json:"description,omitempty"`
	Status         Status       `json:"status"`
	RequiresBuild  bool         `json:"requires_build"`
	RequiresReview bool         `json:"requires_review"`
	AssignedTo     string       `json:"assigned_to,omitempty"`
	AssigneeType   AssigneeType `json:"assignee_type,omitempty"`
	SortOrder      int          `json:"sort_order"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// Task represents a task (child of a story) on the Kanban board.
type Task struct {
	ID           string       `json:"id"`
	NumericID    int          `json:"numeric_id"`
	StoryID      string       `json:"story_id"`
	Title        string       `json:"title"`
	Description  string       `json:"description,omitempty"`
	Status       Status       `json:"status"`
	TaskType     TaskType     `json:"task_type"`
	AssignedTo   string       `json:"assigned_to,omitempty"`
	AssigneeType AssigneeType `json:"assignee_type,omitempty"`
	SortOrder    int          `json:"sort_order"`
	Instructions string       `json:"instructions,omitempty"`
	IsStale      bool         `json:"is_stale"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// TaskDependency represents a finish-to-start dependency between tasks.
type TaskDependency struct {
	TaskID          string `json:"task_id"`
	DependsOnTaskID string `json:"depends_on_task_id"`
}

// AuthorType identifies whether a comment author is a human user or an agent session.
type AuthorType string

const (
	AuthorTypeHuman   AuthorType = "human"
	AuthorTypeSession AuthorType = "session"
)

func (a AuthorType) String() string { return string(a) }

// Comment represents a comment by a human or agent on a work item.
type Comment struct {
	ID           string       `json:"id"`
	WorkItemID   string       `json:"work_item_id"`
	WorkItemType WorkItemType `json:"work_item_type"`
	AuthorID     string       `json:"author_id"`
	AuthorType   AuthorType   `json:"author_type"`
	Body         string       `json:"body,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// ActivityLogEntry represents an immutable entry in the system activity log.
type ActivityLogEntry struct {
	ID           string       `json:"id"`
	WorkItemID   string       `json:"work_item_id"`
	WorkItemType WorkItemType `json:"work_item_type"`
	Action       string       `json:"action"`
	Details      string       `json:"details,omitempty"` // JSON as text
	CreatedAt    time.Time    `json:"created_at"`
}

// PromptTemplate represents a template for generating agent instructions.
type PromptTemplate struct {
	ID        string    `json:"id"`
	TaskType  TaskType  `json:"task_type"`
	Template  string    `json:"template,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UnreadComment tracks which comments a session has not yet read.
type UnreadComment struct {
	SessionID string `json:"session_id"`
	CommentID string `json:"comment_id"`
}
