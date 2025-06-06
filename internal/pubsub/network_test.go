package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockNetworkClient implements NetworkClient interface for testing.
type MockNetworkClient struct {
	id        string
	connected bool
	events    []NetworkEvent
	sendErr   error
	mutex     sync.Mutex
}

func NewMockNetworkClient(id string) *MockNetworkClient {
	return &MockNetworkClient{
		id:        id,
		connected: true,
		events:    make([]NetworkEvent, 0),
	}
}

func (m *MockNetworkClient) SendEvent(event NetworkEvent) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.sendErr != nil {
		return m.sendErr
	}
	
	m.events = append(m.events, event)
	return nil
}

func (m *MockNetworkClient) GetID() string {
	return m.id
}

func (m *MockNetworkClient) IsConnected() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.connected
}

func (m *MockNetworkClient) SetConnected(connected bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.connected = connected
}

func (m *MockNetworkClient) GetEvents() []NetworkEvent {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	events := make([]NetworkEvent, len(m.events))
	copy(events, m.events)
	return events
}

func (m *MockNetworkClient) SetSendError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.sendErr = err
}

func TestNetworkBroker_AddRemoveClient(t *testing.T) {
	broker := NewNetworkBroker[string]("test-source")
	defer broker.Shutdown()
	
	// Test adding clients
	client1 := NewMockNetworkClient("client-1")
	client2 := NewMockNetworkClient("client-2")
	
	broker.AddNetworkClient(client1)
	assert.Equal(t, 1, broker.GetNetworkClientCount())
	
	broker.AddNetworkClient(client2)
	assert.Equal(t, 2, broker.GetNetworkClientCount())
	
	// Test removing clients
	broker.RemoveNetworkClient("client-1")
	assert.Equal(t, 1, broker.GetNetworkClientCount())
	
	broker.RemoveNetworkClient("client-2")
	assert.Equal(t, 0, broker.GetNetworkClientCount())
}

func TestNetworkBroker_PublishToNetworkClients(t *testing.T) {
	broker := NewNetworkBroker[string]("test-source")
	defer broker.Shutdown()
	
	// Add test clients
	client1 := NewMockNetworkClient("client-1")
	client2 := NewMockNetworkClient("client-2")
	
	broker.AddNetworkClient(client1)
	broker.AddNetworkClient(client2)
	
	// Publish an event
	broker.Publish(EventTypeCreated, "test-payload")
	
	// Give time for async broadcast
	time.Sleep(100 * time.Millisecond)
	
	// Verify both clients received the event
	events1 := client1.GetEvents()
	events2 := client2.GetEvents()
	
	assert.Len(t, events1, 1)
	assert.Len(t, events2, 1)
	
	// Verify event content
	assert.Equal(t, EventTypeCreated, events1[0].Type)
	assert.Equal(t, "test-payload", events1[0].Payload)
	assert.Equal(t, "test-source", events1[0].Source)
	
	assert.Equal(t, EventTypeCreated, events2[0].Type)
	assert.Equal(t, "test-payload", events2[0].Payload)
	assert.Equal(t, "test-source", events2[0].Source)
}

func TestNetworkBroker_DisconnectedClientCleanup(t *testing.T) {
	broker := NewNetworkBroker[string]("test-source")
	defer broker.Shutdown()
	
	// Add test clients
	client1 := NewMockNetworkClient("client-1")
	client2 := NewMockNetworkClient("client-2")
	
	broker.AddNetworkClient(client1)
	broker.AddNetworkClient(client2)
	
	assert.Equal(t, 2, broker.GetNetworkClientCount())
	
	// Disconnect one client
	client1.SetConnected(false)
	
	// Trigger cleanup
	broker.CleanupDisconnectedClients()
	
	assert.Equal(t, 1, broker.GetNetworkClientCount())
}

func TestNetworkBroker_PublishLocalAndNetwork(t *testing.T) {
	broker := NewNetworkBroker[string]("test-source")
	defer broker.Shutdown()
	
	// Add network client
	networkClient := NewMockNetworkClient("network-client")
	broker.AddNetworkClient(networkClient)
	
	// Add local subscriber
	ctx := context.Background()
	localSub := broker.Subscribe(ctx)
	
	// Publish event
	broker.Publish(EventTypeUpdated, "test-data")
	
	// Verify local subscriber received event
	select {
	case event := <-localSub:
		assert.Equal(t, EventTypeUpdated, event.Type)
		assert.Equal(t, "test-data", event.Payload)
	case <-time.After(time.Second):
		t.Fatal("Local subscriber did not receive event")
	}
	
	// Give time for network broadcast
	time.Sleep(100 * time.Millisecond)
	
	// Verify network client received event
	networkEvents := networkClient.GetEvents()
	assert.Len(t, networkEvents, 1)
	assert.Equal(t, EventTypeUpdated, networkEvents[0].Type)
	assert.Equal(t, "test-data", networkEvents[0].Payload)
}

