package remote

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenAuthService(t *testing.T) {
	service := NewTokenAuthService()
	
	assert.NotNil(t, service)
	assert.NotNil(t, service.users)
	assert.NotNil(t, service.tokens)
	assert.NotNil(t, service.sessions)
	assert.Equal(t, 24*time.Hour, service.tokenTTL)
	assert.Equal(t, 7*24*time.Hour, service.sessionTTL)
}

func TestNewTokenAuthServiceWithConfig(t *testing.T) {
	tokenTTL := 1 * time.Hour
	sessionTTL := 2 * time.Hour
	
	service := NewTokenAuthServiceWithConfig(tokenTTL, sessionTTL)
	
	assert.Equal(t, tokenTTL, service.tokenTTL)
	assert.Equal(t, sessionTTL, service.sessionTTL)
}

func TestTokenAuthService_CreateUser(t *testing.T) {
	service := NewTokenAuthService()
	
	username := "testuser"
	permissions := []Permission{
		NewAllowPermission(ActionToolRead, "*"),
	}
	
	user, err := service.CreateUser(username, permissions)
	require.NoError(t, err)
	
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, username, user.Username)
	assert.Equal(t, permissions, user.Permissions)
	assert.True(t, user.Active)
	assert.NotNil(t, user.Metadata)
	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())
}

