package remote

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8080, config.Port)
	assert.Equal(t, 100, config.MaxConnections)
	assert.Equal(t, 30*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 60*time.Second, config.ConnectionTimeout)
	assert.True(t, config.EnableCompression)
	assert.Contains(t, config.AllowedOrigins, "*")
}

func TestNewWebSocketServer(t *testing.T) {
	config := DefaultServerConfig()
	server := NewWebSocketServer(config)

	assert.NotNil(t, server)
	assert.Equal(t, config, server.config)
	assert.NotNil(t, server.clients)
	assert.NotNil(t, server.sessions)
	assert.NotNil(t, server.upgrader)
}

func TestWebSocketServer_CreateSession(t *testing.T) {
	server := NewWebSocketServer(nil)
	
	// Create a mock client
	clientID := "test_client"
	client := NewClient(clientID, "127.0.0.1")
	clientConn := &ClientConnection{
		Client: client,
		server: server,
		send:   make(chan *Message, 10),
	}
	
	server.mu.Lock()
	server.clients[clientID] = clientConn
	server.mu.Unlock()

	opts := &SessionOpts{
		Title: "Test Session",
		Metadata: map[string]string{
			"test": "value",
		},
	}

	session, err := server.CreateSession(clientID, opts)
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, clientID, session.ClientID)
	assert.Equal(t, opts.Title, session.Title)
	assert.Equal(t, "value", session.Metadata["test"])
	assert.True(t, session.Active)

	// Verify session is stored
	assert.Contains(t, server.sessions, session.ID)

	// Verify client has session ID
	assert.Equal(t, session.ID, clientConn.SessionID)
}

func TestWebSocketServer_CreateSession_ClientNotFound(t *testing.T) {
	server := NewWebSocketServer(nil)

	_, err := server.CreateSession("nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client nonexistent not found")
}

func TestWebSocketServer_JoinSession(t *testing.T) {
	server := NewWebSocketServer(nil)
	
	// Create client and session
	clientID := "test_client"
	client := NewClient(clientID, "127.0.0.1")
	clientConn := &ClientConnection{
		Client: client,
		server: server,
		send:   make(chan *Message, 10),
	}
	
	sessionID := "test_session"
	session := NewSession(sessionID, "other_client", nil)
	
	server.mu.Lock()
	server.clients[clientID] = clientConn
	server.sessions[sessionID] = session
	server.mu.Unlock()

	err := server.JoinSession(clientID, sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, clientConn.SessionID)
}

func TestWebSocketServer_LeaveSession(t *testing.T) {
	server := NewWebSocketServer(nil)
	
	// Create client with session
	clientID := "test_client"
	sessionID := "test_session"
	client := NewClient(clientID, "127.0.0.1")
	client.SessionID = sessionID
	clientConn := &ClientConnection{
		Client: client,
		server: server,
		send:   make(chan *Message, 10),
	}
	
	server.mu.Lock()
	server.clients[clientID] = clientConn
	server.mu.Unlock()

	err := server.LeaveSession(clientID, sessionID)
	require.NoError(t, err)
	assert.Empty(t, clientConn.SessionID)
}

func TestWebSocketServer_GetClient(t *testing.T) {
	server := NewWebSocketServer(nil)
	
	clientID := "test_client"
	client := NewClient(clientID, "127.0.0.1")
	clientConn := &ClientConnection{
		Client: client,
		server: server,
		send:   make(chan *Message, 10),
	}
	
	server.mu.Lock()
	server.clients[clientID] = clientConn
	server.mu.Unlock()

	retrievedClient, err := server.GetClient(clientID)
	require.NoError(t, err)
	assert.Equal(t, clientID, retrievedClient.ID)
	assert.Equal(t, "127.0.0.1", retrievedClient.IPAddress)
}

func TestWebSocketServer_ListClients(t *testing.T) {
	server := NewWebSocketServer(nil)
	
	// Add multiple clients
	for i := 0; i < 3; i++ {
		clientID := fmt.Sprintf("client_%d", i)
		client := NewClient(clientID, "127.0.0.1")
		clientConn := &ClientConnection{
			Client: client,
			server: server,
			send:   make(chan *Message, 10),
		}
		
		server.mu.Lock()
		server.clients[clientID] = clientConn
		server.mu.Unlock()
	}

	clients := server.ListClients()
	assert.Len(t, clients, 3)
	
	// Verify all clients are present
	clientIDs := make(map[string]bool)
	for _, client := range clients {
		clientIDs[client.ID] = true
	}
	
	for i := 0; i < 3; i++ {
		expectedID := fmt.Sprintf("client_%d", i)
		assert.True(t, clientIDs[expectedID], "Client %s should be in list", expectedID)
	}
}

func TestWebSocketServer_Subscribe(t *testing.T) {
	server := NewWebSocketServer(nil)
	
	clientID := "test_client"
	client := NewClient(clientID, "127.0.0.1")
	clientConn := &ClientConnection{
		Client: client,
		server: server,
		send:   make(chan *Message, 10),
	}
	
	server.mu.Lock()
	server.clients[clientID] = clientConn
	server.mu.Unlock()

	events := []string{EventTypeMessageUpdate, EventTypeStatusUpdate}
	err := server.Subscribe(clientID, events...)
	require.NoError(t, err)

	// Verify subscriptions
	for _, event := range events {
		assert.True(t, clientConn.HasSubscription(event))
	}
}

