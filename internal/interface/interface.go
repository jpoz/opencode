package ui

import (
	"context"
	"time"

	"github.com/sst/opencode/internal/message"
	"github.com/sst/opencode/internal/pubsub"
	"github.com/sst/opencode/internal/session"
	"github.com/sst/opencode/internal/status"
)

// UserInterface defines the interface that all UI implementations must satisfy.
// This allows the core application logic to work with both TUI and remote interfaces.
type UserInterface interface {
	// Input handling
	// GetUserInput waits for user input and returns the request or an error.
	// The context can be used to cancel the operation.
	GetUserInput(ctx context.Context) (*UserRequest, error)

	// Output rendering
	// DisplayMessage renders a message to the user interface.
	DisplayMessage(msg *message.Message) error

	// DisplayStatus shows a status message to the user.
	DisplayStatus(status *status.StatusMessage) error

	// DisplayDialog shows a dialog to the user and waits for a response.
	// Returns DialogResponse or an error if the dialog times out or is cancelled.
	DisplayDialog(dialog *Dialog) (*DialogResponse, error)

	// Session management
	// ShowSessionList displays a list of sessions to the user.
	ShowSessionList(sessions []*session.Session) error

	// ShowFilePicker displays a file picker and returns the selected files.
	ShowFilePicker(opts *FilePickerOpts) (*FileSelection, error)

	// Event handling
	// Subscribe allows the UI to listen for specific event types.
	// Returns a channel that receives events of the specified types.
	Subscribe(events ...pubsub.EventType) <-chan pubsub.Event[interface{}]

	// Close performs cleanup and releases resources.
	Close() error
}

// UserRequest represents a request from the user.
type UserRequest struct {
	Type    RequestType            `json:"type"`
	Content string                 `json:"content"`
	Files   []Attachment           `json:"files,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// RequestType defines the type of user request.
type RequestType string

const (
	RequestTypeChat         RequestType = "chat"
	RequestTypeCommand      RequestType = "command"
	RequestTypeFileUpload   RequestType = "file_upload"
	RequestTypeSessionSwitch RequestType = "session_switch"
)

// Attachment represents a file attachment.
type Attachment struct {
	Name     string `json:"name"`
	Content  []byte `json:"content"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size"`
}

// Dialog represents a dialog to be shown to the user.
type Dialog struct {
	ID      string        `json:"id"`
	Type    DialogType    `json:"type"`
	Title   string        `json:"title"`
	Content string        `json:"content"`
	Options []DialogOption `json:"options,omitempty"`
	Timeout time.Duration `json:"timeout,omitempty"`
}

// DialogResponse represents the user's response to a dialog.
type DialogResponse struct {
	DialogID  string                 `json:"dialog_id"`
	Action    DialogAction           `json:"action"`
	Value     string                 `json:"value,omitempty"`
	Selection []string               `json:"selection,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Cancelled bool                   `json:"cancelled"`
	Timestamp time.Time              `json:"timestamp"`
}

// DialogType defines the type of dialog.
type DialogType string

const (
	DialogTypePermission  DialogType = "permission"
	DialogTypeConfirm     DialogType = "confirm"
	DialogTypeSelect      DialogType = "select"
	DialogTypeMultiSelect DialogType = "multi_select"
	DialogTypeInput       DialogType = "input"
	DialogTypeFilePicker  DialogType = "file_picker"
	DialogTypeCustom      DialogType = "custom"
)

// DialogAction defines the action taken by the user in response to a dialog.
type DialogAction string

const (
	ActionAllow     DialogAction = "allow"
	ActionDeny      DialogAction = "deny"
	ActionAllowOnce DialogAction = "allow_once"
	ActionConfirm   DialogAction = "confirm"
	ActionCancel    DialogAction = "cancel"
	ActionSelect    DialogAction = "select"
	ActionInput     DialogAction = "input"
	ActionSubmit    DialogAction = "submit"
)

// DialogOption represents an option in a dialog.
type DialogOption struct {
	Value       string                 `json:"value"`
	Label       string                 `json:"label"`
	Description string                 `json:"description,omitempty"`
	Default     bool                   `json:"default,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// FilePickerOpts defines options for the file picker.
type FilePickerOpts struct {
	Title        string   `json:"title,omitempty"`
	AllowedTypes []string `json:"allowed_types,omitempty"`
	Multiple     bool     `json:"multiple"`
	Directory    bool     `json:"directory"`
	InitialPath  string   `json:"initial_path,omitempty"`
}

// FileSelection represents the result of a file picker operation.
type FileSelection struct {
	Paths     []string `json:"paths"`
	Cancelled bool     `json:"cancelled"`
}