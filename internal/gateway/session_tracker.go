package gateway

import (
	"fmt"
	"sync"
	"time"
)

// SessionTracker manages in-memory state of all tracked gateway sessions.
// It is safe for concurrent use. Sessions are keyed by a composite key of
// "projectID:agentType".
//
// The tracker is an in-memory overlay on top of the persistent database
// SessionStore. It provides fast lookups and status transitions for the
// gateway event loop without requiring a database round-trip for every
// operation.
type SessionTracker struct {
	mu       sync.RWMutex
	sessions map[string]*GatewaySession
}

// NewSessionTracker creates a new empty SessionTracker.
func NewSessionTracker() *SessionTracker {
	return &SessionTracker{
		sessions: make(map[string]*GatewaySession),
	}
}

// key builds the composite map key for a (projectID, agentType) pair.
func key(projectID, agentType string) string {
	return fmt.Sprintf("%s:%s", projectID, agentType)
}

// RegisterSession creates a new session entry with status SessionCreating.
// If a session already exists for the given (projectID, agentType) pair,
// the existing session is returned (the call is a no-op).
func (st *SessionTracker) RegisterSession(projectID, agentType, sessionID string) *GatewaySession {
	st.mu.Lock()
	defer st.mu.Unlock()

	k := key(projectID, agentType)
	if existing, ok := st.sessions[k]; ok {
		return existing
	}

	now := time.Now().UTC()
	s := &GatewaySession{
		ProjectID:     projectID,
		AgentType:     agentType,
		SessionID:     sessionID,
		Status:        SessionCreating,
		LastHeartbeat: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	st.sessions[k] = s
	return s
}

// GetSession returns the session for the given (projectID, agentType) pair
// and a boolean indicating whether it was found.
func (st *SessionTracker) GetSession(projectID, agentType string) (*GatewaySession, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	s, ok := st.sessions[key(projectID, agentType)]
	return s, ok
}

// GetBySessionID looks up a session by its ACP session ID across all tracked
// entries. It returns the session and a boolean indicating whether it was
// found. This is an O(n) operation.
func (st *SessionTracker) GetBySessionID(sessionID string) (*GatewaySession, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	for _, s := range st.sessions {
		if s.SessionID == sessionID {
			return s, true
		}
	}
	return nil, false
}

// UpdateSessionID updates the ACP session ID for the given (projectID, agentType)
// pair. Returns an error if the session does not exist. This method acquires
// the write lock and is safe for concurrent use.
func (st *SessionTracker) UpdateSessionID(projectID, agentType, sessionID string) (*GatewaySession, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	k := key(projectID, agentType)
	s, ok := st.sessions[k]
	if !ok {
		return nil, fmt.Errorf("session not found for %s:%s", projectID, agentType)
	}

	s.SessionID = sessionID
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

// UpdateStatus updates the gateway session status and updated_at timestamp
// for the given (projectID, agentType) pair. Returns an error if the session
// does not exist.
func (st *SessionTracker) UpdateStatus(projectID, agentType string, status GatewaySessionStatus) (*GatewaySession, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[key(projectID, agentType)]
	if !ok {
		return nil, fmt.Errorf("session not found for %s:%s", projectID, agentType)
	}

	s.Status = status
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

// AssignTask assigns a task to a session and sets its status to SessionBusy.
// Returns an error if the session does not exist.
func (st *SessionTracker) AssignTask(projectID, agentType, taskID string) (*GatewaySession, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[key(projectID, agentType)]
	if !ok {
		return nil, fmt.Errorf("session not found for %s:%s", projectID, agentType)
	}

	s.AssignedTaskID = taskID
	s.Status = SessionBusy
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

// CompleteTask clears the assigned task from a session and sets its status
// back to SessionIdle. Returns an error if the session does not exist.
func (st *SessionTracker) CompleteTask(projectID, agentType string) (*GatewaySession, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[key(projectID, agentType)]
	if !ok {
		return nil, fmt.Errorf("session not found for %s:%s", projectID, agentType)
	}

	s.AssignedTaskID = ""
	s.Status = SessionIdle
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

// Heartbeat updates the last heartbeat timestamp for the given session.
// Returns an error if the session does not exist.
func (st *SessionTracker) Heartbeat(projectID, agentType string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[key(projectID, agentType)]
	if !ok {
		return fmt.Errorf("session not found for %s:%s", projectID, agentType)
	}

	s.LastHeartbeat = time.Now().UTC()
	return nil
}

// ListAll returns all tracked sessions.
func (st *SessionTracker) ListAll() []*GatewaySession {
	st.mu.RLock()
	defer st.mu.RUnlock()

	result := make([]*GatewaySession, 0, len(st.sessions))
	for _, s := range st.sessions {
		result = append(result, s)
	}
	return result
}

// Count returns the total number of tracked sessions.
func (st *SessionTracker) Count() int {
	st.mu.RLock()
	defer st.mu.RUnlock()

	return len(st.sessions)
}
