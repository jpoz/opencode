package remote

import (
	"fmt"
	"net"
	"time"

	ui "github.com/sst/opencode/internal/interface"
)

// Protocol defines the interface for handling remote client communication.
type Protocol interface {
	// Client connections
	HandleConnection(conn net.Conn) error
	
	// Message routing
	SendRequest(clientID string, req *Request) error
	BroadcastEvent(event *Event) error
	
	// Session management
	CreateSession(clientID string, opts *SessionOpts) (*Session, error)
	JoinSession(clientID string, sessionID string) error
	LeaveSession(clientID string, sessionID string) error
	
	// Client management
	GetClient(clientID string) (*Client, error)
	ListClients() []*Client
	DisconnectClient(clientID string) error
	
	// Event broadcasting
	Subscribe(clientID string, events ...string) error
	Unsubscribe(clientID string, events ...string) error
	
	// Cleanup
	Close() error
}

// Message represents a protocol message sent between client and server.
type Message struct {
	ID        string                 `json:"id"`
	Type      MessageType            `json:"type"`
	SessionID string                 `json:"session_id,omitempty"`
	ClientID  string                 `json:"client_id,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	ReplyTo   string                 `json:"reply_to,omitempty"`
}

// MessageType defines the type of protocol message.
type MessageType string

const (
	MsgTypeRequest      MessageType = "request"
	MsgTypeResponse     MessageType = "response"
	MsgTypeEvent        MessageType = "event"
	MsgTypeStatus       MessageType = "status"
	MsgTypeDialog       MessageType = "dialog"
	MsgTypeDialogReply  MessageType = "dialog_reply"
	MsgTypeError        MessageType = "error"
	MsgTypeHeartbeat    MessageType = "heartbeat"
	MsgTypeAuth         MessageType = "auth"
	MsgTypeSubscribe    MessageType = "subscribe"
	MsgTypeUnsubscribe  MessageType = "unsubscribe"
)

// Request represents a request from client to server.
type Request struct {
	ID      string                 `json:"id"`
	Action  string                 `json:"action"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Timeout time.Duration          `json:"timeout,omitempty"`
}

// Response represents a response from server to client.
type Response struct {
	ID      string                 `json:"id"`
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   *ErrorInfo             `json:"error,omitempty"`
}

