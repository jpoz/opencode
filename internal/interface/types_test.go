package ui

import (
	"testing"
	"time"
)

func TestNewDialog(t *testing.T) {
	id := "test_dialog"
	dialogType := DialogTypeConfirm
	title := "Test Dialog"
	content := "Test content"

	dialog := NewDialog(id, dialogType, title, content)

	if dialog.ID != id {
		t.Errorf("Expected ID %s, got %s", id, dialog.ID)
	}
	if dialog.Type != dialogType {
		t.Errorf("Expected type %s, got %s", dialogType, dialog.Type)
	}
	if dialog.Title != title {
		t.Errorf("Expected title %s, got %s", title, dialog.Title)
	}
	if dialog.Content != content {
		t.Errorf("Expected content %s, got %s", content, dialog.Content)
	}
	if dialog.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", dialog.Timeout)
	}
}

func TestNewPermissionDialog(t *testing.T) {
	id := "perm_123"
	tool := "bash"
	action := "execute"
	description := "Execute command"

	dialog := NewPermissionDialog(id, tool, action, description)

	if dialog.Type != DialogTypePermission {
		t.Errorf("Expected type %s, got %s", DialogTypePermission, dialog.Type)
	}
	if dialog.Title != "Permission Required: bash" {
		t.Errorf("Expected title 'Permission Required: bash', got %s", dialog.Title)
	}
	if len(dialog.Options) != 3 {
		t.Errorf("Expected 3 options, got %d", len(dialog.Options))
	}

	// Check options
	expectedOptions := []string{string(ActionAllow), string(ActionDeny), string(ActionAllowOnce)}
	for i, option := range dialog.Options {
		if option.Value != expectedOptions[i] {
			t.Errorf("Expected option %d value %s, got %s", i, expectedOptions[i], option.Value)
		}
	}

	// Deny should be default
	if !dialog.Options[1].Default {
		t.Error("Expected Deny option to be default")
	}
}

func TestNewConfirmDialog(t *testing.T) {
	id := "confirm_123"
	title := "Confirm Action"
	content := "Are you sure?"

	dialog := NewConfirmDialog(id, title, content)

	if dialog.Type != DialogTypeConfirm {
		t.Errorf("Expected type %s, got %s", DialogTypeConfirm, dialog.Type)
	}
	if len(dialog.Options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(dialog.Options))
	}

	// Check options
	if dialog.Options[0].Value != string(ActionConfirm) {
		t.Errorf("Expected first option to be %s, got %s", ActionConfirm, dialog.Options[0].Value)
	}
	if dialog.Options[1].Value != string(ActionCancel) {
		t.Errorf("Expected second option to be %s, got %s", ActionCancel, dialog.Options[1].Value)
	}

	// Cancel should be default
	if !dialog.Options[1].Default {
		t.Error("Expected Cancel option to be default")
	}
}

func TestNewInputDialog(t *testing.T) {
	id := "input_123"
	title := "Enter Value"
	content := "Please enter a value"
	placeholder := "Enter text here"

	dialog := NewInputDialog(id, title, content, placeholder)

	if dialog.Type != DialogTypeInput {
		t.Errorf("Expected type %s, got %s", DialogTypeInput, dialog.Type)
	}
	if dialog.Timeout != 0 {
		t.Errorf("Expected no timeout for input dialog, got %v", dialog.Timeout)
	}
	if len(dialog.Options) != 1 {
		t.Errorf("Expected 1 option, got %d", len(dialog.Options))
	}

	// Check placeholder in option data
	if dialog.Options[0].Data["placeholder"] != placeholder {
		t.Errorf("Expected placeholder %s, got %v", placeholder, dialog.Options[0].Data["placeholder"])
	}
}

