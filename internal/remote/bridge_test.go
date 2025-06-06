package remote

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sst/opencode/internal/llm/tools"
)

// MockTool implements BaseTool for testing
type MockTool struct {
	name        string
	description string
	response    tools.ToolResponse
	err         error
}

func NewMockTool(name, description string) *MockTool {
	return &MockTool{
		name:        name,
		description: description,
		response:    tools.NewTextResponse("mock response"),
	}
}

func (m *MockTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name:        m.name,
		Description: m.description,
		Parameters: map[string]any{
			"test_param": "string",
		},
		Required: []string{"test_param"},
	}
}

func (m *MockTool) Run(ctx context.Context, params tools.ToolCall) (tools.ToolResponse, error) {
	return m.response, m.err
}

func (m *MockTool) SetResponse(response tools.ToolResponse) {
	m.response = response
}

func (m *MockTool) SetError(err error) {
	m.err = err
}

// MockFileSystem implements FileSystemBridge for testing
type MockFileSystem struct {
	allowRead    bool
	allowWrite   bool
	allowExecute bool
	validateErr  error
}

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		allowRead:    true,
		allowWrite:   true,
		allowExecute: true,
	}
}

func (fs *MockFileSystem) ValidatePath(path string, user *User) error {
	return fs.validateErr
}

func (fs *MockFileSystem) SandboxPath(path string) string {
	return path
}

func (fs *MockFileSystem) CanRead(path string, user *User) bool {
	return fs.allowRead
}

func (fs *MockFileSystem) CanWrite(path string, user *User) bool {
	return fs.allowWrite
}

func (fs *MockFileSystem) CanExecute(path string, user *User) bool {
	return fs.allowExecute
}

func (fs *MockFileSystem) ReadFile(path string, user *User) ([]byte, error) {
	return []byte("test content"), nil
}

func (fs *MockFileSystem) WriteFile(path string, content []byte, user *User) error {
	return nil
}

func (fs *MockFileSystem) ListFiles(path string, user *User) ([]FileInfo, error) {
	return []FileInfo{
		{Name: "test.txt", Path: "/test/test.txt", Size: 100, IsDir: false},
	}, nil
}

func TestNewRemoteToolBridge(t *testing.T) {
	auth := NewTokenAuthService()
	fs := NewMockFileSystem()
	
	bridge := NewRemoteToolBridge(auth, fs)
	
	assert.NotNil(t, bridge)
	assert.Equal(t, auth, bridge.auth)
	assert.Equal(t, fs, bridge.fileSystem)
	assert.NotNil(t, bridge.allowedTools)
	assert.NotNil(t, bridge.tools)
}

func TestRemoteToolBridge_RegisterTool(t *testing.T) {
	bridge := NewRemoteToolBridge(NewTokenAuthService(), NewMockFileSystem())
	tool := NewMockTool("test_tool", "Test tool")
	
	err := bridge.RegisterTool(tool)
	require.NoError(t, err)
	
	// Verify tool was registered
	bridge.mu.RLock()
	registeredTool, exists := bridge.tools["test_tool"]
	bridge.mu.RUnlock()
	
	assert.True(t, exists)
	assert.Equal(t, tool, registeredTool)
}

func TestRemoteToolBridge_RegisterTools(t *testing.T) {
	bridge := NewRemoteToolBridge(NewTokenAuthService(), NewMockFileSystem())
	
	tools := []tools.BaseTool{
		NewMockTool("tool1", "Tool 1"),
		NewMockTool("tool2", "Tool 2"),
	}
	
	err := bridge.RegisterTools(tools)
	require.NoError(t, err)
	
	// Verify all tools were registered
	bridge.mu.RLock()
	assert.Len(t, bridge.tools, 2)
	assert.Contains(t, bridge.tools, "tool1")
	assert.Contains(t, bridge.tools, "tool2")
	bridge.mu.RUnlock()
}

func TestRemoteToolBridge_SetAllowedTools(t *testing.T) {
	bridge := NewRemoteToolBridge(NewTokenAuthService(), NewMockFileSystem())
	
	allowedTools := []string{"bash", "edit", "view"}
	bridge.SetAllowedTools(allowedTools)
	
	// Test allowed tools
	assert.True(t, bridge.IsToolAllowed("bash"))
	assert.True(t, bridge.IsToolAllowed("edit"))
	assert.True(t, bridge.IsToolAllowed("view"))
	
	// Test disallowed tool
	assert.False(t, bridge.IsToolAllowed("delete"))
}

