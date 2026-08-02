package tui

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/justincordova/dotcor/internal/stow"
)

type View int

const (
	DashboardView View = iota
	AddView
	DiffView
	HistoryView
	HelpView
	LogsView
	SettingsView
)

// ─── Messages ────────────────────────────────────────────────────────────────

type packagesMsg struct {
	packages []stow.Package
	err      error
}

type gitStatusMsg struct {
	status git.StatusInfo
	err    error
}

type statusMsg string

type errMsg struct {
	err error
}

func (e errMsg) Error() string { return e.err.Error() }

type tickMsg time.Time

// autoCommittedMsg fires after autoCommitCmd completes (success or
// silent-fail). Carrying it through the message bus lets refreshAll be
// chained AFTER the commit so the dashboard's git status reflects the
// post-commit state, instead of racing the commit and reporting the
// pre-commit dirty state.
type autoCommittedMsg struct{}

// repoSizeMsg carries the asynchronously computed repo size (in bytes).
// Filed after packagesMsg so the dashboard renders immediately and the
// size pill updates a moment later — large repos no longer block the
// post-stow refresh.
type repoSizeMsg int64

type stowResultMsg struct {
	msg       string
	err       error
	conflicts []string
	pkgName   string
}

type syncResultMsg struct {
	msg string
	err error
}

type logsLoadedMsg struct {
	lines []string
}

type recentCommitsMsg struct {
	commits []git.CommitInfo
}

type classifyPlanMsg struct {
	plan *stow.ClassificationPlan
	err  error
}

type classifyResultMsg struct {
	result *stow.ClassificationResult
	err    error
}

// ─── Model ───────────────────────────────────────────────────────────────────

type Model struct {
	cfg          *config.Config
	version      string
	repoDir      string
	homeDir      string
	packages     []stow.Package
	selectedPkg  int
	selectedFile int
	activeView   View
	gitStatus    git.StatusInfo
	width        int
	height       int
	statusMsg    string
	err          error
	spinner      spinner.Model
	help         help.Model
	keys         keyMap
	searchInput  textinput.Model
	searching    bool
	searchQuery  string
	viewport     viewport.Model
	logs         []string
	logLevel     string
	loading      bool
	expanded     map[int]bool

	addStep int

	// Preview step (step 1) state. The preview is a scroll-only review
	// screen — no per-row cursor — so we only track the viewport offset.
	previewPlan    *stow.ClassificationPlan
	previewRows    []previewRow // cached flat row list; rebuilt when plan changes
	previewToggles map[string]bool
	previewScroll  int

	// Confirm step (step 2) scroll offset. The confirm view renders the
	// full list of files to be executed — for a large selection (e.g. an
	// entire `.config/` tree) this easily exceeds the viewport, so the
	// body paginates just like the preview step.
	confirmScroll int

	classifyResult *stow.ClassificationResult

	browserEntries  map[string][]os.DirEntry
	browserExpanded map[string]bool
	browserCursor   int
	browserScroll   int
	browserItems    []browserItem
	browserSelected map[string]bool
	// browserFileCounts caches the recursive file count for each selected
	// directory, computed once on selection so View never walks the disk.
	browserFileCounts map[string]int
	browserJumping    bool
	browserJumpInput  textinput.Model

	commits        []git.CommitInfo
	selectedCommit int
	recentCommits  []git.CommitInfo

	settingsStep          int
	settingsInput         textinput.Model
	settingsEditingIgnore bool
	settingsIgnoreIdx     int
	backups               []core.BackupInfo

	initStep int

	sortMode int

	repoSizeCached int64

	confirmOpen       bool
	confirmAction     string
	confirmTarget     string
	confirmTitle      string
	confirmBody       string
	confirmHint       string
	confirmDanger     bool
	confirmRestoreRef string
	// confirmFilePath captures the file path at the moment a restore/diff
	// dialog is opened. Without this, `restoreFromCommit` and `diffFromCommit`
	// recompute the path from m.selectedPkg/m.selectedFile/m.expanded at
	// confirm-time, which TOCTOU-races against background packagesMsg
	// arrivals that can shift indices and the expansion map (which is
	// keyed by index, not name). See ISSUES.md #7.
	confirmFilePath string
}

