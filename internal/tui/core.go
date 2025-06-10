package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sst/opencode/internal/tui/components"
)

// CoreModel is the main application model.
type CoreModel struct {
	viewport viewport.Model

	messageRenderer components.MessageItem
	textarea        components.TextArea
	statusBar       components.StatusBar

	isProcessing bool

	width    int
	height   int
	quitting bool
	ready    bool
}

// NewCoreModel creates a new instance of the core application model.
func NewCoreModel() CoreModel {
	ta := components.NewTextArea("Type your message and press Enter...")
	vp := viewport.New(10, 5)
	mr := components.NewMessageItem()
	sb := components.NewStatusBar()

	m := CoreModel{
		textarea:        ta,
		viewport:        vp,
		statusBar:       sb,
		messageRenderer: mr,
		isProcessing:    false,
		ready:           false,
	}
	content := "PLACEHOLDER MESSAGES GO HERE"
	contentHeight := lipgloss.Height(content)
	if contentHeight == 0 {
		contentHeight = 1
	}
	m.viewport.Height = contentHeight
	return m
}

// Init is the first command that will be run when the model is instantiated.
func (m CoreModel) Init() tea.Cmd {
	return tea.Batch(m.textarea.Init(), m.viewport.Init(), m.statusBar.Init())
}

// Update is called when a message is received.
func (m CoreModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmdTA, cmdVP, cmdSB tea.Cmd

	if m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		inputAreaHeight := m.textarea.InputLineHeight()
		if inputAreaHeight < 1 {
			inputAreaHeight = 1
		}

		// Calculate content height dynamically
		content := "PLACEHOLDER MESSAGES GO HERE"
		contentHeight := lipgloss.Height(content)
		if contentHeight == 0 {
			contentHeight = 1 // Minimum height for "No messages yet..."
		}

		m.viewport.Width = m.width
		m.viewport.Height = contentHeight
		m.textarea.SetSize(m.width, inputAreaHeight)
		m.statusBar.SetSize(m.width, 1)
		m.viewport.SetContent(content)
		if !m.ready {
			m.ready = true
			m.viewport.GotoBottom()
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			cmds = append(cmds, tea.Quit)
			return m, tea.Batch(cmds...)
		}

	case components.SubmittedValueMsg:
		userMessageText := strings.TrimSpace(msg.Value)
		if userMessageText != "" && !m.isProcessing {
			// TODO: hook up to actually send agent the message

			content := userMessageText
			contentHeight := lipgloss.Height(content)
			if contentHeight == 0 {
				contentHeight = 1
			}
			m.viewport.Height = contentHeight
			m.viewport.SetContent(content)
			m.viewport.GotoBottom()

			// Start agent processing
			m.isProcessing = true
		}
	}

	// Store previous textarea height to detect changes
	prevTextareaHeight := m.textarea.InputLineHeight()

	m.textarea, cmdTA = m.textarea.Update(msg)
	cmds = append(cmds, cmdTA)

	// If textarea height changed, recalculate viewport height
	newTextareaHeight := m.textarea.InputLineHeight()
	if prevTextareaHeight != newTextareaHeight && m.ready {
		content := "TODO REPLACE WITH ACTUAL MESSAGES"
		contentHeight := lipgloss.Height(content)
		if contentHeight == 0 {
			contentHeight = 1
		}
		m.viewport.Height = contentHeight
		m.viewport.SetContent(content)
	}

	m.viewport, cmdVP = m.viewport.Update(msg)
	cmds = append(cmds, cmdVP)

	m.statusBar, cmdSB = m.statusBar.Update(msg)
	cmds = append(cmds, cmdSB)

	return m, tea.Batch(cmds...)
}

// View renders the UI.
func (m CoreModel) View() string {
	if m.quitting {
		return "Bye!\n"
	}
	if !m.ready {
		return "Initializing...\n"
	}

	inputView := m.textarea.View()
	viewportView := m.viewport.View()

	// Only show status bar when bot is actively streaming or agent is processing
	views := []string{viewportView}
	if m.isProcessing {
		statusBarView := m.statusBar.View()
		views = append(views, statusBarView)
	}
	views = append(views, inputView)

	finalView := lipgloss.JoinVertical(
		lipgloss.Left,
		views...,
	)

	return finalView
}
