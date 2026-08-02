package tui

import (
	"fmt"
	"io/fs"
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
		footer = plainFooter(innerW, browserFooterHints(m)...)
		body := renderAddStep0(m, footer, errLine) + errLine
		content := lipgloss.JoinVertical(lipgloss.Left,
			renderAddStepper(innerW, m.addStep),
			lipgloss.NewStyle().Padding(1, 0).Render(body),
			footer,
		)
		dialog := boxStyle.Width(cw - 2).Render(content)
		dialog = clampDialogHeight(dialog, m.height)
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
		body = renderPreviewStep(m, innerW, cw) + errLine
		footer = plainFooter(innerW,
			kbd("↑/k", "up"), kbd("↓/j", "down"),
			kbd("pgup/pgdn", "page"), kbd("g/G", "top/bot"),
			kbd("enter", "confirm"), kbd("esc", "back"),
		)
	case addStepConfirm:
		body = renderConfirmStep(m, innerW, cw) + errLine
		footer = plainFooter(innerW,
			kbd("↑/k", "up"), kbd("↓/j", "down"),
			kbd("pgup/pgdn", "page"), kbd("g/G", "top/bot"),
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
	dialog = clampDialogHeight(dialog, m.height)
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color(colBase)),
	)
}

// clampDialogHeight ensures the dialog fits within maxLines by removing
// lines from the middle of the content area (between header and footer).
func clampDialogHeight(dialog string, maxLines int) string {
	lines := strings.Split(dialog, "\n")
	if len(lines) <= maxLines {
		return dialog
	}
	// Keep top border + stepper + padding + header, and footer + border.
	// Remove lines from the middle until we fit.
	const (
		top    = 5
		bottom = 2
	)

	// Nothing sensible to trim from a dialog smaller than its own chrome;
	// the loop below would otherwise drive excess negative and the slice
	// expressions would panic.
	if len(lines) < top+bottom {
		return dialog
	}

	excess := len(lines) - maxLines
	for len(lines)-excess < top+bottom {
		excess--
	}
	clamped := make([]string, 0, len(lines)-excess)
	clamped = append(clamped, lines[:top]...)
	clamped = append(clamped, lines[top+excess:]...)
	return strings.Join(clamped, "\n")
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

func renderAddStep0(m Model, footer string, errLine string) string {
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

	items := m.browserItemsForRender()

	if len(items) == 0 {
		b.WriteString("\n")
		b.WriteString("  " + dimStyle.Render("○") + " " + textStyle.Render("Nothing to add here."))
		b.WriteString("\n\n")
		b.WriteString("  " + dimStyle.Render("Every file is already managed, or the directory is empty."))
		b.WriteString("\n")
		b.WriteString("  " + dimStyle.Render("Press ") + kbd("/", "jump") + dimStyle.Render(" to go to another path, ") + kbd("esc", "cancel") + dimStyle.Render("."))
		return b.String()
	}


	// Shared with browserAdjustScroll and the paging keys so the drawn
	// viewport and the scroll maths can never disagree.
	ch := browserContentHeight(m)

	// Clamp start to a valid range. Downward navigation on an empty item
	// list drives browserCursor to len-1 == -1, which browserAdjustScroll
	// mirrors into a negative browserScroll. renderAddStep0 currently
	// early-returns for len(items)==0 so this loop isn't reached while
	// empty — but if the list becomes non-empty while scroll is still
	// negative, items[start] would index items[-1] and panic. Guard at the
	// point of use so this loop is safe regardless of how scroll got here.
	start := m.browserScroll
	if start < 0 {
		start = 0
	}
	end := start + ch
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
				// Read the count cached when the directory was selected;
				// never walk the filesystem from View.
				if count, ok := m.browserFileCounts[item.path]; ok {
					styledName += dimStyle.Render(fmt.Sprintf(" (%d files)", count))
				}
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
			cursor = accentStyle.Render(selectionMarker)
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

		// Package header title + separator rows (non-navigable). Each
		// header row represents exactly one visual terminal line so the
		// viewport's row-based pagination math stays honest.
		rows = append(rows, previewRow{
			pkgName:     pkg.Name,
			isHeader:    true,
			headerLabel: "pkg:" + pkg.Name,
		})
		rows = append(rows, previewRow{
			pkgName:     pkg.Name,
			isHeader:    true,
			headerLabel: "pkgsep",
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

func renderPreviewStep(m Model, innerW, cw int) string {
	if m.previewPlan == nil {
		return dimStyle.Render("  Classifying files…")
	}

	var b strings.Builder

	rows := m.previewRows
	bw := bodyWidth(m.width)

	// Shared with previewHandleKey so maxScroll and the drawn viewport can
	// never disagree.
	contentHeight := previewContentHeight(m)

	// Clamp scroll to [0, max(0, len-contentHeight)]. This handles
	// window resizes shrinking the viewport underneath an existing
	// scroll, and guarantees the user can always reach the very first
	// row (scroll = 0) — including header rows like "Package: foo".
	maxScroll := len(rows) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	start := m.previewScroll
	if start > maxScroll {
		start = maxScroll
	}
	if start < 0 {
		start = 0
	}
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

		line := renderPreviewFileRow(*cf, toggled, row.pkgName, bw)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Scroll position indicator — only shown when the list overflows
	// the viewport. Mirrors the confirm step so the scrollability is
	// discoverable without the user having to press keys to find out.
	if len(rows) > contentHeight {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d-%d of %d  ↑/↓ to scroll · pgup/pgdn page · g/G top/bottom", start+1, end, len(rows))))
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
		return accentStyle.Render("  Package: " + pkgName)
	}
	if row.headerLabel == "pkgsep" {
		return subtleStyle.Render("  " + strings.Repeat("─", max(bw-2, 4)))
	}

	// Class section header. Labels are lowercase to match the summary
	// count pills below and the app-wide status lexicon — the same class
	// must not read as FOREIGN here and "foreign N" a few rows down.
	class := row.class
	var label string
	var color string
	switch class {
	case stow.ClassAdopt:
		label, color = "adopt", colGreen
	case stow.ClassAdd:
		label, color = "add", colBlue
	case stow.ClassTrack:
		label, color = "track", colMauve
	case stow.ClassForeign:
		label, color = "foreign", colYellow
	case stow.ClassManaged:
		label, color = "managed", colOverlay0
	default:
		label, color = "unknown", colOverlay0
	}
	return "  " + pill(" "+label+" ", colBase, color)
}

func renderPreviewFileRow(cf stow.ClassifiedFile, toggled bool, pkgName string, bw int) string {
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
		detail = warningStyle.Render(fmt.Sprintf("→ %s", target))
	case stow.ClassManaged:
		detail = dimStyle.Render(fmt.Sprintf("already in repo/%s/", pkgName))
	}

	// Fix the name to a single column width so the detail column lines up
	// straight regardless of name length — long names truncate rather than
	// shoving detail out of alignment, short names pad up to the column.
	const nameCol = 24
	displayName := truncate(name, nameCol)
	if isManaged || !toggled {
		displayName = dimStyle.Render(padRight(displayName, nameCol))
	} else {
		displayName = textStyle.Render(padRight(displayName, nameCol))
	}
	line := fmt.Sprintf("  %s%s  %s", checkbox, displayName, detail)

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
	if n := len(plan.Warnings); n > 0 {
		parts = append(parts, pill(fmt.Sprintf(" warn %d ", n), colBase, colYellow))
	}

	active := dimStyle.Render(fmt.Sprintf("  %d active", activeCount))
	if len(parts) == 0 {
		return "  " + active
	}
	return "  " + strings.Join(parts, " ") + "  " + active
}

// ─── Step 2: Confirm ───────────────────────────────────────────────────────────

func renderConfirmStep(m Model, innerW, cw int) string {
	if m.previewPlan == nil {
		return dimStyle.Render("  No plan to confirm.")
	}

	lines := buildConfirmLines(m.previewPlan, m.previewToggles, bodyWidth(m.width))

	// Shared with confirmHandleKey so maxScroll and the drawn viewport can
	// never disagree.
	contentHeight := confirmContentHeight(m)

	// Clamp scroll to [0, max(0, len-contentHeight)]. This also handles
	// window resizes shrinking the viewport underneath an existing scroll.
	maxScroll := len(lines) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.confirmScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	end := scroll + contentHeight
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	for i := scroll; i < end; i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
	}

	// Sticky footer: scroll position + execute hint.
	if len(lines) > contentHeight {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d-%d of %d  ↑/↓ to scroll", scroll+1, end, len(lines))))
		b.WriteString("\n")
	}
	b.WriteString(dimStyle.Render("  enter to execute · esc to go back"))

	return b.String()
}

// confirmContentHeight returns the number of body rows available for the
// confirm step. Mirrors the chrome accounting done by viewAdd: stepper
// (1) + padding top/bottom (2) + footer (1) + box border (2) + two
// sticky bottom rows = 8. A hard floor of 3 keeps the view usable on
// tiny terminals.
// confirmContentHeight returns the number of confirm rows that actually fit.
//
// It measures the chrome it will render rather than assuming a fixed number
// of lines. The key handler and the renderer previously computed this
// independently — the handler with a hardcoded `height - 14` — so maxScroll
// disagreed with what was drawn and G/end stopped several rows short. Those
// unreachable rows are the tail of the list of files the user is being asked
// to approve.
func confirmContentHeight(m Model) int {
	cw := contentWidth(m.width)
	innerW := cw - 4

	fixedContent := lipgloss.JoinVertical(lipgloss.Left,
		renderAddStepper(innerW, addStepConfirm),
		"", "",
		"enter to execute · esc to go back",
		plainFooter(innerW,
			kbd("↑/k", "up"), kbd("↓/j", "down"),
			kbd("pgup/pgdn", "page"), kbd("g/G", "top/bot"),
			kbd("enter", "execute"), kbd("esc", "back"),
		),
	)
	fixedLines := strings.Count(boxStyle.Width(cw-2).Render(fixedContent), "\n") + 1
	if m.err != nil {
		fixedLines += 2
	}
	// The renderer emits TWO sticky bottom rows once the list overflows: the
	// "%d-%d of %d" scroll indicator and the execute hint. fixedContent
	// models only the hint, so the dialog came out exactly one line too tall
	// and clampDialogHeight deleted lines[5] — the third visible body row.
	// At any non-zero scroll offset that is a file row, so one entry in the
	// list the user is being asked to approve was invisible, while the
	// counter still reported the full range.
	fixedLines++

	h := m.height - fixedLines
	if h < 3 {
		h = 3
	}
	return h
}

// buildConfirmLines produces the flat list of rendered rows for the
// confirm step — one slice element per visual terminal row. Keeping the
// output as []string lets the view paginate without re-doing the layout
// on every scroll tick.
func buildConfirmLines(plan *stow.ClassificationPlan, toggles map[string]bool, bw int) []string {
	var lines []string

	sep := subtleStyle.Render("  " + strings.Repeat("─", max(bw-2, 4)))
	lines = append(lines, accentStyle.Render("  Confirm changes"))
	lines = append(lines, sep)

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

		lines = append(lines, "")
		lines = append(lines, accentStyle.Render("  Package: "+pkg.Name))
		lines = append(lines, sep)

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

			lines = append(lines, "  "+pill(fmt.Sprintf(" %s %d ", label, len(active)), colBase, color))
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
				lines = append(lines, dimStyle.Render(line))
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
		lines = append(lines, "")
		lines = append(lines, "  "+pill(fmt.Sprintf(" MANAGED %d (skipped) ", managedCount), colBase, colOverlay0))
	}

	return lines
}

// ─── Browser helpers (shared with step 0) ────────────────────────────────────

var browserSkipDirs = map[string]bool{
	".git": true, "node_modules": true, ".cache": true, "__pycache__": true,
	"Library": true, ".Trash": true, ".dotcor": true,
	".npm": true, ".nvm": true, ".local": true,
	".cargo": true, ".rustup": true, ".vscode": true, ".vscode-server": true,
	".gradle": true, ".m2": true, ".maven": true, ".docker": true,
	".pyenv": true, ".rbenv": true, ".oh-my-zsh": true, ".oh-my-bash": true,
	".zprezto": true, ".iterm2": true, ".kube": true, ".aws": true,
	".config": true, ".bun": true, ".ollama": true, ".android": true,
	".dotnet": true, ".nuget": true, ".swift": true, ".swiftpm": true,
	".DS_Store":    true,
	"Applications": true, "Desktop": true, "Documents": true,
	"Downloads": true, "Movies": true, "Music": true, "Pictures": true,
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

// buildBrowserItems returns the flattened browser tree, memoising it on the
// Model. Only call this from Update, where the returned Model is propagated —
// the memo is otherwise written to a copy and thrown away.
func (m *Model) buildBrowserItems() []browserItem {
	if m.browserItems != nil {
		return m.browserItems
	}
	var items []browserItem
	m.walkBrowserDir(m.homeDir, 0, &items)
	m.browserItems = items
	return items
}

// browserItemsForRender returns the browser tree for the view layer.
//
// View receives the Model by value, so calling buildBrowserItems there writes
// the memo to a copy that is discarded when the function returns — the cache
// never survived a frame and every render re-walked the home directory,
// running Lstat and EvalSymlinks per entry to decide what is already managed.
// Update populates the memo; this only reads it, and rebuilds solely as a
// fallback for the first frame after the view opens.
func (m Model) browserItemsForRender() []browserItem {
	if m.browserItems != nil {
		return m.browserItems
	}
	var items []browserItem
	m.walkBrowserDir(m.homeDir, 0, &items)
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
	ch := browserContentHeight(*m)
	if ch < 1 {
		ch = 1
	}

	// Clamp the cursor to the current list first. Collapsing an ancestor
	// removes every descendant row, so the tree can shrink far below where
	// the cursor was — and with cursor and scroll both past the end the
	// renderer's loop never runs and the browser goes completely blank, with
	// j/G appearing dead because they are gated on len(items)-1.
	items := m.buildBrowserItems()
	if m.browserCursor > len(items)-1 {
		m.browserCursor = len(items) - 1
	}
	if m.browserCursor < 0 {
		m.browserCursor = 0
	}

	if m.browserCursor < m.browserScroll {
		m.browserScroll = m.browserCursor
	}
	if m.browserCursor >= m.browserScroll+ch {
		m.browserScroll = m.browserCursor - ch + 1
	}
	// Never scroll further than the last full window. When the tree shrinks
	// under the cursor, following the cursor alone leaves the offset high
	// and the viewport shows a handful of trailing rows with empty space
	// below, instead of the whole (now short) list.
	if maxScroll := len(items) - ch; m.browserScroll > maxScroll {
		m.browserScroll = maxScroll
	}
	// An empty item list drives browserCursor to -1 (len-1), which would
	// otherwise leave browserScroll negative. Keep it non-negative.
	if m.browserScroll < 0 {
		m.browserScroll = 0
	}
}

// browserContentHeight returns the number of browser rows that fit.
//
// It measures the chrome renderAddStep0 actually draws rather than using a
// hardcoded constant. The two disagreed by two rows, so pgdown paged by less
// than a screenful (re-showing rows) and maxScroll was computed against the
// smaller number, leaving the bottom two rows of the viewport permanently
// blank at the end of the list. Same class of defect as the one already
// fixed for the confirm and preview steps.
func browserContentHeight(m Model) int {
	cw := contentWidth(m.width)
	innerW := cw - 4

	fixedContent := lipgloss.JoinVertical(lipgloss.Left,
		renderAddStepper(innerW, addStepSelect),
		"", "",
		plainFooter(innerW, browserFooterHints(m)...),
	)
	fixedLines := strings.Count(boxStyle.Width(cw-2).Render(fixedContent), "\n") + 1
	fixedLines += 2 // path header + horizontal rule
	fixedLines++    // the "… N more" row emitted when the list overflows
	if m.err != nil {
		fixedLines += 2
	}
	if m.browserJumping {
		fixedLines += 4 // blank, jump input, rule, and its trailing newline
	}

	ch := m.height - fixedLines - 1
	if ch < 1 {
		ch = 1
	}
	return ch
}

// browserFooterHints is the single source of the step-0 footer, shared by
// viewAdd and browserContentHeight so the measurement matches what is drawn.
func browserFooterHints(m Model) []string {
	hints := []string{
		kbd("↑/k", "up"), kbd("↓/j", "down"),
		kbd("pgup/pgdn", "page"), kbd("g/G", "top/bot"),
		kbd("space", "select"), kbd("enter", "expand/confirm"),
		kbd("h", "collapse"), kbd("/", "jump to path"),
		kbd("esc", "cancel"),
	}
	if sc := selectionCount(m.browserSelected); sc != "" {
		hints = append(hints, sc)
	}
	return hints
}

// previewContentHeight returns the number of rows available for the
// preview file list. Chrome budget:
//   - 2 box border + 1 stepper + 2 padding + 1 footer        = 6
//   - "… N more" hint when overflow                          = 1
//   - blank separator + renderPreviewCounts                  = 2
//   - errLine (2 rows) when an error is showing              = 2
//
// A hard floor of 4 keeps navigation usable on very small terminals.
// previewContentHeight returns the number of preview rows that actually fit.
// Same measured approach, and same reason, as confirmContentHeight.
func previewContentHeight(m Model) int {
	cw := contentWidth(m.width)
	innerW := cw - 4

	fixedContent := lipgloss.JoinVertical(lipgloss.Left,
		renderAddStepper(innerW, addStepPreview),
		"", "",
		"", renderPreviewCounts(m.previewPlan, m.previewToggles),
		plainFooter(innerW,
			kbd("↑/k", "up"), kbd("↓/j", "down"),
			kbd("pgup/pgdn", "page"), kbd("g/G", "top/bot"),
			kbd("enter", "confirm"), kbd("esc", "back"),
		),
	)
	fixedLines := strings.Count(boxStyle.Width(cw-2).Render(fixedContent), "\n") + 1
	if m.err != nil {
		fixedLines += 2
	}
	fixedLines++ // scroll indicator or counts separator

	h := m.height - fixedLines
	if h < 4 {
		h = 4
	}
	return h
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
			// Stepping back from confirm to preview — reset confirm
			// scroll so the next entry to this step starts at the top.
			m.confirmScroll = 0
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
					// Rebuild here, on the update goroutine. Leaving the memo
					// cold pushes the $HOME walk into View, which runs on
					// every message — and the program enables mouse cell
					// motion, so every pixel of mouse travel would re-walk
					// the tree. browserAdjustScroll also re-clamps the
					// cursor for the new list size.
					m.browserAdjustScroll()
					return m, nil
				}
				return m.browserSelectAndClassify(item.path)

			case addStepPreview:
				if m.previewPlan == nil {
					return m, nil
				}
				m.addStep = addStepConfirm
				m.confirmScroll = 0
				m.err = nil
				return m, nil

			case addStepConfirm:
				if m.previewPlan == nil {
					return m, nil
				}
				return m, runClassification(m.previewPlan, stow.CopyToggles(m.previewToggles), m.repoDir, m.homeDir)
			}

		}

		if m.addStep == addStepSelect {
			return m.browserHandleKey(keyMsg)
		}

		if m.addStep == addStepPreview {
			return m.previewHandleKey(keyMsg)
		}

		if m.addStep == addStepConfirm {
			return m.confirmHandleKey(keyMsg)
		}
	}

	return m, nil
}

