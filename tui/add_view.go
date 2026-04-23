package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/justincordova/dotcor/internal/stow"
)

// addStep constants — keep magic numbers in one place.
const (
	addStepSelect  = 0
	addStepPreview = 1
	addStepConfirm = 2
)

type browserItem struct {
	path   string
	name   string
	isDir  bool
	indent int
}

func viewAdd(m Model) string {
	errLine := ""
	if m.err != nil {
		errLine = "\n" + errorStyle.Render(fmt.Sprintf("  ✗ %v", m.err)) + "\n"
	}

	cw := contentWidth(m.width)
	innerW := cw - 4
	var footer string

	if m.addStep == addStepSelect {
		body := renderAddStep0(m) + errLine
		footerHints := []string{
			kbd("↑/k", "up"), kbd("↓/j", "down"),
			kbd("space", "select"), kbd("enter", "expand/confirm"),
			kbd("h", "collapse"), kbd("/", "jump to path"),
			kbd("esc", "cancel"),
		}
		if sc := selectionCount(m.browserSelected); sc != "" {
			footerHints = append(footerHints, sc)
		}
		footer = plainFooter(innerW, footerHints...)
		content := lipgloss.JoinVertical(lipgloss.Left,
			renderAddStepper(innerW, m.addStep),
			lipgloss.NewStyle().Padding(1, 0).Render(body),
			footer,
		)
		dialog := boxStyle.Width(cw - 2).Render(content)
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			dialog,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color(colBase)),
		)
	}

	var body string
	switch m.addStep {
	case addStepPreview:
		body = renderPreviewStep(m) + errLine
		footer = plainFooter(innerW,
			kbd("↑/k", "up"), kbd("↓/j", "down"),
			kbd("space", "toggle"), kbd("enter", "confirm"),
			kbd("esc", "back"),
		)
	case addStepConfirm:
		body = renderConfirmStep(m) + errLine
		footer = plainFooter(innerW,
			kbd("enter", "execute"), kbd("esc", "back"),
		)
	default:
		body = dimStyle.Render("  (internal error — press esc)")
		footer = plainFooter(innerW, kbd("esc", "back"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		renderAddStepper(innerW, m.addStep),
		lipgloss.NewStyle().Padding(1, 0).Render(body),
		footer,
	)
	dialog := boxStyle.Width(cw - 2).Render(content)
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color(colBase)),
	)
}

func renderAddStepper(width, step int) string {
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colMauve)).
		Bold(true).
		Render("◆ Add / Adopt")

	steps := []string{"Select", "Preview", "Confirm"}
	var parts []string
	for i, s := range steps {
		num := fmt.Sprintf("%d", i+1)
		label := fmt.Sprintf(" %s %s ", num, s)
		switch {
		case i < step:
			parts = append(parts, pill(label, colBase, colGreen))
		case i == step:
			parts = append(parts, pill(label, colBase, colMauve))
		default:
			parts = append(parts, lipgloss.NewStyle().
				Foreground(lipgloss.Color(colOverlay0)).
				Padding(0, 1).
				Render(label))
		}
	}
	stepRow := lipgloss.JoinHorizontal(lipgloss.Center, parts...)

	rightW := lipgloss.Width(stepRow)
	gap := width - lipgloss.Width(title) - rightW - 4
	if gap < 2 {
		gap = 2
	}

	row := title + strings.Repeat(" ", gap) + stepRow
	return lipgloss.NewStyle().Width(width).Padding(0, 2).Render(row)
}

// ─── Step 0: File browser ─────────────────────────────────────────────────────