func NewModel(cfg *config.Config, version string) Model {
	homeDir, err := config.GetHomeDir()
	if err != nil {
		homeDir = "/"
	}
	repoDir, err := config.GetConfigDir()
	if err != nil {
		repoDir = filepath.Join(homeDir, ".dotcor")
	}
	_ = err

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colMauve))

	si := textinput.New()
	si.Placeholder = "search packages…"
	si.CharLimit = 50

	sti := textinput.New()
	sti.Placeholder = "https://github.com/…"
	sti.CharLimit = 200

	vp := viewport.New(80, 20)

	return Model{
		cfg:               cfg,
		version:           version,
		repoDir:           repoDir,
		homeDir:           homeDir,
		spinner:           sp,
		help:              newHelpModel(),
		keys:              newKeyMap(),
		searchInput:       si,
		settingsInput:     sti,
		viewport:          vp,
		expanded:          make(map[int]bool),
		logLevel:          "info",
		loading:           true,
		width:             80,
		height:            24,
		browserEntries:    make(map[string][]os.DirEntry),
		browserExpanded:   make(map[string]bool),
		browserSelected:   make(map[string]bool),
		browserFileCounts: make(map[string]int),
		browserJumpInput:  textinput.New(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		discoverPackages(m.repoDir, m.homeDir),
		fetchGitStatus(m.repoDir),
		fetchRecentCommits(m.repoDir),
	)
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Clamp to non-negative; a tiny terminal must not produce negative
		// viewport dimensions or panel widths downstream. Lipgloss treats
		// negative widths as "no clamp", which then overflows the screen.
		w := msg.Width
		h := msg.Height
		if w < 0 {
			w = 0
		}
		if h < 0 {
			h = 0
		}
		m.width = w
		m.height = h
		m.help.Width = w
		vpW := w - 4
		vpH := h - 6
		if vpW < 0 {
			vpW = 0
		}
		if vpH < 0 {
			vpH = 0
		}
		m.viewport.Width = vpW
		m.viewport.Height = vpH
		return m, nil

	case spinner.TickMsg:
		// The spinner is only rendered while loading, but spinner.Update
		// re-issues its tick unconditionally. Left unguarded that keeps a
		// full View() re-render running at spinner FPS for the entire
		// process lifetime — which, since the Add view's renderer walks the
		// home directory tree, means re-walking $HOME continuously.
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		m.statusMsg = ""
		return m, nil

	case packagesMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// The expansion state is keyed by slice index. When the package
		// list is replaced — e.g. after a delete shifts every later
		// package down one index — those indices would point at the wrong
		// packages, marking an unrelated package as expanded. Remap the
		// expansion set by package NAME across the old→new slices so
		// expansion follows the package it belonged to (and is dropped for
		// packages that no longer exist).
		expandedNames := make(map[string]bool)
		for idx, isExpanded := range m.expanded {
			if isExpanded && idx < len(m.packages) {
				expandedNames[m.packages[idx].Name] = true
			}
		}
		m.packages = msg.packages
		newExpanded := make(map[int]bool, len(expandedNames))
		for idx, pkg := range m.packages {
			if expandedNames[pkg.Name] {
				newExpanded[idx] = true
			}
		}
		m.expanded = newExpanded
		// Repo size walk is expensive on large repos and runs after every
		// stow/unstow/sync. Compute it asynchronously so the package list
		// renders immediately; the pill updates a moment later via repoSizeMsg.
		if m.selectedPkg >= len(m.packages) {
			m.selectedPkg = 0
		}
		// Clamp the file cursor too: the newly-discovered package at
		// selectedPkg may have fewer files than before, leaving
		// selectedFile dangling past the slice. A stale selectedFile
		// silently changes which file the remove/diff/history keys act on.
		if m.selectedPkg < len(m.packages) && m.selectedFile >= len(m.packages[m.selectedPkg].Files) {
			m.selectedFile = 0
		}
		return m, computeRepoSize(m.repoDir)

	case repoSizeMsg:
		m.repoSizeCached = int64(msg)
		return m, nil

	case gitStatusMsg:
		if msg.err == nil {
			m.gitStatus = msg.status
		}
		return m, nil

	case statusMsg:
		m.statusMsg = string(msg)
		m.err = nil
		cmds = append(cmds, clearStatusAfter(3*time.Second))
		cmds = append(cmds, m.refreshAll())
		return m, tea.Batch(cmds...)

	case errMsg:
		m.err = msg.err
		cmds = append(cmds, m.refreshAll())
		return m, tea.Batch(cmds...)

	case stowResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else if len(msg.conflicts) > 0 && !confirmModalIsVisible(m.activeView) {
			// Only the dashboard and history views render confirmModal.
			// Opening it from a view that neither draws nor consumes it left
			// an invisible dialog armed: the user could return to the
			// dashboard later and find a stale prompt, or press enter in a
			// view that dispatches a different action entirely. Report it as
			// a status line instead and let the user re-run the stow.
			m.statusMsg = fmt.Sprintf("%s: %d conflicts — press s on the dashboard to resolve",
				msg.pkgName, len(msg.conflicts))
			cmds = append(cmds, clearStatusAfter(5*time.Second))
		} else if len(msg.conflicts) > 0 {
			// Reset any dialog already on screen before repurposing the
			// fields, so a stale restore ref or file path can't leak into
			// this action.
			m.clearConfirm()
			m.confirmOpen = true
			m.confirmAction = "resolve-conflicts"
			m.confirmTarget = msg.pkgName
			m.confirmTitle = fmt.Sprintf("%d conflicts detected", len(msg.conflicts))
			var lines []string
			for _, c := range msg.conflicts {
				lines = append(lines, "  • "+c)
			}
			m.confirmBody = strings.Join(lines, "\n") + "\n\nBackup originals and replace with symlinks?"
			m.confirmHint = "enter confirm · esc cancel"
			m.confirmDanger = false
		} else {
			m.statusMsg = msg.msg
			m.err = nil
			cmds = append(cmds, clearStatusAfter(3*time.Second))
			cmds = append(cmds, m.autoCommitCmd("stow: "+msg.msg))
		}
		cmds = append(cmds, m.refreshAll())
		return m, tea.Batch(cmds...)

	case syncResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.statusMsg = msg.msg
			m.err = nil
			cmds = append(cmds, clearStatusAfter(3*time.Second))
		}
		cmds = append(cmds, fetchGitStatus(m.repoDir), fetchRecentCommits(m.repoDir))
		return m, tea.Batch(cmds...)

	case autoCommittedMsg:
		// Auto-commit just finished. Re-fetch git status and recent commits
		// so the dashboard reflects the post-commit state. We avoid a full
		// refreshAll here because the message that triggered the commit
		// already kicked one off — this just catches up the git-side view.
		cmds = append(cmds, fetchGitStatus(m.repoDir), fetchRecentCommits(m.repoDir))
		return m, tea.Batch(cmds...)

	case classifyPlanMsg:
		if msg.err != nil {
			m.err = msg.err
			m.addStep = addStepSelect
		} else {
			m.previewPlan = msg.plan
			m.previewRows = buildPreviewRows(msg.plan) // cache once
			m.previewToggles = stow.BuildDefaultToggles(msg.plan)
			m.previewScroll = 0
			m.addStep = addStepPreview
			m.err = nil
		}
		return m, nil

	case classifyResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.addStep = addStepConfirm
		} else {
			m.classifyResult = msg.result
			m.activeView = DashboardView
			m.err = nil
			r := msg.result
			parts := classifyResultParts(r)
			total := r.Added + r.Adopted + r.Tracked + r.Foreign
			if total > 0 {
				commitMsg := "add: " + strings.Join(parts, ", ")
				cmds = append(cmds, m.autoCommitCmd(commitMsg))
			}
			if len(parts) > 0 {
				m.statusMsg = "Added: " + strings.Join(parts, ", ")
				cmds = append(cmds, clearStatusAfter(3*time.Second))
			} else if r.Managed > 0 {
				// Zero-feedback case: every selection was already managed.
				// Without this branch the user completes the wizard and
				// lands on the dashboard with no signal anything happened.
				m.statusMsg = fmt.Sprintf("%d already managed — nothing to do", r.Managed)
				cmds = append(cmds, clearStatusAfter(3*time.Second))
			}
		}
		cmds = append(cmds, m.refreshAll())
		return m, tea.Batch(cmds...)

	case diffMsg:
		return m.updateDiff(msg)

	case historyMsg:
		return m.updateHistory(msg)

	case settingsMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			// Only now that the write succeeded does the model adopt the
			// value, so a rejected URL never shows as if it were configured.
			if msg.gitRemote != nil && m.cfg != nil {
				m.cfg.GitRemote = *msg.gitRemote
			}
			m.statusMsg = msg.msg
			m.err = nil
			cmds = append(cmds, clearStatusAfter(3*time.Second))
		}
		// refreshAll reloads packages, git status and commits — not the
		// backup list. Cleaning backups reports "Cleaned N backups" while
		// the list underneath still shows every deleted entry with its size,
		// and the only way to refresh it is to leave the pane and re-enter.
		if m.settingsStep == settingsStepBackups {
			cmds = append(cmds, m.loadBackups())
		}
		cmds = append(cmds, m.refreshAll())
		return m, tea.Batch(cmds...)

	case backupsMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.backups = msg.backups
		return m, nil

	case logsLoadedMsg:
		m.logs = msg.lines
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
		m.viewport.GotoBottom()
		return m, nil

	case recentCommitsMsg:
		m.recentCommits = msg.commits
		return m, nil
	}

	// ctrl+c must work from every view, including the ones with a focused
	// text input. Bubble Tea has no built-in handler for it, and it puts the
	// terminal in raw mode so SIGINT is never delivered either — the binding
	// was only matched in the dashboard, help and logs views, leaving the
	// Add wizard, Settings, Diff, History and the init flow with no way out.
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	if m.searching {
		return m.updateSearch(msg)
	}

	if m.initStep > 0 {
		return m.updateInit(msg)
	}

	switch m.activeView {
	case HelpView:
		return m.updateHelp(msg)
	case LogsView:
		return m.updateLogs(msg)
	case AddView:
		return m.updateAdd(msg)
	case DiffView:
		return m.updateDiff(msg)
	case HistoryView:
		return m.updateHistory(msg)
	case SettingsView:
		return m.updateSettings(msg)
	}

	return m.updateDashboard(msg)
}