// Event represents an event broadcasted by the server.
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// ErrorInfo represents detailed error information in responses.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Client represents a connected client.
type Client struct {
	ID            string            `json:"id"`
	UserID        string            `json:"user_id,omitempty"`
	SessionID     string            `json:"session_id,omitempty"`
	IPAddress     string            `json:"ip_address"`
	UserAgent     string            `json:"user_agent,omitempty"`
	Connected     time.Time         `json:"connected"`
	LastSeen      time.Time         `json:"last_seen"`
	Subscriptions []string          `json:"subscriptions"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// Session represents a remote session.
type Session struct {
	ID        string            `json:"id"`
	ClientID  string            `json:"client_id"`
	UserID    string            `json:"user_id,omitempty"`
	Title     string            `json:"title"`
	Created   time.Time         `json:"created"`
	Updated   time.Time         `json:"updated"`
	Active    bool              `json:"active"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// SessionOpts represents options for creating a session.
type SessionOpts struct {
	Title    string            `json:"title,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// AuthInfo represents authentication information.
type AuthInfo struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// HeartbeatInfo represents heartbeat information.
type HeartbeatInfo struct {
	Timestamp time.Time `json:"timestamp"`
	ClientID  string    `json:"client_id"`
}

// Request action constants.
const (
	ActionUserInput      = "user_input"
	ActionSessionSwitch  = "session_switch"
	ActionSessionList    = "session_list"
	ActionFileUpload     = "file_upload"
	ActionFilePicker     = "file_picker"
	ActionStatusUpdate   = "status_update"
	ActionPing           = "ping"
)

// Event type constants.
const (
	EventTypeMessageUpdate    = "message_update"
	EventTypeStatusUpdate     = "status_update"
	EventTypeDialogShow       = "dialog_show"
	EventTypeSessionUpdate    = "session_update"
	EventTypeClientConnected  = "client_connected"
	EventTypeClientDisconnected = "client_disconnected"
	EventTypeError            = "error"
)

// NewMessage creates a new protocol message.
func NewMessage(msgType MessageType, data map[string]interface{}) *Message {
	return &Message{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewRequest creates a new request.
func NewRequest(action string, data map[string]interface{}) *Request {
	return &Request{
		Action: action,
		Data:   data,
	}
}

// NewResponse creates a new successful response.
func NewResponse(requestID string, data map[string]interface{}) *Response {
	return &Response{
		ID:      requestID,
		Success: true,
		Data:    data,
	}
}

// NewErrorResponse creates a new error response.
func NewErrorResponse(requestID, code, message, details string) *Response {
	return &Response{
		ID:      requestID,
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// NewEvent creates a new event.
func NewEvent(eventType string, data map[string]interface{}) *Event {
	return &Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewClient creates a new client instance.
func NewClient(id, ipAddress string) *Client {
	now := time.Now()
	return &Client{
		ID:            id,
		IPAddress:     ipAddress,
		Connected:     now,
		LastSeen:      now,
		Subscriptions: make([]string, 0),
		Metadata:      make(map[string]string),
	}
}

// NewSession creates a new session instance.
func NewSession(id, clientID string, opts *SessionOpts) *Session {
	now := time.Now()
	session := &Session{
		ID:       id,
		ClientID: clientID,
		Created:  now,
		Updated:  now,
		Active:   true,
		Metadata: make(map[string]string),
	}
	
	if opts != nil {
		session.Title = opts.Title
		if opts.Metadata != nil {
			for k, v := range opts.Metadata {
				session.Metadata[k] = v
			}
		}
	}
	
	return session
}

// Validate validates a message.
func (m *Message) Validate() error {
	if m.Type == "" {
		return fmt.Errorf("message type is required")
	}
	if m.Data == nil {
		m.Data = make(map[string]interface{})
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}
	return nil
}

// IsExpired checks if a request has expired based on its timeout.
func (r *Request) IsExpired(createdAt time.Time) bool {
	if r.Timeout == 0 {
		return false // No timeout
	}
	return time.Since(createdAt) > r.Timeout
}

// Validate validates a request.
func (r *Request) Validate() error {
	if r.Action == "" {
		return fmt.Errorf("request action is required")
	}
	if r.Data == nil {
		r.Data = make(map[string]interface{})
	}
	return nil
}

// HasSubscription checks if a client is subscribed to an event type.
func (c *Client) HasSubscription(eventType string) bool {
	for _, sub := range c.Subscriptions {
		if sub == eventType {
			return true
		}
	}
	return false
}

// AddSubscription adds an event subscription to the client.
func (c *Client) AddSubscription(eventType string) {
	if !c.HasSubscription(eventType) {
		c.Subscriptions = append(c.Subscriptions, eventType)
	}
}

// RemoveSubscription removes an event subscription from the client.
func (c *Client) RemoveSubscription(eventType string) {
	for i, sub := range c.Subscriptions {
		if sub == eventType {
			c.Subscriptions = append(c.Subscriptions[:i], c.Subscriptions[i+1:]...)
			break
		}
	}
}

// UpdateLastSeen updates the client's last seen timestamp.
func (c *Client) UpdateLastSeen() {
	c.LastSeen = time.Now()
}

// IsStale checks if a client connection is stale based on last seen time.
func (c *Client) IsStale(timeout time.Duration) bool {
	return time.Since(c.LastSeen) > timeout
}

// Update updates the session's updated timestamp.
func (s *Session) Update() {
	s.Updated = time.Now()
}

// ConvertToDialogMessage converts a UI dialog to a protocol message.
func ConvertToDialogMessage(dialog *ui.Dialog) *Message {
	return &Message{
		Type: MsgTypeDialog,
		Data: map[string]interface{}{
			"dialog": dialog,
		},
		Timestamp: time.Now(),
	}
}

// ConvertFromDialogResponse converts a protocol message to a UI dialog response.
func ConvertFromDialogResponse(msg *Message) (*ui.DialogResponse, error) {
	if msg.Type != MsgTypeDialogReply {
		return nil, fmt.Errorf("invalid message type for dialog response: %s", msg.Type)
	}
	
	// Extract dialog response from message data
	responseData, ok := msg.Data["response"]
	if !ok {
		return nil, fmt.Errorf("missing response data in dialog reply message")
	}
	
	// Convert to DialogResponse - this would need proper JSON unmarshaling in practice
	response := &ui.DialogResponse{
		Timestamp: msg.Timestamp,
	}
	
	if dialogID, ok := responseData.(map[string]interface{})["dialog_id"].(string); ok {
		response.DialogID = dialogID
	}
	if action, ok := responseData.(map[string]interface{})["action"].(string); ok {
		response.Action = ui.DialogAction(action)
	}
	if value, ok := responseData.(map[string]interface{})["value"].(string); ok {
		response.Value = value
	}
	if cancelled, ok := responseData.(map[string]interface{})["cancelled"].(bool); ok {
		response.Cancelled = cancelled
	}
	
	return response, nil
}

// ConvertToUserRequest converts a protocol message to a UI user request.
func ConvertToUserRequest(msg *Message) (*ui.UserRequest, error) {
	if msg.Type != MsgTypeRequest {
		return nil, fmt.Errorf("invalid message type for user request: %s", msg.Type)
	}
	
	request := &ui.UserRequest{
		Options: make(map[string]interface{}),
	}
	
	if content, ok := msg.Data["content"].(string); ok {
		request.Content = content
	}
	if reqType, ok := msg.Data["type"].(string); ok {
		request.Type = ui.RequestType(reqType)
	}
	if options, ok := msg.Data["options"].(map[string]interface{}); ok {
		request.Options = options
	}
	
	return request, nil
}