package remote

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	ui "github.com/sst/opencode/internal/interface"
	"github.com/sst/opencode/internal/message"
	"github.com/sst/opencode/internal/pubsub"
	"github.com/sst/opencode/internal/session"
	"github.com/sst/opencode/internal/status"
)

func TestNewRemoteUI(t *testing.T) {
	clientID := "test_client"
	server := NewWebSocketServer(nil)
	
	remoteUI := NewRemoteUI(clientID, server)
	
	assert.Equal(t, clientID, remoteUI.clientID)
	assert.Equal(t, server, remoteUI.server)
	assert.NotNil(t, remoteUI.eventChan)
	assert.NotNil(t, remoteUI.pendingDialogs)
	assert.NotNil(t, remoteUI.ctx)
	assert.NotNil(t, remoteUI.cancel)
}

func TestRemoteUI_SetGetSessionID(t *testing.T) {
	remoteUI := NewRemoteUI("test_client", NewWebSocketServer(nil))
	
	sessionID := "test_session"
	remoteUI.SetSessionID(sessionID)
	
	assert.Equal(t, sessionID, remoteUI.GetSessionID())
}

func TestRemoteUI_GetClientID(t *testing.T) {
	clientID := "test_client"
	remoteUI := NewRemoteUI(clientID, NewWebSocketServer(nil))
	
	assert.Equal(t, clientID, remoteUI.GetClientID())
}