func TestRemoteToolBridge_IsToolAllowed_EmptyList(t *testing.T) {
	bridge := NewRemoteToolBridge(NewTokenAuthService(), NewMockFileSystem())
	
	// When no allowed tools are set, all tools should be allowed
	assert.True(t, bridge.IsToolAllowed("any_tool"))
}

func TestRemoteToolBridge_ExecuteTool(t *testing.T) {
	auth := NewTokenAuthService()
	bridge := NewRemoteToolBridge(auth, NewMockFileSystem())
	
	// Create user with permissions
	user, err := auth.CreateUser("testuser", []Permission{
		NewAllowPermission(ActionToolExecute, "*"),
	})
	require.NoError(t, err)
	
	// Register a tool
	tool := NewMockTool("test_tool", "Test tool")
	expectedResponse := tools.NewTextResponse("test output")
	tool.SetResponse(expectedResponse)
	
	err = bridge.RegisterTool(tool)
	require.NoError(t, err)
	
	// Execute the tool
	execution := &ToolExecution{
		ID:       "exec_123",
		UserID:   user.ID,
		ToolName: "test_tool",
		Input:    `{"test_param": "value"}`,
	}
	
	result, err := bridge.ExecuteTool(context.Background(), execution)
	require.NoError(t, err)
	
	assert.Equal(t, execution.ID, result.ID)
	assert.Equal(t, expectedResponse, result.Response)
	assert.Empty(t, result.Error)
}

func TestRemoteToolBridge_ExecuteTool_UserNotFound(t *testing.T) {
	bridge := NewRemoteToolBridge(NewTokenAuthService(), NewMockFileSystem())
	
	execution := &ToolExecution{
		ID:       "exec_123",
		UserID:   "nonexistent",
		ToolName: "test_tool",
		Input:    `{}`,
	}
	
	_, err := bridge.ExecuteTool(context.Background(), execution)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get user")
}

func TestRemoteToolBridge_ExecuteTool_ToolNotAllowed(t *testing.T) {
	auth := NewTokenAuthService()
	bridge := NewRemoteToolBridge(auth, NewMockFileSystem())
	
	// Create user
	user, err := auth.CreateUser("testuser", nil)
	require.NoError(t, err)
	
	// Set allowed tools (excluding the test tool)
	bridge.SetAllowedTools([]string{"bash"})
	
	execution := &ToolExecution{
		ID:       "exec_123",
		UserID:   user.ID,
		ToolName: "test_tool",
		Input:    `{}`,
	}
	
	_, err = bridge.ExecuteTool(context.Background(), execution)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool test_tool is not allowed")
}

func TestRemoteToolBridge_ExecuteTool_NoPermission(t *testing.T) {
	auth := NewTokenAuthService()
	bridge := NewRemoteToolBridge(auth, NewMockFileSystem())
	
	// Create user without permissions
	user, err := auth.CreateUser("testuser", []Permission{
		NewDenyPermission(ActionToolExecute, "*"),
	})
	require.NoError(t, err)
	
	execution := &ToolExecution{
		ID:       "exec_123",
		UserID:   user.ID,
		ToolName: "test_tool",
		Input:    `{}`,
	}
	
	_, err = bridge.ExecuteTool(context.Background(), execution)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authorization failed")
}

func TestRemoteToolBridge_ExecuteTool_ToolNotFound(t *testing.T) {
	auth := NewTokenAuthService()
	bridge := NewRemoteToolBridge(auth, NewMockFileSystem())
	
	// Create user with permissions
	user, err := auth.CreateUser("testuser", []Permission{
		NewAllowPermission(ActionToolExecute, "*"),
	})
	require.NoError(t, err)
	
	execution := &ToolExecution{
		ID:       "exec_123",
		UserID:   user.ID,
		ToolName: "nonexistent_tool",
		Input:    `{}`,
	}
	
	_, err = bridge.ExecuteTool(context.Background(), execution)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool nonexistent_tool not found")
}

func TestRemoteToolBridge_ListAvailableTools(t *testing.T) {
	auth := NewTokenAuthService()
	bridge := NewRemoteToolBridge(auth, NewMockFileSystem())
	
	// Create user with read permissions
	user, err := auth.CreateUser("testuser", []Permission{
		NewAllowPermission(ActionToolRead, "*"),
	})
	require.NoError(t, err)
	
	// Register tools
	tools := []tools.BaseTool{
		NewMockTool("tool1", "Tool 1"),
		NewMockTool("tool2", "Tool 2"),
	}
	err = bridge.RegisterTools(tools)
	require.NoError(t, err)
	
	// List available tools
	availableTools, err := bridge.ListAvailableTools(user)
	require.NoError(t, err)
	
	assert.Len(t, availableTools, 2)
	
	toolNames := make(map[string]bool)
	for _, tool := range availableTools {
		toolNames[tool.Name] = true
	}
	
	assert.True(t, toolNames["tool1"])
	assert.True(t, toolNames["tool2"])
}

