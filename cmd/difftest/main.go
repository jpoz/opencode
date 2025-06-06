package main

import (
   "fmt"
   "io"
   "os"
   "strings"

   tea "github.com/charmbracelet/bubbletea"
   "github.com/charmbracelet/lipgloss"
   xansi "github.com/charmbracelet/x/ansi"
   "github.com/sst/opencode/internal/tui/layout"
   "github.com/sst/opencode/internal/diff"
)

type model struct {
	content   string
	width     int
	height    int
	scrollY   int
	maxScroll int
}

func initialModel() model {
	return model{
		content: "",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Re-render content with new width
		if m.content != "" {
			rendered, err := diff.FormatDiff(m.content, diff.WithTotalWidth(m.width))
			if err != nil {
				rendered = fmt.Sprintf("Error rendering diff: %v", err)
			}
			lines := strings.Split(rendered, "\n")
			m.maxScroll = max(0, len(lines)-m.height+2)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.scrollY > 0 {
				m.scrollY--
			}
		case "down", "j":
			if m.scrollY < m.maxScroll {
				m.scrollY++
			}
		case "pageup":
			m.scrollY = max(0, m.scrollY-m.height+2)
		case "pagedown":
			m.scrollY = min(m.maxScroll, m.scrollY+m.height-2)
		case "home":
			m.scrollY = 0
		case "end":
			m.scrollY = m.maxScroll
		}
	}

	return m, nil
}

// View renders the diff in a hovering popup box over a blank background.
func (m model) View() string {
   if m.content == "" || m.height < 4 || m.width < 10 {
       return "Loading diff..."
   }
   // Calculate inner content width (box width minus border and padding)
   // Calculate available width for diff content inside the box
   diffWidth := m.width - 6
   // Render side-by-side diff
   rendered, err := diff.FormatDiff(m.content, diff.WithTotalWidth(diffWidth))
   if err != nil {
       return fmt.Sprintf("Error rendering diff: %v", err)
   }
   // Manually wrap long lines to diffWidth to avoid box overflow
   rawLines := strings.Split(rendered, "\n")
   var lines []string
   for _, rl := range rawLines {
       rem := rl
       for lipgloss.Width(rem) > diffWidth {
           part := xansi.Cut(rem, diffWidth, lipgloss.Width(rem))
           lines = append(lines, part)
           rem = strings.TrimPrefix(rem, part)
       }
       lines = append(lines, rem)
   }
   // Apply scrolling to wrapped lines
   total := len(lines)
   start := max(0, m.scrollY)
   end := min(total, start+m.height-6)
   display := strings.Join(lines[start:end], "\n")
   // Status line for navigation
   status := fmt.Sprintf("Lines %d-%d of %d | q: quit | ↑↓/jk: scroll | PgUp/PgDn | Home/End", start+1, end, total)
   // Combine content and status
   inner := display + "\n" + status
   // Style popup box with consistent width and padding
   boxWidth := m.width - 2
   box := lipgloss.NewStyle().
       Border(lipgloss.RoundedBorder()).
       BorderForeground(lipgloss.Color("240")).
       Padding(1, 1).
       Width(boxWidth).
       Height(min(len(strings.Split(inner, "\n"))+2, m.height-4)).
       Render(inner)
   // Blank background full size
   bg := lipgloss.NewStyle().
       Width(m.width).
       Height(m.height).
       Render(" ")
   // Overlay popup, centering horizontally with 1-cell margin
   x := 1
   y := (m.height - lipgloss.Height(box)) / 2
   return layout.PlaceOverlay(x, y, box, bg, true)
}

func loadDiffFromStdin() (string, error) {
	var sb strings.Builder
	_, err := io.Copy(&sb, os.Stdin)
	return sb.String(), err
}

func loadDiffFromFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	return string(data), err
}

func loadSampleDiff() string {
	return `--- a/example.go
+++ b/example.go
@@ -1,10 +1,12 @@
 package main
 
 import (
 	"fmt"
+	"os"
+	"log"
 )
 
-func main() {
-	fmt.Println("Hello, World!")
+func main() {
+	fmt.Println("Hello, OpenCode!")
+	log.Println("Application started")
 }
`
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	var diffContent string
	var err error

	// Check for --static flag for non-interactive mode
	staticMode := false
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--static" {
		staticMode = true
		args = args[1:]
	}

	if len(args) > 0 {
		// Load diff from file
		diffContent, err = loadDiffFromFile(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Check if stdin has data
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Data is being piped in
			diffContent, err = loadDiffFromStdin()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Use sample diff
			diffContent = loadSampleDiff()
		}
	}

	if strings.TrimSpace(diffContent) == "" {
		fmt.Fprintf(os.Stderr, "No diff content provided\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [--static] [diff-file] or pipe diff via stdin\n", os.Args[0])
		os.Exit(1)
	}

	if staticMode {
		// Just render and print the diff without bubbletea
		rendered, err := diff.FormatDiff(diffContent, diff.WithTotalWidth(120))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error rendering diff: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(rendered)
		return
	}

	m := initialModel()
	m.content = diffContent

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

