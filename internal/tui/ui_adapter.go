package tui

import (
	"context"
	"fmt"

	ui "github.com/sst/opencode/internal/interface"
	"github.com/sst/opencode/internal/message"
	"github.com/sst/opencode/internal/pubsub"
	"github.com/sst/opencode/internal/session"
	"github.com/sst/opencode/internal/status"
)

// TUIAdapter adapts the existing TUI to implement the UserInterface.
// This allows the TUI to work with the new interface-based architecture.
type TUIAdapter struct {
	model *appModel
}

// NewTUIAdapter creates a new TUI adapter.
func NewTUIAdapter(model *appModel) *TUIAdapter {
	return &TUIAdapter{
		model: model,
	}
}

// GetUserInput implements UserInterface.GetUserInput.
func (a *TUIAdapter) GetUserInput(ctx context.Context) (*ui.UserRequest, error) {
	// This would need to be implemented to capture user input from the TUI
	// For now, return an error indicating this is not yet implemented
	return nil, fmt.Errorf("GetUserInput not yet implemented for TUI adapter")
}

// DisplayMessage implements UserInterface.DisplayMessage.
func (a *TUIAdapter) DisplayMessage(msg *message.Message) error {
	// The TUI already handles message display through its message handling system
	// This is a no-op since the TUI gets messages through the pubsub system
	return nil
}

// DisplayStatus implements UserInterface.DisplayStatus.
func (a *TUIAdapter) DisplayStatus(status *status.StatusMessage) error {
	// The TUI already handles status display through its status system
	// This is a no-op since the TUI gets status updates through the pubsub system
	return nil
}

// DisplayDialog implements UserInterface.DisplayDialog.
func (a *TUIAdapter) DisplayDialog(dialog *ui.Dialog) (*ui.DialogResponse, error) {
	// This would need to be implemented to show dialogs in the TUI
	// For now, return a default cancel response
	return &ui.DialogResponse{
		DialogID:  dialog.ID,
		Action:    ui.ActionCancel,
		Cancelled: true,
	}, nil
}

// ShowSessionList implements UserInterface.ShowSessionList.
func (a *TUIAdapter) ShowSessionList(sessions []*session.Session) error {
	// The TUI already handles session list display
	// This is a no-op since the TUI manages sessions through its own interface
	return nil
}

// ShowFilePicker implements UserInterface.ShowFilePicker.
func (a *TUIAdapter) ShowFilePicker(opts *ui.FilePickerOpts) (*ui.FileSelection, error) {
	// This would need to be implemented to show file picker in the TUI
	// For now, return a cancelled selection
	return &ui.FileSelection{
		Paths:     []string{},
		Cancelled: true,
	}, nil
}

// Subscribe implements UserInterface.Subscribe.
func (a *TUIAdapter) Subscribe(events ...pubsub.EventType) <-chan pubsub.Event[interface{}] {
	// Create a channel and return it
	// The TUI already handles events through its own pubsub system
	ch := make(chan pubsub.Event[interface{}], 100)
	
	// In a full implementation, this would wire up the TUI's event system
	// to forward events to this channel
	
	return ch
}

// Close implements UserInterface.Close.
func (a *TUIAdapter) Close() error {
	// The TUI has its own lifecycle management
	// This is a no-op for now
	return nil
}