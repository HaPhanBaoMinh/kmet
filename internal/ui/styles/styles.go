package styles

import "github.com/charmbracelet/lipgloss"

var (
	Title     = lipgloss.NewStyle().Bold(true)
	TabActive = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7DCE13"))
	Tab       = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	Header    = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	Footer    = lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
	Box       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	Danger    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87"))
	Warn      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAF00"))
	Good      = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FD7AF"))
	Faint     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C6C6C"))
)
