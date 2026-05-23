package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ubenmackin/loom/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// UserStore provides operations for user authentication and profile management.
type UserStore struct {
	db *sql.DB
}

// NewUserStore creates a new UserStore.
func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

// scanUserRow is a helper to scan a user row from a *sql.Row or *sql.Rows.
// This variant scans the 6-column projection (without password_hash).
func scanUserRow(scanner interface{ Scan(...any) error }) (*models.User, error) {
	user := &models.User{}
	var email, displayName, role sql.NullString
	var createdAt sql.NullTime

	err := scanner.Scan(&user.ID, &user.Username, &email, &displayName, &role, &createdAt)
	if err != nil {
		return nil, err
	}

	user.Email = stringOrZero(email)
	user.DisplayName = stringOrZero(displayName)
	user.CreatedAt = timeOrZero(createdAt)

	if role.Valid && role.String == string(models.RoleAdmin) {
		user.Role = models.RoleAdmin
	} else {
		user.Role = models.RoleNormal
	}

	return user, nil
}

// CreateUser registers a new human user with the given role.
func (s *UserStore) CreateUser(ctx context.Context, username, email, displayName, plaintextPassword string, role models.UserRole) (*models.User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(strings.ToLower(email))
	displayName = strings.TrimSpace(displayName)

	if username == "" {
		return nil, errors.New("username is required")
	}
	if email == "" {
		return nil, errors.New("email is required")
	}
	if len(plaintextPassword) < 6 {
		return nil, errors.New("password must be at least 6 characters")
	}
	if role == "" {
		role = models.RoleNormal
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	passwordHash := string(hashBytes)

	user := &models.User{
		ID:           uuid.New().String(),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		DisplayName:  displayName,
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	}

	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}

	var emailExists int
	err = s.db.QueryRowContext(ctx, "SELECT 1 FROM users WHERE email = ?", email).Scan(&emailExists)
	if err == nil {
		return nil, errors.New("email address already registered")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("check email uniqueness: %w", err)
	}

	var usernameExists int
	err = s.db.QueryRowContext(ctx, "SELECT 1 FROM users WHERE username = ?", username).Scan(&usernameExists)
	if err == nil {
		return nil, errors.New("username already taken")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("check username uniqueness: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (id, username, email, password_hash, display_name, role, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.DisplayName, string(user.Role), user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// AuthenticateUser verifies username/email and password.
func (s *UserStore) AuthenticateUser(ctx context.Context, usernameOrEmail, password string) (*models.User, error) {
	usernameOrEmail = strings.TrimSpace(usernameOrEmail)

	query := `SELECT id, username, email, password_hash, display_name, role, created_at
	          FROM users
	          WHERE username = ? OR email = ?`

	row := s.db.QueryRowContext(ctx, query, usernameOrEmail, strings.ToLower(usernameOrEmail))

	user := &models.User{}
	var email, passwordHash, displayName, role sql.NullString
	var createdAt sql.NullTime

	err := row.Scan(&user.ID, &user.Username, &email, &passwordHash, &displayName, &role, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("invalid credentials")
		}
		return nil, fmt.Errorf("query user failed: %w", err)
	}

	user.Email = stringOrZero(email)
	user.PasswordHash = stringOrZero(passwordHash)
	user.DisplayName = stringOrZero(displayName)
	user.CreatedAt = timeOrZero(createdAt)

	if role.Valid && role.String == string(models.RoleAdmin) {
		user.Role = models.RoleAdmin
	} else {
		user.Role = models.RoleNormal
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}

// CreateSession generates a new active session token.
func (s *UserStore) CreateSession(ctx context.Context, userID string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("create user session: %w", err)
	}

	return token, nil
}

// GetUserBySessionToken validates the token and returns the user.
func (s *UserStore) GetUserBySessionToken(ctx context.Context, token string) (*models.User, error) {
	query := `SELECT u.id, u.username, u.email, u.display_name, u.role, u.created_at
	          FROM user_sessions s
	          JOIN users u ON s.user_id = u.id
	          WHERE s.token = ? AND s.expires_at > ?`

	row := s.db.QueryRowContext(ctx, query, token, time.Now().UTC())

	user := &models.User{}
	var email, displayName, role sql.NullString
	var createdAt sql.NullTime

	err := row.Scan(&user.ID, &user.Username, &email, &displayName, &role, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("session expired or invalid")
		}
		return nil, fmt.Errorf("query session user: %w", err)
	}

	user.Email = stringOrZero(email)
	user.DisplayName = stringOrZero(displayName)
	user.CreatedAt = timeOrZero(createdAt)

	if role.Valid && role.String == string(models.RoleAdmin) {
		user.Role = models.RoleAdmin
	} else {
		user.Role = models.RoleNormal
	}

	return user, nil
}

// DeleteSession invalidates a session token (logout).
func (s *UserStore) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM user_sessions WHERE token = ?", token)
	if err != nil {
		return fmt.Errorf("delete user session: %w", err)
	}
	return nil
}

// CleanupExpiredSessions removes all expired session tokens from the database.
func (s *UserStore) CleanupExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM user_sessions WHERE expires_at < ?", time.Now().UTC())
	if err != nil {
		return fmt.Errorf("cleanup expired sessions: %w", err)
	}
	return nil
}

// CountUsers returns the total count of registered users.
func (s *UserStore) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

// ListAll returns all registered human users ordered by username.
func (s *UserStore) ListAll(ctx context.Context) ([]*models.User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, username, email, display_name, role, created_at FROM users ORDER BY username")
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	var users []*models.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

// DeleteUser removes a user by ID.
func (s *UserStore) DeleteUser(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user rows affected: %w", err)
	}
	if n == 0 {
		return errors.New("user not found")
	}
	return nil
}
