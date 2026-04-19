package tui

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
)

type addResultMsg struct {
	msg string
	err error
}

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

	if m.addStep == 0 {
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
			renderStepper(innerW, m.addStep),
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
	case 1:
		body = renderAddStep1(m) + errLine
	case 2:
		body = renderAddStep2(m) + errLine
	case 3:
		body = renderAddStep3(m)
	}

	footer = plainFooter(innerW, addFooterHints(m)...)

	content := lipgloss.JoinVertical(lipgloss.Left,
		renderStepper(innerW, m.addStep),
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

func renderStepper(width, step int) string {
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colMauve)).
		Bold(true).
		Render("◆ Add File")

	steps := []string{"Select", "Package", "Review"}
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
			selected := m.browserSelected[item.path]
			expanded := m.browserExpanded[item.path]
			if expanded {
				icon = "▾"
			} else if selected {
				icon = "●"
			} else {
				icon = "▸"
			}
			if selected {
				styledName = successStyle.Render(name + "/")
			} else {
				styledName = accentStyle.Render(name + "/")
			}
			if selected && !expanded {
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
			managed, _ := fs.SymlinkPointsToRepo(item.path, m.repoDir)
			if managed {
				styledName = successStyle.Render(name + " → dotcor")
			} else {
				styledName = warningStyle.Render(name + " → " + display + " ⚠ foreign")
			}
		} else {
			if m.browserSelected[item.path] {
				icon = "●"
			} else {
				icon = "○"
			}
			styledName = textStyle.Render(name)
		}

		if m.browserSelected[item.path] || (item.isDir && m.browserSelected[item.path]) {
			icon = successStyle.Render(icon)
		}

		line := fmt.Sprintf("  %s%s %s", indent, icon, styledName)
		bw := bodyWidth(m.width)
		if i == m.browserCursor {
			line = selectedRowStyle.Width(bw).Render(line)
		} else {
			line = lipgloss.NewStyle().Width(bw).Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if end < len(items) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  … %d more", len(items)-end)))
		b.WriteString("\n")
	}

	return b.String()
}

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