// confirmModalIsVisible reports whether the given view renders confirmModal
// and routes enter/esc to confirmAccept. Keep this in sync with viewDashboard
// and viewHistory.
func confirmModalIsVisible(view View) bool {
	return view == DashboardView || view == HistoryView
}

// confirmAccept runs the pending confirmation and clears the modal.
//
// This must be shared by every view that can display the modal. The confirm
// state is set from the top-level Update — stowResultMsg opens the
// "N conflicts detected" modal regardless of which view is active — and
// viewHistory renders it, so History could show a conflict prompt while its
// own handler unconditionally ran a git restore with an empty ref and path.
// Pressing enter then produced an unrelated git error and silently dropped
// the conflict resolution.
//
// An unrecognised action is treated as a cancel rather than falling through
// to some other view's destructive default.
func (m Model) confirmAccept() (tea.Model, tea.Cmd) {
	action := m.confirmAction
	target := m.confirmTarget
	restoreRef := m.confirmRestoreRef
	restorePath := m.confirmFilePath
	m.clearConfirm()

	switch action {
	case "stow":
		return m, m.stowPackage(target)
	case "unstow":
		return m, m.unstowPackage(target)
	case "stow-all":
		return m, m.stowAllPackages()
	case "delete":
		return m, m.deletePackage(target)
	case "remove":
		return m, m.removeFileFromPackage(target, restorePath)
	case "resolve-conflicts":
		return m, m.resolveConflicts(target)
	case "adopt":
		return m, m.adoptPackage(target)
	case "restore":
		return m, m.restoreFromCommit(restoreRef, restorePath)
	}
	return m, nil
}

