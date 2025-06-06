package ui

import (
	"context"
	"testing"
	"time"

	"github.com/sst/opencode/internal/message"
	"github.com/sst/opencode/internal/pubsub"
	"github.com/sst/opencode/internal/session"
	"github.com/sst/opencode/internal/status"
)

// MockUI is a test implementation of UserInterface for testing purposes.
type MockUI struct {
	userInputResponses   []UserRequest
	userInputErrors      []error
	userInputCallCount   int
	displayedMessages    []*message.Message
	displayedStatuses    []*status.StatusMessage
	displayedDialogs     []*Dialog
	dialogResponses      []DialogResponse
	dialogErrors         []error
	dialogCallCount      int
	sessionLists         [][]*session.Session
	filePickerResponses  []FileSelection
	filePickerErrors     []error
	filePickerCallCount  int
	eventSubscriptions   [][]pubsub.EventType
	eventChannels        []chan pubsub.Event[interface{}]
	closed               bool
}

func NewMockUI() *MockUI {
	return &MockUI{
		eventChannels: make([]chan pubsub.Event[interface{}], 0),
	}
}

func (m *MockUI) GetUserInput(ctx context.Context) (*UserRequest, error) {
	if m.userInputCallCount >= len(m.userInputResponses) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	response := m.userInputResponses[m.userInputCallCount]
	var err error
	if m.userInputCallCount < len(m.userInputErrors) {
		err = m.userInputErrors[m.userInputCallCount]
	}
	m.userInputCallCount++
	return &response, err
}

func (m *MockUI) DisplayMessage(msg *message.Message) error {
	m.displayedMessages = append(m.displayedMessages, msg)
	return nil
}

func (m *MockUI) DisplayStatus(status *status.StatusMessage) error {
	m.displayedStatuses = append(m.displayedStatuses, status)
	return nil
}

func (m *MockUI) DisplayDialog(dialog *Dialog) (*DialogResponse, error) {
	m.displayedDialogs = append(m.displayedDialogs, dialog)
	
	if m.dialogCallCount >= len(m.dialogResponses) {
		return nil, context.DeadlineExceeded
	}

	response := m.dialogResponses[m.dialogCallCount]
	var err error
	if m.dialogCallCount < len(m.dialogErrors) {
		err = m.dialogErrors[m.dialogCallCount]
	}
	m.dialogCallCount++
	return &response, err
}

func (m *MockUI) ShowSessionList(sessions []*session.Session) error {
	m.sessionLists = append(m.sessionLists, sessions)
	return nil
}

func (m *MockUI) ShowFilePicker(opts *FilePickerOpts) (*FileSelection, error) {
	if m.filePickerCallCount >= len(m.filePickerResponses) {
		return &FileSelection{Cancelled: true}, nil
	}

	response := m.filePickerResponses[m.filePickerCallCount]
	var err error
	if m.filePickerCallCount < len(m.filePickerErrors) {
		err = m.filePickerErrors[m.filePickerCallCount]
	}
	m.filePickerCallCount++
	return &response, err
}

func (m *MockUI) Subscribe(events ...pubsub.EventType) <-chan pubsub.Event[interface{}] {
	m.eventSubscriptions = append(m.eventSubscriptions, events)
	ch := make(chan pubsub.Event[interface{}], 1)
	m.eventChannels = append(m.eventChannels, ch)
	return ch
}

func (m *MockUI) Close() error {
	m.closed = true
	// Close all event channels
	for _, ch := range m.eventChannels {
		close(ch)
	}
	return nil
}

// Helper methods for testing
func (m *MockUI) SetUserInputResponses(responses []UserRequest, errors []error) {
	m.userInputResponses = responses
	m.userInputErrors = errors
}

func (m *MockUI) SetDialogResponses(responses []DialogResponse, errors []error) {
	m.dialogResponses = responses
	m.dialogErrors = errors
}

func (m *MockUI) SetFilePickerResponses(responses []FileSelection, errors []error) {
	m.filePickerResponses = responses
	m.filePickerErrors = errors
}

func (m *MockUI) GetDisplayedMessages() []*message.Message {
	return m.displayedMessages
}

func (m *MockUI) GetDisplayedStatuses() []*status.StatusMessage {
	return m.displayedStatuses
}

func (m *MockUI) GetDisplayedDialogs() []*Dialog {
	return m.displayedDialogs
}

func (m *MockUI) IsClosed() bool {
	return m.closed
}