func renderAddStep1(m Model) string {
	var b strings.Builder

	if m.addPkgEditing {
		b.WriteString(textStyle.Render("Select package:"))
		b.WriteString("\n\n")
		for i, name := range m.addPkgChoices {
			line := fmt.Sprintf("  ○ %s", name)
			if i == m.addPkgIdx {
				line = selectedRowStyle.Width(bodyWidth(m.width)).Render(" ▸ " + name)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(m.addPreviewSummary()))
	} else {
		b.WriteString(textStyle.Render("Package name:"))
		b.WriteString("\n\n")
		b.WriteString(m.addInput.View())
		b.WriteString("\n\n")
		if len(m.addPkgChoices) > 0 {
			b.WriteString(dimStyle.Render("tab to pick from existing packages"))
			b.WriteString("\n")
		}
		b.WriteString(dimStyle.Render(m.addPreviewSummary()))
	}

	return b.String()
}

func (m Model) addPreviewSummary() string {
	files := m.addPreviewFiles()
	if len(files) == 1 {
		return fmt.Sprintf("adding %s", collapseHome(files[0], m.homeDir))
	}
	return fmt.Sprintf("adding %s", countLabel(len(files), "file"))
}

func renderAddStep2(m Model) string {
	var b strings.Builder
	b.WriteString(accentStyle.Render("Review"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")

	files := m.addPreviewFiles()

	if len(files) == 1 {
		path := collapseHome(files[0], m.homeDir)
		relPath, _ := filepath.Rel(m.homeDir, files[0])
		repoPath := filepath.Join(m.repoDir, m.addPkgName, relPath)
		info, _ := os.Stat(files[0])
		isForeignLink := isSymlink(files[0])

		if info != nil && info.IsDir() {
			n := countFiles(files[0])
			b.WriteString("  " + padRight(textStyle.Render("Folder"), 12) + "  " + textStyle.Render(path) + "\n")
			b.WriteString("  " + padRight(textStyle.Render("Package"), 12) + "  " + accentStyle.Render(m.addPkgName) + "\n")
			b.WriteString("  " + padRight(textStyle.Render("Files"), 12) + "  " + dimStyle.Render(countLabel(n, "file")) + "\n")
			b.WriteString("  " + padRight(textStyle.Render("Destination"), 12) + "  " + dimStyle.Render(collapseHome(repoPath, m.homeDir)) + "\n")
		} else {
			b.WriteString("  " + padRight(textStyle.Render("File"), 12) + "  " + textStyle.Render(path) + "\n")
			b.WriteString("  " + padRight(textStyle.Render("Package"), 12) + "  " + accentStyle.Render(m.addPkgName) + "\n")
			b.WriteString("  " + padRight(textStyle.Render("Destination"), 12) + "  " + dimStyle.Render(collapseHome(repoPath, m.homeDir)) + "\n")
		}

		if isForeignLink {
			target, _ := os.Readlink(files[0])
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(files[0]), target)
			}
			b.WriteString("\n")
			b.WriteString(pill(" ADOPT ", colBase, colYellow) + "\n")
			b.WriteString("  " + warningStyle.Render("Foreign symlink detected") + "\n")
			b.WriteString("  " + dimStyle.Render("Currently points to:") + "\n")
			b.WriteString("  " + dimStyle.Render("  "+collapseHome(filepath.Clean(target), m.homeDir)) + "\n")
			b.WriteString("  " + dimStyle.Render("Will be reparented to dotcor repo.") + "\n")
		}
	} else {
		b.WriteString("  " + padRight(textStyle.Render("Files"), 12) + "  " + dimStyle.Render(countLabel(len(files), "file")) + "\n")
		b.WriteString("  " + padRight(textStyle.Render("Package"), 12) + "  " + accentStyle.Render(m.addPkgName) + "\n")
		b.WriteString("\n")
		maxShow := 8
		for i, f := range files {
			if i >= maxShow {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  … %d more", len(files)-maxShow)))
				b.WriteString("\n")
				break
			}
			b.WriteString("  " + dimStyle.Render(collapseHome(f, m.homeDir)) + "\n")
		}
	}

	if len(m.addSecrets) > 0 {
		b.WriteString("\n")
		b.WriteString(pill(" SECRETS DETECTED ", colBase, colRed))
		b.WriteString("\n")
		for _, s := range m.addSecrets {
			fmt.Fprintf(&b, "  %s %s\n",
				errorStyle.Render("⚠"),
				warningStyle.Render(truncate(s, 80)),
			)
		}
		b.WriteString("\n")
		b.WriteString(warningStyle.Render("Review before committing — these may be sensitive."))
	}

	return b.String()
}

func renderAddStep3(m Model) string {
	files := m.addPreviewFiles()
	label := "Adding file…"
	if len(files) > 1 {
		label = fmt.Sprintf("Adding %d files…", len(files))
	}
	return fmt.Sprintf("%s  %s", m.spinner.View(), textStyle.Render(label))
}

func (m Model) addPreviewFiles() []string {
	if m.addPreview == "" {
		return nil
	}
	files := strings.Split(m.addPreview, "\n")
	var clean []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f != "" {
			clean = append(clean, f)
		}
	}
	return clean
}

func addFooterHints(m Model) []string {
	switch m.addStep {
	case 1:
		if m.addPkgEditing {
			return []string{kbd("↑/k", "up"), kbd("↓/j", "down"), kbd("enter", "select"), kbd("esc", "back")}
		}
		hints := []string{kbd("enter", "confirm"), kbd("esc", "back")}
		if len(m.addPkgChoices) > 0 {
			hints = append(hints, kbd("tab", "existing packages"))
		}
		return hints
	case 2:
		return []string{kbd("enter", "confirm"), kbd("esc", "back")}
	default:
		return []string{kbd("esc", "back")}
	}
}

