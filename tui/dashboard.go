package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justincordova/dotcor/internal/stow"
)

func viewDashboard(m Model) string {
	header := renderHeader(m)
	main := renderMain(m)
	gitBar := renderGitBar(m)
	footer := renderFooter(m)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		main,
		gitBar,
		footer,
	)
}

func renderHeader(m Model) string {
	version := "v1.0.4"
	title := accentStyle.Bold(true).Render("DotCor " + version)

	pkgCount := fmt.Sprintf("%d package", len(m.packages))
	if len(m.packages) != 1 {
		pkgCount += "s"
	}
	count := dimStyle.Render(pkgCount)

	spacer := lipgloss.NewStyle().
		Width(m.width - lipgloss.Width(title) - lipgloss.Width(count) - 4).
		Render("")

	return lipgloss.NewStyle().
		Width(m.width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color(muted)).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Top, title, spacer, count),
		)
}

func renderMain(m Model) string {
	leftWidth := m.width * 2 / 5
	rightWidth := m.width - leftWidth - 4

	left := renderPackageList(m, leftWidth)
	right := renderFileDetail(m, rightWidth)

	leftStyled := boxStyle.Width(leftWidth - 2).Height(m.height - 10).Render(left)
	rightStyled := activeBoxStyle.Width(rightWidth - 2).Height(m.height - 10).Render(right)

	main := lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, rightStyled)

	linesNeeded := m.height - 10
	currentLines := strings.Count(main, "\n") + 1
	if currentLines < linesNeeded {
		padding := strings.Repeat("\n", linesNeeded-currentLines)
		main = main + padding
	}

	return main
}

func renderPackageList(m Model, width int) string {
	var b strings.Builder

	header := accentStyle.Bold(true).Render(fmt.Sprintf("Packages (%d)", len(m.packages)))
	b.WriteString(header)
	b.WriteString("\n")

	for i, pkg := range m.packages {
		cursor := " "
		if i == m.selectedPkg {
			cursor = selectedStyle.Render("▶")
		}

		name := pkg.Name
		if i == m.selectedPkg {
			name = selectedStyle.Render(name)
		} else {
			name = textStyle.Render(name)
		}

		indicator := statusIndicator(pkg.Status)

		padWidth := width - 6 - len(pkg.Name)
		if padWidth < 1 {
			padWidth = 1
		}
		padding := strings.Repeat(" ", padWidth)

		b.WriteString(fmt.Sprintf("%s %s%s%s\n", cursor, name, padding, indicator))
	}

	return b.String()
}

func renderFileDetail(m Model, width int) string {
	if m.selectedPkg >= len(m.packages) {
		return dimStyle.Render("  No package selected")
	}

	pkg := m.packages[m.selectedPkg]

	var b strings.Builder

	b.WriteString(accentStyle.Bold(true).Render(pkg.Name))
	b.WriteString("\n")

	if len(pkg.Files) == 0 {
		b.WriteString(dimStyle.Render("  No files"))
		return b.String()
	}

	for _, f := range pkg.Files {
		arrow := textStyle.Render("→")
		status := fileStatus(f)

		line := fmt.Sprintf("  %s %s %s  %s",
			dimStyle.Render(f.RelPath),
			arrow,
			dimStyle.Render(f.TargetPath),
			status,
		)

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func renderGitBar(m Model) string {
	var parts []string

	if m.gitStatus.Branch != "" {
		branch := fmt.Sprintf("git/%s", m.gitStatus.Branch)
		parts = append(parts, accentStyle.Render(branch))
	}

	if m.gitStatus.HasUncommitted {
		count := fmt.Sprintf("%d uncommitted change", len(m.gitStatus.ChangedFiles))
		if len(m.gitStatus.ChangedFiles) != 1 {
			count += "s"
		}
		parts = append(parts, warningStyle.Render("● "+count))
	} else if m.gitStatus.Branch != "" {
		parts = append(parts, successStyle.Render("● clean"))
	}

	if m.gitStatus.AheadBy > 0 {
		parts = append(parts, gitAheadStyle.Render(fmt.Sprintf("↑%d ahead", m.gitStatus.AheadBy)))
	}

	if m.gitStatus.BehindBy > 0 {
		parts = append(parts, warningStyle.Render(fmt.Sprintf("↓%d behind", m.gitStatus.BehindBy)))
	}

	if len(parts) == 0 {
		parts = append(parts, dimStyle.Render("no git info"))
	}

	content := strings.Join(parts, "  ")

	if m.statusMsg != "" {
		content = successStyle.Render(m.statusMsg)
	}

	if m.err != nil {
		content = errorStyle.Render(fmt.Sprintf("error: %v", m.err))
	}

	return statusBarStyle.Width(m.width).Render(content)
}

func renderFooter(m Model) string {
	if m.searching {
		return lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color(muted)).
			Render(
				keyStyle.Render("/") + " " + m.searchInput.View(),
			)
	}

	helpStr := m.help.View(m.keys)

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color(muted)).
		Render(helpStr)
}

func statusIndicator(status stow.PackageStatus) string {
	switch status {
	case stow.StatusLinked:
		return successStyle.Render("✓")
	case stow.StatusPartial:
		return warningStyle.Render("⚠")
	case stow.StatusUnlinked:
		return errorStyle.Render("✗")
	default:
		return dimStyle.Render("?")
	}
}

func fileStatus(f stow.FileEntry) string {
	if f.IsLinked {
		return successStyle.Render("linked")
	}
	if f.Exists && !f.IsSymlink {
		return errorStyle.Render("conflict")
	}
	return dimStyle.Render("unlinked")
}