func (m Model) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.confirmOpen {
		switch {
		case key.Matches(keyMsg, m.keys.Enter):
			return m.confirmAccept()
		default:
			m.clearConfirm()
			return m, nil
		}
	}

	switch {
	case key.Matches(keyMsg, m.keys.Tab):
		m.sortMode = (m.sortMode + 1) % 3
		return m, nil

	case key.Matches(keyMsg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(keyMsg, m.keys.Up):
		if m.expanded[m.selectedPkg] && len(m.currentFiles()) > 0 {
			if m.selectedFile > 0 {
				m.selectedFile--
			}
		} else {
			m.selectedPkg = m.prevSortedPkg()
			m.selectedFile = 0
		}

	case key.Matches(keyMsg, m.keys.Down):
		if m.expanded[m.selectedPkg] && len(m.currentFiles()) > 0 {
			if m.selectedFile < len(m.currentFiles())-1 {
				m.selectedFile++
			}
		} else {
			m.selectedPkg = m.nextSortedPkg()
			m.selectedFile = 0
		}

	case keyMsg.String() == "pgup" || keyMsg.String() == "ctrl+b":
		if m.expanded[m.selectedPkg] && len(m.currentFiles()) > 0 {
			m.selectedFile -= 5
			if m.selectedFile < 0 {
				m.selectedFile = 0
			}
		} else {
			for i := 0; i < 5; i++ {
				m.selectedPkg = m.prevSortedPkg()
			}
			m.selectedFile = 0
		}

	case keyMsg.String() == "pgdown" || keyMsg.String() == "ctrl+f":
		if m.expanded[m.selectedPkg] && len(m.currentFiles()) > 0 {
			m.selectedFile += 5
			if m.selectedFile > len(m.currentFiles())-1 {
				m.selectedFile = len(m.currentFiles()) - 1
			}
		} else {
			for i := 0; i < 5; i++ {
				m.selectedPkg = m.nextSortedPkg()
			}
			m.selectedFile = 0
		}

	case keyMsg.String() == "ctrl+u":
		if m.expanded[m.selectedPkg] && len(m.currentFiles()) > 0 {
			m.selectedFile -= 3
			if m.selectedFile < 0 {
				m.selectedFile = 0
			}
		} else {
			for i := 0; i < 3; i++ {
				m.selectedPkg = m.prevSortedPkg()
			}
			m.selectedFile = 0
		}

	case keyMsg.String() == "ctrl+d":
		if m.expanded[m.selectedPkg] && len(m.currentFiles()) > 0 {
			m.selectedFile += 3
			if m.selectedFile > len(m.currentFiles())-1 {
				m.selectedFile = len(m.currentFiles()) - 1
			}
		} else {
			for i := 0; i < 3; i++ {
				m.selectedPkg = m.nextSortedPkg()
			}
			m.selectedFile = 0
		}

	case keyMsg.String() == "g" || keyMsg.String() == "home":
		if m.expanded[m.selectedPkg] && len(m.currentFiles()) > 0 {
			m.selectedFile = 0
		} else {
			indices := sortedPackages(m)
			if len(indices) > 0 {
				m.selectedPkg = indices[0]
			}
			m.selectedFile = 0
		}

	case keyMsg.String() == "G" || keyMsg.String() == "end":
		if m.expanded[m.selectedPkg] && len(m.currentFiles()) > 0 {
			m.selectedFile = len(m.currentFiles()) - 1
		} else {
			indices := sortedPackages(m)
			if len(indices) > 0 {
				m.selectedPkg = indices[len(indices)-1]
			}
			m.selectedFile = 0
		}

	case key.Matches(keyMsg, m.keys.Enter):
		m.expanded[m.selectedPkg] = !m.expanded[m.selectedPkg]
		m.selectedFile = 0

	case key.Matches(keyMsg, m.keys.Stow):
		m.clearErr()
		if m.selectedPkg < len(m.packages) {
			pkg := m.packages[m.selectedPkg]
			var linked, toLink, conflicts, foreign int
			for _, f := range pkg.Files {
				if f.IsLinked {
					linked++
				} else if f.Exists && f.IsSymlink {
					foreign++
				} else if f.Exists {
					conflicts++
				} else {
					toLink++
				}
			}
			m.confirmOpen = true
			m.confirmAction = "stow"
			m.confirmTarget = pkg.Name
			m.confirmTitle = fmt.Sprintf("Stow %s?", pkg.Name)
			parts := []string{fmt.Sprintf("%d to link", toLink)}
			if foreign > 0 {
				parts = append(parts, fmt.Sprintf("%d foreign", foreign))
			}
			if conflicts > 0 {
				parts = append(parts, fmt.Sprintf("%d conflict", conflicts))
			}
			parts = append(parts, fmt.Sprintf("%d linked", linked))
			m.confirmBody = strings.Join(parts, " · ")
			m.confirmHint = "enter confirm · any key cancel"
			m.confirmDanger = false
		}
		return m, nil

	case key.Matches(keyMsg, m.keys.Unstow):
		m.clearErr()
		if m.selectedPkg < len(m.packages) {
			pkg := m.packages[m.selectedPkg]
			linked, _ := countLinked(pkg.Files)
			m.confirmOpen = true
			m.confirmAction = "unstow"
			m.confirmTarget = pkg.Name
			m.confirmTitle = fmt.Sprintf("Unstow %s?", pkg.Name)
			m.confirmBody = fmt.Sprintf("%d symlinks to remove", linked)
			m.confirmHint = "enter confirm · any key cancel"
			m.confirmDanger = false
		}
		return m, nil

	case key.Matches(keyMsg, m.keys.Adopt):
		m.clearErr()
		if m.selectedPkg < len(m.packages) {
			pkg := m.packages[m.selectedPkg]
			var adoptable []string
			for _, f := range pkg.Files {
				if !f.InRepo && f.IsSymlink && !f.IsLinked {
					adoptable = append(adoptable, f.RelPath)
				}
			}
			if len(adoptable) == 0 {
				m.statusMsg = "No foreign symlinks to adopt"
				return m, clearStatusAfter(3 * time.Second)
			}
			m.confirmOpen = true
			m.confirmAction = "adopt"
			m.confirmTarget = pkg.Name
			m.confirmTitle = fmt.Sprintf("Adopt %d foreign symlinks into %s?", len(adoptable), pkg.Name)
			var lines []string
			for _, a := range adoptable {
				lines = append(lines, "  • "+a)
			}
			m.confirmBody = strings.Join(lines, "\n")
			m.confirmHint = "enter confirm · any key cancel"
			m.confirmDanger = false
		}
		return m, nil

	case key.Matches(keyMsg, m.keys.StowAll):
		m.clearErr()
		var count, totalFiles int
		for _, pkg := range m.packages {
			if pkg.Status != stow.StatusLinked {
				count++
				totalFiles += len(pkg.Files)
			}
		}
		if count > 0 {
			m.confirmOpen = true
			m.confirmAction = "stow-all"
			m.confirmTarget = fmt.Sprintf("%d unlinked packages", count)
			m.confirmTitle = "Stow all?"
			m.confirmBody = fmt.Sprintf("%d packages, %d total files", count, totalFiles)
			m.confirmHint = "enter confirm · any key cancel"
			m.confirmDanger = false
		} else {
			m.statusMsg = "All packages already stowed"
			return m, clearStatusAfter(3 * time.Second)
		}
		return m, nil

	case key.Matches(keyMsg, m.keys.Remove):
		m.clearErr()
		if m.selectedPkg >= len(m.packages) {
			return m, nil
		}
		if m.expanded[m.selectedPkg] && m.selectedFile < len(m.packages[m.selectedPkg].Files) {
			f := m.packages[m.selectedPkg].Files[m.selectedFile]
			m.confirmOpen = true
			m.confirmAction = "remove"
			// Capture package name and file rel-path at dialog-open time so
			// the confirmed action resolves the exact same target even if a
			// background packagesMsg shifts m.selectedPkg/m.selectedFile
			// before Enter is pressed.
			m.confirmTarget = m.packages[m.selectedPkg].Name
			m.confirmFilePath = f.RelPath
			m.confirmTitle = fmt.Sprintf("Remove %s?", f.RelPath)
			m.confirmBody = fmt.Sprintf("Removes from package %s.\nFile stays on disk.", m.packages[m.selectedPkg].Name)
			m.confirmHint = "enter confirm · any key cancel"
			m.confirmDanger = false
		} else {
			pkg := m.packages[m.selectedPkg]
			m.confirmOpen = true
			m.confirmAction = "delete"
			m.confirmTarget = pkg.Name
			m.confirmTitle = fmt.Sprintf("Delete %s?", pkg.Name)
			m.confirmBody = fmt.Sprintf("Permanently removes %d tracked files.\nThis cannot be undone.", len(pkg.Files))
			m.confirmHint = "enter confirm · any key cancel"
			m.confirmDanger = true
		}
		return m, nil

	case key.Matches(keyMsg, m.keys.Sync):
		m.clearErr()
		return m, m.syncRepo()

	case key.Matches(keyMsg, m.keys.Init):
		m.clearErr()
		if git.IsRepo(m.repoDir) {
			m.statusMsg = "git already initialized"
			// Top up the ignore entries on an existing repository too. One
			// created before these patterns existed would otherwise keep
			// sweeping backups/ — verbatim copies of files such as ~/.ssh
			// material — into every commit and push.
			repoDir := m.repoDir
			return m, tea.Batch(
				func() tea.Msg {
					if err := git.EnsureIgnorePatterns(repoDir); err != nil {
						return settingsMsg{err: fmt.Errorf("updating .gitignore: %w", err)}
					}
					return nil
				},
				clearStatusAfter(3*time.Second),
			)
		}
		m.initStep = 1
		return m, nil

	case key.Matches(keyMsg, m.keys.Help):
		m.clearErr()
		m.activeView = HelpView
		m.help.ShowAll = true

	case key.Matches(keyMsg, m.keys.Logs):
		m.clearErr()
		m.activeView = LogsView
		return m, loadLogs(m.logLevel)

	case key.Matches(keyMsg, m.keys.Search):
		m.searching = true
		m.searchInput.Focus()
		return m, textinput.Blink

	case key.Matches(keyMsg, m.keys.Push):
		m.clearErr()
		return m, m.pushRepo()

	case key.Matches(keyMsg, m.keys.Pull):
		m.clearErr()
		return m, m.pullRepo()

	case key.Matches(keyMsg, m.keys.Add):
		m.clearErr()
		m.activeView = AddView
		// Full reset, not a partial one. classifyResultMsg returns to the
		// dashboard WITHOUT calling resetAddState, so a stale previewPlan,
		// preview/confirm scroll offsets, and classifyResult can survive
		// into the next Add session. resetAddState clears all of that
		// (and the browser state) so each Add starts clean.
		m.resetAddState()
		// Warm the browser memo here, on the update goroutine, so the first
		// frame renders from cache instead of walking $HOME inside View.
		m.buildBrowserItems()
		return m, nil

	case key.Matches(keyMsg, m.keys.Diff):
		m.clearErr()
		if !git.IsRepo(m.repoDir) {
			m.err = fmt.Errorf("git not initialized — press i to set up git")
			return m, nil
		}
		m.activeView = DiffView
		return m, getDiff(m)

	case key.Matches(keyMsg, m.keys.History):
		m.clearErr()
		if !git.IsRepo(m.repoDir) {
			m.err = fmt.Errorf("git not initialized — press i to set up git")
			return m, nil
		}
		m.activeView = HistoryView
		m.commits = nil
		m.selectedCommit = 0
		return m, getFileHistory(m)

	case key.Matches(keyMsg, m.keys.Settings):
		m.clearErr()
		m.activeView = SettingsView
		m.settingsStep = settingsStepMain
	}

	return m, nil
}