func renderAddStep0(m Model) string {
	var b strings.Builder

	displayPath := collapseHome(m.homeDir, m.homeDir)
	if displayPath == "" {
		displayPath = m.homeDir
	}
	b.WriteString(accentStyle.Render("  " + displayPath))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(strings.Repeat("─", max(bodyWidth(m.width), 4))))
	b.WriteString("\n")

	if m.browserJumping {
		b.WriteString("\n")
		b.WriteString("  " + accentStyle.Render("/") + " " + m.browserJumpInput.View())
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(strings.Repeat("─", max(bodyWidth(m.width), 4))))
		b.WriteString("\n")
	}

	items := m.buildBrowserItems()

	if len(items) == 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  (empty directory)"))
		return b.String()
	}

	contentHeight := m.height - 6
	if contentHeight < 1 {
		contentHeight = 1
	}

	start := m.browserScroll
	end := start + contentHeight
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		item := items[i]
		indent := strings.Repeat("  ", item.indent)
		name := item.name

		var icon string
		var styledName string
		if item.isDir {
			expanded := m.browserExpanded[item.path]
			if expanded {
				icon = "▾"
			} else if m.browserSelected[item.path] {
				icon = "●"
			} else {
				icon = "▸"
			}
			if m.browserSelected[item.path] {
				styledName = successStyle.Render(name + "/")
			} else {
				styledName = accentStyle.Render(name + "/")
			}
			if m.browserSelected[item.path] && !expanded {
				count := countFilesRecursive(item.path)
				styledName += dimStyle.Render(fmt.Sprintf(" (%d files)", count))
			}
		} else if isSymlink(item.path) {
			if m.browserSelected[item.path] {
				icon = "●"
			} else {
				icon = "◆"
			}
			target, _ := os.Readlink(item.path)
			display := truncate(filepath.Base(target), 20)
			styledName = warningStyle.Render(name + " → " + display + " ⚠ foreign")
		} else {
			if m.browserSelected[item.path] {
				icon = "●"
			} else {
				icon = "○"
			}
			styledName = textStyle.Render(name)
		}

		if m.browserSelected[item.path] {
			icon = successStyle.Render(icon)
		}

		cursor := " "
		if i == m.browserCursor {
			cursor = accentStyle.Render("▌")
		}
		line := fmt.Sprintf(" %s%s%s %s", indent, cursor, icon, styledName)
		bw := bodyWidth(m.width)
		line = lipgloss.NewStyle().Width(bw).Render(line)
		b.WriteString(line)
		b.WriteString("\n")
	}

	if end < len(items) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  … %d more", len(items)-end)))
		b.WriteString("\n")
	}

	return b.String()
}

// ─── Step 1: Preview ──────────────────────────────────────────────────────────

// previewRow is a single entry in the flat row list used for preview rendering
// and cursor navigation.
type previewRow struct {
	pkgName     string
	relPath     string
	class       stow.Class
	isHeader    bool
	headerLabel string
}

// buildPreviewRows returns the flat row list from the plan, for cursor navigation.
func buildPreviewRows(plan *stow.ClassificationPlan) []previewRow {
	if plan == nil {
		return nil
	}

	classOrder := []stow.Class{stow.ClassAdopt, stow.ClassAdd, stow.ClassTrack, stow.ClassForeign, stow.ClassManaged}

	var rows []previewRow
	for _, pkg := range plan.Packages {
		// Group files by class within this package.
		byClass := make(map[stow.Class][]stow.ClassifiedFile)
		for _, cf := range pkg.Files {
			byClass[cf.Class] = append(byClass[cf.Class], cf)
		}

		// Package header row (non-navigable, used for display only).
		rows = append(rows, previewRow{
			pkgName:     pkg.Name,
			isHeader:    true,
			headerLabel: "pkg:" + pkg.Name,
		})

		for _, class := range classOrder {
			files, ok := byClass[class]
			if !ok || len(files) == 0 {
				continue
			}
			// Section header row (non-navigable).
			rows = append(rows, previewRow{
				pkgName:     pkg.Name,
				class:       class,
				isHeader:    true,
				headerLabel: class.String(),
			})
			for _, cf := range files {
				rows = append(rows, previewRow{
					pkgName: pkg.Name,
					relPath: cf.RelPath,
					class:   class,
				})
			}
		}
	}
	return rows
}