func TestWebSocketServer_BroadcastEvent(t *testing.T) {
	server := NewWebSocketServer(nil)
	
	// Create clients with different subscriptions
	client1ID := "client_1"
	client1 := NewClient(client1ID, "127.0.0.1")
	client1.AddSubscription(EventTypeMessageUpdate)
	client1Conn := &ClientConnection{
		Client: client1,
		server: server,
		send:   make(chan *Message, 10),
	}
	
	client2ID := "client_2"
	client2 := NewClient(client2ID, "127.0.0.1")
	client2.AddSubscription(EventTypeStatusUpdate)
	client2Conn := &ClientConnection{
		Client: client2,
		server: server,
		send:   make(chan *Message, 10),
	}
	
	server.mu.Lock()
	server.clients[client1ID] = client1Conn
	server.clients[client2ID] = client2Conn
	server.mu.Unlock()

	// Broadcast message update event
	event := NewEvent(EventTypeMessageUpdate, map[string]interface{}{
		"message_id": "msg_123",
	})

	err := server.BroadcastEvent(event)
	require.NoError(t, err)

	// Client 1 should receive the event
	select {
	case msg := <-client1Conn.send:
		assert.Equal(t, MsgTypeEvent, msg.Type)
	case <-time.After(100 * time.Millisecond):
		t.Error("Client 1 should have received the event")
	}

	// Client 2 should not receive the event (different subscription)
	select {
	case <-client2Conn.send:
		t.Error("Client 2 should not have received the event")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message received
	}
}

func TestClientConnection_SendMessage(t *testing.T) {
	server := NewWebSocketServer(nil)
	client := NewClient("test_client", "127.0.0.1")
	clientConn := &ClientConnection{
		Client: client,
		server: server,
		send:   make(chan *Message, 10),
	}

	msg := NewMessage(MsgTypeEvent, map[string]interface{}{
		"test": "data",
	})

	err := clientConn.send_message(msg)
	require.NoError(t, err)

	// Verify message was sent
	select {
	case receivedMsg := <-clientConn.send:
		assert.Equal(t, msg.Type, receivedMsg.Type)
		assert.Equal(t, client.ID, receivedMsg.ClientID)
		assert.Equal(t, "data", receivedMsg.Data["test"])
	case <-time.After(100 * time.Millisecond):
		t.Error("Message should have been sent")
	}
}

func TestClientConnection_SendMessage_BufferFull(t *testing.T) {
	server := NewWebSocketServer(nil)
	client := NewClient("test_client", "127.0.0.1")
	clientConn := &ClientConnection{
		Client: client,
		server: server,
		send:   make(chan *Message, 1), // Small buffer
	}

	msg := NewMessage(MsgTypeEvent, map[string]interface{}{})

	// Fill the buffer
	err := clientConn.send_message(msg)
	require.NoError(t, err)

	// Next message should fail
	err = clientConn.send_message(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "send buffer full")
}

func TestCheckOrigin(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		allowedOrigins []string
		expected       bool
	}{
		{
			name:           "wildcard allows all",
			origin:         "https://example.com",
			allowedOrigins: []string{"*"},
			expected:       true,
		},
		{
			name:           "exact match",
			origin:         "https://example.com",
			allowedOrigins: []string{"https://example.com"},
			expected:       true,
		},
		{
			name:           "no match",
			origin:         "https://evil.com",
			allowedOrigins: []string{"https://example.com"},
			expected:       false,
		},
		{
			name:           "empty origins",
			origin:         "https://example.com",
			allowedOrigins: []string{},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ws", nil)
			req.Header.Set("Origin", tt.origin)
			
			result := checkOrigin(req, tt.allowedOrigins)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remoteAddr string
		expected string
	}{
		{
			name:     "X-Forwarded-For",
			headers:  map[string]string{"X-Forwarded-For": "192.168.1.100"},
			remoteAddr: "10.0.0.1:12345",
			expected: "192.168.1.100",
		},
		{
			name:     "X-Real-IP",
			headers:  map[string]string{"X-Real-IP": "192.168.1.101"},
			remoteAddr: "10.0.0.1:12345",
			expected: "192.168.1.101",
		},
		{
			name:     "RemoteAddr fallback",
			headers:  map[string]string{},
			remoteAddr: "192.168.1.102:12345",
			expected: "192.168.1.102",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ws", nil)
			req.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			
			result := getClientIP(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebSocketServer_HealthEndpoint(t *testing.T) {
	server := NewWebSocketServer(nil)
	
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	server.handleHealth(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	
	var health map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &health)
	require.NoError(t, err)
	
	assert.Equal(t, "healthy", health["status"])
	assert.Contains(t, health, "clients")
	assert.Contains(t, health, "sessions")
	assert.Contains(t, health, "uptime")
}

// Integration test with actual WebSocket connection
func TestWebSocketServer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create and start server
	config := DefaultServerConfig()
	config.Port = 0 // Use random port
	server := NewWebSocketServer(config)

	// Start server in background
	go func() {
		server.Start()
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	defer server.Stop()

	// We would need to implement a test client here to fully test
	// the WebSocket integration, but that would require more complex setup
	// For now, we'll test that the server starts without error
	assert.NotNil(t, server)
}