func TestRemoteUI_DisplayMessage(t *testing.T) {
	server := NewWebSocketServer(nil)
	remoteUI := NewRemoteUI("test_client", server)
	remoteUI.SetSessionID("test_session")
	
	msg := &message.Message{
		ID:        "msg_123",
		Role:      message.Assistant,
		SessionID: "test_session",
		Parts: []message.ContentPart{
			message.TextContent{Text: "Hello, world!"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	err := remoteUI.DisplayMessage(msg)
	// Since there are no subscribed clients, this may not error
	// but we can test that the method doesn't panic
	assert.NoError(t, err)
}

func TestRemoteUI_DisplayStatus(t *testing.T) {
	server := NewWebSocketServer(nil)
	remoteUI := NewRemoteUI("test_client", server)
	
	status := &status.StatusMessage{
		Level:     status.LevelInfo,
		Message:   "Test status message",
		Timestamp: time.Now(),
		Critical:  false,
	}
	
	err := remoteUI.DisplayStatus(status)
	// Since there are no subscribed clients, this may not error
	// but we can test that the method doesn't panic
	assert.NoError(t, err)
}

func TestRemoteUI_DisplayDialog(t *testing.T) {
	server := NewWebSocketServer(nil)
	remoteUI := NewRemoteUI("test_client", server)
	
	dialog := &ui.Dialog{
		Type:    ui.DialogTypeConfirm,
		Title:   "Test Dialog",
		Content: "Are you sure?",
		Timeout: 100 * time.Millisecond, // Short timeout for test
	}
	
	// This should timeout since no client is connected
	response, err := remoteUI.DisplayDialog(dialog)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Cancelled)
	assert.Equal(t, ui.ActionCancel, response.Action)
}

func TestRemoteUI_DisplayDialog_WithResponse(t *testing.T) {
	server := NewWebSocketServer(nil)
	remoteUI := NewRemoteUI("test_client", server)
	
	dialog := &ui.Dialog{
		ID:      "dialog_123",
		Type:    ui.DialogTypeConfirm,
		Title:   "Test Dialog",
		Content: "Are you sure?",
		Timeout: 1 * time.Second,
	}
	
	// Start dialog in background
	var response *ui.DialogResponse
	var err error
	done := make(chan bool)
	
	go func() {
		response, err = remoteUI.DisplayDialog(dialog)
		done <- true
	}()
	
	// Wait a moment for dialog to be set up
	time.Sleep(50 * time.Millisecond)
	
	// Send response
	dialogResponse := &ui.DialogResponse{
		DialogID:  "dialog_123",
		Action:    ui.ActionConfirm,
		Timestamp: time.Now(),
	}
	
	err2 := remoteUI.HandleDialogResponse("dialog_123", dialogResponse)
	assert.NoError(t, err2)
	
	// Wait for dialog to complete
	<-done
	
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, ui.ActionConfirm, response.Action)
	assert.False(t, response.Cancelled)
}

func TestRemoteUI_ShowSessionList(t *testing.T) {
	server := NewWebSocketServer(nil)
	remoteUI := NewRemoteUI("test_client", server)
	
	sessions := []*session.Session{
		{
			ID:           "session_1",
			Title:        "Session 1",
			MessageCount: 5,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "session_2",
			Title:        "Session 2",
			MessageCount: 3,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}
	
	err := remoteUI.ShowSessionList(sessions)
	// Since there are no subscribed clients, this may not error
	// but we can test that the method doesn't panic
	assert.NoError(t, err)
}

func TestRemoteUI_Subscribe(t *testing.T) {
	server := NewWebSocketServer(nil)
	remoteUI := NewRemoteUI("test_client", server)
	
	events := []pubsub.EventType{"event1", "event2"}
	ch := remoteUI.Subscribe(events...)
	
	assert.NotNil(t, ch)
	// Convert to comparable types for assertion
	assert.Equal(t, (<-chan pubsub.Event[interface{}])(remoteUI.eventChan), ch)
}

func TestRemoteUI_PublishEvent(t *testing.T) {
	server := NewWebSocketServer(nil)
	remoteUI := NewRemoteUI("test_client", server)
	
	eventType := pubsub.EventType("test_event")
	payload := map[string]interface{}{"test": "data"}
	
	// Subscribe to events
	ch := remoteUI.Subscribe(eventType)
	
	// Publish event
	remoteUI.PublishEvent(eventType, payload)
	
	// Check if event was received
	select {
	case event := <-ch:
		assert.Equal(t, eventType, event.Type)
		assert.Equal(t, payload, event.Payload)
	case <-time.After(100 * time.Millisecond):
		t.Error("Event was not received")
	}
}

func TestRemoteUI_HandleDialogResponse_NotFound(t *testing.T) {
	remoteUI := NewRemoteUI("test_client", NewWebSocketServer(nil))
	
	response := &ui.DialogResponse{
		DialogID: "nonexistent",
		Action:   ui.ActionConfirm,
	}
	
	err := remoteUI.HandleDialogResponse("nonexistent", response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pending dialog found")
}

func TestRemoteUI_HandleDialogResponse_InvalidResponse(t *testing.T) {
	remoteUI := NewRemoteUI("test_client", NewWebSocketServer(nil))
	
	// Create a permission dialog
	dialog := &ui.Dialog{
		ID:   "dialog_123",
		Type: ui.DialogTypePermission,
	}
	
	pending := &PendingDialog{
		ID:       dialog.ID,
		Dialog:   dialog,
		Response: make(chan *ui.DialogResponse, 1),
		Created:  time.Now(),
	}
	
	remoteUI.mu.Lock()
	remoteUI.pendingDialogs[dialog.ID] = pending
	remoteUI.mu.Unlock()
	
	// Send invalid response (wrong action for permission dialog)
	response := &ui.DialogResponse{
		DialogID: "dialog_123",
		Action:   ui.ActionSubmit, // Invalid for permission dialog
	}
	
	err := remoteUI.HandleDialogResponse("dialog_123", response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid dialog response")
}

func TestRemoteUI_Close(t *testing.T) {
	server := NewWebSocketServer(nil)
	remoteUI := NewRemoteUI("test_client", server)
	
	err := remoteUI.Close()
	// Will error because client doesn't exist in server, but should not panic
	assert.Error(t, err)
	
	// Check that context was cancelled
	select {
	case <-remoteUI.ctx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled after Close()")
	}
}

func TestRemoteUI_CleanupExpiredDialogs(t *testing.T) {
	remoteUI := NewRemoteUI("test_client", NewWebSocketServer(nil))
	
	// Create an expired dialog
	expiredDialog := &ui.Dialog{
		ID:      "expired_dialog",
		Type:    ui.DialogTypeConfirm,
		Timeout: 10 * time.Millisecond,
	}
	
	pending := &PendingDialog{
		ID:       expiredDialog.ID,
		Dialog:   expiredDialog,
		Response: make(chan *ui.DialogResponse, 1),
		Created:  time.Now().Add(-1 * time.Minute), // Created 1 minute ago
	}
	
	remoteUI.mu.Lock()
	remoteUI.pendingDialogs[expiredDialog.ID] = pending
	remoteUI.mu.Unlock()
	
	// Cleanup expired dialogs
	remoteUI.CleanupExpiredDialogs()
	
	// Check that dialog was removed
	remoteUI.mu.RLock()
	_, exists := remoteUI.pendingDialogs[expiredDialog.ID]
	remoteUI.mu.RUnlock()
	
	assert.False(t, exists, "Expired dialog should be removed")
	
	// Check that timeout response was sent
	select {
	case response := <-pending.Response:
		assert.Equal(t, ui.ActionCancel, response.Action)
		assert.True(t, response.Cancelled)
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout response should have been sent")
	}
}

func TestRemoteUI_SerializeMethods(t *testing.T) {
	remoteUI := NewRemoteUI("test_client", NewWebSocketServer(nil))
	
	// Test serializeMessage
	msg := &message.Message{
		ID:        "msg_123",
		Role:      message.Assistant,
		SessionID: "session_123",
		Parts: []message.ContentPart{
			message.TextContent{Text: "Hello"},
			message.ToolCall{
				ID:    "call_123",
				Name:  "test_tool",
				Input: `{"arg": "value"}`,
				Type:  "function",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	serialized := remoteUI.serializeMessage(msg)
	assert.Equal(t, msg.ID, serialized["id"])
	assert.Equal(t, string(msg.Role), serialized["role"])
	assert.Equal(t, msg.SessionID, serialized["session_id"])
	
	parts, ok := serialized["parts"].([]map[string]interface{})
	assert.True(t, ok)
	assert.Len(t, parts, 2)
	assert.Equal(t, "text", parts[0]["type"])
	assert.Equal(t, "tool_call", parts[1]["type"])
	
	// Test serializeStatus
	status := &status.StatusMessage{
		Level:     status.LevelInfo,
		Message:   "Test message",
		Timestamp: time.Now(),
		Critical:  true,
		Duration:  5 * time.Second,
	}
	
	serializedStatus := remoteUI.serializeStatus(status)
	assert.Equal(t, string(status.Level), serializedStatus["level"])
	assert.Equal(t, status.Message, serializedStatus["message"])
	assert.Equal(t, status.Critical, serializedStatus["critical"])
	
	// Test serializeSessions
	sessions := []*session.Session{
		{
			ID:           "session_1",
			Title:        "Test Session",
			MessageCount: 5,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}
	
	serializedSessions := remoteUI.serializeSessions(sessions)
	assert.Len(t, serializedSessions, 1)
	assert.Equal(t, "session_1", serializedSessions[0]["id"])
	assert.Equal(t, "Test Session", serializedSessions[0]["title"])
	assert.Equal(t, int64(5), serializedSessions[0]["message_count"])
}