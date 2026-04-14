package tui

import "github.com/charmbracelet/lipgloss"

var (
	rose    = "#ebbcba"
	pine    = "#31748f"
	gold    = "#f6c177"
	love    = "#eb6f92"
	iris    = "#c4a7e7"
	muted   = "#6e6a86"
	foam    = "#9ccfd8"
	overlay = "#1f1d2e"
	surface = "#191724"
	base    = "#1b1b2f"

	accentStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(rose))
	successStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(pine))
	warningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(gold))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(love))
	highlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(iris))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color(muted))
	textStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(foam))
	subtleStyle    = lipgloss.NewStyle().Background(lipgloss.Color(overlay))

	borderStyle = lipgloss.NewStyle().
			BorderForeground(lipgloss.Color(muted))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(iris)).
			Bold(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(rose)).
			Bold(true).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(muted)).
			Padding(0, 1)

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(iris)).
			Bold(true)

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(muted))

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(rose)).
			Bold(true).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(muted)).
			Padding(0, 1)

	activeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(iris)).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(foam)).
			Background(lipgloss.Color(overlay)).
			Padding(0, 1)

	gitCleanStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(pine))
	gitDirtyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(gold))
	gitAheadStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(iris))
)
