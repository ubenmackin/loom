package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/testhelpers"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	sessionStore := NewSessionStore(dbConn)
	ctx := context.Background()

	session := &models.Session{
		ID:           "sess-001",
		HarnessType:  "opencode",
		Capabilities: `["code","build"]`,
		Status:       models.SessionStatusActive,
	}

	if err := sessionStore.Register(ctx, session); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if session.ID != "sess-001" {
		t.Errorf("Register() ID = %q, want %q", session.ID, "sess-001")
	}
	if session.HarnessType != "opencode" {
		t.Errorf("Register() HarnessType = %q, want %q", session.HarnessType, "opencode")
	}
	if session.Status != models.SessionStatusActive {
		t.Errorf("Register() Status = %q, want %q", session.Status, models.SessionStatusActive)
	}
	if session.CreatedAt.IsZero() {
		t.Fatal("Register() CreatedAt is zero")
	}
	if session.LastSeenAt.IsZero() {
		t.Fatal("Register() LastSeenAt is zero")
	}
}

func TestSessionGetByID(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	sessionStore := NewSessionStore(dbConn)
	ctx := context.Background()

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code", "review"})
		s.Capabilities = string(data)
	})

	got, err := sessionStore.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.ID != session.ID {
		t.Errorf("GetByID() ID = %q, want %q", got.ID, session.ID)
	}
	if got.HarnessType != "opencode" {
		t.Errorf("GetByID() HarnessType = %q, want %q", got.HarnessType, "opencode")
	}
}

func TestUpdateLastSeen(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	sessionStore := NewSessionStore(dbConn)
	ctx := context.Background()

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
	})
	originalLastSeen := session.LastSeenAt

	time.Sleep(10 * time.Millisecond)

	if err := sessionStore.UpdateLastSeen(ctx, session.ID); err != nil {
		t.Fatalf("UpdateLastSeen() error = %v", err)
	}

	got, err := sessionStore.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if !got.LastSeenAt.After(originalLastSeen) {
		t.Errorf("UpdateLastSeen() LastSeenAt = %v, should be after %v", got.LastSeenAt, originalLastSeen)
	}
}

func TestDisconnect(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	sessionStore := NewSessionStore(dbConn)
	ctx := context.Background()

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
	})

	if err := sessionStore.Disconnect(ctx, session.ID); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}

	got, err := sessionStore.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.Status != models.SessionStatusDisconnected {
		t.Errorf("Disconnect() Status = %q, want %q", got.Status, models.SessionStatusDisconnected)
	}
}

func TestGetStaleSessions(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	sessionStore := NewSessionStore(dbConn)
	ctx := context.Background()

	activeSession := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})
	staleSession := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})
	testhelpers.SetSessionLastSeen(t, dbConn, staleSession.ID, time.Now().UTC().Add(-2*time.Hour))

	stale, err := sessionStore.GetStaleSessions(ctx, 1*time.Hour)
	if err != nil {
		t.Fatalf("GetStaleSessions() error = %v", err)
	}

	if len(stale) != 1 {
		t.Fatalf("GetStaleSessions() returned %d sessions, want 1", len(stale))
	}
	if stale[0].ID != staleSession.ID {
		t.Errorf("GetStaleSessions() session ID = %q, want %q", stale[0].ID, staleSession.ID)
	}

	for _, s := range stale {
		if s.ID == activeSession.ID {
			t.Error("GetStaleSessions() should not include active session")
		}
	}
}

func TestGetByCapabilities(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	sessionStore := NewSessionStore(dbConn)
	ctx := context.Background()

	sessionA := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code", "build"})
		s.Capabilities = string(data)
	})
	sessionB := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"review"})
		s.Capabilities = string(data)
	})

	t.Run("filter by code capability", func(t *testing.T) {
		sessions, err := sessionStore.GetByCapabilities(ctx, "code")
		if err != nil {
			t.Fatalf("GetByCapabilities() error = %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("GetByCapabilities(code) returned %d sessions, want 1", len(sessions))
		}
		if sessions[0].ID != sessionA.ID {
			t.Errorf("GetByCapabilities(code) session ID = %q, want %q", sessions[0].ID, sessionA.ID)
		}
	})

	t.Run("filter by review capability", func(t *testing.T) {
		sessions, err := sessionStore.GetByCapabilities(ctx, "review")
		if err != nil {
			t.Fatalf("GetByCapabilities() error = %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("GetByCapabilities(review) returned %d sessions, want 1", len(sessions))
		}
		if sessions[0].ID != sessionB.ID {
			t.Errorf("GetByCapabilities(review) session ID = %q, want %q", sessions[0].ID, sessionB.ID)
		}
	})

	t.Run("filter by build capability", func(t *testing.T) {
		sessions, err := sessionStore.GetByCapabilities(ctx, "build")
		if err != nil {
			t.Fatalf("GetByCapabilities() error = %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("GetByCapabilities(build) returned %d sessions, want 1", len(sessions))
		}
	})
}

func TestFlagStale(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	sessionStore := NewSessionStore(dbConn)
	ctx := context.Background()

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
	})

	if err := sessionStore.FlagStale(ctx, session.ID); err != nil {
		t.Fatalf("FlagStale() error = %v", err)
	}

	got, err := sessionStore.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.Status != models.SessionStatusStale {
		t.Errorf("FlagStale() Status = %q, want %q", got.Status, models.SessionStatusStale)
	}
}
