package remote

import (
	"testing"
	"time"

	ui "github.com/sst/opencode/internal/interface"
)

func TestNewMessage(t *testing.T) {
	msgType := MsgTypeRequest
	data := map[string]interface{}{
		"action": "test_action",
		"value":  42,
	}

	msg := NewMessage(msgType, data)

	if msg.Type != msgType {
		t.Errorf("Expected type %s, got %s", msgType, msg.Type)
	}
	if msg.Data["action"] != "test_action" {
		t.Errorf("Expected action 'test_action', got %v", msg.Data["action"])
	}
	if msg.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

func TestNewRequest(t *testing.T) {
	action := ActionUserInput
	data := map[string]interface{}{
		"content": "Hello, world!",
	}

	req := NewRequest(action, data)

	if req.Action != action {
		t.Errorf("Expected action %s, got %s", action, req.Action)
	}
	if req.Data["content"] != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got %v", req.Data["content"])
	}
}

func TestNewResponse(t *testing.T) {
	requestID := "req_123"
	data := map[string]interface{}{
		"result": "success",
	}

	resp := NewResponse(requestID, data)

	if resp.ID != requestID {
		t.Errorf("Expected ID %s, got %s", requestID, resp.ID)
	}
	if !resp.Success {
		t.Error("Expected success to be true")
	}
	if resp.Data["result"] != "success" {
		t.Errorf("Expected result 'success', got %v", resp.Data["result"])
	}
	if resp.Error != nil {
		t.Error("Expected error to be nil for successful response")
	}
}

func TestNewErrorResponse(t *testing.T) {
	requestID := "req_123"
	code := "INVALID_REQUEST"
	message := "Invalid request format"
	details := "Missing required field 'action'"

	resp := NewErrorResponse(requestID, code, message, details)

	if resp.ID != requestID {
		t.Errorf("Expected ID %s, got %s", requestID, resp.ID)
	}
	if resp.Success {
		t.Error("Expected success to be false")
	}
	if resp.Error == nil {
		t.Fatal("Expected error to be set")
	}
	if resp.Error.Code != code {
		t.Errorf("Expected error code %s, got %s", code, resp.Error.Code)
	}
	if resp.Error.Message != message {
		t.Errorf("Expected error message %s, got %s", message, resp.Error.Message)
	}
	if resp.Error.Details != details {
		t.Errorf("Expected error details %s, got %s", details, resp.Error.Details)
	}
}

func TestNewEvent(t *testing.T) {
	eventType := EventTypeMessageUpdate
	data := map[string]interface{}{
		"message_id": "msg_123",
		"content":    "Updated content",
	}

	event := NewEvent(eventType, data)

	if event.Type != eventType {
		t.Errorf("Expected type %s, got %s", eventType, event.Type)
	}
	if event.Data["message_id"] != "msg_123" {
		t.Errorf("Expected message_id 'msg_123', got %v", event.Data["message_id"])
	}
	if event.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

func TestNewClient(t *testing.T) {
	id := "client_123"
	ipAddress := "192.168.1.100"

	client := NewClient(id, ipAddress)

	if client.ID != id {
		t.Errorf("Expected ID %s, got %s", id, client.ID)
	}
	if client.IPAddress != ipAddress {
		t.Errorf("Expected IP address %s, got %s", ipAddress, client.IPAddress)
	}
	if client.Connected.IsZero() {
		t.Error("Expected connected timestamp to be set")
	}
	if client.LastSeen.IsZero() {
		t.Error("Expected last seen timestamp to be set")
	}
	if client.Subscriptions == nil {
		t.Error("Expected subscriptions to be initialized")
	}
	if client.Metadata == nil {
		t.Error("Expected metadata to be initialized")
	}
}

func TestNewSession(t *testing.T) {
	id := "session_123"
	clientID := "client_123"
	opts := &SessionOpts{
		Title: "Test Session",
		Metadata: map[string]string{
			"key1": "value1",
		},
	}

	session := NewSession(id, clientID, opts)

	if session.ID != id {
		t.Errorf("Expected ID %s, got %s", id, session.ID)
	}
	if session.ClientID != clientID {
		t.Errorf("Expected client ID %s, got %s", clientID, session.ClientID)
	}
	if session.Title != opts.Title {
		t.Errorf("Expected title %s, got %s", opts.Title, session.Title)
	}
	if !session.Active {
		t.Error("Expected session to be active")
	}
	if session.Created.IsZero() {
		t.Error("Expected created timestamp to be set")
	}
	if session.Updated.IsZero() {
		t.Error("Expected updated timestamp to be set")
	}
	if session.Metadata["key1"] != "value1" {
		t.Errorf("Expected metadata key1 'value1', got %s", session.Metadata["key1"])
	}
}

func TestMessage_Validate(t *testing.T) {
	tests := []struct {
		name      string
		message   *Message
		expectErr bool
	}{
		{
			name: "valid message",
			message: &Message{
				Type: MsgTypeRequest,
				Data: map[string]interface{}{"key": "value"},
			},
			expectErr: false,
		},
		{
			name: "missing type",
			message: &Message{
				Data: map[string]interface{}{"key": "value"},
			},
			expectErr: true,
		},
		{
			name: "nil data gets initialized",
			message: &Message{
				Type: MsgTypeRequest,
				Data: nil,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Expected error %v, got %v", tt.expectErr, err != nil)
			}
			if !tt.expectErr {
				if tt.message.Data == nil {
					t.Error("Expected data to be initialized")
				}
				if tt.message.Timestamp.IsZero() {
					t.Error("Expected timestamp to be set")
				}
			}
		})
	}
}

