package remote

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	ui "github.com/sst/opencode/internal/interface"
	"github.com/sst/opencode/internal/message"
	"github.com/sst/opencode/internal/pubsub"
	"github.com/sst/opencode/internal/session"
	"github.com/sst/opencode/internal/status"
)

// RemoteUI implements the UserInterface for remote clients via WebSocket connections.
type RemoteUI struct {
	clientID       string
	sessionID      string
	server         *WebSocketServer
	eventChan      chan pubsub.Event[interface{}]
	pendingDialogs map[string]*PendingDialog
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
}

// PendingDialog represents a dialog waiting for user response.
type PendingDialog struct {
	ID       string
	Dialog   *ui.Dialog
	Response chan *ui.DialogResponse
	Created  time.Time
}

// NewRemoteUI creates a new RemoteUI instance.
func NewRemoteUI(clientID string, server *WebSocketServer) *RemoteUI {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &RemoteUI{
		clientID:       clientID,
		server:         server,
		eventChan:      make(chan pubsub.Event[interface{}], 100),
		pendingDialogs: make(map[string]*PendingDialog),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// GetUserInput implements UserInterface.GetUserInput.
func (r *RemoteUI) GetUserInput(ctx context.Context) (*ui.UserRequest, error) {
	// Send request to client asking for user input
	request := NewRequest(ActionUserInput, map[string]interface{}{
		"prompt": "waiting for user input",
	})

	if err := r.server.SendRequest(r.clientID, request); err != nil {
		return nil, fmt.Errorf("failed to send user input request: %w", err)
	}

	// Wait for response from client
	// In a real implementation, this would listen for WebSocket messages
	// and match them to pending requests
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute): // Timeout after 5 minutes
		return nil, fmt.Errorf("user input request timed out")
	}
}

// DisplayMessage implements UserInterface.DisplayMessage.
func (r *RemoteUI) DisplayMessage(msg *message.Message) error {
	// Convert message to network format and send to client
	event := NewEvent(EventTypeMessageUpdate, map[string]interface{}{
		"message": r.serializeMessage(msg),
	})
	event.SessionID = r.sessionID

	if err := r.server.BroadcastEvent(event); err != nil {
		return fmt.Errorf("failed to send message to client: %w", err)
	}

	slog.Debug("Message sent to remote client", "client_id", r.clientID, "message_id", msg.ID)
	return nil
}

// DisplayStatus implements UserInterface.DisplayStatus.
func (r *RemoteUI) DisplayStatus(status *status.StatusMessage) error {
	// Convert status to network format and send to client
	event := NewEvent(EventTypeStatusUpdate, map[string]interface{}{
		"status": r.serializeStatus(status),
	})
	event.SessionID = r.sessionID

	if err := r.server.BroadcastEvent(event); err != nil {
		return fmt.Errorf("failed to send status to client: %w", err)
	}

	slog.Debug("Status sent to remote client", "client_id", r.clientID, "level", status.Level)
	return nil
}

// DisplayDialog implements UserInterface.DisplayDialog.
func (r *RemoteUI) DisplayDialog(dialog *ui.Dialog) (*ui.DialogResponse, error) {
	if dialog.ID == "" {
		dialog.ID = uuid.New().String()
	}

	// Create pending dialog
	pendingDialog := &PendingDialog{
		ID:       dialog.ID,
		Dialog:   dialog,
		Response: make(chan *ui.DialogResponse, 1),
		Created:  time.Now(),
	}

	r.mu.Lock()
	r.pendingDialogs[dialog.ID] = pendingDialog
	r.mu.Unlock()

	// Clean up pending dialog when done
	defer func() {
		r.mu.Lock()
		delete(r.pendingDialogs, dialog.ID)
		r.mu.Unlock()
		close(pendingDialog.Response)
	}()

	// Send dialog to client
	event := NewEvent(EventTypeDialogShow, map[string]interface{}{
		"dialog": dialog,
	})
	event.SessionID = r.sessionID

	if err := r.server.BroadcastEvent(event); err != nil {
		return nil, fmt.Errorf("failed to send dialog to client: %w", err)
	}

	// Wait for response
	timeout := dialog.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute // Default timeout
	}

	select {
	case response := <-pendingDialog.Response:
		return response, nil
	case <-time.After(timeout):
		return &ui.DialogResponse{
			DialogID:  dialog.ID,
			Action:    ui.ActionCancel,
			Cancelled: true,
			Timestamp: time.Now(),
		}, nil
	case <-r.ctx.Done():
		return nil, r.ctx.Err()
	}
}

