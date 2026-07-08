package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/justincordova/dotcor/internal/stow"
)

func viewDashboard(m Model) string {
	base := renderDashboardBase(m)
	if m.confirmOpen {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			confirmModal(m),
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color(colBase)),
		)
	}
	return base
}

func shouldShowActivity(m Model) bool {
	return m.height >= 30
}

func renderDashboardBase(m Model) string {
	header := renderHeader(m)
	stats := renderStatsStrip(m)
	activity := ""
	if shouldShowActivity(m) {
		activity = renderActivityPanel(m)
	}
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

// renderLogo paints "dotcor" with a mauve → pink per-char gradient.
func renderLogo() string {
	text := "◆ dotcor"
	gradient := []string{colMauve, colMauve, colLavender, colPink, colPink, colFlamingo, colFlamingo, colPink}
	var b strings.Builder
	// Index by rune position, not byte offset: "◆" is a multi-byte rune, so
	// a byte-indexed range would skip gradient stops and misalign the ramp.
	for i, r := range []rune(text) {
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
		kbd(",", "settings"),
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
	if m.repoSizeCached > 0 {
		repoVal += metaSep() + humanSize(m.repoSizeCached)
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
	// Hard-floor on terminal width: under ~46 cols neither panel can
	// render anything meaningful. Show a placeholder rather than emit
	// negative widths that overflow the screen.
	const minTotal = 46
	if m.width < minTotal {
		return dimStyle.Render(fmt.Sprintf(" terminal too narrow (need ≥ %d cols, have %d)", minTotal, m.width))
	}

	leftWidth := m.width * 2 / 5
	if leftWidth < 36 {
		leftWidth = 36
	}
	rightWidth := m.width - leftWidth
	// Guarantee a usable right panel; if the left clamp leaves <10 cols
	// for the right, shrink the left.
	if rightWidth < 10 {
		rightWidth = 10
		leftWidth = m.width - rightWidth
		if leftWidth < 0 {
			leftWidth = 0
		}
	}

	left := renderPackagePanel(m, leftWidth, mainHeight)
	right := renderDetailPanel(m, rightWidth, mainHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func sortedPackages(m Model) []int {
	indices := make([]int, len(m.packages))
	for i := range indices {
		indices[i] = i
	}
	switch m.sortMode {
	case 0:
		sort.Slice(indices, func(a, b int) bool {
			return m.packages[indices[a]].Name < m.packages[indices[b]].Name
		})
	case 1:
		sort.Slice(indices, func(a, b int) bool {
			return m.packages[indices[a]].Status < m.packages[indices[b]].Status
		})
	case 2:
		sort.Slice(indices, func(a, b int) bool {
			aTime := m.packages[indices[a]].ModTime
			bTime := m.packages[indices[b]].ModTime
			return aTime.After(bTime)
		})
	}
	return indices
}

func renderPackagePanel(m Model, width, height int) string {
	sortLabels := []string{"alpha", "status", "recent"}
	title := fmt.Sprintf(" Packages %s %s", dimStyle.Render(fmt.Sprintf("(%d)", len(m.packages))), dimStyle.Render("↑"+sortLabels[m.sortMode]))
	body := renderPackageList(m, width-4, height-4)
	active := !m.expanded[m.selectedPkg] && !m.searching
	return panel(title, body, width, height, active)
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
	active := m.expanded[m.selectedPkg] && !m.searching
	return panel(title, body, width, height, active)
}

// ─── Package list as cards ───────────────────────────────────────────────────

func renderPackageList(m Model, width, maxLines int) string {
	if len(m.packages) == 0 {
		return renderEmptyPackages(width)
	}

	if m.searching {
		var filtered []int
		for i, pkg := range m.packages {
			if fuzzyMatch(m.searchQuery, pkg.Name) {
				filtered = append(filtered, i)
			}
		}

		var b strings.Builder
		b.WriteString(renderSearchInput(m, width))
		b.WriteString("\n")

		if len(filtered) == 0 {
			b.WriteString(dimStyle.Render("  no matches"))
			return b.String()
		}

		searchLines := 3
		cardHeight := 2
		availableLines := maxLines - searchLines - 1
		if availableLines < 2 {
			availableLines = 2
		}
		maxCards := availableLines / cardHeight
		if maxCards < 1 {
			maxCards = 1
		}

		selInFiltered := 0
		for fi, oi := range filtered {
			if oi == m.selectedPkg {
				selInFiltered = fi
				break
			}
		}

		start, end := visibleRange(selInFiltered, len(filtered), maxCards)
		for fi := start; fi < end; fi++ {
			b.WriteString(renderPackageCard(m, filtered[fi], width))
			if fi < end-1 {
				b.WriteString("\n")
			}
		}

		return b.String()
	}

	cardHeight := 2
	maxCards := maxLines / cardHeight
	if maxCards < 1 {
		maxCards = 1
	}

	indices := sortedPackages(m)

	selPos := 0
	for i, idx := range indices {
		if idx == m.selectedPkg {
			selPos = i
			break
		}
	}

	start, end := visibleRange(selPos, len(indices), maxCards)

	var parts []string
	for i := start; i < end; i++ {
		parts = append(parts, renderPackageCard(m, indices[i], width))
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

	linked, conflicts, total := 0, 0, len(pkg.Files)
	for _, f := range pkg.Files {
		if f.IsLinked {
			linked++
		} else if f.Exists {
			conflicts++
		}
	}

	tag := categoryTag(pkg.Name)
	if conflicts > 0 {
		tag = pill(fmt.Sprintf("⚠%d", conflicts), colBase, colRed) + " " + tag
	}
	tagW := lipgloss.Width(tag)

	// Truncate long names so the card doesn't overflow the panel.
	// Reserve 3 cols of breathing room between name and tag.
	maxNameRunes := contentWidth - tagW - 3
	if maxNameRunes < 4 {
		maxNameRunes = 4
	}
	displayName := pkg.Name
	runes := []rune(displayName)
	if len(runes) > maxNameRunes {
		if maxNameRunes <= 1 {
			displayName = string(runes[:1])
		} else {
			displayName = string(runes[:maxNameRunes-1]) + "…"
		}
	}
	// Highlight the focused package's name with a filled background so the
	// selection is scannable at a glance, not just a bar-color change.
	nameStyle := textStyle.Bold(true)
	if selected {
		nameStyle = selectedRowStyle.Foreground(lipgloss.Color(colText))
	}
	name := nameStyle.Render(" " + displayName + " ")

	leftW := lipgloss.Width(name)
	gap := contentWidth - leftW - tagW
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
	line2 := indent + progress + metaSep() + modified

	return lipgloss.JoinVertical(lipgloss.Left, line1, line2)
}

func renderEmptyPackages(width int) string {
	return strings.Join([]string{
		"",
		dimStyle.Render("○") + " " + textStyle.Render("No packages yet."),
		"",
		dimStyle.Render("Press ") + kbd("a", "add") + dimStyle.Render(" to stow your first dotfile."),
	}, "\n")
}

func renderSearchInput(m Model, width int) string {
	prompt := accentStyle.Render("/")
	return fmt.Sprintf("%s %s\n\n%s",
		prompt,
		m.searchInput.View(),
		dimStyle.Render("esc to clear"),
	)
}

// ─── File detail ─────────────────────────────────────────────────────────────

func renderFileDetail(m Model, width, maxLines int) string {
	if m.selectedPkg >= len(m.packages) {
		return dimStyle.Render("No package selected")
	}

	pkg := m.packages[m.selectedPkg]

	if len(pkg.Files) == 0 {
		return strings.Join([]string{
			"",
			dimStyle.Render("○") + " " + textStyle.Render("This package is empty."),
			"",
			dimStyle.Render("Add files to ") + accentStyle.Render(pkg.Name) + dimStyle.Render(" in your repo, then ") + kbd("s", "stow") + dimStyle.Render("."),
		}, "\n")
	}

	var b strings.Builder

	linked, conflicts, foreign := 0, 0, 0
	for _, f := range pkg.Files {
		if f.IsLinked {
			linked++
		} else if f.Exists && f.IsSymlink {
			foreign++
		} else if f.Exists {
			conflicts++
		}
	}

	summary := []string{
		pill(fmt.Sprintf("%s %d", statusLinked, linked), colBase, colGreen),
	}
	if foreign > 0 {
		summary = append(summary, pill(fmt.Sprintf("%s %d", statusForeign, foreign), colBase, colYellow))
	}
	if conflicts > 0 {
		summary = append(summary, pill(fmt.Sprintf("%s %d", statusConflict, conflicts), colBase, colRed))
	}
	unlinked := len(pkg.Files) - linked - conflicts - foreign
	if unlinked > 0 {
		summary = append(summary, pill(fmt.Sprintf("%s %d", statusUnlinked, unlinked), colBase, colOverlay0))
	}
	b.WriteString(strings.Join(summary, " "))
	b.WriteString("\n")
	b.WriteString(hRule(width))
	b.WriteString("\n")

	// Each file entry renders as 2 lines (name + target path).
	// Reserve 3 lines for: summary pills, separator, and optional overflow notice.
	// Divide remaining lines by 2 to get the number of file entries that fit.
	fileSlots := (maxLines - 3) / 2
	if fileSlots < 1 {
		fileSlots = 1
	}
	start, end := visibleRange(m.selectedFile, len(pkg.Files), fileSlots)
	for i := start; i < end; i++ {
		f := pkg.Files[i]
		selected := i == m.selectedFile && m.expanded[m.selectedPkg]

		statusBadge := fileBadge(f)
		rel := f.RelPath
		target := collapseHome(f.TargetPath, m.homeDir)

		barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colSurface1))
		if selected {
			barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colMauve)).Bold(true)
		}
		bar := barStyle.Render("▌")

		fmt.Fprintf(&b, "%s %s %s\n", bar, statusBadge, textStyle.Render(rel))
		pad := strings.Repeat(" ", lipgloss.Width(statusBadge))
		fmt.Fprintf(&b, "%s %s%s %s\n", bar, pad, dimStyle.Render("→"), dimStyle.Render(target))
	}

	if len(pkg.Files) > fileSlots {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  … %d–%d of %d", start+1, end, len(pkg.Files))))
	}

	return b.String()
}

// ─── Activity strip ──────────────────────────────────────────────────────────

func renderActivityPanel(m Model) string {
	var body string
	if len(m.recentCommits) == 0 {
		body = dimStyle.Render(" No commits yet — press ") + kbd("S", "sync") + dimStyle.Render(" to commit changes")
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

	panelHeight := lipgloss.Height(body) + 4
	if panelHeight < 4 {
		panelHeight = 4
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Render(panel(" Recent activity", body, m.width-2, panelHeight, false))
}

// ─── Git + footer ────────────────────────────────────────────────────────────

func renderGitBar(m Model) string {
	if m.initStep == 1 {
		return lipgloss.NewStyle().
			Width(m.width).
			Foreground(lipgloss.Color(colYellow)).
			Bold(true).
			Render(fmt.Sprintf("  Initialize git in %s? %s %s %s",
				collapseHome(m.repoDir, m.homeDir),
				kbd("enter", "confirm"), dimStyle.Render("·"), kbd("esc", "cancel"),
			))
	}
	if m.initStep == 2 {
		return lipgloss.NewStyle().
			Width(m.width).
			Render("  " + accentStyle.Render("Remote URL") + " " + m.settingsInput.View() + dimStyle.Render(" (leave empty to skip)"))
	}

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
		// Detached HEAD (rebase, bisect, fresh repo with no commits, or
		// explicit checkout of a SHA) is rendered as a yellow pill so the
		// user can tell apart "on a branch" from "floating".
		if m.gitStatus.Detached {
			parts = append(parts, pill("⎇ detached: "+m.gitStatus.Branch, colBase, colYellow))
		} else {
			parts = append(parts, pill("⎇ "+m.gitStatus.Branch, colBase, colLavender))
		}
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
		// "no git repository" only when the directory genuinely isn't a
		// repo. A repo with detached HEAD or unusual state shouldn't fall
		// through to this branch — IsRepo confirms it.
		if git.IsRepo(m.repoDir) {
			parts = append(parts, dimStyle.Render("git: initializing"))
		} else {
			parts = append(parts, dimStyle.Render("no git repository"))
		}
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

	// Show only the primary actions here; the full keymap lives behind ?.
	// A permanent 14-key wall reads as noise and costs vertical rows.
	allHints := []string{
		kbd("↑↓", "nav"),
		kbd("s", "stow"),
		kbd("a", "add"),
		kbd("S", "sync"),
		kbd("/", "search"),
		kbd("?", "more"),
		kbd("q", "quit"),
	}

	sep := dimStyle.Render("  ·  ")
	sepW := lipgloss.Width(sep)

	// Greedily pack hints into rows that fit within m.width.
	var rows []string
	rowHints := []string{}
	rowW := 0
	for _, h := range allHints {
		hw := lipgloss.Width(h)
		needed := hw
		if len(rowHints) > 0 {
			needed += sepW
		}
		if len(rowHints) > 0 && rowW+needed > m.width {
			rows = append(rows, joinHints(rowHints...))
			rowHints = []string{h}
			rowW = hw
		} else {
			rowHints = append(rowHints, h)
			rowW += needed
		}
	}
	if len(rowHints) > 0 {
		rows = append(rows, joinHints(rowHints...))
	}

	// Center each row within m.width.
	centerRow := func(row string) string {
		w := lipgloss.Width(row)
		gap := (m.width - w) / 2
		if gap < 0 {
			gap = 0
		}
		return lipgloss.NewStyle().Width(m.width).Render(strings.Repeat(" ", gap) + row)
	}

	centeredRows := make([]string, len(rows))
	for i, row := range rows {
		centeredRows[i] = centerRow(row)
	}

	return strings.Join(centeredRows, "\n")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// Canonical status vocabulary — one lexicon the whole UI shares so a user
// learns the four states once. Detail pills, file badges, and card summaries
// all draw from these labels and colors.
const (
	statusLinked   = "linked"
	statusForeign  = "foreign"
	statusConflict = "conflict"
	statusUnlinked = "unlinked"
)

// fileStatus reduces a file entry to its canonical status label + color.
func fileStatus(f stow.FileEntry) (label, color string) {
	switch {
	case f.IsLinked:
		return statusLinked, colGreen
	case f.Exists && f.IsSymlink:
		return statusForeign, colYellow
	case f.Exists:
		return statusConflict, colRed
	default:
		return statusUnlinked, colOverlay0
	}
}

func fileBadge(f stow.FileEntry) string {
	label, color := fileStatus(f)
	// "unlinked" is the absence of state — render it recessive (dim text,
	// no fill) so present/conflicting files carry the visual weight.
	if label == statusUnlinked {
		return dimStyle.Render(label)
	}
	return pill(label, colBase, color)
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
	// Render as a subtle pill (category color on a recessed surface) so it
	// reads as a deliberate tag alongside the filled conflict pill, rather
	// than as bare floating text.
	return pill(tag, color, colSurface0)
}

func fuzzyMatch(query, target string) bool {
	if query == "" {
		return true
	}
	// Iterate both sides as runes so multi-byte queries (Cyrillic, CJK,
	// emoji-tagged names) match correctly. The original byte-wise compare
	// chopped any rune > 0x7F into its first byte and never matched.
	qRunes := []rune(strings.ToLower(query))
	qi := 0
	for _, c := range strings.ToLower(target) {
		if qi < len(qRunes) && c == qRunes[qi] {
			qi++
		}
	}
	return qi == len(qRunes)
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

// repoSizeBytes returns the total size of the repo dir in bytes (rough
// estimate from top-level, excluding .git/logs/backups).
func repoSizeBytes(repoDir string) int64 {
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
	return total
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
	if home != "" && (path == home || strings.HasPrefix(path, home+string(filepath.Separator))) {
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