// firstFileRow returns the index of the first non-header row at or after start,
// searching forward. Returns start if no file row is found.
func firstFileRow(rows []previewRow, start int) int {
	for i := start; i < len(rows); i++ {
		if !rows[i].isHeader {
			return i
		}
	}
	return start
}

// lastFileRow returns the index of the last non-header row at or before start,
// searching backward. Returns start if no file row is found.
func lastFileRow(rows []previewRow, start int) int {
	for i := start; i >= 0; i-- {
		if !rows[i].isHeader {
			return i
		}
	}
	return start
}

func renderPreviewStep(m Model) string {
	if m.previewPlan == nil {
		return dimStyle.Render("  Classifying files…")
	}

	var b strings.Builder

	rows := m.previewRows
	bw := bodyWidth(m.width)

	contentHeight := m.height - 10
	if contentHeight < 4 {
		contentHeight = 4
	}

	start := m.previewScroll
	end := start + contentHeight
	if end > len(rows) {
		end = len(rows)
	}

	for i := start; i < end; i++ {
		row := rows[i]
		if row.isHeader {
			b.WriteString(renderPreviewHeader(row, bw))
			b.WriteString("\n")
			continue
		}

		// File row.
		cf := findClassifiedFile(m.previewPlan, row.pkgName, row.relPath)
		if cf == nil {
			continue
		}
		id := stow.FileID(row.pkgName, row.relPath)
		toggled := m.previewToggles[id]
		selected := i == m.previewCursor

		line := renderPreviewFileRow(*cf, toggled, row.pkgName, selected, bw)
		b.WriteString(line)
		b.WriteString("\n")
	}

	if end < len(rows) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  … %d more", len(rows)-end)))
		b.WriteString("\n")
	}

	// Summary counts.
	b.WriteString("\n")
	b.WriteString(renderPreviewCounts(m.previewPlan, m.previewToggles))

	return b.String()
}

func renderPreviewHeader(row previewRow, bw int) string {
	if strings.HasPrefix(row.headerLabel, "pkg:") {
		pkgName := strings.TrimPrefix(row.headerLabel, "pkg:")
		return accentStyle.Render("  Package: "+pkgName) + "\n" +
			subtleStyle.Render("  "+strings.Repeat("─", max(bw-2, 4)))
	}

	// Class section header.
	class := row.class
	var label string
	var color string
	switch class {
	case stow.ClassAdopt:
		label, color = "ADOPT", colGreen
	case stow.ClassAdd:
		label, color = "ADD", colBlue
	case stow.ClassTrack:
		label, color = "TRACK", colMauve
	case stow.ClassForeign:
		label, color = "FOREIGN", colYellow
	case stow.ClassManaged:
		label, color = "MANAGED", colOverlay0
	default:
		label, color = "UNKNOWN", colOverlay0
	}
	return "  " + pill(" "+label+" ", colBase, color)
}