// ShowSessionList implements UserInterface.ShowSessionList.
func (r *RemoteUI) ShowSessionList(sessions []*session.Session) error {
	// Convert sessions to network format and send to client
	event := NewEvent(EventTypeSessionUpdate, map[string]interface{}{
		"action":   "list",
		"sessions": r.serializeSessions(sessions),
	})

	if err := r.server.BroadcastEvent(event); err != nil {
		return fmt.Errorf("failed to send session list to client: %w", err)
	}

	slog.Debug("Session list sent to remote client", "client_id", r.clientID, "count", len(sessions))
	return nil
}

// ShowFilePicker implements UserInterface.ShowFilePicker.
func (r *RemoteUI) ShowFilePicker(opts *ui.FilePickerOpts) (*ui.FileSelection, error) {
	// Send file picker request to client
	request := NewRequest(ActionFilePicker, map[string]interface{}{
		"options": opts,
	})

	if err := r.server.SendRequest(r.clientID, request); err != nil {
		return nil, fmt.Errorf("failed to send file picker request: %w", err)
	}

	// Wait for response from client
	// In a real implementation, this would listen for WebSocket messages
	select {
	case <-time.After(2 * time.Minute): // Timeout after 2 minutes
		return &ui.FileSelection{
			Paths:     []string{},
			Cancelled: true,
		}, nil
	case <-r.ctx.Done():
		return nil, r.ctx.Err()
	}
}

// Subscribe implements UserInterface.Subscribe.
func (r *RemoteUI) Subscribe(events ...pubsub.EventType) <-chan pubsub.Event[interface{}] {
	// Convert event types to strings and subscribe client
	eventStrings := make([]string, len(events))
	for i, event := range events {
		eventStrings[i] = string(event)
	}

	if err := r.server.Subscribe(r.clientID, eventStrings...); err != nil {
		slog.Error("Failed to subscribe client to events", "client_id", r.clientID, "events", eventStrings, "error", err)
	}

	return r.eventChan
}

// Close implements UserInterface.Close.
func (r *RemoteUI) Close() error {
	r.cancel()
	close(r.eventChan)

	// Disconnect client from server
	if err := r.server.DisconnectClient(r.clientID); err != nil {
		slog.Error("Failed to disconnect client", "client_id", r.clientID, "error", err)
		return err
	}

	slog.Info("Remote UI closed", "client_id", r.clientID)
	return nil
}

// HandleDialogResponse handles a dialog response from the client.
func (r *RemoteUI) HandleDialogResponse(dialogID string, response *ui.DialogResponse) error {
	r.mu.RLock()
	pendingDialog, exists := r.pendingDialogs[dialogID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no pending dialog found with ID %s", dialogID)
	}

	// Validate response
	if result := pendingDialog.Dialog.Validate(response); !result.Valid {
		return fmt.Errorf("invalid dialog response: %v", result.Errors)
	}

	// Send response to waiting goroutine
	select {
	case pendingDialog.Response <- response:
		return nil
	default:
		return fmt.Errorf("dialog response channel is full or closed")
	}
}

// HandleUserInput handles a user input response from the client.
func (r *RemoteUI) HandleUserInput(userRequest *ui.UserRequest) error {
	// In a real implementation, this would forward the user input to the waiting
	// GetUserInput method. For now, we'll just log it.
	slog.Debug("Received user input from remote client", 
		"client_id", r.clientID, 
		"type", userRequest.Type, 
		"content_length", len(userRequest.Content))
	return nil
}

// HandleFilePickerResponse handles a file picker response from the client.
func (r *RemoteUI) HandleFilePickerResponse(selection *ui.FileSelection) error {
	// In a real implementation, this would forward the file selection to the waiting
	// ShowFilePicker method. For now, we'll just log it.
	slog.Debug("Received file picker response from remote client", 
		"client_id", r.clientID, 
		"files_selected", len(selection.Paths), 
		"cancelled", selection.Cancelled)
	return nil
}

// SetSessionID sets the current session ID for this UI.
func (r *RemoteUI) SetSessionID(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessionID = sessionID
}

// GetSessionID returns the current session ID.
func (r *RemoteUI) GetSessionID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionID
}

// GetClientID returns the client ID.
func (r *RemoteUI) GetClientID() string {
	return r.clientID
}

// Helper methods for serialization

