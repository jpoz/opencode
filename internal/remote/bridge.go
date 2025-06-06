package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sst/opencode/internal/llm/tools"
)

// RemoteToolBridge provides secure tool execution for remote clients.
type RemoteToolBridge struct {
	allowedTools map[string]bool
	fileSystem   FileSystemBridge
	auth         AuthService
	tools        map[string]tools.BaseTool
	mu           sync.RWMutex
}

// FileSystemBridge defines the interface for secure file system operations.
type FileSystemBridge interface {
	// Secure file operations
	ReadFile(path string, user *User) ([]byte, error)
	WriteFile(path string, content []byte, user *User) error
	ListFiles(path string, user *User) ([]FileInfo, error)
	
	// Path validation
	ValidatePath(path string, user *User) error
	SandboxPath(path string) string
	
	// Permission checks
	CanRead(path string, user *User) bool
	CanWrite(path string, user *User) bool
	CanExecute(path string, user *User) bool
}

// FileInfo represents file information.
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
}

// ToolExecution represents a tool execution request.
type ToolExecution struct {
	ID       string      `json:"id"`
	UserID   string      `json:"user_id"`
	ToolName string      `json:"tool_name"`
	Input    string      `json:"input"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// ToolExecutionResult represents the result of a tool execution.
type ToolExecutionResult struct {
	ID       string        `json:"id"`
	Response tools.ToolResponse `json:"response"`
	Error    string        `json:"error,omitempty"`
	Duration string        `json:"duration"`
}

// NewRemoteToolBridge creates a new remote tool bridge.
func NewRemoteToolBridge(auth AuthService, fileSystem FileSystemBridge) *RemoteToolBridge {
	return &RemoteToolBridge{
		allowedTools: make(map[string]bool),
		fileSystem:   fileSystem,
		auth:         auth,
		tools:        make(map[string]tools.BaseTool),
	}
}

// RegisterTool registers a tool with the bridge.
func (b *RemoteToolBridge) RegisterTool(tool tools.BaseTool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	toolName := tool.Info().Name
	b.tools[toolName] = tool
	
	slog.Debug("Tool registered with bridge", "tool", toolName)
	return nil
}

// RegisterTools registers multiple tools with the bridge.
func (b *RemoteToolBridge) RegisterTools(tools []tools.BaseTool) error {
	for _, tool := range tools {
		if err := b.RegisterTool(tool); err != nil {
			return fmt.Errorf("failed to register tool %s: %w", tool.Info().Name, err)
		}
	}
	return nil
}

// SetAllowedTools sets the list of tools that are allowed to be executed.
func (b *RemoteToolBridge) SetAllowedTools(toolNames []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.allowedTools = make(map[string]bool)
	for _, name := range toolNames {
		b.allowedTools[name] = true
	}
	
	slog.Info("Allowed tools updated", "tools", toolNames)
}

// IsToolAllowed checks if a tool is allowed to be executed.
func (b *RemoteToolBridge) IsToolAllowed(toolName string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// If no allowed tools are set, all tools are allowed
	if len(b.allowedTools) == 0 {
		return true
	}
	
	return b.allowedTools[toolName]
}

// ExecuteTool executes a tool with the given parameters for a specific user.
func (b *RemoteToolBridge) ExecuteTool(ctx context.Context, execution *ToolExecution) (*ToolExecutionResult, error) {
	// Get user for authorization
	user, err := b.auth.GetUser(execution.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	// Check if tool is allowed
	if !b.IsToolAllowed(execution.ToolName) {
		return nil, fmt.Errorf("tool %s is not allowed", execution.ToolName)
	}
	
	// Check user permissions for tool execution
	if err := b.auth.Authorize(user, ActionToolExecute, execution.ToolName); err != nil {
		return nil, fmt.Errorf("authorization failed: %w", err)
	}
	
	// Get the tool
	b.mu.RLock()
	tool, exists := b.tools[execution.ToolName]
	b.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("tool %s not found", execution.ToolName)
	}
	
	// Validate and sanitize input based on tool type
	if err := b.validateToolInput(execution.ToolName, execution.Input, user); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}
	
	// Create tool call
	toolCall := tools.ToolCall{
		ID:    execution.ID,
		Name:  execution.ToolName,
		Input: execution.Input,
	}
	
	// Add context values if provided
	if execution.Context != nil {
		if sessionID, ok := execution.Context["session_id"].(string); ok {
			ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
		}
		if messageID, ok := execution.Context["message_id"].(string); ok {
			ctx = context.WithValue(ctx, tools.MessageIDContextKey, messageID)
		}
	}
	
	// Execute the tool
	slog.Info("Executing tool for remote user", 
		"tool", execution.ToolName, 
		"user_id", execution.UserID, 
		"execution_id", execution.ID)
	
	response, err := tool.Run(ctx, toolCall)
	
	result := &ToolExecutionResult{
		ID:       execution.ID,
		Response: response,
	}
	
	if err != nil {
		result.Error = err.Error()
		slog.Error("Tool execution failed", 
			"tool", execution.ToolName, 
			"user_id", execution.UserID, 
			"error", err)
	} else {
		slog.Debug("Tool execution completed", 
			"tool", execution.ToolName, 
			"user_id", execution.UserID)
	}
	
	return result, nil
}

// validateToolInput validates and sanitizes tool input based on tool type and user permissions.
func (b *RemoteToolBridge) validateToolInput(toolName, input string, user *User) error {
	switch toolName {
	case "bash":
		return b.validateBashInput(input, user)
	case "edit", "write":
		return b.validateFileInput(input, user, true) // write access
	case "view", "read":
		return b.validateFileInput(input, user, false) // read access
	case "patch":
		return b.validateFileInput(input, user, true) // write access
	default:
		// For other tools, perform basic validation
		return b.validateGenericInput(input, user)
	}
}

// validateBashInput validates bash command input.
func (b *RemoteToolBridge) validateBashInput(input string, user *User) error {
	// Parse the input to extract the command
	// This is a simplified validation - in production, you'd want more sophisticated parsing
	var command struct {
		Command string `json:"command"`
	}
	
	if err := parseJSON(input, &command); err != nil {
		return fmt.Errorf("invalid bash input format: %w", err)
	}
	
	// Check for dangerous commands
	dangerousCommands := []string{
		"rm -rf /",
		"format",
		"fdisk",
		"mkfs",
		"dd if=",
		":(){ :|:& };:",
	}
	
	commandLower := strings.ToLower(command.Command)
	for _, dangerous := range dangerousCommands {
		if strings.Contains(commandLower, dangerous) {
			return fmt.Errorf("dangerous command detected: %s", dangerous)
		}
	}
	
	// Check user permissions for specific commands
	cmdParts := strings.Fields(command.Command)
	if len(cmdParts) > 0 {
		mainCmd := cmdParts[0]
		if err := b.auth.Authorize(user, ActionToolExecute, mainCmd); err != nil {
			return fmt.Errorf("not authorized to execute command %s: %w", mainCmd, err)
		}
	}
	
	return nil
}

// validateFileInput validates file operation input.
func (b *RemoteToolBridge) validateFileInput(input string, user *User, writeAccess bool) error {
	// Parse input to extract file path
	var fileOp struct {
		FilePath string `json:"file_path,omitempty"`
		Path     string `json:"path,omitempty"`
	}
	
	if err := parseJSON(input, &fileOp); err != nil {
		return fmt.Errorf("invalid file input format: %w", err)
	}
	
	// Get the file path
	filePath := fileOp.FilePath
	if filePath == "" {
		filePath = fileOp.Path
	}
	
	if filePath == "" {
		return fmt.Errorf("file path is required")
	}
	
	// Validate the path
	if err := b.fileSystem.ValidatePath(filePath, user); err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}
	
	// Check permissions
	if writeAccess {
		if !b.fileSystem.CanWrite(filePath, user) {
			return fmt.Errorf("write access denied for path: %s", filePath)
		}
		
		// Check authorization
		if err := b.auth.Authorize(user, ActionFileWrite, filePath); err != nil {
			return fmt.Errorf("not authorized to write file: %w", err)
		}
	} else {
		if !b.fileSystem.CanRead(filePath, user) {
			return fmt.Errorf("read access denied for path: %s", filePath)
		}
		
		// Check authorization
		if err := b.auth.Authorize(user, ActionFileRead, filePath); err != nil {
			return fmt.Errorf("not authorized to read file: %w", err)
		}
	}
	
	return nil
}

// validateGenericInput performs basic validation for generic tools.
func (b *RemoteToolBridge) validateGenericInput(input string, user *User) error {
	// Basic input size validation
	if len(input) > 10*1024*1024 { // 10MB limit
		return fmt.Errorf("input too large: %d bytes", len(input))
	}
	
	// Check for potential injection attacks
	suspiciousPatterns := []string{
		"<script",
		"javascript:",
		"vbscript:",
		"onload=",
		"onerror=",
	}
	
	inputLower := strings.ToLower(input)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(inputLower, pattern) {
			return fmt.Errorf("suspicious pattern detected: %s", pattern)
		}
	}
	
	return nil
}

// ListAvailableTools returns a list of tools available to the user.
func (b *RemoteToolBridge) ListAvailableTools(user *User) ([]ToolInfo, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	var availableTools []ToolInfo
	
	for _, tool := range b.tools {
		toolName := tool.Info().Name
		
		// Check if tool is allowed
		if !b.IsToolAllowed(toolName) {
			continue
		}
		
		// Check user permissions
		if err := b.auth.Authorize(user, ActionToolRead, toolName); err != nil {
			continue // Skip tools the user can't access
		}
		
		availableTools = append(availableTools, ToolInfo{
			Name:        tool.Info().Name,
			Description: tool.Info().Description,
			Parameters:  tool.Info().Parameters,
			Required:    tool.Info().Required,
		})
	}
	
	return availableTools, nil
}

// ToolInfo represents information about a tool.
type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Required    []string       `json:"required"`
}

// GetToolInfo returns information about a specific tool.
func (b *RemoteToolBridge) GetToolInfo(toolName string, user *User) (*ToolInfo, error) {
	// Check if tool is allowed
	if !b.IsToolAllowed(toolName) {
		return nil, fmt.Errorf("tool %s is not allowed", toolName)
	}
	
	// Check user permissions
	if err := b.auth.Authorize(user, ActionToolRead, toolName); err != nil {
		return nil, fmt.Errorf("not authorized to access tool info: %w", err)
	}
	
	b.mu.RLock()
	tool, exists := b.tools[toolName]
	b.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("tool %s not found", toolName)
	}
	
	info := tool.Info()
	return &ToolInfo{
		Name:        info.Name,
		Description: info.Description,
		Parameters:  info.Parameters,
		Required:    info.Required,
	}, nil
}

// Utility functions

// parseJSON is a helper function to parse JSON input safely.
func parseJSON(input string, v interface{}) error {
	if input == "" {
		return fmt.Errorf("empty input")
	}
	
	return json.Unmarshal([]byte(input), v)
}

// SandboxFileSystemBridge implements FileSystemBridge with sandboxing.
type SandboxFileSystemBridge struct {
	allowedPaths []string
	deniedPaths  []string
}

// NewSandboxFileSystemBridge creates a new sandboxed file system bridge.
func NewSandboxFileSystemBridge(allowedPaths, deniedPaths []string) *SandboxFileSystemBridge {
	return &SandboxFileSystemBridge{
		allowedPaths: allowedPaths,
		deniedPaths:  deniedPaths,
	}
}

// ValidatePath validates that a path is within allowed boundaries.
func (fs *SandboxFileSystemBridge) ValidatePath(path string, user *User) error {
	// Clean the path
	cleanPath := filepath.Clean(path)
	
	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal detected")
	}
	
	// Check denied paths first
	for _, denied := range fs.deniedPaths {
		if strings.HasPrefix(cleanPath, denied) {
			return fmt.Errorf("access denied to path: %s", cleanPath)
		}
	}
	
	// Check allowed paths
	if len(fs.allowedPaths) > 0 {
		allowed := false
		for _, allowedPath := range fs.allowedPaths {
			if strings.HasPrefix(cleanPath, allowedPath) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path not in allowed directories: %s", cleanPath)
		}
	}
	
	return nil
}

// SandboxPath returns a sandboxed version of the path.
func (fs *SandboxFileSystemBridge) SandboxPath(path string) string {
	return filepath.Clean(path)
}

// CanRead checks if user can read the file.
func (fs *SandboxFileSystemBridge) CanRead(path string, user *User) bool {
	return fs.ValidatePath(path, user) == nil
}

// CanWrite checks if user can write the file.
func (fs *SandboxFileSystemBridge) CanWrite(path string, user *User) bool {
	return fs.ValidatePath(path, user) == nil
}

// CanExecute checks if user can execute the file.
func (fs *SandboxFileSystemBridge) CanExecute(path string, user *User) bool {
	return fs.ValidatePath(path, user) == nil
}

// ReadFile reads a file securely.
func (fs *SandboxFileSystemBridge) ReadFile(path string, user *User) ([]byte, error) {
	if err := fs.ValidatePath(path, user); err != nil {
		return nil, err
	}
	
	// In a real implementation, this would read the actual file
	return []byte("file content placeholder"), nil
}

// WriteFile writes a file securely.
func (fs *SandboxFileSystemBridge) WriteFile(path string, content []byte, user *User) error {
	if err := fs.ValidatePath(path, user); err != nil {
		return err
	}
	
	// In a real implementation, this would write the actual file
	return nil
}

// ListFiles lists files in a directory securely.
func (fs *SandboxFileSystemBridge) ListFiles(path string, user *User) ([]FileInfo, error) {
	if err := fs.ValidatePath(path, user); err != nil {
		return nil, err
	}
	
	// In a real implementation, this would list actual files
	return []FileInfo{
		{
			Name:    "example.txt",
			Path:    filepath.Join(path, "example.txt"),
			Size:    1024,
			Mode:    "-rw-r--r--",
			IsDir:   false,
			ModTime: "2024-01-01T00:00:00Z",
		},
	}, nil
}