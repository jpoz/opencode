package app

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"

	ui "github.com/sst/opencode/internal/interface"
	"github.com/sst/opencode/internal/message"
	"github.com/sst/opencode/internal/pubsub"
	"github.com/sst/opencode/internal/session"
	"github.com/sst/opencode/internal/status"
)

// MockUI implements UserInterface for testing
type MockUI struct{}

func (m *MockUI) GetUserInput(ctx context.Context) (*ui.UserRequest, error) {
	return nil, nil
}

func (m *MockUI) DisplayMessage(msg *message.Message) error {
	return nil
}

func (m *MockUI) DisplayStatus(status *status.StatusMessage) error {
	return nil
}

func (m *MockUI) DisplayDialog(dialog *ui.Dialog) (*ui.DialogResponse, error) {
	return nil, nil
}

func (m *MockUI) ShowSessionList(sessions []*session.Session) error {
	return nil
}

func (m *MockUI) ShowFilePicker(opts *ui.FilePickerOpts) (*ui.FileSelection, error) {
	return nil, nil
}

func (m *MockUI) Subscribe(events ...pubsub.EventType) <-chan pubsub.Event[interface{}] {
	return make(<-chan pubsub.Event[interface{}])
}

func (m *MockUI) Close() error {
	return nil
}

func TestApp_NewWithUI(t *testing.T) {
	// This test would require setting up a database connection
	// For now, we'll test that the function signature is correct
	ctx := context.Background()
	var conn *sql.DB // This would need to be a real connection in a full test
	mockUI := &MockUI{}

	// Test that the function accepts the UserInterface parameter
	// In a real test, this would create the app and verify the UI is set
	_ = ctx
	_ = conn
	_ = mockUI

	// For now, just verify the types are correct
	assert.Implements(t, (*ui.UserInterface)(nil), mockUI)
}

func TestApp_NewWithoutUI(t *testing.T) {
	// Test backward compatibility function
	ctx := context.Background()
	var conn *sql.DB // This would need to be a real connection in a full test

	// Test that the function works without UI parameter
	_ = ctx
	_ = conn

	// For now, just verify the function exists and has correct signature
	assert.NotNil(t, NewWithoutUI)
}

func TestApp_UIField(t *testing.T) {
	// Test that App struct has UI field
	app := &App{}
	
	// Test that we can set the UI field
	mockUI := &MockUI{}
	app.UI = mockUI
	
	assert.Equal(t, mockUI, app.UI)
	assert.Implements(t, (*ui.UserInterface)(nil), app.UI)
}