func (r *RemoteUI) serializeMessage(msg *message.Message) map[string]interface{} {
	return map[string]interface{}{
		"id":         msg.ID,
		"role":       string(msg.Role),
		"session_id": msg.SessionID,
		"parts":      r.serializeMessageParts(msg.Parts),
		"model":      string(msg.Model),
		"created_at": msg.CreatedAt.Format(time.RFC3339Nano),
		"updated_at": msg.UpdatedAt.Format(time.RFC3339Nano),
	}
}

func (r *RemoteUI) serializeMessageParts(parts []message.ContentPart) []map[string]interface{} {
	serialized := make([]map[string]interface{}, len(parts))
	for i, part := range parts {
		switch p := part.(type) {
		case message.TextContent:
			serialized[i] = map[string]interface{}{
				"type": "text",
				"text": p.Text,
			}
		case message.ReasoningContent:
			serialized[i] = map[string]interface{}{
				"type":     "reasoning",
				"thinking": p.Thinking,
			}
		case message.ImageURLContent:
			serialized[i] = map[string]interface{}{
				"type":   "image_url",
				"url":    p.URL,
				"detail": p.Detail,
			}
		case message.BinaryContent:
			serialized[i] = map[string]interface{}{
				"type":      "binary",
				"data":      p.Data,
				"mime_type": p.MIMEType,
				"path":      p.Path,
			}
		case message.ToolCall:
			serialized[i] = map[string]interface{}{
				"type":     "tool_call",
				"id":       p.ID,
				"name":     p.Name,
				"input":    p.Input,
				"type_str": p.Type,
				"finished": p.Finished,
			}
		case message.ToolResult:
			serialized[i] = map[string]interface{}{
				"type":        "tool_result",
				"tool_call_id": p.ToolCallID,
				"name":        p.Name,
				"content":     p.Content,
				"metadata":    p.Metadata,
				"is_error":    p.IsError,
			}
		case message.Finish:
			serialized[i] = map[string]interface{}{
				"type":   "finish",
				"reason": string(p.Reason),
				"time":   p.Time.Format(time.RFC3339Nano),
			}
		default:
			serialized[i] = map[string]interface{}{
				"type": "unknown",
				"data": fmt.Sprintf("%+v", part),
			}
		}
	}
	return serialized
}

func (r *RemoteUI) serializeStatus(status *status.StatusMessage) map[string]interface{} {
	return map[string]interface{}{
		"level":     string(status.Level),
		"message":   status.Message,
		"timestamp": status.Timestamp.Format(time.RFC3339Nano),
		"critical":  status.Critical,
		"duration":  status.Duration.String(),
	}
}

func (r *RemoteUI) serializeSessions(sessions []*session.Session) []map[string]interface{} {
	serialized := make([]map[string]interface{}, len(sessions))
	for i, s := range sessions {
		serialized[i] = map[string]interface{}{
			"id":                s.ID,
			"parent_session_id": s.ParentSessionID,
			"title":             s.Title,
			"message_count":     s.MessageCount,
			"prompt_tokens":     s.PromptTokens,
			"completion_tokens": s.CompletionTokens,
			"cost":              s.Cost,
			"summary":           s.Summary,
			"summarized_at":     s.SummarizedAt.Format(time.RFC3339Nano),
			"created_at":        s.CreatedAt.Format(time.RFC3339Nano),
			"updated_at":        s.UpdatedAt.Format(time.RFC3339Nano),
		}
	}
	return serialized
}

// PublishEvent publishes an event to the UI's event channel.
func (r *RemoteUI) PublishEvent(eventType pubsub.EventType, payload interface{}) {
	event := pubsub.Event[interface{}]{
		Type:    eventType,
		Payload: payload,
	}

	select {
	case r.eventChan <- event:
	default:
		slog.Warn("Event channel full, dropping event", "client_id", r.clientID, "event_type", eventType)
	}
}

// CleanupExpiredDialogs removes expired pending dialogs.
func (r *RemoteUI) CleanupExpiredDialogs() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for id, dialog := range r.pendingDialogs {
		// Check if dialog has expired (5 minutes default)
		timeout := dialog.Dialog.Timeout
		if timeout == 0 {
			timeout = 5 * time.Minute
		}

		if now.Sub(dialog.Created) > timeout {
			// Send timeout response
			response := &ui.DialogResponse{
				DialogID:  id,
				Action:    ui.ActionCancel,
				Cancelled: true,
				Timestamp: now,
			}

			select {
			case dialog.Response <- response:
			default:
				// Channel already closed or full
			}

			delete(r.pendingDialogs, id)
			slog.Debug("Cleaned up expired dialog", "client_id", r.clientID, "dialog_id", id)
		}
	}
}