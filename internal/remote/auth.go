package remote

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuthService defines the interface for authentication and authorization.
type AuthService interface {
	// Authentication
	Authenticate(token string) (*User, error)
	CreateToken(userID string, permissions []Permission) (string, error)
	ValidateToken(token string) (*AuthSession, error)
	RevokeToken(token string) error
	
	// Authorization  
	Authorize(user *User, action string, resource string) error
	
	// Session management
	CreateSession(user *User) (*AuthSession, error)
	ValidateSession(sessionID string) (*AuthSession, error)
	RefreshSession(sessionID string) (*AuthSession, error)
	RevokeSession(sessionID string) error
	
	// User management
	CreateUser(username string, permissions []Permission) (*User, error)
	GetUser(userID string) (*User, error)
	UpdateUser(user *User) error
	DeleteUser(userID string) error
	ListUsers() ([]*User, error)
	
	// Permission management
	AddUserPermission(userID string, permission Permission) error
	RemoveUserPermission(userID string, action, resource string) error
	ListUserPermissions(userID string) ([]Permission, error)
}

// User represents an authenticated user.
type User struct {
	ID          string       `json:"id"`
	Username    string       `json:"username"`
	Permissions []Permission `json:"permissions"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Active      bool         `json:"active"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Permission represents an authorization permission.
type Permission struct {
	Action   string `json:"action"`   // "tool.execute", "session.create", "file.read", etc.
	Resource string `json:"resource"` // "*", "*.go", "/specific/path", "bash", etc.
	Allow    bool   `json:"allow"`    // true for allow, false for deny
}

// AuthSession represents an authenticated session.
type AuthSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	LastUsed  time.Time `json:"last_used"`
	Active    bool      `json:"active"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// TokenAuthService implements AuthService using token-based authentication.
type TokenAuthService struct {
	users    map[string]*User
	tokens   map[string]*AuthSession
	sessions map[string]*AuthSession
	mu       sync.RWMutex
	
	// Configuration
	tokenTTL   time.Duration
	sessionTTL time.Duration
}

// NewTokenAuthService creates a new token-based authentication service.
func NewTokenAuthService() *TokenAuthService {
	return &TokenAuthService{
		users:      make(map[string]*User),
		tokens:     make(map[string]*AuthSession),
		sessions:   make(map[string]*AuthSession),
		tokenTTL:   24 * time.Hour,    // 24 hours
		sessionTTL: 7 * 24 * time.Hour, // 7 days
	}
}

// NewTokenAuthServiceWithConfig creates a new auth service with custom configuration.
func NewTokenAuthServiceWithConfig(tokenTTL, sessionTTL time.Duration) *TokenAuthService {
	service := NewTokenAuthService()
	service.tokenTTL = tokenTTL
	service.sessionTTL = sessionTTL
	return service
}

// Authenticate implements AuthService.Authenticate.
func (a *TokenAuthService) Authenticate(token string) (*User, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	session, exists := a.tokens[token]
	if !exists {
		return nil, fmt.Errorf("invalid token")
	}
	
	if !session.Active {
		return nil, fmt.Errorf("token is inactive")
	}
	
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("token has expired")
	}
	
	user, exists := a.users[session.UserID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	
	if !user.Active {
		return nil, fmt.Errorf("user is inactive")
	}
	
	// Update last used
	session.LastUsed = time.Now()
	
	slog.Debug("User authenticated", "user_id", user.ID, "username", user.Username)
	return user, nil
}

// CreateToken implements AuthService.CreateToken.
func (a *TokenAuthService) CreateToken(userID string, permissions []Permission) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	user, exists := a.users[userID]
	if !exists {
		return "", fmt.Errorf("user not found")
	}
	
	if !user.Active {
		return "", fmt.Errorf("user is inactive")
	}
	
	// Generate secure token
	token, err := generateSecureToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	
	// Create auth session
	session := &AuthSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(a.tokenTTL),
		LastUsed:  time.Now(),
		Active:    true,
		Metadata:  make(map[string]string),
	}
	
	a.tokens[token] = session
	
	slog.Info("Token created", "user_id", userID, "session_id", session.ID)
	return token, nil
}

// ValidateToken implements AuthService.ValidateToken.
func (a *TokenAuthService) ValidateToken(token string) (*AuthSession, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	session, exists := a.tokens[token]
	if !exists {
		return nil, fmt.Errorf("invalid token")
	}
	
	if !session.Active {
		return nil, fmt.Errorf("token is inactive")
	}
	
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("token has expired")
	}
	
	return session, nil
}

// RevokeToken implements AuthService.RevokeToken.
func (a *TokenAuthService) RevokeToken(token string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	session, exists := a.tokens[token]
	if !exists {
		return fmt.Errorf("token not found")
	}
	
	session.Active = false
	slog.Info("Token revoked", "session_id", session.ID, "user_id", session.UserID)
	return nil
}

// Authorize implements AuthService.Authorize.
func (a *TokenAuthService) Authorize(user *User, action string, resource string) error {
	// Check user permissions
	for _, perm := range user.Permissions {
		if a.matchesPermission(perm, action, resource) {
			if perm.Allow {
				slog.Debug("Permission granted", "user_id", user.ID, "action", action, "resource", resource)
				return nil
			} else {
				slog.Debug("Permission denied by explicit deny rule", "user_id", user.ID, "action", action, "resource", resource)
				return fmt.Errorf("permission denied: %s on %s", action, resource)
			}
		}
	}
	
	// Default deny
	slog.Debug("Permission denied by default", "user_id", user.ID, "action", action, "resource", resource)
	return fmt.Errorf("permission denied: %s on %s", action, resource)
}

// CreateSession implements AuthService.CreateSession.
func (a *TokenAuthService) CreateSession(user *User) (*AuthSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	session := &AuthSession{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(a.sessionTTL),
		LastUsed:  time.Now(),
		Active:    true,
		Metadata:  make(map[string]string),
	}
	
	a.sessions[session.ID] = session
	
	slog.Info("Session created", "session_id", session.ID, "user_id", user.ID)
	return session, nil
}

// ValidateSession implements AuthService.ValidateSession.
func (a *TokenAuthService) ValidateSession(sessionID string) (*AuthSession, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	session, exists := a.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	
	if !session.Active {
		return nil, fmt.Errorf("session is inactive")
	}
	
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session has expired")
	}
	
	return session, nil
}

// RefreshSession implements AuthService.RefreshSession.
func (a *TokenAuthService) RefreshSession(sessionID string) (*AuthSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	session, exists := a.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	
	if !session.Active {
		return nil, fmt.Errorf("session is inactive")
	}
	
	// Extend expiration
	session.ExpiresAt = time.Now().Add(a.sessionTTL)
	session.LastUsed = time.Now()
	
	slog.Debug("Session refreshed", "session_id", sessionID)
	return session, nil
}

// RevokeSession implements AuthService.RevokeSession.
func (a *TokenAuthService) RevokeSession(sessionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	session, exists := a.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found")
	}
	
	session.Active = false
	slog.Info("Session revoked", "session_id", sessionID, "user_id", session.UserID)
	return nil
}

// CreateUser implements AuthService.CreateUser.
func (a *TokenAuthService) CreateUser(username string, permissions []Permission) (*User, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Check if username already exists
	for _, user := range a.users {
		if user.Username == username {
			return nil, fmt.Errorf("username already exists")
		}
	}
	
	user := &User{
		ID:          uuid.New().String(),
		Username:    username,
		Permissions: permissions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Active:      true,
		Metadata:    make(map[string]string),
	}
	
	a.users[user.ID] = user
	
	slog.Info("User created", "user_id", user.ID, "username", username)
	return user, nil
}

// GetUser implements AuthService.GetUser.
func (a *TokenAuthService) GetUser(userID string) (*User, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	user, exists := a.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	
	return user, nil
}

// UpdateUser implements AuthService.UpdateUser.
func (a *TokenAuthService) UpdateUser(user *User) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	existingUser, exists := a.users[user.ID]
	if !exists {
		return fmt.Errorf("user not found")
	}
	
	// Update fields
	existingUser.Username = user.Username
	existingUser.Permissions = user.Permissions
	existingUser.UpdatedAt = time.Now()
	existingUser.Active = user.Active
	if user.Metadata != nil {
		existingUser.Metadata = user.Metadata
	}
	
	slog.Info("User updated", "user_id", user.ID)
	return nil
}

// DeleteUser implements AuthService.DeleteUser.
func (a *TokenAuthService) DeleteUser(userID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	user, exists := a.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}
	
	// Revoke all user's sessions and tokens
	for _, session := range a.sessions {
		if session.UserID == userID {
			session.Active = false
		}
	}
	
	for _, session := range a.tokens {
		if session.UserID == userID {
			session.Active = false
		}
	}
	
	delete(a.users, userID)
	
	slog.Info("User deleted", "user_id", userID, "username", user.Username)
	return nil
}

// ListUsers implements AuthService.ListUsers.
func (a *TokenAuthService) ListUsers() ([]*User, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	users := make([]*User, 0, len(a.users))
	for _, user := range a.users {
		users = append(users, user)
	}
	
	return users, nil
}

// AddUserPermission implements AuthService.AddUserPermission.
func (a *TokenAuthService) AddUserPermission(userID string, permission Permission) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	user, exists := a.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}
	
	// Check if permission already exists
	for i, perm := range user.Permissions {
		if perm.Action == permission.Action && perm.Resource == permission.Resource {
			// Update existing permission
			user.Permissions[i] = permission
			user.UpdatedAt = time.Now()
			slog.Debug("User permission updated", "user_id", userID, "action", permission.Action, "resource", permission.Resource)
			return nil
		}
	}
	
	// Add new permission
	user.Permissions = append(user.Permissions, permission)
	user.UpdatedAt = time.Now()
	
	slog.Debug("User permission added", "user_id", userID, "action", permission.Action, "resource", permission.Resource)
	return nil
}

// RemoveUserPermission implements AuthService.RemoveUserPermission.
func (a *TokenAuthService) RemoveUserPermission(userID string, action, resource string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	user, exists := a.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}
	
	// Find and remove permission
	for i, perm := range user.Permissions {
		if perm.Action == action && perm.Resource == resource {
			user.Permissions = append(user.Permissions[:i], user.Permissions[i+1:]...)
			user.UpdatedAt = time.Now()
			slog.Debug("User permission removed", "user_id", userID, "action", action, "resource", resource)
			return nil
		}
	}
	
	return fmt.Errorf("permission not found")
}

// ListUserPermissions implements AuthService.ListUserPermissions.
func (a *TokenAuthService) ListUserPermissions(userID string) ([]Permission, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	user, exists := a.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	
	return user.Permissions, nil
}

// Utility methods

// matchesPermission checks if a permission rule matches the given action and resource.
func (a *TokenAuthService) matchesPermission(perm Permission, action, resource string) bool {
	// Check action match
	if !a.matchesPattern(perm.Action, action) {
		return false
	}
	
	// Check resource match
	if !a.matchesPattern(perm.Resource, resource) {
		return false
	}
	
	return true
}

// matchesPattern checks if a pattern matches a string (supports wildcards).
func (a *TokenAuthService) matchesPattern(pattern, str string) bool {
	if pattern == "*" {
		return true
	}
	
	if pattern == str {
		return true
	}
	
	// Handle simple wildcard patterns
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(str, prefix)
	}
	
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(str, suffix)
	}
	
	return false
}

// CleanupExpiredSessions removes expired sessions and tokens.
func (a *TokenAuthService) CleanupExpiredSessions() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	now := time.Now()
	
	// Cleanup expired tokens
	for token, session := range a.tokens {
		if now.After(session.ExpiresAt) {
			delete(a.tokens, token)
			slog.Debug("Expired token cleaned up", "session_id", session.ID)
		}
	}
	
	// Cleanup expired sessions
	for sessionID, session := range a.sessions {
		if now.After(session.ExpiresAt) {
			delete(a.sessions, sessionID)
			slog.Debug("Expired session cleaned up", "session_id", sessionID)
		}
	}
}

// generateSecureToken generates a cryptographically secure random token.
func generateSecureToken() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	
	// Create hash for additional security
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:]), nil
}

// Predefined permission constants
const (
	// Actions
	ActionToolExecute    = "tool.execute"
	ActionToolRead       = "tool.read"
	ActionSessionCreate  = "session.create"
	ActionSessionRead    = "session.read"
	ActionSessionUpdate  = "session.update"
	ActionSessionDelete  = "session.delete"
	ActionFileRead       = "file.read"
	ActionFileWrite      = "file.write"
	ActionFileDelete     = "file.delete"
	ActionAdminUser      = "admin.user"
	ActionAdminSystem    = "admin.system"
)

// Permission helpers

// NewAllowPermission creates an allow permission.
func NewAllowPermission(action, resource string) Permission {
	return Permission{
		Action:   action,
		Resource: resource,
		Allow:    true,
	}
}

// NewDenyPermission creates a deny permission.
func NewDenyPermission(action, resource string) Permission {
	return Permission{
		Action:   action,
		Resource: resource,
		Allow:    false,
	}
}

// AdminPermissions returns a set of admin permissions.
func AdminPermissions() []Permission {
	return []Permission{
		NewAllowPermission("*", "*"),
	}
}

// ReadOnlyPermissions returns a set of read-only permissions.
func ReadOnlyPermissions() []Permission {
	return []Permission{
		NewAllowPermission(ActionToolRead, "*"),
		NewAllowPermission(ActionSessionRead, "*"),
		NewAllowPermission(ActionFileRead, "*"),
	}
}

// DeveloperPermissions returns a set of developer permissions.
func DeveloperPermissions() []Permission {
	return []Permission{
		NewAllowPermission(ActionToolExecute, "*"),
		NewAllowPermission(ActionToolRead, "*"),
		NewAllowPermission(ActionSessionCreate, "*"),
		NewAllowPermission(ActionSessionRead, "*"),
		NewAllowPermission(ActionSessionUpdate, "*"),
		NewAllowPermission(ActionFileRead, "*"),
		NewAllowPermission(ActionFileWrite, "*"),
		NewDenyPermission(ActionFileDelete, "/etc/*"),
		NewDenyPermission(ActionFileDelete, "/usr/*"),
		NewDenyPermission(ActionFileDelete, "/var/*"),
	}
}