func renderPreviewFileRow(cf stow.ClassifiedFile, toggled bool, pkgName string, selected bool, bw int) string {
	isManaged := cf.Class == stow.ClassManaged

	var checkbox string
	if isManaged {
		checkbox = "    " // no checkbox for managed
	} else if toggled {
		checkbox = successStyle.Render("[x] ")
	} else {
		checkbox = dimStyle.Render("[ ] ")
	}

	name := cf.RelPath
	var detail string
	switch cf.Class {
	case stow.ClassAdopt:
		detail = dimStyle.Render(fmt.Sprintf("~/%s → repo/%s/%s", filepath.Base(cf.HomeSymlink), pkgName, cf.RelPath))
	case stow.ClassAdd:
		detail = dimStyle.Render(fmt.Sprintf("→ repo/%s/%s", pkgName, cf.RelPath))
	case stow.ClassTrack:
		detail = dimStyle.Render(fmt.Sprintf("repo/%s/%s (no $HOME link)", pkgName, cf.RelPath))
	case stow.ClassForeign:
		target := truncate(cf.ForeignTarget, 30)
		detail = warningStyle.Render(fmt.Sprintf("→ %s (toggle to adopt)", target))
	case stow.ClassManaged:
		detail = dimStyle.Render(fmt.Sprintf("already in repo/%s/", pkgName))
	}

	var styledName string
	if isManaged {
		styledName = dimStyle.Render(name)
	} else if toggled {
		styledName = textStyle.Render(name)
	} else {
		styledName = dimStyle.Render(name)
	}

	prefix := "  "
	if selected && !isManaged {
		prefix = accentStyle.Render("▌ ")
	}

	nameW := lipgloss.Width(styledName)
	namePad := ""
	if nameW < 24 {
		namePad = strings.Repeat(" ", 24-nameW)
	}
	line := fmt.Sprintf("%s%s%s%s  %s", prefix, checkbox, styledName, namePad, detail)

	return lipgloss.NewStyle().Width(bw).Render(line)
}

func findClassifiedFile(plan *stow.ClassificationPlan, pkgName, relPath string) *stow.ClassifiedFile {
	for i := range plan.Packages {
		if plan.Packages[i].Name != pkgName {
			continue
		}
		for j := range plan.Packages[i].Files {
			if plan.Packages[i].Files[j].RelPath == relPath {
				return &plan.Packages[i].Files[j]
			}
		}
	}
	return nil
}

func renderPreviewCounts(plan *stow.ClassificationPlan, toggles map[string]bool) string {
	counts := make(map[stow.Class]int)
	activeCount := 0
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			counts[cf.Class]++
			id := stow.FileID(pkg.Name, cf.RelPath)
			if toggles[id] {
				activeCount++
			}
		}
	}

	var parts []string
	if n := counts[stow.ClassAdopt]; n > 0 {
		parts = append(parts, pill(fmt.Sprintf(" adopt %d ", n), colBase, colGreen))
	}
	if n := counts[stow.ClassAdd]; n > 0 {
		parts = append(parts, pill(fmt.Sprintf(" add %d ", n), colBase, colBlue))
	}
	if n := counts[stow.ClassTrack]; n > 0 {
		parts = append(parts, pill(fmt.Sprintf(" track %d ", n), colBase, colMauve))
	}
	if n := counts[stow.ClassForeign]; n > 0 {
		parts = append(parts, pill(fmt.Sprintf(" foreign %d ", n), colBase, colYellow))
	}
	if n := counts[stow.ClassManaged]; n > 0 {
		parts = append(parts, pill(fmt.Sprintf(" managed %d ", n), colBase, colOverlay0))
	}
	if n := len(plan.Filtered); n > 0 {
		parts = append(parts, pill(fmt.Sprintf(" filtered %d ", n), colBase, colRed))
	}

	active := dimStyle.Render(fmt.Sprintf("  %d active", activeCount))
	if len(parts) == 0 {
		return "  " + active
	}
	return "  " + strings.Join(parts, " ") + "  " + active
}

// ─── Step 2: Confirm ───────────────────────────────────────────────────────────