func (m Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.searching = false
			m.searchInput.Blur()
			m.searchQuery = ""
			m.searchInput.SetValue("")
			return m, nil
		case "enter":
			m.searching = false
			m.searchInput.Blur()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.searchQuery = m.searchInput.Value()
	for i, pkg := range m.packages {
		if fuzzyMatch(m.searchQuery, pkg.Name) {
			m.selectedPkg = i
			m.selectedFile = 0
			break
		}
	}
	return m, cmd
}

func (m Model) updateInit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.initStep {
	case 1:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(keyMsg, m.keys.Enter):
				// git init forks a subprocess with a 30s ceiling; running it
				// inline froze input and rendering for its whole duration.
				m.initStep = 2
				m.settingsInput.SetValue("")
				m.settingsInput.Focus()
				return m, tea.Batch(m.initRepo(), textinput.Blink)
			case key.Matches(keyMsg, m.keys.Esc):
				m.initStep = 0
				return m, nil
			}
		}
		return m, nil

	case 2:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(keyMsg, m.keys.Enter):
				url := strings.TrimSpace(m.settingsInput.Value())
				m.settingsInput.Blur()
				m.initStep = 0
				if url == "" {
					m.statusMsg = "git initialized"
					return m, tea.Batch(m.refreshAll(), clearStatusAfter(3*time.Second))
				}
				// Validate inline (pure string work), then hand the git and
				// config writes to a command. SetRemote forks up to two
				// subprocesses at 30s each — the same reason the settings
				// flow moved this work off the update loop.
				if err := git.ValidateRemoteURL(url); err != nil {
					m.err = fmt.Errorf("invalid remote URL: %w", err)
					return m, nil
				}
				m.statusMsg = "git initialized + remote configured"
				return m, tea.Batch(
					m.applyGitRemote(url),
					m.refreshAll(),
					clearStatusAfter(3*time.Second),
				)
			case key.Matches(keyMsg, m.keys.Esc):
				m.settingsInput.Blur()
				m.initStep = 0
				cmds := []tea.Cmd{m.refreshAll(), clearStatusAfter(3 * time.Second)}
				m.statusMsg = "git initialized"
				return m, tea.Batch(cmds...)
			}
		}
		var cmd tea.Cmd
		m.settingsInput, cmd = m.settingsInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Esc),
			key.Matches(keyMsg, m.keys.Help),
			key.Matches(keyMsg, m.keys.Quit):
			m.activeView = DashboardView
			m.help.ShowAll = false
		}
	}
	return m, nil
}

func (m Model) updateLogs(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Esc),
			key.Matches(keyMsg, m.keys.Logs),
			key.Matches(keyMsg, m.keys.Quit):
			m.activeView = DashboardView
			return m, nil
		}
		switch keyMsg.String() {
		case "1":
			m.logLevel = "debug"
			return m, loadLogs(m.logLevel)
		case "2":
			m.logLevel = "info"
			return m, loadLogs(m.logLevel)
		case "3":
			m.logLevel = "warn"
			return m, loadLogs(m.logLevel)
		case "4":
			m.logLevel = "error"
			return m, loadLogs(m.logLevel)
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// ─── View ────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			fmt.Sprintf("%s %s",
				m.spinner.View(),
				textStyle.Render("Loading packages…"),
			),
		)
	}

	switch m.activeView {
	case HelpView:
		return viewHelp(m)
	case LogsView:
		return viewLogs(m)
	case AddView:
		return viewAdd(m)
	case DiffView:
		return viewDiff(m)
	case HistoryView:
		return viewHistory(m)
	case SettingsView:
		return viewSettings(m)
	default:
		return viewDashboard(m)
	}
}

// ─── State helpers ───────────────────────────────────────────────────────────

func (m *Model) clearErr() { m.err = nil }
func (m *Model) clearConfirm() {
	m.confirmOpen = false
	m.confirmAction = ""
	m.confirmTarget = ""
	m.confirmTitle = ""
	m.confirmBody = ""
	m.confirmHint = ""
	m.confirmDanger = false
	m.confirmRestoreRef = ""
	m.confirmFilePath = ""
}

func classifyResultParts(r *stow.ClassificationResult) []string {
	var parts []string
	if r.Adopted > 0 {
		parts = append(parts, fmt.Sprintf("%d adopted", r.Adopted))
	}
	if r.Added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", r.Added))
	}
	if r.Tracked > 0 {
		parts = append(parts, fmt.Sprintf("%d tracked", r.Tracked))
	}
	if r.Foreign > 0 {
		parts = append(parts, fmt.Sprintf("%d foreign", r.Foreign))
	}
	if len(r.Failures) > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", len(r.Failures)))
	}
	return parts
}

func countLinked(files []stow.FileEntry) (linked, total int) {
	total = len(files)
	for _, f := range files {
		if f.IsLinked {
			linked++
		}
	}
	return
}
func (m Model) currentFiles() []stow.FileEntry {
	if m.selectedPkg >= len(m.packages) {
		return nil
	}
	return m.packages[m.selectedPkg].Files
}

// packageByName resolves a package by its stable name. Destructive and
// mutating actions resolve their target this way at execute-time rather
// than re-reading m.selectedPkg, which can point at a different package
// after a background packagesMsg replaces m.packages between the confirm
// dialog opening and the user pressing Enter. Returns nil if no package
// with that name exists (e.g. it was deleted out from under us).
func (m Model) packageByName(name string) *stow.Package {
	for i := range m.packages {
		if m.packages[i].Name == name {
			return &m.packages[i]
		}
	}
	return nil
}

func (m Model) sortedPkgPos() int {
	indices := sortedPackages(m)
	for i, idx := range indices {
		if idx == m.selectedPkg {
			return i
		}
	}
	return 0
}

func (m Model) prevSortedPkg() int {
	indices := sortedPackages(m)
	pos := m.sortedPkgPos()
	if pos > 0 {
		return indices[pos-1]
	}
	return m.selectedPkg
}