func (m Model) updateAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Esc):
			if m.addStep == 0 {
				m.activeView = DashboardView
				m.resetAddState()
				m.err = nil
				return m, nil
			}
			if m.addStep == 1 && m.addPkgEditing {
				m.addPkgEditing = false
				m.addInput.Blur()
				m.err = nil
				return m, nil
			}
			m.addStep--
			m.err = nil
			return m, nil

		case key.Matches(keyMsg, m.keys.Enter):
			switch m.addStep {
			case 0:
				if len(m.browserSelected) > 0 {
					return m.confirmBrowserSelection()
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
				return m.browserSelectFile(item.path)

			case 1:
				if m.addPkgEditing {
					if m.addPkgIdx < len(m.addPkgChoices) {
						m.addPkgName = m.addPkgChoices[m.addPkgIdx]
						m.addStep = 2
						return m, nil
					}
					m.addPkgEditing = false
					return m, nil
				}
				pkgName := m.addInput.Value()
				if pkgName == "" {
					return m, nil
				}
				if err := validatePackageName(pkgName); err != nil {
					m.err = err
					return m, nil
				}
				m.addPkgName = pkgName
				m.addInput.Blur()
				m.addStep = 2
				return m, nil

			case 2:
				m.addStep = 3
				return m, m.executeAdd()
			}

		case key.Matches(keyMsg, m.keys.Tab) && m.addStep == 1:
			if !m.addPkgEditing && len(m.addPkgChoices) > 0 {
				m.addPkgEditing = true
				m.addPkgIdx = 0
				return m, nil
			} else if m.addPkgEditing {
				m.addPkgEditing = false
				m.addInput.SetValue(m.addPkgName)
				m.addInput.SetCursor(len(m.addPkgName))
				return m, nil
			}
			return m, nil
		}

		if m.addStep == 0 {
			return m.browserHandleKey(keyMsg)
		}

		if m.addStep == 1 && !m.addPkgEditing {
			var cmd tea.Cmd
			m.addInput, cmd = m.addInput.Update(msg)
			return m, cmd
		}

		if m.addStep == 1 && m.addPkgEditing {
			return m.step1HandleKey(keyMsg)
		}
	}

	if m.addStep == 1 {
		if m.addPkgEditing {
			var cmd tea.Cmd
			m.addInput, cmd = m.addInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func (m Model) step1HandleKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxIdx := len(m.addPkgChoices) - 1
	switch keyMsg.String() {
	case "up", "k":
		if m.addPkgIdx > 0 {
			m.addPkgIdx--
		}
		return m, nil
	case "down", "j":
		if m.addPkgIdx < maxIdx {
			m.addPkgIdx++
		}
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
			m.browserSelected[item.path] = !m.browserSelected[item.path]
		}
		return m, nil

	case "h":
		if m.browserCursor >= 0 && m.browserCursor < len(items) {
			item := items[m.browserCursor]
			if item.isDir && m.browserExpanded[item.path] {
				m.browserExpanded[item.path] = false
				delete(m.browserEntries, item.path)
				m.browserItems = nil
				return m, nil
			}
			for dir := range m.browserExpanded {
				if strings.HasPrefix(item.path, dir+string(filepath.Separator)) {
					m.browserExpanded[dir] = false
					delete(m.browserEntries, dir)
					m.browserItems = nil
					return m, nil
				}
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

func (m Model) confirmBrowserSelection() (tea.Model, tea.Cmd) {
	var files []string
	for path, selected := range m.browserSelected {
		if !selected {
			continue
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			_ = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() || isSymlink(p) {
					return nil
				}
				files = append(files, p)
				return nil
			})
		} else {
			files = append(files, path)
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		return m, nil
	}

	m.addPreview = strings.Join(files, "\n")
	m.err = nil

	seen := make(map[string]bool)
	var choices []string
	for _, pkg := range m.packages {
		if !seen[pkg.Name] {
			seen[pkg.Name] = true
			choices = append(choices, pkg.Name)
		}
	}
	m.addPkgChoices = choices
	m.addPkgIdx = 0
	m.addPkgEditing = false

	detected := detectPackageNameMulti(files, m.homeDir)
	for i, name := range choices {
		if name == detected {
			m.addPkgIdx = i
			break
		}
	}
	m.addPkgName = detected
	m.addInput.SetValue(detected)
	m.addInput.SetCursor(len(detected))
	m.addInput.Focus()
	m.addStep = 1
	return m, nil
}

func (m *Model) toggleDirSelection(dirPath string) {
	if m.browserSelected[dirPath] {
		delete(m.browserSelected, dirPath)
		return
	}
	m.browserSelected[dirPath] = true
}

func (m Model) browserSelectFile(fullPath string) (tea.Model, tea.Cmd) {
	if isSymlink(fullPath) {
		managed, _ := fs.SymlinkPointsToRepo(fullPath, m.repoDir)
		if managed {
			m.err = fmt.Errorf("already managed by dotcor — use the dashboard to manage this file")
			return m, nil
		}
	}

	if _, err := os.Stat(fullPath); err != nil {
		m.err = fmt.Errorf("file not found: %s", fullPath)
		return m, nil
	}
	warnings, err := core.ValidateAll(fullPath, m.cfg)
	if err != nil {
		m.err = err
		return m, nil
	}
	m.err = nil
	m.addPreview = fullPath
	m.addSecrets = warnings

	seen := make(map[string]bool)
	var choices []string
	for _, pkg := range m.packages {
		if !seen[pkg.Name] {
			seen[pkg.Name] = true
			choices = append(choices, pkg.Name)
		}
	}
	m.addPkgChoices = choices
	m.addPkgIdx = 0
	m.addPkgEditing = false

	detected := detectPackageName(fullPath)
	for i, name := range choices {
		if name == detected {
			m.addPkgIdx = i
			break
		}
	}
	m.addPkgName = detected
	m.addInput.SetValue(detected)
	m.addInput.SetCursor(len(detected))
	m.addInput.Focus()
	m.addStep = 1
	return m, nil
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

func countFiles(dir string) int {
	count := 0
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		count++
		return nil
	})
	return count
}

func (m Model) executeAdd() tea.Cmd {
	files := m.addPreviewFiles()
	pkgName := m.addPkgName
	repoDir := m.repoDir
	homeDir := m.homeDir
	logger := m.cfg.Logger

	return func() tea.Msg {
		pkgDir := filepath.Join(repoDir, pkgName)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			return addResultMsg{err: fmt.Errorf("creating package directory: %w", err)}
		}

		if len(files) == 1 {
			info, err := os.Stat(files[0])
			if err != nil {
				return addResultMsg{err: fmt.Errorf("source not found: %w", err)}
			}
			if info.IsDir() {
				return executeAddDir(files[0], pkgDir, pkgName, homeDir, logger)
			}
			return executeAddFile(files[0], pkgDir, pkgName, homeDir, logger)
		}

		var totalLinked, totalSkipped int
		var firstErr string
		for _, f := range files {
			result := executeAddFile(f, pkgDir, pkgName, homeDir, logger)
			if result.err != nil {
				if firstErr == "" {
					firstErr = result.err.Error()
				}
				totalSkipped++
				continue
			}
			totalLinked++
		}

		if firstErr != "" {
			return addResultMsg{err: fmt.Errorf("errors adding files: %s", firstErr)}
		}

		msg := fmt.Sprintf("Added %d file(s) → %s", totalLinked, pkgName)
		if totalSkipped > 0 {
			msg += fmt.Sprintf(" (%d skipped)", totalSkipped)
		}
		return addResultMsg{msg: msg}
	}
}

func executeAddFile(srcPath, pkgDir, pkgName, homeDir string, logger *slog.Logger) addResultMsg {
	relPath, err := filepath.Rel(homeDir, srcPath)
	if err != nil {
		relPath = filepath.Base(srcPath)
	}

	if err := validateRelPath(relPath); err != nil {
		return addResultMsg{err: err}
	}

	dstPath := filepath.Join(pkgDir, relPath)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return addResultMsg{err: fmt.Errorf("creating destination directory: %w", err)}
	}

	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return addResultMsg{err: fmt.Errorf("reading source file: %w", err)}
	}

	srcInfo, err := os.Stat(srcPath)
	srcPerm := os.FileMode(0644)
	if err == nil {
		srcPerm = srcInfo.Mode().Perm()
	}

	if err := os.WriteFile(dstPath, srcData, srcPerm); err != nil {
		return addResultMsg{err: fmt.Errorf("writing to repo: %w", err)}
	}

	relSymlink, err := filepath.Rel(filepath.Dir(srcPath), dstPath)
	if err != nil {
		return addResultMsg{err: fmt.Errorf("computing symlink path: %w", err)}
	}

	tempLink := srcPath + ".dotcor-tmp"
	if err := os.Symlink(relSymlink, tempLink); err != nil {
		return addResultMsg{err: fmt.Errorf("creating temp symlink: %w", err)}
	}

	if err := os.Remove(srcPath); err != nil {
		_ = os.Remove(tempLink)
		if logger != nil {
			logger.Warn("failed to remove original file", "file", srcPath, "error", err)
		}
		return addResultMsg{err: fmt.Errorf("removing original file: %w", err)}
	}

	if err := os.Rename(tempLink, srcPath); err != nil {
		_ = os.Remove(tempLink)
		if writeErr := os.WriteFile(srcPath, srcData, srcPerm); writeErr != nil {
			if logger != nil {
				logger.Error("failed to restore original file after rename failure", "file", srcPath, "error", writeErr)
			}
		}
		return addResultMsg{err: fmt.Errorf("moving symlink into place: %w", err)}
	}

	return addResultMsg{msg: fmt.Sprintf("Added %s → %s", filepath.Base(srcPath), pkgName)}
}

