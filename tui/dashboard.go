package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justincordova/dotcor/internal/stow"
)

func viewDashboard(m Model) string {
	header := renderHeader(m)
	stats := renderStatsStrip(m)
	activity := renderActivityStrip(m)
	gitBar := renderGitBar(m)
	footer := renderFooter(m)

	fixedHeight := lipgloss.Height(header) + lipgloss.Height(stats) + lipgloss.Height(activity) + lipgloss.Height(gitBar) + lipgloss.Height(footer)
	mainHeight := m.height - fixedHeight
	if mainHeight < 10 {
		mainHeight = 10
	}
	main := renderMainWithHeight(m, mainHeight)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		stats,
		main,
		activity,
		gitBar,
		footer,
	)
}

// ─── Header ──────────────────────────────────────────────────────────────────

func renderHeader(m Model) string {
	logo := renderLogo()
	ver := dimStyle.Render(m.version)

	nav := renderNav()
	left := lipgloss.JoinHorizontal(lipgloss.Center, logo, " ", ver)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(nav) - 2
	if gap < 1 {
		gap = 1
	}
	spacer := strings.Repeat(" ", gap)

	row := lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, nav)

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Render(row)
}

// renderLogo paints "DotCor" with a mauve → pink per-char gradient.
func renderLogo() string {
	text := "◆ DotCor"
	gradient := []string{colMauve, colMauve, colLavender, colPink, colPink, colFlamingo, colFlamingo, colPink}
	var b strings.Builder
	for i, r := range text {
		color := gradient[i%len(gradient)]
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(color)).
			Bold(true).
			Render(string(r)))
	}
	return b.String()
}

func renderNav() string {
	items := []string{
		kbd("D", "diff"),
		kbd("H", "history"),
		kbd("L", "logs"),
		kbd("g", "settings"),
		kbd("?", "help"),
	}
	return strings.Join(items, dimStyle.Render("  "))
}

// ─── Stats strip ─────────────────────────────────────────────────────────────

func renderStatsStrip(m Model) string {
	var linkedFiles, totalFiles int
	for _, p := range m.packages {
		for _, f := range p.Files {
			totalFiles++
			if f.IsLinked {
				linkedFiles++
			}
		}
	}

	filesVal := fmt.Sprintf("%d/%d", linkedFiles, totalFiles)
	filesColor := colGreen
	if linkedFiles < totalFiles {
		filesColor = colYellow
	}
	if totalFiles == 0 {
		filesVal = "—"
		filesColor = colOverlay0
	}

	branchVal := "—"
	branchColor := colOverlay0
	if m.gitStatus.Branch != "" {
		branchVal = m.gitStatus.Branch
		branchColor = colLavender
		if m.gitStatus.AheadBy > 0 {
			branchVal += fmt.Sprintf(" ↑%d", m.gitStatus.AheadBy)
		}
		if m.gitStatus.BehindBy > 0 {
			branchVal += fmt.Sprintf(" ↓%d", m.gitStatus.BehindBy)
		}
	}

	syncVal := "—"
	syncColor := colOverlay0
	if len(m.recentCommits) > 0 && !m.recentCommits[0].Date.IsZero() {
		syncVal = formatRelativeTime(m.recentCommits[0].Date)
		syncColor = colBlue
	}

	repoVal := fmt.Sprintf("%d %s", len(m.packages), pluralize(len(m.packages), "pkg"))
	if size := repoSizeMB(m.repoDir); size > 0 {
		repoVal += fmt.Sprintf(" · %.1fMB", size)
	}

	items := []string{
		statInline("FILES", filesVal, filesColor),
		statInline("BRANCH", branchVal, branchColor),
		statInline("SYNC", syncVal, syncColor),
		statInline("REPO", repoVal, colMauve),
	}

	sep := dimStyle.Render("  │  ")
	row := strings.Join(items, sep)

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 2).
		Render(row)
}

func statInline(label, value, valueColor string) string {
	l := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colOverlay1)).
		Bold(true).
		Render(label)
	v := lipgloss.NewStyle().
		Foreground(lipgloss.Color(valueColor)).
		Bold(true).
		Render(value)
	return l + " " + v
}

// ─── Main (packages + detail) ────────────────────────────────────────────────