func renderConfirmStep(m Model) string {
	if m.previewPlan == nil {
		return dimStyle.Render("  No plan to confirm.")
	}

	plan := m.previewPlan
	toggles := m.previewToggles
	bw := bodyWidth(m.width)

	var b strings.Builder

	b.WriteString(accentStyle.Render("  Confirm changes"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("  " + strings.Repeat("─", max(bw-2, 4))))
	b.WriteString("\n")

	classOrder := []stow.Class{stow.ClassAdopt, stow.ClassAdd, stow.ClassTrack, stow.ClassForeign}
	for _, pkg := range plan.Packages {
		byClass := make(map[stow.Class][]stow.ClassifiedFile)
		for _, cf := range pkg.Files {
			byClass[cf.Class] = append(byClass[cf.Class], cf)
		}

		var hasActive bool
		for _, class := range classOrder {
			for _, cf := range byClass[class] {
				if toggles[stow.FileID(pkg.Name, cf.RelPath)] {
					hasActive = true
					break
				}
			}
			if hasActive {
				break
			}
		}
		if !hasActive {
			continue
		}

		b.WriteString("\n")
		b.WriteString(accentStyle.Render("  Package: " + pkg.Name))
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render("  " + strings.Repeat("─", max(bw-2, 4))))
		b.WriteString("\n")

		for _, class := range classOrder {
			files := byClass[class]
			if len(files) == 0 {
				continue
			}
			var active []stow.ClassifiedFile
			for _, cf := range files {
				if toggles[stow.FileID(pkg.Name, cf.RelPath)] {
					active = append(active, cf)
				}
			}
			if len(active) == 0 {
				continue
			}

			var label, color string
			switch class {
			case stow.ClassAdopt:
				label, color = "ADOPT", colGreen
			case stow.ClassAdd:
				label, color = "ADD", colBlue
			case stow.ClassTrack:
				label, color = "TRACK", colMauve
			case stow.ClassForeign:
				label, color = "FOREIGN", colYellow
			}

			b.WriteString("  " + pill(fmt.Sprintf(" %s %d ", label, len(active)), colBase, color))
			b.WriteString("\n")
			for _, cf := range active {
				var detail string
				switch class {
				case stow.ClassAdopt:
					detail = fmt.Sprintf("← %s → repo/%s/%s", cf.HomeSymlink, pkg.Name, cf.RelPath)
				case stow.ClassAdd:
					detail = fmt.Sprintf("→ repo/%s/%s", pkg.Name, cf.RelPath)
				case stow.ClassTrack:
					detail = fmt.Sprintf("repo/%s/%s (no $HOME link)", pkg.Name, cf.RelPath)
				case stow.ClassForeign:
					detail = fmt.Sprintf("→ %s → repo/%s/%s", truncate(cf.ForeignTarget, 30), pkg.Name, cf.RelPath)
				}
				line := fmt.Sprintf("      %-28s  %s", cf.RelPath, detail)
				if lipgloss.Width(line) > bw {
					line = truncate(line, bw-2)
				}
				b.WriteString(dimStyle.Render(line))
				b.WriteString("\n")
			}
		}
	}

	var managedCount int
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			if cf.Class == stow.ClassManaged {
				managedCount++
			}
		}
	}
	if managedCount > 0 {
		b.WriteString("\n")
		b.WriteString("  " + pill(fmt.Sprintf(" MANAGED %d (skipped) ", managedCount), colBase, colOverlay0))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  enter to execute · esc to go back"))

	return b.String()
}

// ─── Browser helpers (shared with step 0) ────────────────────────────────────

var browserSkipDirs = map[string]bool{
	".git": true, "node_modules": true, ".cache": true, "__pycache__": true,
	"Library": true, ".Trash": true, ".dotcor": true,
}

func loadBrowserDir(path string) []os.DirEntry {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var filtered []os.DirEntry
	for _, e := range entries {
		if e.IsDir() && browserSkipDirs[e.Name()] {
			continue
		}
		filtered = append(filtered, e)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].IsDir() != filtered[j].IsDir() {
			return filtered[i].IsDir()
		}
		return strings.ToLower(filtered[i].Name()) < strings.ToLower(filtered[j].Name())
	})
	return filtered
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

func (m *Model) buildBrowserItems() []browserItem {
	if m.browserItems != nil {
		return m.browserItems
	}
	var items []browserItem
	m.walkBrowserDir(m.homeDir, 0, &items)
	m.browserItems = items
	return items
}