func (m Model) nextSortedPkg() int {
	indices := sortedPackages(m)
	pos := m.sortedPkgPos()
	if pos < len(indices)-1 {
		return indices[pos+1]
	}
	return m.selectedPkg
}

func (m *Model) resetAddState() {
	m.addStep = addStepSelect
	m.browserExpanded = make(map[string]bool)
	m.browserCursor = 0
	m.browserScroll = 0
	m.browserEntries = make(map[string][]os.DirEntry)
	m.browserItems = nil
	m.browserSelected = make(map[string]bool)
	m.browserFileCounts = make(map[string]int)
	m.browserJumping = false
	m.browserJumpInput.SetValue("")
	m.browserJumpInput.Blur()
	m.previewPlan = nil
	m.previewRows = nil
	m.previewToggles = nil
	m.previewScroll = 0
	m.confirmScroll = 0
	m.classifyResult = nil
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) autoCommitCmd(message string) tea.Cmd {
	repoDir := m.repoDir
	var logger *slog.Logger
	if m.cfg != nil {
		logger = m.cfg.Logger
	}
	return func() tea.Msg {
		if !git.IsRepo(repoDir) {
			return autoCommittedMsg{}
		}
		if err := git.AutoCommit(repoDir, message, logger); err != nil && logger != nil {
			logger.Warn("auto-commit failed", "error", err)
		}
		return autoCommittedMsg{}
	}
}

// ─── Commands ────────────────────────────────────────────────────────────────

func discoverPackages(repoDir, homeDir string) tea.Cmd {
	return func() tea.Msg {
		packages, err := stow.DiscoverPackages(repoDir, homeDir)
		return packagesMsg{packages: packages, err: err}
	}
}

func computeRepoSize(repoDir string) tea.Cmd {
	return func() tea.Msg {
		return repoSizeMsg(repoSizeBytes(repoDir))
	}
}

func fetchGitStatus(repoDir string) tea.Cmd {
	return func() tea.Msg {
		if !git.IsRepo(repoDir) {
			return gitStatusMsg{}
		}
		status, err := git.GetStatus(repoDir)
		return gitStatusMsg{status: status, err: err}
	}
}

// initRepo runs `git init` off the update loop and reports the outcome.
func (m Model) initRepo() tea.Cmd {
	repoDir := m.repoDir
	return func() tea.Msg {
		if err := git.InitRepo(repoDir); err != nil {
			return settingsMsg{err: err}
		}
		return settingsMsg{msg: "git initialized"}
	}
}

func (m Model) refreshAll() tea.Cmd {
	return tea.Batch(
		discoverPackages(m.repoDir, m.homeDir),
		fetchGitStatus(m.repoDir),
		fetchRecentCommits(m.repoDir),
	)
}

func fetchRecentCommits(repoDir string) tea.Cmd {
	return func() tea.Msg {
		if !git.IsRepo(repoDir) {
			return recentCommitsMsg{}
		}
		commits, err := git.GetFileHistory(repoDir, ".", 6)
		if err != nil {
			return recentCommitsMsg{}
		}
		return recentCommitsMsg{commits: commits}
	}
}

func (m Model) stowPackage(name string) tea.Cmd {
	pkg := m.packageByName(name)
	if pkg == nil {
		return func() tea.Msg {
			return stowResultMsg{err: fmt.Errorf("package %q no longer exists", name)}
		}
	}
	repoDir, homeDir := m.repoDir, m.homeDir
	return func() tea.Msg {
		result, err := stow.Link(repoDir, homeDir, pkg.Name)
		if err != nil {
			return stowResultMsg{err: err}
		}
		if len(result.Conflicts) > 0 {
			return stowResultMsg{
				conflicts: result.Conflicts,
				pkgName:   pkg.Name,
			}
		}
		msg := fmt.Sprintf("Stowed %s (%d linked", pkg.Name, result.Linked)
		if result.Skipped > 0 {
			msg += fmt.Sprintf(", %d skipped", result.Skipped)
		}
		if n := len(result.Foreign); n > 0 {
			msg += fmt.Sprintf(", %d foreign — use 'o' to adopt", n)
		}
		msg += ")"
		return stowResultMsg{msg: msg}
	}
}

func (m Model) resolveConflicts(pkgName string) tea.Cmd {
	repoDir := m.repoDir
	homeDir := m.homeDir
	backupDir := filepath.Join(repoDir, "backups")
	return func() tea.Msg {
		result, err := stow.LinkWithBackup(repoDir, homeDir, pkgName, backupDir)
		if err != nil {
			return stowResultMsg{err: err}
		}

		// RestoreFailures means the symlink swap failed AND the rollback to
		// the original file also failed: the file may be missing from $HOME
		// with only the backup to recover from. That is the single most
		// dangerous outcome this command can produce, so it is reported as
		// an error rather than folded into a status line.
		if n := len(result.RestoreFailures); n > 0 {
			return stowResultMsg{err: fmt.Errorf(
				"%d file(s) may be missing from $HOME — recover from the backup dir: %s",
				n, strings.Join(result.RestoreFailures, ", "))}
		}

		// Report Resolved, not Linked. Linked also counts files that were
		// already linked cleanly by the preceding Link pass, so using it
		// overstated how many conflicts were actually dealt with.
		msg := fmt.Sprintf("Resolved %d conflicts in %s", result.Resolved, pkgName)
		if n := len(result.Conflicts); n > 0 {
			msg += fmt.Sprintf(" (%d unresolved)", n)
		}
		if n := len(result.Foreign); n > 0 {
			msg += fmt.Sprintf(", %d foreign — use 'o' to adopt", n)
		}
		return stowResultMsg{msg: msg}
	}
}

func (m Model) adoptPackage(name string) tea.Cmd {
	pkg := m.packageByName(name)
	if pkg == nil {
		return func() tea.Msg {
			return stowResultMsg{err: fmt.Errorf("package %q no longer exists", name)}
		}
	}
	repoDir, homeDir := m.repoDir, m.homeDir
	return func() tea.Msg {
		result, err := stow.Adopt(repoDir, homeDir, pkg.Name)
		if err != nil {
			return stowResultMsg{err: err}
		}
		msg := fmt.Sprintf("Adopted %s (%d reparented", pkg.Name, result.Adopted)
		if result.Skipped > 0 {
			msg += fmt.Sprintf(", %d skipped", result.Skipped)
		}
		if len(result.Failures) > 0 {
			msg += fmt.Sprintf(", %d failed", len(result.Failures))
		}
		msg += ")"
		return stowResultMsg{msg: msg}
	}
}

func (m Model) unstowPackage(name string) tea.Cmd {
	pkg := m.packageByName(name)
	if pkg == nil {
		return func() tea.Msg {
			return stowResultMsg{err: fmt.Errorf("package %q no longer exists", name)}
		}
	}
	repoDir, homeDir := m.repoDir, m.homeDir
	return func() tea.Msg {
		result, err := stow.Unlink(repoDir, homeDir, pkg.Name)
		if err != nil {
			return stowResultMsg{err: err}
		}
		return stowResultMsg{msg: fmt.Sprintf("Unstowed %s (%d unlinked)", pkg.Name, result.Unlinked)}
	}
}