// confirmHandleKey processes navigation within the confirm step's
// scrollable body. Page keys (pgup/pgdown) jump by contentHeight; g/G
// snap to top/bottom — same bindings the user already knows from the
// logs and history views.
func (m Model) confirmHandleKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.previewPlan == nil {
		return m, nil
	}
	lines := buildConfirmLines(m.previewPlan, m.previewToggles, bodyWidth(m.width))
	contentHeight := confirmContentHeight(m)
	maxScroll := len(lines) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	// Clamp the stored offset, not just the local copy the renderer uses.
	// After a resize shrinks the viewport the model kept a larger value, so
	// "down" was already at its limit and did nothing while "up" had to be
	// pressed several times before anything moved.
	if m.confirmScroll > maxScroll {
		m.confirmScroll = maxScroll
	}
	if m.confirmScroll < 0 {
		m.confirmScroll = 0
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.confirmScroll > 0 {
			m.confirmScroll--
		}
	case "down", "j":
		if m.confirmScroll < maxScroll {
			m.confirmScroll++
		}
	case "pgup", "ctrl+b":
		m.confirmScroll -= contentHeight
		if m.confirmScroll < 0 {
			m.confirmScroll = 0
		}
	case "pgdown", " ", "ctrl+f":
		m.confirmScroll += contentHeight
		if m.confirmScroll > maxScroll {
			m.confirmScroll = maxScroll
		}
	case "ctrl+u":
		m.confirmScroll -= contentHeight / 2
		if m.confirmScroll < 0 {
			m.confirmScroll = 0
		}
	case "ctrl+d":
		m.confirmScroll += contentHeight / 2
		if m.confirmScroll > maxScroll {
			m.confirmScroll = maxScroll
		}
	case "g", "home":
		m.confirmScroll = 0
	case "G", "end":
		m.confirmScroll = maxScroll
	}
	return m, nil
}

