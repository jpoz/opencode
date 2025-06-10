package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jpoz/falken/internal/tui/style"
)

// SubmittedValueMsg is a message sent when the user presses enter in the textarea.
// It contains the value of the textarea at that moment.
type SubmittedValueMsg struct {
	Value string
}

// TextArea represents a textarea component.
type TextArea struct {
	textarea            textarea.Model
	width               int // Overall width for the component
	height              int // Number of lines for the textarea input itself
	originalPlaceholder string
}

// NewTextArea creates a new textarea model.
func NewTextArea(placeholder string) TextArea {
	ta := textarea.New()

	bgColor := lipgloss.NoColor{}
	textColor := lipgloss.Color("15")
	textMutedColor := lipgloss.Color("7")

	ta.BlurredStyle.Base = style.BaseStyle().Background(bgColor).Foreground(textColor)
	ta.BlurredStyle.CursorLine = style.BaseStyle().Background(bgColor)
	ta.BlurredStyle.Placeholder = style.BaseStyle().Background(bgColor).Foreground(textMutedColor)
	ta.BlurredStyle.Text = style.BaseStyle().Background(bgColor).Foreground(textColor)
	ta.FocusedStyle.Base = style.BaseStyle().Background(bgColor).Foreground(textColor)
	ta.FocusedStyle.CursorLine = style.BaseStyle().Background(bgColor)
	ta.FocusedStyle.Placeholder = style.BaseStyle().Background(bgColor).Foreground(textMutedColor)
	ta.FocusedStyle.Text = style.BaseStyle().Background(bgColor).Foreground(textColor)

	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.Placeholder = placeholder

	ta.Focus()

	initialInputWidth := 50
	initialInputHeight := 1
	ta.SetWidth(initialInputWidth)
	ta.SetHeight(initialInputHeight)

	return TextArea{
		textarea:            ta,
		width:               initialInputWidth + lipgloss.Width(style.Accent.Render(">")) + 1,
		height:              initialInputHeight,
		originalPlaceholder: placeholder,
	}
}

// Init initializes the textarea model.
func (m TextArea) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages for the textarea model.
func (m TextArea) Update(msg tea.Msg) (TextArea, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If the textarea is not focused, it should not process keystrokes (except maybe a focus key if desired)
		if !m.textarea.Focused() {
			return m, nil
		}
		switch msg.Type {
		case tea.KeyEnter:
			value := m.textarea.Value()
			m.textarea.SetValue("")
			m.textarea.Placeholder = m.originalPlaceholder
			return m, func() tea.Msg {
				return SubmittedValueMsg{Value: value}
			}
		}
	}

	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the textarea.
func (m TextArea) View() string {
	promptText := "> "
	promptRendered := style.Accent.Render(string(promptText[0])) + string(promptText[1:])
	promptWidth := lipgloss.Width(promptRendered)

	inputBoxWidth := m.width - promptWidth
	if inputBoxWidth < 1 {
		inputBoxWidth = 1
	}
	m.textarea.SetWidth(inputBoxWidth)

	// Calculate height based on content
	content := m.textarea.Value()
	if content == "" {
		m.textarea.SetHeight(1) // Minimum height when empty
	} else {
		// Calculate how many lines the content needs
		lines := strings.Split(content, "\n")
		contentHeight := len(lines)
		// Add some buffer for wrapping
		for _, line := range lines {
			if len(line) > inputBoxWidth {
				contentHeight += (len(line) / inputBoxWidth)
			}
		}
		if contentHeight < 1 {
			contentHeight = 1
		}
		m.textarea.SetHeight(contentHeight)
	}

	// Ensure the combined result takes up the full width
	combined := lipgloss.JoinHorizontal(lipgloss.Top, promptRendered, m.textarea.View())

	// Apply full width styling to ensure it spans the entire available width
	fullWidthStyle := lipgloss.NewStyle().Width(m.width)
	return fullWidthStyle.Render(combined)
}

// Value returns the current value of the textarea.
func (m TextArea) Value() string {
	return m.textarea.Value()
}

// Focus sets the textarea to be focused.
func (m *TextArea) Focus() tea.Cmd {
	return m.textarea.Focus()
}

// Blur removes focus from the textarea.
func (m *TextArea) Blur() {
	m.textarea.Blur()
}

// SetSize sets the overall width for the component and height (lines) for the textarea input.
func (m *TextArea) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.textarea.SetWidth(width - lipgloss.Width(style.Accent.Render("> ")) - 1)
}

// Focused returns whether the textarea is currently focused.
func (m TextArea) Focused() bool {
	return m.textarea.Focused()
}

// InputLineHeight returns the configured number of lines for the textarea input box.
func (m TextArea) InputLineHeight() int {
	// Calculate dynamic height based on content
	content := m.textarea.Value()
	if content == "" {
		return 1 // Minimum height when empty
	}

	promptWidth := lipgloss.Width("> ")
	inputBoxWidth := m.width - promptWidth
	if inputBoxWidth < 1 {
		inputBoxWidth = 1
	}

	lines := strings.Split(content, "\n")
	contentHeight := len(lines)
	// Add some buffer for wrapping
	for _, line := range lines {
		if len(line) > inputBoxWidth {
			contentHeight += (len(line) / inputBoxWidth)
		}
	}
	if contentHeight < 1 {
		contentHeight = 1
	}
	return contentHeight
}

// SetPlaceholder changes the placeholder text.
func (m *TextArea) SetPlaceholder(text string) {
	m.textarea.Placeholder = text
}

// ResetPlaceholder reverts to the original placeholder text.
func (m *TextArea) ResetPlaceholder() {
	m.textarea.Placeholder = m.originalPlaceholder
}
