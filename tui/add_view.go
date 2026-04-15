package tui

import (
	"fmt"
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
)

type addResultMsg struct {
	msg string
	err error
}

func viewAdd(m Model) string {
	if m.addStep == 0 {
		header := subviewHeader(m.width, "Add File", []string{"browse"})
		body := renderAddStep0(m)
		footer := subviewFooter(m.width,
			kbd("↑/k", "up"), kbd("↓/j", "down"),
			kbd("enter", "select"), kbd("h", "back"),
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
		body = renderAddStep1(m)
	case 2:
		body = renderAddStep2(m)
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

	displayPath := collapseHome(m.browserPath, m.homeDir)
	if displayPath == "" {
		displayPath = m.browserPath
	}
	b.WriteString(accentStyle.Render("  " + displayPath))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(strings.Repeat("─", max(m.width-8, 4))))
	b.WriteString("\n")

	entries := m.browserVisibleEntries()

	if m.browserPath != m.homeDir {
		b.WriteString("  ")
		b.WriteString(dimStyle.Render("▸"))
		b.WriteString(" ")
		b.WriteString(dimStyle.Render(".."))
		b.WriteString("\n")
	}

	if len(entries) == 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  (empty directory)"))
		return b.String()
	}

	contentHeight := m.height - 8
	if m.browserPath != m.homeDir {
		contentHeight--
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	start := m.browserScroll
	end := start + contentHeight
	if end > len(entries) {
		end = len(entries)
	}

	for i := start; i < end; i++ {
		e := entries[i]
		icon := "○"
		name := e.Name()
		if e.IsDir() {
			icon = "▸"
		}
		line := fmt.Sprintf("  %s %s", icon, name)
		if i == m.browserCursor {
			line = selectedRowStyle.Width(m.width - 8).Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if end < len(entries) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  … %d more", len(entries)-end)))
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

func (m *Model) browserVisibleEntries() []os.DirEntry {
	entries, ok := m.browserEntries[m.browserPath]
	if !ok {
		entries = loadBrowserDir(m.browserPath)
		m.browserEntries[m.browserPath] = entries
	}
	return entries
}

func (m *Model) browserAdjustScroll() {
	contentHeight := m.height - 8
	if m.browserPath != m.homeDir {
		contentHeight--
	}
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
	return strings.Join([]string{
		textStyle.Render("Choose a package name:"),
		"",
		m.addInput.View(),
		"",
		dimStyle.Render(fmt.Sprintf("auto-detected from %s", preview)),
		dimStyle.Render("tab to accept, or edit the name"),
	}, "\n")
}

func renderAddStep2(m Model) string {
	var b strings.Builder
	b.WriteString(accentStyle.Render("Review"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")

	path := collapseHome(m.addPreview, m.homeDir)
	repoPath := filepath.Join(m.repoDir, m.addPkgName, filepath.Base(m.addPreview))

	b.WriteString("  " + padRight(textStyle.Render("File"), 12) + "  " + textStyle.Render(path) + "\n")
	b.WriteString("  " + padRight(textStyle.Render("Package"), 12) + "  " + accentStyle.Render(m.addPkgName) + "\n")
	b.WriteString("  " + padRight(textStyle.Render("Destination"), 12) + "  " + dimStyle.Render(collapseHome(repoPath, m.homeDir)) + "\n")

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
		return []string{kbd("enter", "next"), kbd("esc", "back")}
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
			m.addStep = 0
			m.err = nil
			return m, nil

		case key.Matches(keyMsg, m.keys.Enter):
			switch m.addStep {
			case 0:
				return m.browserSelectEntry()

			case 1:
				pkgName := m.addInput.Value()
				if pkgName == "" {
					return m, nil
				}
				if err := validatePackageName(pkgName); err != nil {
					m.err = err
					return m, nil
				}
				m.addPkgName = pkgName
				m.addStep = 2
				m.addInput.Blur()
				return m, nil

			case 2:
				m.addStep = 3
				return m, m.executeAdd()
			}

		case key.Matches(keyMsg, m.keys.Tab) && m.addStep == 1:
			m.addInput.SetValue(m.addPkgName)
			m.addInput.SetCursor(len(m.addPkgName))
			return m, nil
		}

		if m.addStep == 0 {
			return m.browserHandleKey(keyMsg)
		}
	}

	if m.addStep == 1 {
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) browserHandleKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "up", "k":
		if m.browserCursor > 0 {
			m.browserCursor--
		}
		m.browserAdjustScroll()
		return m, nil

	case "down", "j":
		entries := m.browserVisibleEntries()
		if m.browserCursor < len(entries)-1 {
			m.browserCursor++
		}
		m.browserAdjustScroll()
		return m, nil

	case "h":
		parent := filepath.Dir(m.browserPath)
		if parent != m.browserPath && parent != filepath.Dir(m.homeDir) {
			m.browserPath = parent
			m.browserCursor = 0
			m.browserScroll = 0
		}
		return m, nil

	case "l":
		return m.browserSelectEntry()
	}

	return m, nil
}

func (m Model) browserSelectEntry() (tea.Model, tea.Cmd) {
	entries := m.browserVisibleEntries()
	if m.browserCursor < 0 || m.browserCursor >= len(entries) {
		return m, nil
	}

	e := entries[m.browserCursor]
	fullPath := filepath.Join(m.browserPath, e.Name())

	if e.IsDir() {
		m.browserPath = fullPath
		m.browserCursor = 0
		m.browserScroll = 0
		return m, nil
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
	m.addPkgName = detectPackageName(fullPath)
	m.addInput.SetValue(m.addPkgName)
	m.addInput.SetCursor(len(m.addPkgName))
	m.addSecrets = warnings
	m.addStep = 1
	m.addInput.Focus()
	return m, textinput.Blink
}

func (m Model) executeAdd() tea.Cmd {
	srcPath := m.addPreview
	pkgName := m.addPkgName
	repoDir := m.repoDir
	homeDir := m.homeDir
	logger := m.cfg.Logger

	return func() tea.Msg {
		pkgDir := filepath.Join(repoDir, pkgName)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			return addResultMsg{err: fmt.Errorf("creating package directory: %w", err)}
		}

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
			logger.Warn("failed to remove original file", "file", srcPath, "error", err)
			return addResultMsg{err: fmt.Errorf("removing original file: %w", err)}
		}

		if err := os.Rename(tempLink, srcPath); err != nil {
			_ = os.Remove(tempLink)
			return addResultMsg{err: fmt.Errorf("moving symlink into place: %w", err)}
		}

		return addResultMsg{msg: fmt.Sprintf("Added %s → %s", filepath.Base(srcPath), pkgName)}
	}
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

	if len(parts) >= 3 && parts[0] == ".config" {
		return parts[1]
	}

	if len(parts) >= 1 {
		name := strings.TrimPrefix(parts[0], ".")
		if strings.Contains(name, "rc") {
			name = strings.ReplaceAll(name, "rc", "")
		}
		if name != "" {
			return name
		}
	}

	return filepath.Base(filepath.Dir(path))
}