// previewHandleKey processes scroll keys for the preview step. The
// preview is a pure review screen — no per-row cursor, no toggling.
// Bindings mirror the confirm step (j/k, pgup/pgdown, g/G) and the
// half-page jumps (ctrl+d/ctrl+u) common in vim/lazygit.
func (m Model) previewHandleKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.previewPlan == nil {
		return m, nil
	}
	contentHeight := previewContentHeight(m)
	maxScroll := len(m.previewRows) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	// Clamp the stored offset too — see confirmHandleKey.
	if m.previewScroll > maxScroll {
		m.previewScroll = maxScroll
	}
	if m.previewScroll < 0 {
		m.previewScroll = 0
	}
	halfPage := contentHeight / 2
	if halfPage < 1 {
		halfPage = 1
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.previewScroll > 0 {
			m.previewScroll--
		}
	case "down", "j":
		if m.previewScroll < maxScroll {
			m.previewScroll++
		}
	case "ctrl+u":
		m.previewScroll -= halfPage
		if m.previewScroll < 0 {
			m.previewScroll = 0
		}
	case "ctrl+d":
		m.previewScroll += halfPage
		if m.previewScroll > maxScroll {
			m.previewScroll = maxScroll
		}
	case "pgup", "ctrl+b":
		m.previewScroll -= contentHeight
		if m.previewScroll < 0 {
			m.previewScroll = 0
		}
	case "pgdown", " ", "ctrl+f":
		m.previewScroll += contentHeight
		if m.previewScroll > maxScroll {
			m.previewScroll = maxScroll
		}
	case "g", "home":
		m.previewScroll = 0
	case "G", "end":
		m.previewScroll = maxScroll
	}
	return m, nil
}

