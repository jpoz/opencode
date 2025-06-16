package components

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/sst/opencode/internal/tui/style"
)

// MessageItem is a component responsible for rendering a single message.
type MessageItem struct {
	// No internal state needed for now, it's a stateless renderer.
}

// NewMessageItem creates a new MessageItem renderer.
func NewMessageItem() MessageItem {
	return MessageItem{}
}

func (mi MessageItem) View(sender, content string, availableWidth int) string {
	var senderStyle lipgloss.Style

	if sender == "You" {
		senderStyle = style.Accent.Copy().Bold(true)
	} else if sender == "Bot" {
		senderStyle = style.PrimaryText.Copy().Bold(true) // Using PrimaryText for Bot
	} else {
		senderStyle = lipgloss.NewStyle().Bold(true) // Default bold if sender is unknown
	}

	senderStyled := senderStyle.Render(sender + ": ")
	senderWidth := lipgloss.Width(senderStyled)

	contentWidth := availableWidth - senderWidth
	if contentWidth < 0 {
		contentWidth = 0 // Prevent negative width
	}

	// Style for the content part, allowing it to wrap
	contentStyled := lipgloss.NewStyle().Width(contentWidth).Render(content)

	// Combine sender and content horizontally
	combined := lipgloss.JoinHorizontal(lipgloss.Top, senderStyled, contentStyled)

	// Overall style for the message item, ensuring it doesn't exceed availableWidth
	// and adds some padding below.
	messageItemStyle := lipgloss.NewStyle().
		Width(availableWidth).
		PaddingBottom(1)

	return messageItemStyle.Render(combined)
}