func executeAddDir(srcPath, pkgDir, pkgName, homeDir string, logger *slog.Logger) addResultMsg {
	var linked, skipped int
	var firstErr string

	_ = filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if isSymlink(path) {
			skipped++
			return nil
		}

		relPath, err := filepath.Rel(homeDir, path)
		if err != nil || strings.HasPrefix(relPath, "..") {
			skipped++
			return nil
		}

		dstPath := filepath.Join(pkgDir, relPath)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			if firstErr == "" {
				firstErr = err.Error()
			}
			skipped++
			return nil
		}

		srcData, err := os.ReadFile(path)
		if err != nil {
			if firstErr == "" {
				firstErr = err.Error()
			}
			skipped++
			return nil
		}

		srcPerm := info.Mode().Perm()
		if err := os.WriteFile(dstPath, srcData, srcPerm); err != nil {
			if firstErr == "" {
				firstErr = err.Error()
			}
			skipped++
			return nil
		}

		relSymlink, err := filepath.Rel(filepath.Dir(path), dstPath)
		if err != nil {
			skipped++
			return nil
		}

		tempLink := path + ".dotcor-tmp"
		if err := os.Symlink(relSymlink, tempLink); err != nil {
			skipped++
			return nil
		}

		if err := os.Remove(path); err != nil {
			_ = os.Remove(tempLink)
			skipped++
			return nil
		}

		if err := os.Rename(tempLink, path); err != nil {
			_ = os.Remove(tempLink)
			if writeErr := os.WriteFile(path, srcData, srcPerm); writeErr != nil {
				skipped++
				return nil
			}
			skipped++
			return nil
		}

		linked++
		return nil
	})

	if firstErr != "" {
		return addResultMsg{err: fmt.Errorf("errors adding directory: %s", firstErr)}
	}

	msg := fmt.Sprintf("Added %s/ → %s (%d linked", filepath.Base(srcPath), pkgName, linked)
	if skipped > 0 {
		msg += fmt.Sprintf(", %d skipped", skipped)
	}
	msg += ")"
	return addResultMsg{msg: msg}
}

