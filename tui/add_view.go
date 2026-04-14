package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type addResultMsg struct {
	msg string
	err error
}

func viewAdd(m Model) string {
	var b strings.Builder

	b.WriteString(accentStyle.Bold(true).Render("Add File"))
	b.WriteString("\n\n")

	switch m.addStep {
	case 0:
		b.WriteString(textStyle.Render("Enter file path to add:"))
		b.WriteString("\n")
		b.WriteString(m.addInput.View())
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("Enter the path to a dotfile (e.g. ~/.config/nvim/init.lua)"))

	case 1:
		b.WriteString(textStyle.Render("Confirm package name:"))
		b.WriteString("\n")
		b.WriteString(m.addInput.View())
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("Auto-detected from: %s", m.addPreview)))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("Tab to accept, or edit the package name"))

	case 2:
		b.WriteString(accentStyle.Render("Preview"))
		b.WriteString("\n\n")

		path := strings.Replace(m.addPreview, m.homeDir, "~", 1)
		b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render("File:"), dimStyle.Render(path)))
		b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render("Package:"), textStyle.Render(m.addPkgName)))

		repoPath := filepath.Join(m.repoDir, m.addPkgName, filepath.Base(m.addPreview))
		b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render("Repo:"), dimStyle.Render(repoPath)))

		if len(m.addSecrets) > 0 {
			b.WriteString("\n")
			b.WriteString(errorStyle.Bold(true).Render("Secrets detected:"))
			b.WriteString("\n")
			for _, s := range m.addSecrets {
				b.WriteString(fmt.Sprintf("  %s %s\n", errorStyle.Render("⚠"), warningStyle.Render(s)))
			}
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render("Enter to confirm, Esc to cancel"))

	case 3:
		b.WriteString(textStyle.Render("Adding file..."))
	}

	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Esc to cancel"))

	content := b.String()
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Padding(1, 2).
		Render(content)
}

func (m Model) updateAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(msg, m.keys.Esc):
			m.activeView = DashboardView
			m.addStep = 0
			m.addInput.Blur()
			m.addInput.SetValue("")
			m.addPkgName = ""
			m.addPreview = ""
			m.addSecrets = nil
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			switch m.addStep {
			case 0:
				path := expandHome(m.addInput.Value())
				if path == "" {
					return m, nil
				}

				if _, err := os.Stat(path); err != nil {
					m.err = fmt.Errorf("file not found: %s", path)
					return m, nil
				}

				m.addPreview = path
				m.addPkgName = detectPackageName(path)
				m.addInput.SetValue(m.addPkgName)
				m.addInput.SetCursor(len(m.addPkgName))
				m.addStep = 1
				return m, nil

			case 1:
				pkgName := m.addInput.Value()
				if pkgName == "" {
					return m, nil
				}
				m.addPkgName = pkgName
				m.addSecrets = scanForSecrets(m.addPreview)
				m.addStep = 2
				m.addInput.Blur()
				return m, nil

			case 2:
				return m, m.executeAdd()
			}

		case key.Matches(msg, m.keys.Tab) && m.addStep == 1:
			m.addInput.SetValue(m.addPkgName)
			m.addInput.SetCursor(len(m.addPkgName))
			return m, nil
		}
	}

	if m.addStep <= 1 {
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
		return m, cmd
	}

	return m, nil
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

		dstPath := filepath.Join(pkgDir, relPath)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return addResultMsg{err: fmt.Errorf("creating destination directory: %w", err)}
		}

		srcData, err := os.ReadFile(srcPath)
		if err != nil {
			return addResultMsg{err: fmt.Errorf("reading source file: %w", err)}
		}

		if err := os.WriteFile(dstPath, srcData, 0644); err != nil {
			return addResultMsg{err: fmt.Errorf("writing to repo: %w", err)}
		}

		if err := os.Remove(srcPath); err != nil {
			logger.Warn("failed to remove original file", "file", srcPath, "error", err)
		}

		relSymlink, err := filepath.Rel(filepath.Dir(srcPath), dstPath)
		if err != nil {
			return addResultMsg{err: fmt.Errorf("computing symlink path: %w", err)}
		}

		if err := os.Symlink(relSymlink, srcPath); err != nil {
			return addResultMsg{err: fmt.Errorf("creating symlink: %w", err)}
		}

		return addResultMsg{msg: fmt.Sprintf("Added %s to package %s", filepath.Base(srcPath), pkgName)}
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func detectPackageName(path string) string {
	home, _ := os.UserHomeDir()
	rel, err := filepath.Rel(home, path)
	if err != nil {
		rel = path
	}

	parts := strings.Split(rel, string(filepath.Separator))

	if len(parts) >= 3 && parts[0] == ".config" {
		return parts[1]
	}

	if len(parts) >= 1 {
		name := parts[0]
		if strings.HasPrefix(name, ".") {
			name = name[1:]
		}
		if strings.Contains(name, "rc") {
			name = strings.ReplaceAll(name, "rc", "")
		}
		if name != "" {
			return name
		}
	}

	return filepath.Base(filepath.Dir(path))
}

func scanForSecrets(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	content := string(data)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(api_key|apikey|api-secret)\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)secret\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)token\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)password\s*[:=]\s*\S+`),
		regexp.MustCompile(`-----BEGIN\s+RSA\s+PRIVATE\s+KEY-----`),
		regexp.MustCompile(`-----BEGIN\s+PRIVATE\s+KEY-----`),
		regexp.MustCompile(`(?i)bearer\s+\S+`),
	}

	var found []string
	for _, p := range patterns {
		matches := p.FindAllString(content, -1)
		for _, match := range matches {
			found = append(found, match)
		}
	}

	return found
}