func (m *Model) walkBrowserDir(dir string, indent int, items *[]browserItem) {
	entries, ok := m.browserEntries[dir]
	if !ok {
		entries = loadBrowserDir(dir)
		m.browserEntries[dir] = entries
	}

	for _, e := range entries {
		fullPath := filepath.Join(dir, e.Name())

		if e.IsDir() {
			if allFilesManaged(fullPath, m.repoDir) {
				continue
			}
		} else if isSymlink(fullPath) {
			if managed, _ := managedSymlink(fullPath, m.repoDir); managed {
				continue
			}
		}

		*items = append(*items, browserItem{
			path:   fullPath,
			name:   e.Name(),
			isDir:  e.IsDir(),
			indent: indent,
		})
		if e.IsDir() && m.browserExpanded[fullPath] {
			m.walkBrowserDir(fullPath, indent+1, items)
		}
	}
}

func (m *Model) browserAdjustScroll() {
	contentHeight := m.height - 6
	if contentHeight < 1 {
		contentHeight = 1
	}

	if m.browserCursor < m.browserScroll {
		m.browserScroll = m.browserCursor
	}
	if m.browserCursor >= m.browserScroll+contentHeight {
		m.browserScroll = m.browserCursor - contentHeight + 1
	}
}

func (m *Model) previewAdjustScroll() {
	contentHeight := m.height - 10
	if contentHeight < 4 {
		contentHeight = 4
	}

	if m.previewCursor < m.previewScroll {
		m.previewScroll = m.previewCursor
	}
	if m.previewCursor >= m.previewScroll+contentHeight {
		m.previewScroll = m.previewCursor - contentHeight + 1
	}
	if m.previewScroll < 0 {
		m.previewScroll = 0
	}
}

// ─── Update handlers ──────────────────────────────────────────────────────────

func (m Model) updateAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Esc):
			if m.addStep == addStepSelect {
				m.activeView = DashboardView
				m.resetAddState()
				m.err = nil
				return m, nil
			}
			m.addStep--
			m.err = nil
			return m, nil

		case key.Matches(keyMsg, m.keys.Enter):
			switch m.addStep {
			case addStepSelect:
				if len(m.browserSelected) > 0 {
					return m.confirmBrowserSelectionAndClassify()
				}
				items := m.buildBrowserItems()
				if m.browserCursor < 0 || m.browserCursor >= len(items) {
					return m, nil
				}
				item := items[m.browserCursor]
				if item.isDir {
					m.browserExpanded[item.path] = !m.browserExpanded[item.path]
					m.browserItems = nil
					return m, nil
				}
				return m.browserSelectAndClassify(item.path)

			case addStepPreview:
				if m.previewPlan == nil {
					return m, nil
				}
				m.addStep = addStepConfirm
				m.err = nil
				return m, nil

			case addStepConfirm:
				if m.previewPlan == nil {
					return m, nil
				}
				return m, runClassification(m.previewPlan, stow.CopyToggles(m.previewToggles), m.repoDir, m.homeDir)
			}

		case keyMsg.String() == " " && m.addStep == addStepPreview:
			// Toggle row in preview.
			rows := m.previewRows
			if m.previewCursor < 0 || m.previewCursor >= len(rows) {
				return m, nil
			}
			row := rows[m.previewCursor]
			if row.isHeader {
				return m, nil
			}
			if row.class == stow.ClassManaged {
				return m, nil // no-op for managed
			}
			id := stow.FileID(row.pkgName, row.relPath)
			m.previewToggles[id] = !m.previewToggles[id]
			return m, nil
		}

		if m.addStep == addStepSelect {
			return m.browserHandleKey(keyMsg)
		}

		if m.addStep == addStepPreview {
			return m.previewHandleKey(keyMsg)
		}
	}

	return m, nil
}