// Tests
func TestUserRequest_JSON(t *testing.T) {
	req := UserRequest{
		Type:    RequestTypeChat,
		Content: "Hello, world!",
		Files: []Attachment{
			{
				Name:     "test.txt",
				Content:  []byte("test content"),
				MimeType: "text/plain",
				Size:     12,
			},
		},
		Options: map[string]interface{}{
			"model": "claude-3",
		},
	}

	if req.Type != RequestTypeChat {
		t.Errorf("Expected type %s, got %s", RequestTypeChat, req.Type)
	}
	if req.Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got '%s'", req.Content)
	}
	if len(req.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(req.Files))
	}
	if req.Files[0].Name != "test.txt" {
		t.Errorf("Expected file name 'test.txt', got '%s'", req.Files[0].Name)
	}
}

func TestDialogResponse_JSON(t *testing.T) {
	timestamp := time.Now()
	resp := DialogResponse{
		DialogID:  "dialog_123",
		Action:    ActionConfirm,
		Value:     "yes",
		Selection: []string{"option1", "option2"},
		Data: map[string]interface{}{
			"extra": "data",
		},
		Cancelled: false,
		Timestamp: timestamp,
	}

	if resp.DialogID != "dialog_123" {
		t.Errorf("Expected dialog ID 'dialog_123', got '%s'", resp.DialogID)
	}
	if resp.Action != ActionConfirm {
		t.Errorf("Expected action %s, got %s", ActionConfirm, resp.Action)
	}
	if resp.Cancelled {
		t.Errorf("Expected cancelled to be false")
	}
	if !resp.Timestamp.Equal(timestamp) {
		t.Errorf("Expected timestamp %v, got %v", timestamp, resp.Timestamp)
	}
}

func TestMockUI_UserInput(t *testing.T) {
	mock := NewMockUI()
	
	// Set up responses
	responses := []UserRequest{
		{Type: RequestTypeChat, Content: "Hello"},
		{Type: RequestTypeCommand, Content: "/help"},
	}
	mock.SetUserInputResponses(responses, nil)

	ctx := context.Background()

	// Test first call
	req1, err := mock.GetUserInput(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if req1.Type != RequestTypeChat {
		t.Errorf("Expected type %s, got %s", RequestTypeChat, req1.Type)
	}

	// Test second call
	req2, err := mock.GetUserInput(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if req2.Type != RequestTypeCommand {
		t.Errorf("Expected type %s, got %s", RequestTypeCommand, req2.Type)
	}
}

func TestMockUI_DisplayMessage(t *testing.T) {
	mock := NewMockUI()
	
	msg := &message.Message{
		ID:   "msg_123",
		Role: message.Assistant,
	}

	err := mock.DisplayMessage(msg)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	displayed := mock.GetDisplayedMessages()
	if len(displayed) != 1 {
		t.Errorf("Expected 1 displayed message, got %d", len(displayed))
	}
	if displayed[0].ID != "msg_123" {
		t.Errorf("Expected message ID 'msg_123', got '%s'", displayed[0].ID)
	}
}

func TestMockUI_DisplayDialog(t *testing.T) {
	mock := NewMockUI()
	
	// Set up dialog response
	response := DialogResponse{
		DialogID: "dialog_123",
		Action:   ActionConfirm,
	}
	mock.SetDialogResponses([]DialogResponse{response}, nil)

	dialog := &Dialog{
		ID:      "dialog_123",
		Type:    DialogTypeConfirm,
		Title:   "Confirm Action",
		Content: "Are you sure?",
	}

	resp, err := mock.DisplayDialog(dialog)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp.Action != ActionConfirm {
		t.Errorf("Expected action %s, got %s", ActionConfirm, resp.Action)
	}

	displayed := mock.GetDisplayedDialogs()
	if len(displayed) != 1 {
		t.Errorf("Expected 1 displayed dialog, got %d", len(displayed))
	}
}

func TestMockUI_Subscribe(t *testing.T) {
	mock := NewMockUI()
	
	events := []pubsub.EventType{"event1", "event2"}
	ch := mock.Subscribe(events...)

	if ch == nil {
		t.Error("Expected non-nil channel")
	}

	// Verify subscription was recorded
	if len(mock.eventSubscriptions) != 1 {
		t.Errorf("Expected 1 subscription, got %d", len(mock.eventSubscriptions))
	}
	if len(mock.eventSubscriptions[0]) != 2 {
		t.Errorf("Expected 2 event types, got %d", len(mock.eventSubscriptions[0]))
	}
}

func TestMockUI_Close(t *testing.T) {
	mock := NewMockUI()
	
	// Subscribe to create a channel
	mock.Subscribe("test")
	
	err := mock.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if !mock.IsClosed() {
		t.Error("Expected UI to be closed")
	}
}