func (m Model) browserHandleKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.browserJumping {
		return m.browserHandleJumpKey(keyMsg)
	}

	items := m.buildBrowserItems()
	// With no items to navigate, keep the cursor/scroll pinned at 0 so the
	// downward-motion keys below can't drive browserCursor to len-1 == -1,
	// which then cascades into a negative browserScroll (incoherent state
	// that becomes an items[-1] panic once the list is non-empty again).
	// The "/" jump key still needs to work, so handle it before bailing.
	if len(items) == 0 {
		if keyMsg.String() == "/" {
			m.browserJumping = true
			m.browserJumpInput.Placeholder = "~/.config/nvim"
			m.browserJumpInput.SetValue("")
			m.browserJumpInput.Focus()
			return m, textinput.Blink
		}
		m.browserCursor = 0
		m.browserScroll = 0
		return m, nil
	}
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

	case "pgup", "ctrl+b":
		m.browserCursor -= browserContentHeight(m)
		if m.browserCursor < 0 {
			m.browserCursor = 0
		}
		m.browserAdjustScroll()
		return m, nil

	case "pgdown", "ctrl+f":
		m.browserCursor += browserContentHeight(m)
		if m.browserCursor > len(items)-1 {
			m.browserCursor = len(items) - 1
		}
		m.browserAdjustScroll()
		return m, nil

	case "ctrl+u":
		m.browserCursor -= browserContentHeight(m) / 2
		if m.browserCursor < 0 {
			m.browserCursor = 0
		}
		m.browserAdjustScroll()
		return m, nil

	case "ctrl+d":
		m.browserCursor += browserContentHeight(m) / 2
		if m.browserCursor > len(items)-1 {
			m.browserCursor = len(items) - 1
		}
		m.browserAdjustScroll()
		return m, nil

	case "g", "home":
		m.browserCursor = 0
		m.browserAdjustScroll()
		return m, nil

	case "G", "end":
		m.browserCursor = len(items) - 1
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
				// Rebuild and re-clamp: the tree just shrank, and the cursor
				// may now point past its end.
				m.browserAdjustScroll()
				return m, nil
			}
			// Find the nearest (deepest) expanded ancestor. Collapsing it
			// removes every descendant row, so the shrink can be large.
			nearest := m.nearestExpandedAncestor(item.path)
			if nearest != "" {
				m.browserExpanded[nearest] = false
				delete(m.browserEntries, nearest)
				m.browserItems = nil
				m.browserAdjustScroll()
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

		targetPath, expandErr := expandBrowserPath(raw, m.homeDir)
		if expandErr != nil {
			m.err = expandErr
			return m, nil
		}
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
		// Reset cursor and scroll so the user lands at the top of the
		// jumped-to directory rather than at whatever index they were on
		// in the previous view.
		m.browserCursor = 0
		m.browserScroll = 0
		m.err = nil
		// Repopulate the memo so the first frame after the jump renders
		// from cache instead of walking $HOME inside View.
		m.buildBrowserItems()
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

// expandBrowserPath resolves a user-typed jump target relative to homeDir.
//
// Security: dotcor's add browser is intentionally constrained to $HOME.
// Without bounds checking, a path like "../../etc" or even an absolute
// /etc/sudoers would resolve to a real directory outside $HOME and let
// the user select system files into a public dotfiles repo.
//
// Resolution applies filepath.Clean (which normalises ".." segments)
// and then verifies the result is under homeDir. Anything outside is
// rejected with an explicit error so the UI can show "outside home".
func expandBrowserPath(raw, homeDir string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty path")
	}

	var resolved string
	switch {
	case raw == "~":
		resolved = homeDir
	case strings.HasPrefix(raw, "~/"):
		resolved = filepath.Join(homeDir, raw[2:])
	case !filepath.IsAbs(raw):
		resolved = filepath.Join(homeDir, raw)
	default:
		resolved = raw
	}

	resolved = filepath.Clean(resolved)

	// Bound check: resolved must be homeDir itself or a descendant.
	homeClean := filepath.Clean(homeDir)
	if resolved == homeClean {
		return resolved, nil
	}
	if !strings.HasPrefix(resolved, homeClean+string(filepath.Separator)) {
		return "", fmt.Errorf("outside home: %s", raw)
	}
	return resolved, nil
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
		delete(m.browserFileCounts, dirPath)
		return
	}
	m.browserSelected[dirPath] = true
	// Count once, here on the update goroutine. The renderer used to call
	// countFilesRecursive — a full WalkDir — for every selected directory on
	// every frame, and the program runs with mouse cell motion enabled, so
	// each mouse movement produced a frame. Selecting a large tree made the
	// UI stall on every pixel of mouse travel.
	if m.browserFileCounts == nil {
		m.browserFileCounts = make(map[string]int)
	}
	m.browserFileCounts[dirPath] = countFilesRecursive(dirPath)
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
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			return false
		}
		path := filepath.Join(dir, e.Name())
		lfi, err := os.Lstat(path)
		if err != nil || lfi.Mode()&os.ModeSymlink == 0 {
			return false
		}
		if ok, _ := managedSymlink(path, repoDir); !ok {
			return false
		}
	}
	return true
}

func countFilesRecursive(dir string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if browserSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
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
	// Snapshot the pattern list before it crosses the goroutine boundary.
	// The settings view edits m.cfg.IgnorePatterns in place —
	// `append(s[:i], s[i+1:]...)` shifts elements within the same backing
	// array — so a classification running concurrently would read patterns
	// as they are being overwritten. These are the rules that decide whether
	// a private key enters the repo; they must not be read mid-shuffle.
	patterns := append([]string(nil), ignorePatterns...)

	return func() tea.Msg {
		plan, err := stow.ClassifyFiles(selections, repoDir, homeDir, patterns)
		return classifyPlanMsg{plan: plan, err: err}
	}
}

func runClassification(plan *stow.ClassificationPlan, toggles map[string]bool, repoDir, homeDir string) tea.Cmd {
	return func() tea.Msg {
		result, err := stow.ExecuteClassification(plan, toggles, repoDir, homeDir)
		return classifyResultMsg{result: result, err: err}
	}
}
