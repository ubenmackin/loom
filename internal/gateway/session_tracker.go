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

// SetAvailableModes stashes the advertised available modes on the per-subprocess
// session record for the given (projectID, agentType) pair. The write is
// performed while holding the tracker's write lock, making it safe against
// concurrent GetAvailableModes reads on the same entry. If the session does
// not exist, the call is a no-op (callers register the session before setting
// modes). A defensive copy of the slice is stored so later mutation of the
// caller's slice does not corrupt the stashed state.
//
// This accessor exists to close the data race documented in the Stage-4
// security audit: direct field writes/reads on a *GatewaySession returned
// from GetSession/GetBySessionID race with the tracker's internal lock, which
// only guards map membership — not the struct fields.
func (st *SessionTracker) SetAvailableModes(projectID, agentType string, modes []string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[key(projectID, agentType)]
	if !ok {
		return
	}
	if modes == nil {
		s.AvailableModes = nil
		return
	}
	cp := make([]string, len(modes))
	copy(cp, modes)
	s.AvailableModes = cp
}

// GetAvailableModes returns a defensive copy of the available modes stashed
// on the per-subprocess session record for the given (projectID, agentType)
// pair, or nil if the session does not exist. The copy is taken while
// holding the tracker's read lock so the read does not race with
// SetAvailableModes writes.
func (st *SessionTracker) GetAvailableModes(projectID, agentType string) []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	s, ok := st.sessions[key(projectID, agentType)]
	if !ok {
		return nil
	}
	if s.AvailableModes == nil {
		return nil
	}
	cp := make([]string, len(s.AvailableModes))
	copy(cp, s.AvailableModes)
	return cp
}

// SetAvailableModesBySessionID is the session-ID-keyed variant of
// SetAvailableModes for paths that only have an ACP session ID (e.g. the
// resume path). It is O(n) over the tracked entries. No-op if not found.
func (st *SessionTracker) SetAvailableModesBySessionID(sessionID string, modes []string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	for _, s := range st.sessions {
		if s.SessionID == sessionID {
			if modes == nil {
				s.AvailableModes = nil
				return
			}
			cp := make([]string, len(modes))
			copy(cp, modes)
			s.AvailableModes = cp
			return
		}
	}
}

// GetAvailableModesBySessionID is the session-ID-keyed variant of
// GetAvailableModes. Returns nil if the session is not tracked.
func (st *SessionTracker) GetAvailableModesBySessionID(sessionID string) []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	for _, s := range st.sessions {
		if s.SessionID == sessionID {
			if s.AvailableModes == nil {
				return nil
			}
			cp := make([]string, len(s.AvailableModes))
			copy(cp, s.AvailableModes)
			return cp
		}
	}
	return nil
}
