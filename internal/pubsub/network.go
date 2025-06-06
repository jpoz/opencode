package pubsub

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// NetworkClient represents a remote client connection that can receive events.
type NetworkClient interface {
	// SendEvent sends an event to the remote client
	SendEvent(event NetworkEvent) error
	// GetID returns the unique client identifier
	GetID() string
	// IsConnected returns whether the client is still connected
	IsConnected() bool
}

// NetworkEvent represents an event that can be serialized and sent over the network.
type NetworkEvent struct {
	Type      EventType   `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"`
}

// NetworkBroker extends the regular broker to support broadcasting events to network clients.
type NetworkBroker[T any] struct {
	*Broker[T]
	clients     map[string]NetworkClient
	clientsMutex sync.RWMutex
	source      string
}

// NewNetworkBroker creates a new network-aware broker.
func NewNetworkBroker[T any](source string) *NetworkBroker[T] {
	return &NetworkBroker[T]{
		Broker:  NewBroker[T](),
		clients: make(map[string]NetworkClient),
		source:  source,
	}
}

// AddNetworkClient registers a network client to receive events.
func (nb *NetworkBroker[T]) AddNetworkClient(client NetworkClient) {
	nb.clientsMutex.Lock()
	defer nb.clientsMutex.Unlock()
	
	nb.clients[client.GetID()] = client
	slog.Debug("Network client added to broker", 
		"client_id", client.GetID(), 
		"total_clients", len(nb.clients))
}

// RemoveNetworkClient unregisters a network client.
func (nb *NetworkBroker[T]) RemoveNetworkClient(clientID string) {
	nb.clientsMutex.Lock()
	defer nb.clientsMutex.Unlock()
	
	delete(nb.clients, clientID)
	slog.Debug("Network client removed from broker", 
		"client_id", clientID, 
		"total_clients", len(nb.clients))
}

// GetNetworkClientCount returns the number of connected network clients.
func (nb *NetworkBroker[T]) GetNetworkClientCount() int {
	nb.clientsMutex.RLock()
	defer nb.clientsMutex.RUnlock()
	return len(nb.clients)
}

// Publish broadcasts an event to both local subscribers and network clients.
func (nb *NetworkBroker[T]) Publish(eventType EventType, payload T) {
	// First publish to local subscribers
	nb.Broker.Publish(eventType, payload)
	
	// Then broadcast to network clients
	nb.broadcastToNetworkClients(eventType, payload)
}

// broadcastToNetworkClients sends an event to all connected network clients.
func (nb *NetworkBroker[T]) broadcastToNetworkClients(eventType EventType, payload T) {
	nb.clientsMutex.RLock()
	defer nb.clientsMutex.RUnlock()
	
	if len(nb.clients) == 0 {
		return
	}
	
	// Create network event
	networkEvent := NetworkEvent{
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
		Source:    nb.source,
	}
	
	// Send to all clients concurrently
	var wg sync.WaitGroup
	for clientID, client := range nb.clients {
		if !client.IsConnected() {
			// Schedule for removal
			delete(nb.clients, clientID)
			continue
		}
		
		wg.Add(1)
		go func(c NetworkClient, event NetworkEvent) {
			defer wg.Done()
			
			if err := c.SendEvent(event); err != nil {
				slog.Warn("Failed to send event to network client", 
					"client_id", c.GetID(), 
					"event_type", event.Type,
					"error", err)
			}
		}(client, networkEvent)
	}
	
	// Wait for all sends to complete with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All sends completed
	case <-time.After(5 * time.Second):
		slog.Warn("Timeout waiting for network event broadcast to complete")
	}
}

// CleanupDisconnectedClients removes clients that are no longer connected.
func (nb *NetworkBroker[T]) CleanupDisconnectedClients() {
	nb.clientsMutex.Lock()
	defer nb.clientsMutex.Unlock()
	
	for clientID, client := range nb.clients {
		if !client.IsConnected() {
			delete(nb.clients, clientID)
			slog.Debug("Cleaned up disconnected client", "client_id", clientID)
		}
	}
}

// Shutdown gracefully shuts down the network broker.
func (nb *NetworkBroker[T]) Shutdown() {
	// First shutdown the regular broker
	nb.Broker.Shutdown()
	
	// Clear network clients
	nb.clientsMutex.Lock()
	defer nb.clientsMutex.Unlock()
	
	for clientID := range nb.clients {
		delete(nb.clients, clientID)
	}
	
	slog.Debug("Network broker shut down", "source", nb.source)
}

// EventFilter represents a function that can filter events before broadcasting.
type EventFilter[T any] func(eventType EventType, payload T) bool

// FilteredNetworkBroker is a network broker that can filter events before broadcasting.
type FilteredNetworkBroker[T any] struct {
	*NetworkBroker[T]
	filters []EventFilter[T]
}

// NewFilteredNetworkBroker creates a new filtered network broker.
func NewFilteredNetworkBroker[T any](source string, filters ...EventFilter[T]) *FilteredNetworkBroker[T] {
	return &FilteredNetworkBroker[T]{
		NetworkBroker: NewNetworkBroker[T](source),
		filters:       filters,
	}
}

// Publish broadcasts an event only if it passes all filters.
func (fnb *FilteredNetworkBroker[T]) Publish(eventType EventType, payload T) {
	// Check all filters
	for _, filter := range fnb.filters {
		if !filter(eventType, payload) {
			// Event filtered out
			return
		}
	}
	
	// Event passed all filters, publish normally
	fnb.NetworkBroker.Publish(eventType, payload)
}

// AddFilter adds a new event filter.
func (fnb *FilteredNetworkBroker[T]) AddFilter(filter EventFilter[T]) {
	fnb.filters = append(fnb.filters, filter)
}

// SerializableEvent represents an event that can be safely serialized for network transmission.
type SerializableEvent struct {
	Type      EventType       `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	Source    string          `json:"source"`
}

// ToSerializable converts a NetworkEvent to a SerializableEvent.
func (ne NetworkEvent) ToSerializable() (*SerializableEvent, error) {
	payloadBytes, err := json.Marshal(ne.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	return &SerializableEvent{
		Type:      ne.Type,
		Payload:   payloadBytes,
		Timestamp: ne.Timestamp,
		Source:    ne.Source,
	}, nil
}

// FromSerializable creates a NetworkEvent from a SerializableEvent.
func (se SerializableEvent) FromSerializable(payloadType interface{}) (*NetworkEvent, error) {
	if err := json.Unmarshal(se.Payload, payloadType); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	
	// Dereference pointer if needed to get the actual value
	actualPayload := payloadType
	if ptr, ok := payloadType.(*string); ok {
		actualPayload = *ptr
	}
	
	return &NetworkEvent{
		Type:      se.Type,
		Payload:   actualPayload,
		Timestamp: se.Timestamp,
		Source:    se.Source,
	}, nil
}