func (m Model) stowAllPackages() tea.Cmd {
	repoDir := m.repoDir
	homeDir := m.homeDir
	packages := m.packages
	logger := m.cfg.Logger

	return func() tea.Msg {
		var toStow []stow.Package
		for _, pkg := range packages {
			if pkg.Status != stow.StatusLinked {
				toStow = append(toStow, pkg)
			}
		}
		if len(toStow) == 0 {
			return stowResultMsg{msg: "All packages already stowed"}
		}

		var totalLinked, totalSkipped, totalForeign int
		var stowedNames []string
		for _, pkg := range toStow {
			result, err := stow.Link(repoDir, homeDir, pkg.Name)
			if err != nil {
				if logger != nil {
					logger.Warn("stow all: failed to stow package", "name", pkg.Name, "error", err)
				}
				continue
			}
			totalLinked += result.Linked
			totalSkipped += result.Skipped
			totalForeign += len(result.Foreign)
			stowedNames = append(stowedNames, pkg.Name)
		}

		msg := fmt.Sprintf("Stowed %d packages (%d linked", len(stowedNames), totalLinked)
		if totalSkipped > 0 {
			msg += fmt.Sprintf(", %d skipped", totalSkipped)
		}
		if totalForeign > 0 {
			msg += fmt.Sprintf(", %d foreign — use 'o' to adopt", totalForeign)
		}
		msg += ")"
		return stowResultMsg{msg: msg}
	}
}
func (m Model) deletePackage(name string) tea.Cmd {
	pkg := m.packageByName(name)
	if pkg == nil {
		return func() tea.Msg {
			return stowResultMsg{err: fmt.Errorf("package %q no longer exists", name)}
		}
	}
	repoDir := m.repoDir
	homeDir := m.homeDir
	backupDir := filepath.Join(repoDir, "backups")
	logger := m.cfg.Logger

	return func() tea.Msg {
		result, err := stow.Unlink(repoDir, homeDir, pkg.Name)
		if err != nil {
			return stowResultMsg{err: fmt.Errorf("unstow before delete failed: %w", err)}
		}

		pkgDir := filepath.Join(repoDir, pkg.Name)
		ts := time.Now().Format("2006-01-02_15-04-05")
		backupPath := filepath.Join(backupDir, "pre-delete-"+ts, pkg.Name)
		// 0700: this copy can hold anything the package tracked, including
		// ~/.ssh and ~/.gnupg contents. It must not be more exposed than the
		// originals.
		if err := os.MkdirAll(filepath.Dir(backupPath), 0700); err != nil {
			return stowResultMsg{err: fmt.Errorf("creating backup directory: %w", err)}
		}
		if err := copyDir(pkgDir, backupPath); err != nil {
			// Clean partial backup so the user isn't left with a misleading recovery path.
			_ = os.RemoveAll(backupPath)
			return stowResultMsg{err: fmt.Errorf("backing up package before delete: %w", err)}
		}

		if err := os.RemoveAll(pkgDir); err != nil {
			return stowResultMsg{err: fmt.Errorf("removing package directory: %w", err)}
		}
		if logger != nil {
			logger.Info("deleted package", "name", pkg.Name, "unlinked", result.Unlinked, "backup", backupPath)
		}
		return stowResultMsg{msg: fmt.Sprintf("Deleted %s (%d unlinked, backup: %s)", pkg.Name, result.Unlinked, backupPath)}
	}
}

// copyDir walks src and recreates its tree under dst. Symlinks are
// preserved as symlinks (not followed) so a package containing internal
// references like `nvim/after/ftplugin -> ../ftplugin` round-trips
// faithfully through backup/restore.
//
// Errors are propagated. The previous implementation swallowed
// per-file ReadFile/Symlink/Mkdir failures and continued the walk —
// fine for a best-effort copy, but unsafe for the delete-package code
// path that calls copyDir as the BACKUP step before os.RemoveAll. A
// silent skip there meant unreadable files were destroyed with no
// backup. Now any failure aborts the walk so the caller can refuse to
// proceed with the destructive step. Broken symlinks (Readlink returns
// ENOENT on the target's path-resolution, not on the link itself) are
// the only thing genuinely worth skipping; that branch is preserved.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walking %s: %w", path, walkErr)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("rel %s: %w", path, err)
		}
		target := filepath.Join(dst, rel)

		// Branch on symlink FIRST. info from Lstat reports the symlink
		// itself, not the target — Mode().IsDir() is false even for
		// dir-targeting symlinks, so the IsDir branch wouldn't catch
		// them either way. We explicitly preserve the link regardless.
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", path, err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir for %s: %w", target, err)
			}
			_ = os.Remove(target) // in case a previous run left something
			if err := os.Symlink(linkTarget, target); err != nil {
				return fmt.Errorf("symlink %s: %w", target, err)
			}
			return nil
		}

		if info.IsDir() {
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		return nil
	})
}

func (m Model) removeFileFromPackage(name, relPath string) tea.Cmd {
	pkg := m.packageByName(name)
	if pkg == nil {
		return func() tea.Msg {
			return stowResultMsg{err: fmt.Errorf("package %q no longer exists", name)}
		}
	}
	var file stow.FileEntry
	found := false
	for i := range pkg.Files {
		if pkg.Files[i].RelPath == relPath {
			file = pkg.Files[i]
			found = true
			break
		}
	}
	if !found {
		return func() tea.Msg {
			return stowResultMsg{err: fmt.Errorf("%q is no longer in package %q", relPath, name)}
		}
	}
	repoDir := m.repoDir
	pkgName := pkg.Name
	logger := m.cfg.Logger

	return func() tea.Msg {
		pkgDir := filepath.Join(repoDir, pkgName)
		repoFilePath := filepath.Join(pkgDir, file.RelPath)

		if file.IsLinked {
			// Atomic restore: write to a temp file in the same directory,
			// then rename over the symlink. POSIX rename is atomic on the
			// same filesystem, so the user never sees a missing $HOME file
			// even if disk is full mid-write — the symlink stays intact
			// until the tmp file is fully written.
			data, err := os.ReadFile(repoFilePath)
			if err != nil {
				return stowResultMsg{err: fmt.Errorf("reading repo file: %w", err)}
			}
			repoInfo, statErr := os.Stat(repoFilePath)
			perm := os.FileMode(0644)
			if statErr == nil {
				perm = repoInfo.Mode().Perm()
			}

			tmp := file.TargetPath + ".dotcor-restore"
			_ = os.Remove(tmp)
			if err := os.WriteFile(tmp, data, perm); err != nil {
				return stowResultMsg{err: fmt.Errorf("staging restore file: %w", err)}
			}
			if err := os.Rename(tmp, file.TargetPath); err != nil {
				_ = os.Remove(tmp)
				return stowResultMsg{err: fmt.Errorf("placing restored file: %w", err)}
			}
		}

		if err := os.Remove(repoFilePath); err != nil {
			return stowResultMsg{err: fmt.Errorf("removing from repo: %w", err)}
		}

		// Walk up and remove now-empty parent directories so the repo
		// doesn't accumulate hollow `.config/.../` chains after removing
		// the last file in a deeply nested package. Stop at pkgDir.
		dir := filepath.Dir(repoFilePath)
		for dir != pkgDir && dir != "/" && dir != "." {
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) > 0 {
				break
			}
			if rmErr := os.Remove(dir); rmErr != nil {
				break
			}
			dir = filepath.Dir(dir)
		}

		if logger != nil {
			logger.Info("removed file from package", "file", file.RelPath, "package", pkgName)
		}
		return stowResultMsg{msg: fmt.Sprintf("Removed %s from %s", file.RelPath, pkgName)}
	}
}