func renderMainWithHeight(m Model, mainHeight int) string {
	leftWidth := m.width * 2 / 5
	if leftWidth < 36 {
		leftWidth = 36
	}
	rightWidth := m.width - leftWidth

	left := renderPackagePanel(m, leftWidth, mainHeight)
	right := renderDetailPanel(m, rightWidth, mainHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func renderPackagePanel(m Model, width, height int) string {
	title := fmt.Sprintf(" Packages %s", dimStyle.Render(fmt.Sprintf("(%d)", len(m.packages))))
	body := renderPackageList(m, width-4, height-4)
	return panel(title, body, width, height, !m.searching)
}

func renderDetailPanel(m Model, width, height int) string {
	var title string
	if m.selectedPkg < len(m.packages) {
		p := m.packages[m.selectedPkg]
		title = fmt.Sprintf(" %s %s",
			accentStyle.Render(p.Name),
			dimStyle.Render(fmt.Sprintf("(%d files)", len(p.Files))),
		)
	} else {
		title = " Details"
	}

	body := renderFileDetail(m, width-4, height-4)
	return panel(title, body, width, height, false)
}

// ─── Package list as cards ───────────────────────────────────────────────────

func renderPackageList(m Model, width, maxLines int) string {
	if len(m.packages) == 0 {
		return renderEmptyPackages(width)
	}

	if m.searching {
		return renderSearchInput(m, width)
	}

	// Each card is 3 lines + 1 spacer; show as many as fit.
	cardLines := 2
	perCard := cardLines
	maxCards := maxLines / perCard
	if maxCards < 1 {
		maxCards = 1
	}

	start, end := visibleRange(m.selectedPkg, len(m.packages), maxCards)

	var parts []string
	for i := start; i < end; i++ {
		parts = append(parts, renderPackageCard(m, i, width))
	}

	return strings.Join(parts, "\n")
}

func renderPackageCard(m Model, i, width int) string {
	pkg := m.packages[i]
	selected := i == m.selectedPkg

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colSurface1))
	if selected {
		barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colMauve)).Bold(true)
	}
	bar := barStyle.Render("▌")

	indent := bar + "  "
	contentWidth := width - 3
	if contentWidth < 20 {
		contentWidth = 20
	}

	linked, total := 0, len(pkg.Files)
	for _, f := range pkg.Files {
		if f.IsLinked {
			linked++
		}
	}

	name := textStyle.Bold(true).Render(pkg.Name)
	tag := categoryTag(pkg.Name)

	// Line 1: circle + name + flexible gap + tag
	leftW := lipgloss.Width(name)
	rightW := lipgloss.Width(tag)
	gap := contentWidth - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	line1 := bar + " " + name + strings.Repeat(" ", gap) + tag

	var progress string
	switch {
	case total == 0:
		progress = dimStyle.Render("empty")
	case linked == total:
		progress = successStyle.Render(fmt.Sprintf("✓ %d/%d", linked, total))
	case linked == 0:
		progress = errorStyle.Render(fmt.Sprintf("✗ %d/%d", linked, total))
	default:
		progress = warningStyle.Render(fmt.Sprintf("◐ %d/%d", linked, total))
	}
	modified := dimStyle.Render(relativeModTime(pkg.Path))
	line2 := indent + progress + dimStyle.Render(" · ") + modified

	return lipgloss.JoinVertical(lipgloss.Left, line1, line2)
}

func renderEmptyPackages(width int) string {
	return strings.Join([]string{
		"",
		textStyle.Render("No packages yet."),
		"",
		dimStyle.Render("Press ") + kbd("a", "add") + dimStyle.Render(" to stow your first dotfile."),
	}, "\n")
}

func renderSearchInput(m Model, width int) string {
	prompt := accentStyle.Render("/")
	return fmt.Sprintf("%s %s\n\n%s",
		prompt,
		m.searchInput.View(),
		dimStyle.Render("enter to jump · esc to cancel"),
	)
}

// ─── File detail ─────────────────────────────────────────────────────────────