func TestTokenAuthService_CreateUser_Duplicate(t *testing.T) {
	service := NewTokenAuthService()
	username := "testuser"
	
	// Create first user
	_, err := service.CreateUser(username, nil)
	require.NoError(t, err)
	
	// Try to create duplicate
	_, err = service.CreateUser(username, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username already exists")
}

func TestTokenAuthService_GetUser(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user
	originalUser, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	// Get user
	retrievedUser, err := service.GetUser(originalUser.ID)
	require.NoError(t, err)
	
	assert.Equal(t, originalUser.ID, retrievedUser.ID)
	assert.Equal(t, originalUser.Username, retrievedUser.Username)
}

func TestTokenAuthService_GetUser_NotFound(t *testing.T) {
	service := NewTokenAuthService()
	
	_, err := service.GetUser("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestTokenAuthService_UpdateUser(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	// Update user
	user.Username = "updateduser"
	user.Permissions = []Permission{NewAllowPermission("*", "*")}
	user.Metadata["key"] = "value"
	
	err = service.UpdateUser(user)
	require.NoError(t, err)
	
	// Verify update
	updatedUser, err := service.GetUser(user.ID)
	require.NoError(t, err)
	
	assert.Equal(t, "updateduser", updatedUser.Username)
	assert.Len(t, updatedUser.Permissions, 1)
	assert.Equal(t, "value", updatedUser.Metadata["key"])
}

func TestTokenAuthService_DeleteUser(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	// Delete user
	err = service.DeleteUser(user.ID)
	require.NoError(t, err)
	
	// Verify deletion
	_, err = service.GetUser(user.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestTokenAuthService_ListUsers(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create multiple users
	user1, err := service.CreateUser("user1", nil)
	require.NoError(t, err)
	
	user2, err := service.CreateUser("user2", nil)
	require.NoError(t, err)
	
	// List users
	users, err := service.ListUsers()
	require.NoError(t, err)
	
	assert.Len(t, users, 2)
	
	userIDs := make(map[string]bool)
	for _, user := range users {
		userIDs[user.ID] = true
	}
	
	assert.True(t, userIDs[user1.ID])
	assert.True(t, userIDs[user2.ID])
}

func TestTokenAuthService_CreateToken(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	// Create token
	token, err := service.CreateToken(user.ID, nil)
	require.NoError(t, err)
	
	assert.NotEmpty(t, token)
	
	// Validate token
	session, err := service.ValidateToken(token)
	require.NoError(t, err)
	
	assert.Equal(t, user.ID, session.UserID)
	assert.Equal(t, token, session.Token)
	assert.True(t, session.Active)
	assert.True(t, time.Now().Before(session.ExpiresAt))
}

func TestTokenAuthService_CreateToken_UserNotFound(t *testing.T) {
	service := NewTokenAuthService()
	
	_, err := service.CreateToken("nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestTokenAuthService_Authenticate(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	// Create token
	token, err := service.CreateToken(user.ID, nil)
	require.NoError(t, err)
	
	// Authenticate
	authenticatedUser, err := service.Authenticate(token)
	require.NoError(t, err)
	
	assert.Equal(t, user.ID, authenticatedUser.ID)
	assert.Equal(t, user.Username, authenticatedUser.Username)
}

func TestTokenAuthService_Authenticate_InvalidToken(t *testing.T) {
	service := NewTokenAuthService()
	
	_, err := service.Authenticate("invalid_token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestTokenAuthService_RevokeToken(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user and token
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	token, err := service.CreateToken(user.ID, nil)
	require.NoError(t, err)
	
	// Revoke token
	err = service.RevokeToken(token)
	require.NoError(t, err)
	
	// Try to authenticate with revoked token
	_, err = service.Authenticate(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is inactive")
}

func TestTokenAuthService_CreateSession(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	// Create session
	session, err := service.CreateSession(user)
	require.NoError(t, err)
	
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, user.ID, session.UserID)
	assert.True(t, session.Active)
	assert.True(t, time.Now().Before(session.ExpiresAt))
}

func TestTokenAuthService_ValidateSession(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user and session
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	session, err := service.CreateSession(user)
	require.NoError(t, err)
	
	// Validate session
	validatedSession, err := service.ValidateSession(session.ID)
	require.NoError(t, err)
	
	assert.Equal(t, session.ID, validatedSession.ID)
	assert.Equal(t, session.UserID, validatedSession.UserID)
}

func TestTokenAuthService_RefreshSession(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user and session
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	session, err := service.CreateSession(user)
	require.NoError(t, err)
	
	originalExpiration := session.ExpiresAt
	
	// Wait a moment to ensure different timestamp
	time.Sleep(10 * time.Millisecond)
	
	// Refresh session
	refreshedSession, err := service.RefreshSession(session.ID)
	require.NoError(t, err)
	
	assert.True(t, refreshedSession.ExpiresAt.After(originalExpiration))
}

func TestTokenAuthService_RevokeSession(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user and session
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	session, err := service.CreateSession(user)
	require.NoError(t, err)
	
	// Revoke session
	err = service.RevokeSession(session.ID)
	require.NoError(t, err)
	
	// Try to validate revoked session
	_, err = service.ValidateSession(session.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session is inactive")
}

func TestTokenAuthService_Authorize(t *testing.T) {
	service := NewTokenAuthService()
	
	tests := []struct {
		name        string
		permissions []Permission
		action      string
		resource    string
		expectError bool
	}{
		{
			name: "wildcard allow",
			permissions: []Permission{
				NewAllowPermission("*", "*"),
			},
			action:      "tool.execute",
			resource:    "bash",
			expectError: false,
		},
		{
			name: "specific allow",
			permissions: []Permission{
				NewAllowPermission("tool.execute", "bash"),
			},
			action:      "tool.execute",
			resource:    "bash",
			expectError: false,
		},
		{
			name: "prefix allow",
			permissions: []Permission{
				NewAllowPermission("tool.*", "*"),
			},
			action:      "tool.execute",
			resource:    "bash",
			expectError: false,
		},
		{
			name: "explicit deny",
			permissions: []Permission{
				NewDenyPermission("tool.execute", "rm"),
			},
			action:      "tool.execute",
			resource:    "rm",
			expectError: true,
		},
		{
			name: "default deny",
			permissions: []Permission{
				NewAllowPermission("file.read", "*"),
			},
			action:      "tool.execute",
			resource:    "bash",
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{
				ID:          "test_user",
				Username:    "testuser",
				Permissions: tt.permissions,
				Active:      true,
			}
			
			err := service.Authorize(user, tt.action, tt.resource)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTokenAuthService_AddUserPermission(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	// Add permission
	permission := NewAllowPermission("tool.execute", "bash")
	err = service.AddUserPermission(user.ID, permission)
	require.NoError(t, err)
	
	// Verify permission was added
	permissions, err := service.ListUserPermissions(user.ID)
	require.NoError(t, err)
	
	assert.Len(t, permissions, 1)
	assert.Equal(t, permission.Action, permissions[0].Action)
	assert.Equal(t, permission.Resource, permissions[0].Resource)
	assert.Equal(t, permission.Allow, permissions[0].Allow)
}

func TestTokenAuthService_RemoveUserPermission(t *testing.T) {
	service := NewTokenAuthService()
	
	// Create user with permission
	permission := NewAllowPermission("tool.execute", "bash")
	user, err := service.CreateUser("testuser", []Permission{permission})
	require.NoError(t, err)
	
	// Remove permission
	err = service.RemoveUserPermission(user.ID, permission.Action, permission.Resource)
	require.NoError(t, err)
	
	// Verify permission was removed
	permissions, err := service.ListUserPermissions(user.ID)
	require.NoError(t, err)
	
	assert.Len(t, permissions, 0)
}

func TestTokenAuthService_MatchesPattern(t *testing.T) {
	service := NewTokenAuthService()
	
	tests := []struct {
		pattern  string
		str      string
		expected bool
	}{
		{"*", "anything", true},
		{"exact", "exact", true},
		{"exact", "different", false},
		{"prefix*", "prefix123", true},
		{"prefix*", "otherprefix", false},
		{"*suffix", "anysuffix", true},
		{"*suffix", "suffixother", false},
		{"tool.execute", "tool.execute", true},
		{"tool.*", "tool.execute", true},
		{"tool.*", "file.read", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.str, func(t *testing.T) {
			result := service.matchesPattern(tt.pattern, tt.str)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTokenAuthService_CleanupExpiredSessions(t *testing.T) {
	// Create service with very short TTL
	service := NewTokenAuthServiceWithConfig(10*time.Millisecond, 10*time.Millisecond)
	
	// Create user
	user, err := service.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	// Create token and session
	token, err := service.CreateToken(user.ID, nil)
	require.NoError(t, err)
	
	session, err := service.CreateSession(user)
	require.NoError(t, err)
	
	// Wait for expiration
	time.Sleep(20 * time.Millisecond)
	
	// Cleanup expired sessions
	service.CleanupExpiredSessions()
	
	// Verify token and session were cleaned up
	_, err = service.ValidateToken(token)
	assert.Error(t, err)
	
	_, err = service.ValidateSession(session.ID)
	assert.Error(t, err)
}

func TestGenerateSecureToken(t *testing.T) {
	token1, err := generateSecureToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token1)
	assert.Len(t, token1, 64) // 32 bytes * 2 (hex encoding)
	
	token2, err := generateSecureToken()
	require.NoError(t, err)
	assert.NotEqual(t, token1, token2) // Should be different
}

func TestPermissionHelpers(t *testing.T) {
	// Test permission creation helpers
	allowPerm := NewAllowPermission("action", "resource")
	assert.Equal(t, "action", allowPerm.Action)
	assert.Equal(t, "resource", allowPerm.Resource)
	assert.True(t, allowPerm.Allow)
	
	denyPerm := NewDenyPermission("action", "resource")
	assert.Equal(t, "action", denyPerm.Action)
	assert.Equal(t, "resource", denyPerm.Resource)
	assert.False(t, denyPerm.Allow)
	
	// Test predefined permission sets
	adminPerms := AdminPermissions()
	assert.Len(t, adminPerms, 1)
	assert.Equal(t, "*", adminPerms[0].Action)
	assert.Equal(t, "*", adminPerms[0].Resource)
	assert.True(t, adminPerms[0].Allow)
	
	readOnlyPerms := ReadOnlyPermissions()
	assert.Greater(t, len(readOnlyPerms), 0)
	for _, perm := range readOnlyPerms {
		assert.True(t, perm.Allow)
		assert.Contains(t, perm.Action, "read")
	}
	
	devPerms := DeveloperPermissions()
	assert.Greater(t, len(devPerms), 0)
	
	// Check for some expected permissions
	hasExecutePermission := false
	hasFileDeleteDeny := false
	
	for _, perm := range devPerms {
		if perm.Action == ActionToolExecute && perm.Resource == "*" && perm.Allow {
			hasExecutePermission = true
		}
		if perm.Action == ActionFileDelete && strings.Contains(perm.Resource, "/etc/") && !perm.Allow {
			hasFileDeleteDeny = true
		}
	}
	
	assert.True(t, hasExecutePermission, "Developer permissions should include tool execute")
	assert.True(t, hasFileDeleteDeny, "Developer permissions should deny file delete in /etc/")
}