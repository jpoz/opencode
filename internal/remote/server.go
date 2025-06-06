package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ServerConfig represents the configuration for the WebSocket server.
type ServerConfig struct {
	Host              string        `json:"host"`
	Port              int           `json:"port"`
	TLS               *TLSConfig    `json:"tls,omitempty"`
	MaxConnections    int           `json:"max_connections"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	ReadBufferSize    int           `json:"read_buffer_size"`
	WriteBufferSize   int           `json:"write_buffer_size"`
	EnableCompression bool          `json:"enable_compression"`
	AllowedOrigins    []string      `json:"allowed_origins"`
}

// TLSConfig represents TLS configuration.
type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

// DefaultServerConfig returns a default server configuration.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:              "localhost",
		Port:              8080,
		MaxConnections:    100,
		HeartbeatInterval: 30 * time.Second,
		ConnectionTimeout: 60 * time.Second,
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		EnableCompression: true,
		AllowedOrigins:    []string{"*"},
	}
}

// WebSocketServer implements the Protocol interface using WebSocket connections.
type WebSocketServer struct {
	config   *ServerConfig
	upgrader *websocket.Upgrader
	clients  map[string]*ClientConnection
	sessions map[string]*Session
	server   *http.Server
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// ClientConnection represents a WebSocket client connection.
type ClientConnection struct {
	*Client
	conn          *websocket.Conn
	send          chan *Message
	server        *WebSocketServer
	mu            sync.RWMutex
	lastHeartbeat time.Time
}

// NewWebSocketServer creates a new WebSocket server.
func NewWebSocketServer(config *ServerConfig) *WebSocketServer {
	if config == nil {
		config = DefaultServerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	upgrader := &websocket.Upgrader{
		ReadBufferSize:    config.ReadBufferSize,
		WriteBufferSize:   config.WriteBufferSize,
		EnableCompression: config.EnableCompression,
		CheckOrigin: func(r *http.Request) bool {
			return checkOrigin(r, config.AllowedOrigins)
		},
	}

	return &WebSocketServer{
		config:   config,
		upgrader: upgrader,
		clients:  make(map[string]*ClientConnection),
		sessions: make(map[string]*Session),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start starts the WebSocket server.
func (s *WebSocketServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return fmt.Errorf("server is already running")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		Handler:      mux,
		ReadTimeout:  s.config.ConnectionTimeout,
		WriteTimeout: s.config.ConnectionTimeout,
	}

	// Start heartbeat ticker
	s.wg.Add(1)
	go s.heartbeatTicker()

	// Start connection cleanup ticker
	s.wg.Add(1)
	go s.cleanupTicker()

	var err error
	if s.config.TLS != nil && s.config.TLS.Enabled {
		err = s.server.ListenAndServeTLS(s.config.TLS.CertFile, s.config.TLS.KeyFile)
	} else {
		err = s.server.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed to start: %w", err)
	}

	return nil
}

// Stop stops the WebSocket server.
func (s *WebSocketServer) Stop() error {
	s.cancel()

	// Close all client connections
	s.mu.Lock()
	for _, client := range s.clients {
		client.close()
	}
	s.mu.Unlock()

	// Stop HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	if s.server != nil {
		err = s.server.Shutdown(ctx)
		s.server = nil
	}

	// Wait for goroutines to finish
	s.wg.Wait()

	return err
}

// HandleConnection implements the Protocol interface.
func (s *WebSocketServer) HandleConnection(conn net.Conn) error {
	// This is handled by the HTTP upgrade process in handleWebSocket
	return fmt.Errorf("direct connection handling not supported for WebSocket server")
}

// SendRequest implements the Protocol interface.
func (s *WebSocketServer) SendRequest(clientID string, req *Request) error {
	s.mu.RLock()
	client, exists := s.clients[clientID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	msg := &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeRequest,
		Data:      map[string]interface{}{"request": req},
		Timestamp: time.Now(),
	}

	return client.send_message(msg)
}

// BroadcastEvent implements the Protocol interface.
func (s *WebSocketServer) BroadcastEvent(event *Event) error {
	msg := &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeEvent,
		Data:      map[string]interface{}{"event": event},
		Timestamp: time.Now(),
	}

	s.mu.RLock()
	clients := make([]*ClientConnection, 0, len(s.clients))
	for _, client := range s.clients {
		// Only send to clients subscribed to this event type
		if client.HasSubscription(event.Type) {
			clients = append(clients, client)
		}
	}
	s.mu.RUnlock()

	var lastErr error
	for _, client := range clients {
		if err := client.send_message(msg); err != nil {
			lastErr = err
			slog.Error("Failed to send event to client", "client_id", client.ID, "event_type", event.Type, "error", err)
		}
	}

	return lastErr
}

// CreateSession implements the Protocol interface.
func (s *WebSocketServer) CreateSession(clientID string, opts *SessionOpts) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[clientID]
	if !exists {
		return nil, fmt.Errorf("client %s not found", clientID)
	}

	sessionID := uuid.New().String()
	session := NewSession(sessionID, clientID, opts)
	session.UserID = client.UserID

	s.sessions[sessionID] = session
	client.SessionID = sessionID

	slog.Info("Session created", "session_id", sessionID, "client_id", clientID)
	return session, nil
}

// JoinSession implements the Protocol interface.
func (s *WebSocketServer) JoinSession(clientID string, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[clientID]
	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	session, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	client.SessionID = sessionID
	session.Update()

	slog.Info("Client joined session", "client_id", clientID, "session_id", sessionID)
	return nil
}

// LeaveSession implements the Protocol interface.
func (s *WebSocketServer) LeaveSession(clientID string, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[clientID]
	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	if client.SessionID != sessionID {
		return fmt.Errorf("client %s is not in session %s", clientID, sessionID)
	}

	client.SessionID = ""
	slog.Info("Client left session", "client_id", clientID, "session_id", sessionID)
	return nil
}

// GetClient implements the Protocol interface.
func (s *WebSocketServer) GetClient(clientID string) (*Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, exists := s.clients[clientID]
	if !exists {
		return nil, fmt.Errorf("client %s not found", clientID)
	}

	return client.Client, nil
}

// ListClients implements the Protocol interface.
func (s *WebSocketServer) ListClients() []*Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]*Client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client.Client)
	}

	return clients
}

// DisconnectClient implements the Protocol interface.
func (s *WebSocketServer) DisconnectClient(clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[clientID]
	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	client.close()
	delete(s.clients, clientID)

	slog.Info("Client disconnected", "client_id", clientID)
	return nil
}

// Subscribe implements the Protocol interface.
func (s *WebSocketServer) Subscribe(clientID string, events ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[clientID]
	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	for _, event := range events {
		client.AddSubscription(event)
	}

	slog.Debug("Client subscribed to events", "client_id", clientID, "events", events)
	return nil
}

// Unsubscribe implements the Protocol interface.
func (s *WebSocketServer) Unsubscribe(clientID string, events ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[clientID]
	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	for _, event := range events {
		client.RemoveSubscription(event)
	}

	slog.Debug("Client unsubscribed from events", "client_id", clientID, "events", events)
	return nil
}

// Close implements the Protocol interface.
func (s *WebSocketServer) Close() error {
	return s.Stop()
}

// HTTP handlers

func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket connection", "error", err)
		return
	}

	// Check max connections
	s.mu.RLock()
	clientCount := len(s.clients)
	s.mu.RUnlock()

	if clientCount >= s.config.MaxConnections {
		conn.Close()
		slog.Warn("Maximum connections reached, rejecting new connection")
		return
	}

	// Create client
	clientID := uuid.New().String()
	ipAddress := getClientIP(r)
	client := NewClient(clientID, ipAddress)
	client.UserAgent = r.UserAgent()

	clientConn := &ClientConnection{
		Client:        client,
		conn:          conn,
		send:          make(chan *Message, 256),
		server:        s,
		lastHeartbeat: time.Now(),
	}

	// Register client
	s.mu.Lock()
	s.clients[clientID] = clientConn
	s.mu.Unlock()

	slog.Info("Client connected", "client_id", clientID, "ip", ipAddress)

	// Start client handlers
	go clientConn.writePump()
	go clientConn.readPump()
}

// TODO add a timeout around this incase the mu never gets released
func (s *WebSocketServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	clientCount := len(s.clients)
	sessionCount := len(s.sessions)
	s.mu.RUnlock()

	health := map[string]interface{}{
		"status":   "healthy",
		"clients":  clientCount,
		"sessions": sessionCount,
		"uptime":   time.Since(time.Now()), // This would need a proper start time
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// Background tasks

func (s *WebSocketServer) heartbeatTicker() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.sendHeartbeats()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *WebSocketServer) cleanupTicker() {
	defer s.wg.Done()
	ticker := time.NewTicker(1 * time.Minute) // Cleanup every minute
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupStaleConnections()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *WebSocketServer) sendHeartbeats() {
	heartbeat := &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeHeartbeat,
		Data:      map[string]interface{}{"timestamp": time.Now()},
		Timestamp: time.Now(),
	}

	s.mu.RLock()
	clients := make([]*ClientConnection, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	s.mu.RUnlock()

	for _, client := range clients {
		if err := client.send_message(heartbeat); err != nil {
			slog.Error("Failed to send heartbeat to client", "client_id", client.ID, "error", err)
		}
	}
}

func (s *WebSocketServer) cleanupStaleConnections() {
	timeout := s.config.ConnectionTimeout
	now := time.Now()

	s.mu.Lock()
	for clientID, client := range s.clients {
		if now.Sub(client.lastHeartbeat) > timeout {
			slog.Info("Cleaning up stale connection", "client_id", clientID)
			client.close()
			delete(s.clients, clientID)
		}
	}
	s.mu.Unlock()
}

// ClientConnection methods

func (c *ClientConnection) send_message(msg *Message) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	msg.ClientID = c.ID

	select {
	case c.send <- msg:
		return nil
	default:
		return fmt.Errorf("client send buffer full")
	}
}

func (c *ClientConnection) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	close(c.send)
}

func (c *ClientConnection) readPump() {
	defer func() {
		c.server.DisconnectClient(c.ID)
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB
	c.conn.SetReadDeadline(time.Now().Add(c.server.config.ConnectionTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.lastHeartbeat = time.Now()
		c.conn.SetReadDeadline(time.Now().Add(c.server.config.ConnectionTimeout))
		return nil
	})

	for {
		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket read error", "client_id", c.ID, "error", err)
			}
			break
		}

		c.UpdateLastSeen()
		c.handleMessage(&msg)
	}
}

func (c *ClientConnection) writePump() {
	ticker := time.NewTicker(c.server.config.HeartbeatInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(msg); err != nil {
				slog.Error("WebSocket write error", "client_id", c.ID, "error", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *ClientConnection) handleMessage(msg *Message) {
	switch msg.Type {
	case MsgTypeRequest:
		c.handleRequest(msg)
	case MsgTypeSubscribe:
		c.handleSubscribe(msg)
	case MsgTypeUnsubscribe:
		c.handleUnsubscribe(msg)
	case MsgTypeHeartbeat:
		c.lastHeartbeat = time.Now()
	default:
		slog.Warn("Unknown message type", "client_id", c.ID, "type", msg.Type)
	}
}

func (c *ClientConnection) handleRequest(msg *Message) {
	// Handle different request types
	// This would integrate with the application logic
	slog.Debug("Received request", "client_id", c.ID, "message_id", msg.ID)
}

func (c *ClientConnection) handleSubscribe(msg *Message) {
	if events, ok := msg.Data["events"].([]interface{}); ok {
		eventStrings := make([]string, len(events))
		for i, event := range events {
			if eventStr, ok := event.(string); ok {
				eventStrings[i] = eventStr
			}
		}
		c.server.Subscribe(c.ID, eventStrings...)
	}
}

func (c *ClientConnection) handleUnsubscribe(msg *Message) {
	if events, ok := msg.Data["events"].([]interface{}); ok {
		eventStrings := make([]string, len(events))
		for i, event := range events {
			if eventStr, ok := event.(string); ok {
				eventStrings[i] = eventStr
			}
		}
		c.server.Unsubscribe(c.ID, eventStrings...)
	}
}

// Utility functions

func checkOrigin(r *http.Request, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return false
	}

	origin := r.Header.Get("Origin")
	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	return false
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