func renderFileDetail(m Model, width, maxLines int) string {
	if m.selectedPkg >= len(m.packages) {
		return dimStyle.Render("No package selected")
	}

	pkg := m.packages[m.selectedPkg]

	if len(pkg.Files) == 0 {
		return dimStyle.Render("No files in this package.")
	}

	var b strings.Builder

	linked, conflicts := 0, 0
	for _, f := range pkg.Files {
		if f.IsLinked {
			linked++
		} else if f.Exists && !f.IsSymlink {
			conflicts++
		}
	}

	summary := []string{
		pill(fmt.Sprintf("linked %d", linked), colBase, colGreen),
	}
	if conflicts > 0 {
		summary = append(summary, pill(fmt.Sprintf("conflict %d", conflicts), colBase, colRed))
	}
	unlinked := len(pkg.Files) - linked - conflicts
	if unlinked > 0 {
		summary = append(summary, pill(fmt.Sprintf("unlinked %d", unlinked), colBase, colOverlay0))
	}
	b.WriteString(strings.Join(summary, " "))
	b.WriteString("\n")
	b.WriteString(hRule(width))
	b.WriteString("\n")

	start, end := visibleRange(m.selectedFile, len(pkg.Files), maxLines-3)
	for i := start; i < end; i++ {
		f := pkg.Files[i]
		selected := i == m.selectedFile && m.expanded[m.selectedPkg]

		statusBadge := fileBadge(f)
		rel := truncate(f.RelPath, width/2-4)
		target := truncate(collapseHome(f.TargetPath, m.homeDir), width/2-2)
		arrow := dimStyle.Render("→")

		line := fmt.Sprintf("%s %s %s %s", statusBadge, textStyle.Render(rel), arrow, dimStyle.Render(target))
		if selected {
			line = selectedRowStyle.Width(width).Render("▸ " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(pkg.Files) > maxLines-3 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  … showing %d of %d", end-start, len(pkg.Files))))
	}

	return b.String()
}

// ─── Activity strip ──────────────────────────────────────────────────────────

func renderActivityStrip(m Model) string {
	titleLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colLavender)).
		Bold(true).
		Render("Recent activity")

	var body string
	if len(m.recentCommits) == 0 {
		body = dimStyle.Render("No commits yet — press ") + kbd("S", "sync") + dimStyle.Render(" to commit changes")
	} else {
		var lines []string
		n := 3
		if n > len(m.recentCommits) {
			n = len(m.recentCommits)
		}

		for i := 0; i < n; i++ {
			c := m.recentCommits[i]
			shortHash := c.Hash
			if len(shortHash) > 7 {
				shortHash = shortHash[:7]
			}
			hash := lipgloss.NewStyle().Foreground(lipgloss.Color(colPeach)).Render(shortHash)
			subjectBudget := m.width - 28
			if subjectBudget < 10 {
				subjectBudget = 10
			}
			subject := textStyle.Render(truncate(c.Message, subjectBudget))
			when := dimStyle.Render(padRight(formatRelativeTime(c.Date), 10))
			lines = append(lines, fmt.Sprintf("  %s  %s  %s", hash, when, subject))
		}
		body = strings.Join(lines, "\n")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleLine,
		subtleStyle.Render(strings.Repeat("─", max(m.width-4, 4))),
		body,
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 2).
		Render(content)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ─── Git + footer ────────────────────────────────────────────────────────────

func renderGitBar(m Model) string {
	if m.err != nil {
		return lipgloss.NewStyle().
			Width(m.width).
			Foreground(lipgloss.Color(colRed)).
			Bold(true).
			Render(fmt.Sprintf("  ✗ %v", m.err))
	}
	if m.statusMsg != "" {
		return lipgloss.NewStyle().
			Width(m.width).
			Foreground(lipgloss.Color(colGreen)).
			Bold(true).
			Render(fmt.Sprintf("  ✓ %s", m.statusMsg))
	}

	var parts []string

	if m.gitStatus.Branch != "" {
		parts = append(parts, pill("⎇ "+m.gitStatus.Branch, colBase, colLavender))
	}

	if m.gitStatus.HasUncommitted {
		parts = append(parts,
			pill(fmt.Sprintf("● %s", countLabel(len(m.gitStatus.ChangedFiles), "change")), colBase, colYellow),
		)
	} else if m.gitStatus.Branch != "" {
		parts = append(parts, pill("● clean", colBase, colGreen))
	}

	if m.gitStatus.AheadBy > 0 {
		parts = append(parts, pill(fmt.Sprintf("↑ %d", m.gitStatus.AheadBy), colBase, colBlue))
	}
	if m.gitStatus.BehindBy > 0 {
		parts = append(parts, pill(fmt.Sprintf("↓ %d", m.gitStatus.BehindBy), colBase, colPeach))
	}

	if len(parts) == 0 {
		parts = append(parts, dimStyle.Render("no git repository"))
	}

	left := "  " + strings.Join(parts, " ")
	right := dimStyle.Render(collapseHome(m.repoDir, m.homeDir))
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := m.width - leftW - rightW - 2
	if gap < 2 {
		gap = 2
	}

	return lipgloss.NewStyle().Width(m.width).Render(left + strings.Repeat(" ", gap) + right + " ")
}

func renderFooter(m Model) string {
	if m.searching {
		return lipgloss.NewStyle().
			Width(m.width).
			Render("  " + accentStyle.Render("/") + " " + m.searchInput.View())
	}

	hints := joinHints(
		kbd("↑↓/jk", "nav"),
		kbd("enter", "expand"),
		kbd("s", "stow"),
		kbd("u", "unstow"),
		kbd("a", "add"),
		kbd("S", "sync"),
		kbd("/", "search"),
		kbd("q", "quit"),
	)

	hintsW := lipgloss.Width(hints)
	gap := (m.width - hintsW) / 2
	if gap < 2 {
		gap = 2
	}

	return lipgloss.NewStyle().Width(m.width).Render(strings.Repeat(" ", gap) + hints)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func fileBadge(f stow.FileEntry) string {
	switch {
	case f.IsLinked:
		return pill("LINK", colBase, colGreen)
	case f.Exists && !f.IsSymlink:
		return pill("CONF", colBase, colRed)
	default:
		return pill("NONE", colText, colSurface1)
	}
}

// categoryTag returns a small colored outlined pill based on package name.
func categoryTag(name string) string {
	lower := strings.ToLower(name)
	tag, color := "", ""
	switch {
	case containsAny(lower, "nvim", "vim", "emacs", "helix", "nano", "code", "vscode"):
		tag, color = "editor", colMauve
	case containsAny(lower, "zsh", "bash", "fish", "shell", "starship"):
		tag, color = "shell", colGreen
	case containsAny(lower, "tmux", "kitty", "alacritty", "wezterm", "ghostty", "screen", "foot"):
		tag, color = "terminal", colBlue
	case containsAny(lower, "i3", "sway", "hypr", "bspwm", "river", "dwm", "awesome", "qtile"):
		tag, color = "wm", colPink
	case containsAny(lower, "git", "gh", "lazygit"):
		tag, color = "vcs", colPeach
	case containsAny(lower, "polybar", "waybar", "eww", "rofi", "dunst", "mako"):
		tag, color = "desktop", colSky
	default:
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Render(tag)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func pluralize(n int, singular string) string {
	if n == 1 {
		return singular
	}
	return singular + "s"
}

func relativeModTime(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "—"
	}
	return formatRelativeTime(info.ModTime())
}

// repoSizeMB returns the total size of the repo dir in MB (rough estimate from top-level).
func repoSizeMB(repoDir string) float64 {
	var total int64
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if e.Name() == ".git" || e.Name() == "logs" || e.Name() == "backups" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.IsDir() {
			total += dirSize(filepath.Join(repoDir, e.Name()))
		} else {
			total += info.Size()
		}
	}
	return float64(total) / (1024 * 1024)
}

func dirSize(path string) int64 {
	var total int64
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		full := filepath.Join(path, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.IsDir() {
			total += dirSize(full)
		} else {
			total += info.Size()
		}
	}
	return total
}

func collapseHome(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func visibleRange(sel, total, maxLines int) (int, int) {
	if total <= maxLines || maxLines <= 0 {
		if total < 0 {
			return 0, 0
		}
		return 0, total
	}
	start := sel - maxLines/2
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > total {
		end = total
		start = end - maxLines
		if start < 0 {
			start = 0
		}
	}
	return start, end
}
