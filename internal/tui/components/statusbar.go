package components

import (
	"math/rand"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StatusBar struct {
	width int
	ticker *time.Ticker
	characters []rune
	colors []string
}

type StatusBarUpdateMsg struct{}

func NewStatusBar() StatusBar {
	// Orange color palette
	colors := []string{
		"#C2410C", "#DC2626", "#EA580C", "#F97316",
		"#FB923C", "#FDBA74", "#FED7AA", "#FFEDD5",
		"#FED7AA", "#FDBA74", "#FB923C", "#F97316",
		"#EA580C", "#DC2626", "#C2410C", "#9A3412",
	}
	
	return StatusBar{
		ticker: time.NewTicker(100 * time.Millisecond), // Update every 100ms for fast randomness
		colors: colors,
	}
}

func (m StatusBar) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.tickCmd(),
	)
}

func (m StatusBar) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return StatusBarUpdateMsg{}
	})
}

func (m *StatusBar) Update(msg tea.Msg) (StatusBar, tea.Cmd) {
	switch msg.(type) {
	case StatusBarUpdateMsg:
		// Only change one character per update
		if len(m.characters) > 0 {
			rand.Seed(time.Now().UnixNano())
			randomIndex := rand.Intn(len(m.characters))
			m.characters[randomIndex] = rune(0x2800 + rand.Intn(256))
		}
		return *m, m.tickCmd()
	}
	return *m, nil
}

func (m *StatusBar) View() string {
	if m.width <= 0 {
		return ""
	}
	
	// Initialize characters if not done yet or width changed
	if len(m.characters) != m.width {
		rand.Seed(time.Now().UnixNano())
		m.characters = make([]rune, m.width)
		for i := 0; i < m.width; i++ {
			m.characters[i] = rune(0x2800 + rand.Intn(256))
		}
	}
	
	var result string
	for i := 0; i < m.width; i++ {
		// Random color selection for each character
		rand.Seed(time.Now().UnixNano() + int64(i))
		colorIndex := rand.Intn(len(m.colors))
		
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors[colorIndex]))
		result += style.Render(string(m.characters[i]))
	}
	
	return result
}

func (m *StatusBar) SetSize(width, _ int) {
	m.width = width
}