func (m Model) previewHandleKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.previewPlan == nil {
		return m, nil
	}
	rows := m.previewRows

	switch keyMsg.String() {
	case "up", "k":
		if m.previewCursor > 0 {
			next := lastFileRow(rows, m.previewCursor-1)
			if !rows[next].isHeader {
				m.previewCursor = next
			}
		}
		m.previewAdjustScroll()
		return m, nil

	case "down", "j":
		if m.previewCursor < len(rows)-1 {
			next := firstFileRow(rows, m.previewCursor+1)
			if !rows[next].isHeader {
				m.previewCursor = next
			}
		}
		m.previewAdjustScroll()
		return m, nil
	}
	return m, nil
}

func (m Model) browserHandleKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.browserJumping {
		return m.browserHandleJumpKey(keyMsg)
	}

	items := m.buildBrowserItems()
	switch keyMsg.String() {
	case "up", "k":
		if m.browserCursor > 0 {
			m.browserCursor--
		}
		m.browserAdjustScroll()
		return m, nil

	case "down", "j":
		if m.browserCursor < len(items)-1 {
			m.browserCursor++
		}
		m.browserAdjustScroll()
		return m, nil

	case " ":
		if m.browserCursor < 0 || m.browserCursor >= len(items) {
			return m, nil
		}
		item := items[m.browserCursor]
		if item.isDir {
			m.toggleDirSelection(item.path)
		} else {
			// Use delete-on-clear for consistency with toggleDirSelection.
			if m.browserSelected[item.path] {
				delete(m.browserSelected, item.path)
			} else {
				m.browserSelected[item.path] = true
			}
		}
		return m, nil

	case "h":
		if m.browserCursor >= 0 && m.browserCursor < len(items) {
			item := items[m.browserCursor]
			if item.isDir && m.browserExpanded[item.path] {
				// Collapse the item itself.
				m.browserExpanded[item.path] = false
				delete(m.browserEntries, item.path)
				m.browserItems = nil
				return m, nil
			}
			// Find the nearest (deepest) expanded ancestor.
			nearest := m.nearestExpandedAncestor(item.path)
			if nearest != "" {
				m.browserExpanded[nearest] = false
				delete(m.browserEntries, nearest)
				m.browserItems = nil
				return m, nil
			}
		}
		return m, nil

	case "/":
		m.browserJumping = true
		m.browserJumpInput.Placeholder = "~/.config/nvim"
		m.browserJumpInput.SetValue("")
		m.browserJumpInput.Focus()
		return m, textinput.Blink
	}

	return m, nil
}

// nearestExpandedAncestor returns the deepest expanded ancestor directory of
// path, or "" if none found. This avoids non-deterministic map iteration.
func (m *Model) nearestExpandedAncestor(path string) string {
	best := ""
	for dir, expanded := range m.browserExpanded {
		if !expanded {
			continue
		}
		if strings.HasPrefix(path, dir+string(filepath.Separator)) {
			// dir is an ancestor — prefer the deepest one.
			if len(dir) > len(best) {
				best = dir
			}
		}
	}
	return best
}

func (m Model) browserHandleJumpKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "enter":
		raw := m.browserJumpInput.Value()
		m.browserJumping = false
		m.browserJumpInput.Blur()

		targetPath := expandBrowserPath(raw, m.homeDir)
		info, err := os.Stat(targetPath)
		if err != nil || !info.IsDir() {
			m.err = fmt.Errorf("directory not found: %s", raw)
			return m, nil
		}

		m.browserExpanded = make(map[string]bool)
		rel, relErr := filepath.Rel(m.homeDir, targetPath)
		if relErr == nil && !strings.HasPrefix(rel, "..") {
			parts := strings.Split(rel, string(filepath.Separator))
			current := m.homeDir
			for _, p := range parts {
				current = filepath.Join(current, p)
				m.browserExpanded[current] = true
			}
		}
		m.browserItems = nil
		m.browserEntries = make(map[string][]os.DirEntry)
		m.err = nil
		return m, nil

	case "esc":
		m.browserJumping = false
		m.browserJumpInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.browserJumpInput, cmd = m.browserJumpInput.Update(keyMsg)
	return m, cmd
}

