package ui

import (
	"fmt"
	"time"
)

// EventData represents generic event data that can be sent over the network.
type EventData struct {
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
	SessionID string                 `json:"session_id,omitempty"`
}

// PermissionRequest represents a request for user permission.
type PermissionRequest struct {
	ID          string            `json:"id"`
	Tool        string            `json:"tool"`
	Action      string            `json:"action"`
	Resource    string            `json:"resource,omitempty"`
	Description string            `json:"description"`
	Options     []PermissionOption `json:"options"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
}

// PermissionOption represents an option for a permission request.
type PermissionOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
}

// PermissionResponse represents the user's response to a permission request.
type PermissionResponse struct {
	RequestID string    `json:"request_id"`
	Action    string    `json:"action"`
	Remember  bool      `json:"remember,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// StatusLevel represents the severity level of a status message.
type StatusLevel string

const (
	StatusLevelInfo    StatusLevel = "info"
	StatusLevelWarning StatusLevel = "warning"
	StatusLevelError   StatusLevel = "error"
	StatusLevelSuccess StatusLevel = "success"
)

// ProgressUpdate represents a progress update for long-running operations.
type ProgressUpdate struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Progress    float64       `json:"progress"` // 0.0 to 1.0
	ETA         time.Duration `json:"eta,omitempty"`
	Completed   bool          `json:"completed"`
}

// ErrorInfo represents detailed error information.
type ErrorInfo struct {
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Recoverable bool                   `json:"recoverable"`
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ValidationResult represents the result of a validation operation.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// NetworkMessage represents a message sent over the network.
type NetworkMessage struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// ConnectionInfo represents information about a client connection.
type ConnectionInfo struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent,omitempty"`
	Connected time.Time `json:"connected"`
}

// Utility functions

// NewDialog creates a new dialog with the specified parameters.
func NewDialog(id string, dialogType DialogType, title, content string) *Dialog {
	return &Dialog{
		ID:      id,
		Type:    dialogType,
		Title:   title,
		Content: content,
		Timeout: 30 * time.Second, // Default timeout
	}
}

// NewPermissionDialog creates a permission dialog.
func NewPermissionDialog(id, tool, action, description string) *Dialog {
	return &Dialog{
		ID:      id,
		Type:    DialogTypePermission,
		Title:   fmt.Sprintf("Permission Required: %s", tool),
		Content: description,
		Options: []DialogOption{
			{Value: string(ActionAllow), Label: "Allow", Default: false},
			{Value: string(ActionDeny), Label: "Deny", Default: true},
			{Value: string(ActionAllowOnce), Label: "Allow Once", Default: false},
		},
		Timeout: 60 * time.Second,
	}
}

// NewConfirmDialog creates a confirmation dialog.
func NewConfirmDialog(id, title, content string) *Dialog {
	return &Dialog{
		ID:      id,
		Type:    DialogTypeConfirm,
		Title:   title,
		Content: content,
		Options: []DialogOption{
			{Value: string(ActionConfirm), Label: "Yes", Default: false},
			{Value: string(ActionCancel), Label: "No", Default: true},
		},
		Timeout: 30 * time.Second,
	}
}

// NewInputDialog creates an input dialog.
func NewInputDialog(id, title, content, placeholder string) *Dialog {
	return &Dialog{
		ID:      id,
		Type:    DialogTypeInput,
		Title:   title,
		Content: content,
		Options: []DialogOption{
			{
				Value: "input",
				Label: placeholder,
				Data: map[string]interface{}{
					"placeholder": placeholder,
				},
			},
		},
		Timeout: 0, // No timeout for input dialogs
	}
}

// NewSelectDialog creates a selection dialog.
func NewSelectDialog(id, title, content string, options []DialogOption) *Dialog {
	return &Dialog{
		ID:      id,
		Type:    DialogTypeSelect,
		Title:   title,
		Content: content,
		Options: options,
		Timeout: 60 * time.Second,
	}
}

// IsExpired checks if a dialog has expired based on its timeout.
func (d *Dialog) IsExpired() bool {
	if d.Timeout == 0 {
		return false // No timeout
	}
	// Note: This would need a created timestamp to work properly
	// For now, we'll assume dialogs are checked immediately after creation
	return false
}

// Validate validates a DialogResponse against its dialog.
func (d *Dialog) Validate(response *DialogResponse) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if response.DialogID != d.ID {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "dialog_id",
			Message: "Dialog ID mismatch",
			Value:   response.DialogID,
		})
	}

	// Validate action is appropriate for dialog type
	switch d.Type {
	case DialogTypePermission:
		if response.Action != ActionAllow && response.Action != ActionDeny && response.Action != ActionAllowOnce {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "action",
				Message: "Invalid action for permission dialog",
				Value:   string(response.Action),
			})
		}
	case DialogTypeConfirm:
		if response.Action != ActionConfirm && response.Action != ActionCancel {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "action",
				Message: "Invalid action for confirm dialog",
				Value:   string(response.Action),
			})
		}
	case DialogTypeInput:
		if response.Action != ActionSubmit && response.Action != ActionCancel {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "action",
				Message: "Invalid action for input dialog",
				Value:   string(response.Action),
			})
		}
		if response.Action == ActionSubmit && response.Value == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "value",
				Message: "Input value required for submit action",
				Value:   response.Value,
			})
		}
	case DialogTypeSelect, DialogTypeMultiSelect:
		if response.Action != ActionSelect && response.Action != ActionCancel {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "action",
				Message: "Invalid action for select dialog",
				Value:   string(response.Action),
			})
		}
		if response.Action == ActionSelect && len(response.Selection) == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "selection",
				Message: "Selection required for select action",
				Value:   fmt.Sprintf("%v", response.Selection),
			})
		}
	}

	return result
}

// NewNetworkMessage creates a new network message.
func NewNetworkMessage(msgType string, data map[string]interface{}) *NetworkMessage {
	return &NetworkMessage{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewUserRequest creates a new user request.
func NewUserRequest(reqType RequestType, content string) *UserRequest {
	return &UserRequest{
		Type:    reqType,
		Content: content,
		Options: make(map[string]interface{}),
	}
}

// AddFile adds an attachment to a user request.
func (ur *UserRequest) AddFile(name string, content []byte, mimeType string) {
	if ur.Files == nil {
		ur.Files = make([]Attachment, 0)
	}
	ur.Files = append(ur.Files, Attachment{
		Name:     name,
		Content:  content,
		MimeType: mimeType,
		Size:     int64(len(content)),
	})
}

// SetOption sets an option on a user request.
func (ur *UserRequest) SetOption(key string, value interface{}) {
	if ur.Options == nil {
		ur.Options = make(map[string]interface{})
	}
	ur.Options[key] = value
}

// GetOption gets an option from a user request.
func (ur *UserRequest) GetOption(key string) (interface{}, bool) {
	if ur.Options == nil {
		return nil, false
	}
	value, exists := ur.Options[key]
	return value, exists
}

// HasFiles returns true if the user request has file attachments.
func (ur *UserRequest) HasFiles() bool {
	return len(ur.Files) > 0
}

// TotalFileSize returns the total size of all file attachments.
func (ur *UserRequest) TotalFileSize() int64 {
	var total int64
	for _, file := range ur.Files {
		total += file.Size
	}
	return total
}