func TestDialogValidation(t *testing.T) {
	tests := []struct {
		name     string
		dialog   *Dialog
		response *DialogResponse
		valid    bool
		errors   int
	}{
		{
			name: "valid permission allow",
			dialog: &Dialog{
				ID:   "perm_123",
				Type: DialogTypePermission,
			},
			response: &DialogResponse{
				DialogID: "perm_123",
				Action:   ActionAllow,
			},
			valid:  true,
			errors: 0,
		},
		{
			name: "invalid permission action",
			dialog: &Dialog{
				ID:   "perm_123",
				Type: DialogTypePermission,
			},
			response: &DialogResponse{
				DialogID: "perm_123",
				Action:   ActionSubmit, // Invalid for permission
			},
			valid:  false,
			errors: 1,
		},
		{
			name: "dialog ID mismatch",
			dialog: &Dialog{
				ID:   "perm_123",
				Type: DialogTypePermission,
			},
			response: &DialogResponse{
				DialogID: "different_id",
				Action:   ActionAllow,
			},
			valid:  false,
			errors: 1,
		},
		{
			name: "valid confirm dialog",
			dialog: &Dialog{
				ID:   "confirm_123",
				Type: DialogTypeConfirm,
			},
			response: &DialogResponse{
				DialogID: "confirm_123",
				Action:   ActionConfirm,
			},
			valid:  true,
			errors: 0,
		},
		{
			name: "valid input dialog with value",
			dialog: &Dialog{
				ID:   "input_123",
				Type: DialogTypeInput,
			},
			response: &DialogResponse{
				DialogID: "input_123",
				Action:   ActionSubmit,
				Value:    "user input",
			},
			valid:  true,
			errors: 0,
		},
		{
			name: "invalid input dialog without value",
			dialog: &Dialog{
				ID:   "input_123",
				Type: DialogTypeInput,
			},
			response: &DialogResponse{
				DialogID: "input_123",
				Action:   ActionSubmit,
				Value:    "", // Empty value
			},
			valid:  false,
			errors: 1,
		},
		{
			name: "valid select dialog",
			dialog: &Dialog{
				ID:   "select_123",
				Type: DialogTypeSelect,
			},
			response: &DialogResponse{
				DialogID:  "select_123",
				Action:    ActionSelect,
				Selection: []string{"option1"},
			},
			valid:  true,
			errors: 0,
		},
		{
			name: "invalid select dialog without selection",
			dialog: &Dialog{
				ID:   "select_123",
				Type: DialogTypeSelect,
			},
			response: &DialogResponse{
				DialogID:  "select_123",
				Action:    ActionSelect,
				Selection: []string{}, // Empty selection
			},
			valid:  false,
			errors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dialog.Validate(tt.response)
			
			if result.Valid != tt.valid {
				t.Errorf("Expected valid %v, got %v", tt.valid, result.Valid)
			}
			
			if len(result.Errors) != tt.errors {
				t.Errorf("Expected %d errors, got %d", tt.errors, len(result.Errors))
			}
		})
	}
}

func TestNewUserRequest(t *testing.T) {
	reqType := RequestTypeChat
	content := "Hello, world!"

	req := NewUserRequest(reqType, content)

	if req.Type != reqType {
		t.Errorf("Expected type %s, got %s", reqType, req.Type)
	}
	if req.Content != content {
		t.Errorf("Expected content %s, got %s", content, req.Content)
	}
	if req.Options == nil {
		t.Error("Expected options map to be initialized")
	}
}

func TestUserRequest_AddFile(t *testing.T) {
	req := NewUserRequest(RequestTypeChat, "test")
	
	name := "test.txt"
	content := []byte("test content")
	mimeType := "text/plain"

	req.AddFile(name, content, mimeType)

	if !req.HasFiles() {
		t.Error("Expected request to have files")
	}
	if len(req.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(req.Files))
	}
	if req.Files[0].Name != name {
		t.Errorf("Expected file name %s, got %s", name, req.Files[0].Name)
	}
	if req.Files[0].Size != int64(len(content)) {
		t.Errorf("Expected file size %d, got %d", len(content), req.Files[0].Size)
	}
}

func TestUserRequest_SetGetOption(t *testing.T) {
	req := NewUserRequest(RequestTypeChat, "test")
	
	key := "model"
	value := "claude-3"

	req.SetOption(key, value)

	retrievedValue, exists := req.GetOption(key)
	if !exists {
		t.Error("Expected option to exist")
	}
	if retrievedValue != value {
		t.Errorf("Expected option value %s, got %s", value, retrievedValue)
	}

	// Test non-existent option
	_, exists = req.GetOption("nonexistent")
	if exists {
		t.Error("Expected non-existent option to not exist")
	}
}

func TestUserRequest_TotalFileSize(t *testing.T) {
	req := NewUserRequest(RequestTypeChat, "test")
	
	req.AddFile("file1.txt", []byte("content1"), "text/plain")
	req.AddFile("file2.txt", []byte("content22"), "text/plain")

	expectedSize := int64(len("content1") + len("content22"))
	totalSize := req.TotalFileSize()

	if totalSize != expectedSize {
		t.Errorf("Expected total file size %d, got %d", expectedSize, totalSize)
	}
}

func TestNewNetworkMessage(t *testing.T) {
	msgType := "test_message"
	data := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	msg := NewNetworkMessage(msgType, data)

	if msg.Type != msgType {
		t.Errorf("Expected type %s, got %s", msgType, msg.Type)
	}
	if msg.Data["key1"] != "value1" {
		t.Errorf("Expected key1 value 'value1', got %v", msg.Data["key1"])
	}
	if msg.Data["key2"] != 42 {
		t.Errorf("Expected key2 value 42, got %v", msg.Data["key2"])
	}
	if msg.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}