func expandBrowserPath(raw, homeDir string) string {
	if strings.HasPrefix(raw, "~/") {
		return filepath.Join(homeDir, raw[2:])
	}
	if !filepath.IsAbs(raw) {
		return filepath.Join(homeDir, raw)
	}
	return raw
}

// confirmBrowserSelectionAndClassify gathers selected paths and kicks off
// ClassifyFiles as a Bubble Tea command.
func (m Model) confirmBrowserSelectionAndClassify() (tea.Model, tea.Cmd) {
	var selections []string
	for path, selected := range m.browserSelected {
		if selected {
			selections = append(selections, path)
		}
	}
	sort.Strings(selections)

	if len(selections) == 0 {
		return m, nil
	}

	m.err = nil
	return m, classifySelections(selections, m.repoDir, m.homeDir, m.cfg.IgnorePatterns)
}

func (m Model) browserSelectAndClassify(fullPath string) (tea.Model, tea.Cmd) {
	if isSymlink(fullPath) {
		if managed, _ := managedSymlink(fullPath, m.repoDir); managed {
			m.err = fmt.Errorf("already managed by dotcor — use the dashboard to manage this file")
			return m, nil
		}
	}

	if _, err := os.Stat(fullPath); err != nil {
		m.err = fmt.Errorf("file not found: %s", fullPath)
		return m, nil
	}

	m.err = nil
	return m, classifySelections([]string{fullPath}, m.repoDir, m.homeDir, m.cfg.IgnorePatterns)
}

func (m *Model) toggleDirSelection(dirPath string) {
	if m.browserSelected[dirPath] {
		delete(m.browserSelected, dirPath)
		return
	}
	m.browserSelected[dirPath] = true
}

// ─── File system helpers ──────────────────────────────────────────────────────

// managedSymlink returns true if path is a symlink pointing into repoDir.
func managedSymlink(path, repoDir string) (bool, error) {
	lfi, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if lfi.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	target, err := os.Readlink(path)
	if err != nil {
		return false, err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return false, err
	}
	// Resolve repoDir too.
	resolvedRepo := repoDir
	if r, rerr := filepath.EvalSymlinks(repoDir); rerr == nil {
		resolvedRepo = r
	}
	return strings.HasPrefix(resolved, resolvedRepo+string(filepath.Separator)) || resolved == resolvedRepo, nil
}

// allFilesManaged returns true only when every regular file under dir is a
// symlink that points into repoDir. A single unmanaged or non-symlink file
// makes it return false. An empty directory returns false.
func allFilesManaged(dir, repoDir string) bool {
	total := 0
	managed := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total++
		if isSymlink(path) {
			if ok, _ := managedSymlink(path, repoDir); ok {
				managed++
			}
		}
		return nil
	})
	return total > 0 && managed == total
}

func countFilesRecursive(dir string) int {
	count := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || isSymlink(path) {
			return nil
		}
		count++
		return nil
	})
	return count
}

func selectionCount(selected map[string]bool) string {
	n := 0
	for _, v := range selected {
		if v {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return dimStyle.Render(fmt.Sprintf("%d selected", n))
}

// ─── Tea commands ─────────────────────────────────────────────────────────────

func classifySelections(selections []string, repoDir, homeDir string, ignorePatterns []string) tea.Cmd {
	return func() tea.Msg {
		plan, err := stow.ClassifyFiles(selections, repoDir, homeDir, ignorePatterns)
		return classifyPlanMsg{plan: plan, err: err}
	}
}

func runClassification(plan *stow.ClassificationPlan, toggles map[string]bool, repoDir, homeDir string) tea.Cmd {
	return func() tea.Msg {
		result, err := stow.ExecuteClassification(plan, toggles, repoDir, homeDir)
		return classifyResultMsg{result: result, err: err}
	}
}