func TestFilteredNetworkBroker(t *testing.T) {
	// Create filter that only allows "created" events
	filter := func(eventType EventType, payload string) bool {
		return eventType == EventTypeCreated
	}
	
	broker := NewFilteredNetworkBroker[string]("test-source", filter)
	defer broker.Shutdown()
	
	// Add network client
	networkClient := NewMockNetworkClient("network-client")
	broker.AddNetworkClient(networkClient)
	
	// Publish allowed event
	broker.Publish(EventTypeCreated, "allowed")
	
	// Publish filtered event
	broker.Publish(EventTypeUpdated, "filtered")
	
	// Give time for broadcast
	time.Sleep(100 * time.Millisecond)
	
	// Verify only allowed event was received
	events := networkClient.GetEvents()
	assert.Len(t, events, 1)
	assert.Equal(t, EventTypeCreated, events[0].Type)
	assert.Equal(t, "allowed", events[0].Payload)
}

func TestFilteredNetworkBroker_AddFilter(t *testing.T) {
	broker := NewFilteredNetworkBroker[string]("test-source")
	defer broker.Shutdown()
	
	// Add network client
	networkClient := NewMockNetworkClient("network-client")
	broker.AddNetworkClient(networkClient)
	
	// Add filter that blocks all events
	broker.AddFilter(func(eventType EventType, payload string) bool {
		return false // Block all events
	})
	
	// Publish event
	broker.Publish(EventTypeCreated, "blocked")
	
	// Give time for potential broadcast
	time.Sleep(100 * time.Millisecond)
	
	// Verify no events were received
	events := networkClient.GetEvents()
	assert.Len(t, events, 0)
}

func TestNetworkEvent_Serialization(t *testing.T) {
	originalEvent := NetworkEvent{
		Type:      EventTypeCreated,
		Payload:   "test-payload",
		Timestamp: time.Now(),
		Source:    "test-source",
	}
	
	// Convert to serializable
	serializable, err := originalEvent.ToSerializable()
	require.NoError(t, err)
	
	// Verify JSON serialization works
	jsonBytes, err := json.Marshal(serializable)
	require.NoError(t, err)
	
	// Deserialize back
	var deserializedSerializable SerializableEvent
	err = json.Unmarshal(jsonBytes, &deserializedSerializable)
	require.NoError(t, err)
	
	// Convert back to NetworkEvent
	var payload string
	reconstructedEvent, err := deserializedSerializable.FromSerializable(&payload)
	require.NoError(t, err)
	
	// Verify content
	assert.Equal(t, originalEvent.Type, reconstructedEvent.Type)
	assert.Equal(t, originalEvent.Payload, payload) // Compare with the unmarshaled payload directly
	assert.Equal(t, originalEvent.Source, reconstructedEvent.Source)
	// Note: Timestamp might have slight precision differences due to JSON serialization
}

func TestNetworkBroker_ErrorHandling(t *testing.T) {
	broker := NewNetworkBroker[string]("test-source")
	defer broker.Shutdown()
	
	// Add client that will return an error
	errorClient := NewMockNetworkClient("error-client")
	errorClient.SetSendError(assert.AnError)
	
	// Add normal client
	normalClient := NewMockNetworkClient("normal-client")
	
	broker.AddNetworkClient(errorClient)
	broker.AddNetworkClient(normalClient)
	
	// Publish event
	broker.Publish(EventTypeCreated, "test")
	
	// Give time for broadcast
	time.Sleep(200 * time.Millisecond)
	
	// Verify normal client still received the event
	normalEvents := normalClient.GetEvents()
	assert.Len(t, normalEvents, 1)
	
	// Verify error client has no events due to send error
	errorEvents := errorClient.GetEvents()
	assert.Len(t, errorEvents, 0)
}

func TestNetworkBroker_Shutdown(t *testing.T) {
	broker := NewNetworkBroker[string]("test-source")
	
	// Add clients
	client1 := NewMockNetworkClient("client-1")
	client2 := NewMockNetworkClient("client-2")
	
	broker.AddNetworkClient(client1)
	broker.AddNetworkClient(client2)
	
	assert.Equal(t, 2, broker.GetNetworkClientCount())
	
	// Shutdown
	broker.Shutdown()
	
	// Verify all clients are removed
	assert.Equal(t, 0, broker.GetNetworkClientCount())
}

func BenchmarkNetworkBroker_Publish(b *testing.B) {
	broker := NewNetworkBroker[string]("benchmark-source")
	defer broker.Shutdown()
	
	// Add multiple mock clients
	for i := 0; i < 100; i++ {
		client := NewMockNetworkClient(fmt.Sprintf("client-%d", i))
		broker.AddNetworkClient(client)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		broker.Publish(EventTypeCreated, "benchmark-payload")
	}
}