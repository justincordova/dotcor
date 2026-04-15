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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
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

	if m.addStep == 0 {
		header := subviewHeader(m.width, "Add File", []string{"browse"})
		body := renderAddStep0(m) + errLine
		footer := subviewFooter(m.width,
			kbd("↑/k", "up"), kbd("↓/j", "down"),
			kbd("enter", "expand/add"), kbd("h", "collapse"),
			kbd("esc", "cancel"),
		)
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			subviewContent(m.width, m.height-4, body),
			footer,
		)
	}

	header := subviewHeader(m.width, "Add File", []string{stepLabel(m.addStep)})

	var body string
	switch m.addStep {
	case 1:
		body = renderAddStep1(m) + errLine
	case 2:
		body = renderAddStep2(m) + errLine
	case 3:
		body = renderAddStep3(m)
	}

	footer := subviewFooter(m.width, addFooterHints(m)...)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		renderStepper(m.width, m.addStep),
		subviewContent(m.width, m.height-4, body),
		footer,
	)
}

func stepLabel(step int) string {
	switch step {
	case 0:
		return "path"
	case 1:
		return "package name"
	case 2:
		return "preview"
	case 3:
		return "adding"
	}
	return ""
}

func renderStepper(width, step int) string {
	steps := []string{"Path", "Package", "Preview"}
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
	row := lipgloss.JoinHorizontal(lipgloss.Center, parts...)
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
	b.WriteString(subtleStyle.Render(strings.Repeat("─", max(m.width-8, 4))))
	b.WriteString("\n")

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
			if m.browserExpanded[item.path] {
				icon = "▾"
			} else {
				icon = "▸"
			}
			styledName = accentStyle.Render(name + "/")
		} else if isSymlink(item.path) {
			icon = "◆"
			styledName = dimStyle.Render(name)
		} else {
			icon = "○"
			styledName = textStyle.Render(name)
		}

		line := fmt.Sprintf("  %s%s %s", indent, icon, styledName)
		if i == m.browserCursor {
			line = selectedRowStyle.Width(m.width - 8).Render(line)
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
	preview := collapseHome(m.addPreview, m.homeDir)
	var b strings.Builder

	if m.addPkgEditing {
		b.WriteString(textStyle.Render("Select package:"))
		b.WriteString("\n\n")
		for i, name := range m.addPkgChoices {
			line := fmt.Sprintf("  ○ %s", name)
			if i == m.addPkgIdx {
				line = selectedRowStyle.Width(m.width - 8).Render(" ▸ " + name)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("adding %s", preview)))
	} else {
		b.WriteString(textStyle.Render("Package name:"))
		b.WriteString("\n\n")
		b.WriteString(m.addInput.View())
		b.WriteString("\n\n")
		if len(m.addPkgChoices) > 0 {
			b.WriteString(dimStyle.Render("tab to pick from existing packages"))
			b.WriteString("\n")
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("adding %s", preview)))
	}

	return b.String()
}

func renderAddStep2(m Model) string {
	var b strings.Builder
	b.WriteString(accentStyle.Render("Review"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")

	path := collapseHome(m.addPreview, m.homeDir)
	relPath, _ := filepath.Rel(m.homeDir, m.addPreview)
	repoPath := filepath.Join(m.repoDir, m.addPkgName, relPath)

	info, _ := os.Stat(m.addPreview)
	if info != nil && info.IsDir() {
		n := countFiles(m.addPreview)
		b.WriteString("  " + padRight(textStyle.Render("Folder"), 12) + "  " + textStyle.Render(path) + "\n")
		b.WriteString("  " + padRight(textStyle.Render("Package"), 12) + "  " + accentStyle.Render(m.addPkgName) + "\n")
		b.WriteString("  " + padRight(textStyle.Render("Files"), 12) + "  " + dimStyle.Render(countLabel(n, "file")) + "\n")
		b.WriteString("  " + padRight(textStyle.Render("Destination"), 12) + "  " + dimStyle.Render(collapseHome(repoPath, m.homeDir)) + "\n")
	} else {
		b.WriteString("  " + padRight(textStyle.Render("File"), 12) + "  " + textStyle.Render(path) + "\n")
		b.WriteString("  " + padRight(textStyle.Render("Package"), 12) + "  " + accentStyle.Render(m.addPkgName) + "\n")
		b.WriteString("  " + padRight(textStyle.Render("Destination"), 12) + "  " + dimStyle.Render(collapseHome(repoPath, m.homeDir)) + "\n")
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
	return fmt.Sprintf("%s  %s", m.spinner.View(), textStyle.Render("Adding file…"))
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
			m.addStep = 0
			m.err = nil
			return m, nil

		case key.Matches(keyMsg, m.keys.Enter):
			switch m.addStep {
			case 0:
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

	case "h":
		if m.browserCursor >= 0 && m.browserCursor < len(items) {
			item := items[m.browserCursor]
			if item.isDir && m.browserExpanded[item.path] {
				m.browserExpanded[item.path] = false
				m.browserItems = nil
				return m, nil
			}
			for dir := range m.browserExpanded {
				if strings.HasPrefix(item.path, dir+string(filepath.Separator)) {
					m.browserExpanded[dir] = false
					m.browserItems = nil
					return m, nil
				}
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) browserSelectFile(fullPath string) (tea.Model, tea.Cmd) {
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
	srcPath := m.addPreview
	pkgName := m.addPkgName
	repoDir := m.repoDir
	homeDir := m.homeDir
	logger := m.cfg.Logger

	return func() tea.Msg {
		info, err := os.Stat(srcPath)
		if err != nil {
			return addResultMsg{err: fmt.Errorf("source not found: %w", err)}
		}

		pkgDir := filepath.Join(repoDir, pkgName)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			return addResultMsg{err: fmt.Errorf("creating package directory: %w", err)}
		}

		if info.IsDir() {
			return executeAddDir(srcPath, pkgDir, pkgName, homeDir, logger)
		}
		return executeAddFile(srcPath, pkgDir, pkgName, homeDir, logger)
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