func TestRemoteToolBridge_GetToolInfo(t *testing.T) {
	auth := NewTokenAuthService()
	bridge := NewRemoteToolBridge(auth, NewMockFileSystem())
	
	// Create user with read permissions
	user, err := auth.CreateUser("testuser", []Permission{
		NewAllowPermission(ActionToolRead, "*"),
	})
	require.NoError(t, err)
	
	// Register tool
	tool := NewMockTool("test_tool", "Test tool description")
	err = bridge.RegisterTool(tool)
	require.NoError(t, err)
	
	// Get tool info
	info, err := bridge.GetToolInfo("test_tool", user)
	require.NoError(t, err)
	
	assert.Equal(t, "test_tool", info.Name)
	assert.Equal(t, "Test tool description", info.Description)
	assert.Contains(t, info.Parameters, "test_param")
	assert.Contains(t, info.Required, "test_param")
}

func TestSandboxFileSystemBridge_ValidatePath(t *testing.T) {
	allowedPaths := []string{"/workspace", "/home/user"}
	deniedPaths := []string{"/etc", "/usr"}
	
	fs := NewSandboxFileSystemBridge(allowedPaths, deniedPaths)
	user := &User{ID: "test_user"}
	
	tests := []struct {
		path        string
		expectError bool
		description string
	}{
		{"/workspace/project/file.txt", false, "allowed path"},
		{"/home/user/docs/file.txt", false, "allowed path"},
		{"/etc/passwd", true, "denied path"},
		{"/usr/bin/ls", true, "denied path"},
		{"/workspace/../etc/passwd", true, "path traversal"},
		{"/other/file.txt", true, "not in allowed paths"},
	}
	
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			err := fs.ValidatePath(tt.path, user)
			if tt.expectError {
				assert.Error(t, err, "Expected error for path: %s", tt.path)
			} else {
				assert.NoError(t, err, "Expected no error for path: %s", tt.path)
			}
		})
	}
}

func TestRemoteToolBridge_ValidateBashInput(t *testing.T) {
	bridge := NewRemoteToolBridge(NewTokenAuthService(), NewMockFileSystem())
	user := &User{
		ID:          "test_user",
		Permissions: []Permission{NewAllowPermission(ActionToolExecute, "*")},
	}
	
	tests := []struct {
		input       string
		expectError bool
		description string
	}{
		{`{"command": "ls -la"}`, false, "safe command"},
		{`{"command": "echo hello"}`, false, "safe command"},
		{`{"command": "rm -rf /"}`, true, "dangerous command"},
		{`{"command": "format c:"}`, true, "dangerous command"},
		{`invalid json`, true, "invalid JSON"},
		{`{"command": ""}`, false, "empty command"},
	}
	
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			err := bridge.validateBashInput(tt.input, user)
			if tt.expectError {
				assert.Error(t, err, "Expected error for input: %s", tt.input)
			} else {
				assert.NoError(t, err, "Expected no error for input: %s", tt.input)
			}
		})
	}
}

func TestRemoteToolBridge_ValidateFileInput(t *testing.T) {
	bridge := NewRemoteToolBridge(NewTokenAuthService(), NewMockFileSystem())
	user := &User{
		ID: "test_user",
		Permissions: []Permission{
			NewAllowPermission(ActionFileRead, "*"),
			NewAllowPermission(ActionFileWrite, "*"),
		},
	}
	
	tests := []struct {
		input       string
		writeAccess bool
		expectError bool
		description string
	}{
		{`{"file_path": "/workspace/file.txt"}`, false, false, "read access valid path"},
		{`{"path": "/workspace/file.txt"}`, true, false, "write access valid path"},
		{`{"file_path": ""}`, false, true, "empty path"},
		{`invalid json`, false, true, "invalid JSON"},
		{`{}`, false, true, "missing path"},
	}
	
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			err := bridge.validateFileInput(tt.input, user, tt.writeAccess)
			if tt.expectError {
				assert.Error(t, err, "Expected error for input: %s", tt.input)
			} else {
				assert.NoError(t, err, "Expected no error for input: %s", tt.input)
			}
		})
	}
}