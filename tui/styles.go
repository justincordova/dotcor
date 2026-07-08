package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Catppuccin Mocha — higher contrast than Rose Pine, reads better in TUIs.
const (
	colBase     = "#1e1e2e"
	colMantle   = "#181825"
	colCrust    = "#11111b"
	colSurface0 = "#313244"
	colSurface1 = "#45475a"
	colSurface2 = "#585b70"
	colOverlay0 = "#6c7086"
	colOverlay1 = "#7f849c"
	colText     = "#cdd6f4"
	colSubtext1 = "#bac2de"
	colSubtext0 = "#a6adc8"
	colRed      = "#f38ba8"
	colMaroon   = "#eba0ac"
	colPeach    = "#fab387"
	colYellow   = "#f9e2af"
	colGreen    = "#a6e3a1"
	colTeal     = "#94e2d5"
	colSky      = "#89dceb"
	colBlue     = "#89b4fa"
	colLavender = "#b4befe"
	colMauve    = "#cba6f7"
	colPink     = "#f5c2e7"
	colFlamingo = "#f2cdcd"
)

// Semantic style tokens.
var (
	// Text
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colSubtext0))
	textStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(colText))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colSurface2))

	accentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colPink)).Bold(true)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colGreen))
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colYellow))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colRed))

	selectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(colSurface0)).
				Bold(true)

	keyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colLavender)).Bold(true)
	descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colSubtext0))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colSurface1)).
			Padding(0, 1)

	activeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colMauve)).
			Padding(0, 1)
)

// ─── Reusable components ─────────────────────────────────────────────────────

// selectionMarker is the canonical "this row is focused" glyph. Every list
// surface (packages, history, settings) uses it so selection looks and reads
// identically everywhere, instead of each list inventing its own cue.
const selectionMarker = "▸"

// selectableRow renders one list row with a consistent selection treatment:
// a leading marker + filled background when selected, matching indentation
// when not. width is the row's target display width for the background fill.
func selectableRow(content string, selected bool, width int) string {
	if selected {
		row := accentStyle.Render(selectionMarker+" ") + content
		return selectedRowStyle.Width(width).Render(row)
	}
	return "  " + content
}

// pill renders a label inside a colored background — use for tags, counts, status.
func pill(label string, fg, bg string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg)).
		Padding(0, 1).
		Render(label)
}

// panel wraps content in a titled, bordered box. When active, the border color
// highlights.
func panel(title, body string, width, height int, active bool) string {
	style := boxStyle
	titleColor := colSubtext1
	if active {
		style = activeBoxStyle
		titleColor = colMauve
	}

	style = style.Width(width - 2).Height(height - 2)

	titleBar := ""
	if title != "" {
		titleBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color(titleColor)).
			Bold(true).
			Render(title) + "\n"
	}

	return style.Render(titleBar + body)
}

// kbd renders a keyboard hint: `[key] desc`.
func kbd(k, desc string) string {
	return keyStyle.Render(k) + "\u00a0" + descStyle.Render(desc)
}

// joinHints joins key hints with a spaced separator.
func joinHints(hints ...string) string {
	return strings.Join(hints, dimStyle.Render("  ·  "))
}

// metaSep is the canonical separator for inline metadata (a dimmed middle
// dot). Use this everywhere two inline facts sit side by side so the whole
// UI speaks one separator language.
func metaSep() string {
	return dimStyle.Render(" · ")
}

// hRule draws a horizontal rule of given width in the subtle border color.
func hRule(width int) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colSurface1)).
		Render(strings.Repeat("─", width))
}

// truncate a string to width with an ellipsis.
func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width-1]) + "…"
}

// padRight pads a string to a given display width on the right.
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func humanSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
	)
	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// countLabel returns "N thing" / "N things" for friendly pluralization.
func countLabel(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}