func (m Model) syncRepo() tea.Cmd {
	repoDir := m.repoDir
	logger := m.cfg.Logger
	return func() tea.Msg {
		result, err := git.SyncDetailed(repoDir, logger)
		if err != nil {
			return syncResultMsg{err: err}
		}
		var parts []string
		if result.Committed {
			parts = append(parts, fmt.Sprintf("%d files changed", result.FilesChanged))
		}
		if result.Pushed {
			parts = append(parts, "pushed to "+result.Branch)
		}
		if len(parts) == 0 {
			return syncResultMsg{msg: "Nothing to sync — repo is clean"}
		}
		return syncResultMsg{msg: "Synced: " + strings.Join(parts, ", ")}
	}
}

func (m Model) pushRepo() tea.Cmd {
	repoDir := m.repoDir
	if !git.IsRepo(repoDir) {
		return func() tea.Msg {
			return syncResultMsg{err: fmt.Errorf("git not initialized — press i to set up git")}
		}
	}
	return func() tea.Msg {
		// Inside the command, not outside it. GetRemoteURL carries a 30s
		// timeout and this ran on the update loop, freezing input and
		// rendering for its whole duration on a slow or contended repo.
		remoteURL, err := git.GetRemoteURL(repoDir)
		if err != nil {
			return syncResultMsg{err: err}
		}
		if remoteURL == "" {
			return syncResultMsg{err: fmt.Errorf("no remote configured — press i to set up git")}
		}
		if err := git.PushWithProgress(repoDir); err != nil {
			return syncResultMsg{err: err}
		}
		return syncResultMsg{msg: "Pushed"}
	}
}

func (m Model) pullRepo() tea.Cmd {
	repoDir := m.repoDir
	if !git.IsRepo(repoDir) {
		return func() tea.Msg {
			return syncResultMsg{err: fmt.Errorf("git not initialized — press i to set up git")}
		}
	}
	return func() tea.Msg {
		if err := git.Pull(repoDir); err != nil {
			return syncResultMsg{err: err}
		}
		return syncResultMsg{msg: "Pulled"}
	}
}

// ─── Logs ────────────────────────────────────────────────────────────────────

// logsTailBytes caps how much of the end of the log file loadLogs reads.
// The log rotates at ~10 MB (see logger package), so 2 MB of tail is
// enough to show several thousand entries without loading the whole file
// into memory on slow disks.
const logsTailBytes int64 = 2 * 1024 * 1024

func loadLogs(level string) tea.Cmd {
	return func() tea.Msg {
		configDir, _ := config.GetConfigDir()
		logPath := filepath.Join(configDir, "logs", "dotcor.log")

		f, err := os.Open(logPath)
		if err != nil {
			return logsLoadedMsg{lines: []string{"no logs found — run some commands first"}}
		}
		defer func() { _ = f.Close() }()

		// Seek to the last logsTailBytes so we stream only the tail.
		// Reading the whole file each time the user opens the log view
		// is wasteful on a 10 MB+ log, and the older lines are rarely
		// what the user wants anyway.
		info, statErr := f.Stat()
		if statErr == nil && info.Size() > logsTailBytes {
			if _, seekErr := f.Seek(-logsTailBytes, io.SeekEnd); seekErr != nil {
				// Fall back to full read on seek failure (e.g. pipe).
				_, _ = f.Seek(0, io.SeekStart)
			} else {
				// Discard the partial first line since a mid-line seek
				// almost certainly landed in the middle of a record.
				br := bufio.NewReader(f)
				if _, err := br.ReadString('\n'); err == nil {
					return logsLoadedMsg{lines: readFilteredLogs(br, level)}
				}
			}
		}

		return logsLoadedMsg{lines: readFilteredLogs(bufio.NewReader(f), level)}
	}
}

// readFilteredLogs reads lines from r, keeps the ones matching level,
// and returns at most the last 1000 entries. Uses bufio.Scanner with a
// larger buffer so unusually long log lines don't truncate.
func readFilteredLogs(r io.Reader, level string) []string {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var filtered []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if matchesLevel(line, level) {
			filtered = append(filtered, line)
		}
	}

	if len(filtered) == 0 {
		return []string{"no log entries at level: " + level}
	}
	if len(filtered) > 1000 {
		filtered = filtered[len(filtered)-1000:]
	}
	return filtered
}

// lineLevelRank returns the severity rank of a log line based on the
// charmlog level token it carries (DEBU/INFO/WARN/ERRO — the abbreviated
// forms charmlog writes to the file handler). Lines with no recognizable
// token rank as debug (0) so they are only hidden by the debug filter,
// never dropped from a broader view.
//
// The tokens are matched space-delimited (" WARN ") to avoid a message
// body containing the word "WARN" being misclassified as a warning line.
func lineLevelRank(line string) int {
	switch {
	case strings.Contains(line, " ERRO"):
		return 3
	case strings.Contains(line, " WARN"):
		return 2
	case strings.Contains(line, " INFO"):
		return 1
	default:
		return 0
	}
}

// matchesLevel reports whether a log line should be shown at the selected
// minimum level. Higher-or-equal severity lines pass, matching the usual
// "show this level and above" semantics of a level filter.
//
// charmlog's file handler renders levels as uppercase abbreviations
// (DEBU/INFO/WARN/ERRO), NOT the logfmt `level=warn` form the previous
// implementation looked for — so the warn and error filters matched
// nothing and always rendered an empty log view.
func matchesLevel(line, level string) bool {
	rank := lineLevelRank(line)
	switch level {
	case "debug":
		return true
	case "info":
		return rank >= 1
	case "warn":
		return rank >= 2
	case "error":
		return rank >= 3
	default:
		return true
	}
}