var validPkgName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func validatePackageName(name string) error {
	if !validPkgName.MatchString(name) {
		return fmt.Errorf("invalid package name %q: must contain only letters, numbers, dots, hyphens, underscores", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid package name %q: must not contain '..'", name)
	}
	return nil
}

func validateRelPath(rel string) error {
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("file path escapes home directory: %s", rel)
	}
	return nil
}

func detectPackageName(path string) string {
	home, err := config.GetHomeDir()
	if err != nil || home == "" {
		return filepath.Base(filepath.Dir(path))
	}
	rel, err := filepath.Rel(home, path)
	if err != nil {
		rel = path
	}

	parts := strings.Split(rel, string(filepath.Separator))

	if len(parts) >= 2 && parts[0] == ".config" {
		name := parts[1]
		if idx := strings.LastIndex(name, "."); idx > 0 {
			name = name[:idx]
		}
		if name != "" {
			return name
		}
	}

	if len(parts) >= 1 {
		name := strings.TrimPrefix(parts[0], ".")
		name = strings.TrimSuffix(name, "rc")
		name = strings.TrimSuffix(name, "config")
		if name != "" {
			return name
		}
	}

	return filepath.Base(filepath.Dir(path))
}

func detectPackageNameMulti(paths []string, homeDir string) string {
	if len(paths) == 0 {
		return "misc"
	}
	if len(paths) == 1 {
		return detectPackageName(paths[0])
	}

	common := findCommonParent(paths, homeDir)
	if common != "" {
		return common
	}
	return "misc"
}

func findCommonParent(paths []string, homeDir string) string {
	if len(paths) == 0 {
		return ""
	}

	var partsList [][]string
	for _, p := range paths {
		rel, err := filepath.Rel(homeDir, p)
		if err != nil {
			rel = p
		}
		partsList = append(partsList, strings.Split(rel, string(filepath.Separator)))
	}

	first := partsList[0]
	var common []string
	for i := 0; i < len(first); i++ {
		match := true
		for _, parts := range partsList[1:] {
			if i >= len(parts) || parts[i] != first[i] {
				match = false
				break
			}
		}
		if !match {
			break
		}
		common = append(common, first[i])
	}

	if len(common) == 0 {
		return ""
	}

	candidate := common[len(common)-1]
	name := strings.TrimPrefix(candidate, ".")
	name = strings.TrimSuffix(name, "rc")
	name = strings.TrimSuffix(name, "config")
	if name != "" {
		return name
	}
	return ""
}