func TestRequest_Validate(t *testing.T) {
	tests := []struct {
		name      string
		request   *Request
		expectErr bool
	}{
		{
			name: "valid request",
			request: &Request{
				Action: ActionUserInput,
				Data:   map[string]interface{}{"content": "test"},
			},
			expectErr: false,
		},
		{
			name: "missing action",
			request: &Request{
				Data: map[string]interface{}{"content": "test"},
			},
			expectErr: true,
		},
		{
			name: "nil data gets initialized",
			request: &Request{
				Action: ActionUserInput,
				Data:   nil,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Expected error %v, got %v", tt.expectErr, err != nil)
			}
			if !tt.expectErr && tt.request.Data == nil {
				t.Error("Expected data to be initialized")
			}
		})
	}
}

func TestRequest_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		timeout   time.Duration
		createdAt time.Time
		expected  bool
	}{
		{
			name:      "no timeout",
			timeout:   0,
			createdAt: time.Now().Add(-1 * time.Hour),
			expected:  false,
		},
		{
			name:      "not expired",
			timeout:   1 * time.Minute,
			createdAt: time.Now().Add(-30 * time.Second),
			expected:  false,
		},
		{
			name:      "expired",
			timeout:   1 * time.Minute,
			createdAt: time.Now().Add(-2 * time.Minute),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &Request{Timeout: tt.timeout}
			result := req.IsExpired(tt.createdAt)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestClient_Subscriptions(t *testing.T) {
	client := NewClient("client_123", "192.168.1.100")

	// Test HasSubscription on empty client
	if client.HasSubscription(EventTypeMessageUpdate) {
		t.Error("Expected client to not have subscription initially")
	}

	// Test AddSubscription
	client.AddSubscription(EventTypeMessageUpdate)
	if !client.HasSubscription(EventTypeMessageUpdate) {
		t.Error("Expected client to have subscription after adding")
	}

	// Test adding duplicate subscription
	client.AddSubscription(EventTypeMessageUpdate)
	count := 0
	for _, sub := range client.Subscriptions {
		if sub == EventTypeMessageUpdate {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected 1 subscription, got %d", count)
	}

	// Test RemoveSubscription
	client.RemoveSubscription(EventTypeMessageUpdate)
	if client.HasSubscription(EventTypeMessageUpdate) {
		t.Error("Expected client to not have subscription after removing")
	}
}

func TestClient_LastSeen(t *testing.T) {
	client := NewClient("client_123", "192.168.1.100")
	originalLastSeen := client.LastSeen

	time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamp
	client.UpdateLastSeen()

	if !client.LastSeen.After(originalLastSeen) {
		t.Error("Expected last seen to be updated")
	}

	// Test IsStale
	if client.IsStale(1 * time.Hour) {
		t.Error("Expected client to not be stale with 1 hour timeout")
	}

	// Set last seen to past time to test staleness
	client.LastSeen = time.Now().Add(-2 * time.Hour)
	if !client.IsStale(1 * time.Hour) {
		t.Error("Expected client to be stale with 1 hour timeout")
	}
}

func TestSession_Update(t *testing.T) {
	session := NewSession("session_123", "client_123", nil)
	originalUpdated := session.Updated

	time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamp
	session.Update()

	if !session.Updated.After(originalUpdated) {
		t.Error("Expected updated timestamp to be updated")
	}
}

func TestConvertToDialogMessage(t *testing.T) {
	dialog := &ui.Dialog{
		ID:      "dialog_123",
		Type:    ui.DialogTypeConfirm,
		Title:   "Confirm Action",
		Content: "Are you sure?",
	}

	msg := ConvertToDialogMessage(dialog)

	if msg.Type != MsgTypeDialog {
		t.Errorf("Expected type %s, got %s", MsgTypeDialog, msg.Type)
	}
	
	dialogData, ok := msg.Data["dialog"]
	if !ok {
		t.Fatal("Expected dialog data in message")
	}
	
	convertedDialog, ok := dialogData.(*ui.Dialog)
	if !ok {
		t.Fatal("Expected dialog data to be *ui.Dialog")
	}
	
	if convertedDialog.ID != dialog.ID {
		t.Errorf("Expected dialog ID %s, got %s", dialog.ID, convertedDialog.ID)
	}
}

func TestConvertToUserRequest(t *testing.T) {
	msg := &Message{
		Type: MsgTypeRequest,
		Data: map[string]interface{}{
			"type":    string(ui.RequestTypeChat),
			"content": "Hello, world!",
			"options": map[string]interface{}{
				"model": "claude-3",
			},
		},
	}

	req, err := ConvertToUserRequest(msg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if req.Type != ui.RequestTypeChat {
		t.Errorf("Expected type %s, got %s", ui.RequestTypeChat, req.Type)
	}
	if req.Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got %s", req.Content)
	}
	if req.Options["model"] != "claude-3" {
		t.Errorf("Expected model option 'claude-3', got %v", req.Options["model"])
	}
}

func TestConvertToUserRequest_InvalidType(t *testing.T) {
	msg := &Message{
		Type: MsgTypeEvent, // Wrong type
		Data: map[string]interface{}{
			"content": "Hello, world!",
		},
	}

	_, err := ConvertToUserRequest(msg)
	if err == nil {
		t.Error("Expected error for invalid message type")
	}
}