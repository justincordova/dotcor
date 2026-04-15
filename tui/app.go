package tui

import (
	"fmt"
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

	addInput      textinput.Model
	addStep       int
	addPkgName    string
	addPreview    string
	addSecrets    []string
	addPkgChoices []string
	addPkgIdx     int
	addPkgEditing bool

	browserEntries  map[string][]os.DirEntry
	browserExpanded map[string]bool
	browserCursor   int
	browserScroll   int
	browserItems    []browserItem
	browserSelected map[string]bool

	commits        []git.CommitInfo
	selectedCommit int
	recentCommits  []git.CommitInfo

	settingsStep  int
	settingsInput textinput.Model
	backups       []core.BackupInfo

	initStep int

	confirmOpen   bool
	confirmAction string
	confirmTarget string
	confirmTitle  string
	confirmBody   string
	confirmHint   string
	confirmDanger bool
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

	ai := textinput.New()
	ai.Placeholder = "~/.config/app/config"
	ai.CharLimit = 200

	sti := textinput.New()
	sti.Placeholder = "https://github.com/…"
	sti.CharLimit = 200

	vp := viewport.New(80, 20)

	return Model{
		cfg:             cfg,
		version:         version,
		repoDir:         repoDir,
		homeDir:         homeDir,
		spinner:         sp,
		help:            newHelpModel(),
		keys:            newKeyMap(),
		searchInput:     si,
		addInput:        ai,
		settingsInput:   sti,
		viewport:        vp,
		expanded:        make(map[int]bool),
		logLevel:        "info",
		loading:         true,
		width:           80,
		height:          24,
		browserEntries:  make(map[string][]os.DirEntry),
		browserExpanded: make(map[string]bool),
		browserSelected: make(map[string]bool),
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
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 6
		return m, nil

	case spinner.TickMsg:
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
		m.packages = msg.packages
		if m.selectedPkg >= len(m.packages) {
			m.selectedPkg = 0
		}
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
		return m, tea.Batch(cmds...)

	case errMsg:
		m.err = msg.err
		return m, nil

	case stowResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else if len(msg.conflicts) > 0 {
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

	case addResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.statusMsg = msg.msg
			m.err = nil
			m.resetAddState()
			m.activeView = DashboardView
			cmds = append(cmds, clearStatusAfter(3*time.Second))
			cmds = append(cmds, m.autoCommitCmd("add: "+msg.msg))
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
			m.statusMsg = msg.msg
			m.err = nil
			cmds = append(cmds, clearStatusAfter(3*time.Second))
		}
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

func (m Model) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.confirmOpen {
		switch {
		case key.Matches(keyMsg, m.keys.Enter):
			action := m.confirmAction
			target := m.confirmTarget
			m.clearConfirm()
			switch action {
			case "stow":
				return m, m.stowPackage()
			case "unstow":
				return m, m.unstowPackage()
			case "stow-all":
				return m, m.stowAllPackages()
			case "delete":
				return m, m.deletePackage()
			case "remove":
				return m, m.removeFileFromPackage()
			case "resolve-conflicts":
				return m, m.resolveConflicts(target)
			}
		default:
			m.clearConfirm()
			return m, nil
		}
	}

	switch {
	case key.Matches(keyMsg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(keyMsg, m.keys.Up):
		if m.expanded[m.selectedPkg] && len(m.currentFiles()) > 0 {
			if m.selectedFile > 0 {
				m.selectedFile--
			}
		} else if m.selectedPkg > 0 {
			m.selectedPkg--
			m.selectedFile = 0
		}

	case key.Matches(keyMsg, m.keys.Down):
		if m.expanded[m.selectedPkg] && len(m.currentFiles()) > 0 {
			if m.selectedFile < len(m.currentFiles())-1 {
				m.selectedFile++
			}
		} else if m.selectedPkg < len(m.packages)-1 {
			m.selectedPkg++
			m.selectedFile = 0
		}

	case key.Matches(keyMsg, m.keys.Enter):
		m.expanded[m.selectedPkg] = !m.expanded[m.selectedPkg]
		m.selectedFile = 0

	case key.Matches(keyMsg, m.keys.Stow):
		m.clearErr()
		if m.selectedPkg < len(m.packages) {
			pkg := m.packages[m.selectedPkg]
			linked, total := countLinked(pkg.Files)
			m.confirmOpen = true
			m.confirmAction = "stow"
			m.confirmTarget = pkg.Name
			m.confirmTitle = fmt.Sprintf("Stow %s?", pkg.Name)
			m.confirmBody = fmt.Sprintf("%d files to link, %d already linked", total-linked, linked)
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

	case key.Matches(keyMsg, m.keys.Delete):
		m.clearErr()
		if m.selectedPkg < len(m.packages) {
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

	case key.Matches(keyMsg, m.keys.Remove):
		m.clearErr()
		if m.selectedPkg < len(m.packages) && m.expanded[m.selectedPkg] && m.selectedFile < len(m.packages[m.selectedPkg].Files) {
			f := m.packages[m.selectedPkg].Files[m.selectedFile]
			m.confirmOpen = true
			m.confirmAction = "remove"
			m.confirmTarget = fmt.Sprintf("%s from %s", f.RelPath, m.packages[m.selectedPkg].Name)
			m.confirmTitle = fmt.Sprintf("Remove %s?", f.RelPath)
			m.confirmBody = fmt.Sprintf("Removes from package %s.\nFile stays on disk.", m.packages[m.selectedPkg].Name)
			m.confirmHint = "enter confirm · any key cancel"
			m.confirmDanger = false
		}
		return m, nil

	case key.Matches(keyMsg, m.keys.Sync):
		m.clearErr()
		return m, m.syncRepo()

	case key.Matches(keyMsg, m.keys.Init):
		m.clearErr()
		if git.IsRepo(m.repoDir) {
			m.statusMsg = "git already initialized"
			return m, clearStatusAfter(3 * time.Second)
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
		m.addStep = 0
		m.addInput.SetValue("")
		m.browserExpanded = make(map[string]bool)
		m.browserCursor = 0
		m.browserScroll = 0
		m.browserEntries = make(map[string][]os.DirEntry)
		m.browserItems = nil
		m.browserSelected = make(map[string]bool)
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
		m.settingsStep = 0
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
			m.searchQuery = m.searchInput.Value()
			for i, pkg := range m.packages {
				if fuzzyMatch(m.searchQuery, pkg.Name) {
					m.selectedPkg = i
					m.selectedFile = 0
					break
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m Model) updateInit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.initStep {
	case 1:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(keyMsg, m.keys.Enter):
				if err := git.InitRepo(m.repoDir); err != nil {
					m.err = err
					m.initStep = 0
					return m, nil
				}
				m.initStep = 2
				m.settingsInput.SetValue("")
				m.settingsInput.Focus()
				return m, textinput.Blink
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
				url := m.settingsInput.Value()
				m.settingsInput.Blur()
				m.initStep = 0
				if url != "" {
					if err := git.SetRemote(m.repoDir, "origin", url); err != nil {
						m.err = err
						return m, nil
					}
				}
				cmds := []tea.Cmd{
					m.refreshAll(),
					clearStatusAfter(3 * time.Second),
				}
				if url != "" {
					m.statusMsg = "git initialized + remote configured"
				} else {
					m.statusMsg = "git initialized"
				}
				return m, tea.Batch(cmds...)
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

func (m *Model) resetAddState() {
	m.addStep = 0
	m.addInput.SetValue("")
	m.addInput.Blur()
	m.addPkgName = ""
	m.addPreview = ""
	m.addSecrets = nil
	m.addPkgChoices = nil
	m.addPkgIdx = 0
	m.addPkgEditing = false
	m.browserExpanded = make(map[string]bool)
	m.browserCursor = 0
	m.browserScroll = 0
	m.browserEntries = make(map[string][]os.DirEntry)
	m.browserItems = nil
	m.browserSelected = make(map[string]bool)
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
			return nil
		}
		if err := git.AutoCommit(repoDir, message, logger); err != nil && logger != nil {
			logger.Warn("auto-commit failed", "error", err)
		}
		return nil
	}
}

// ─── Commands ────────────────────────────────────────────────────────────────

func discoverPackages(repoDir, homeDir string) tea.Cmd {
	return func() tea.Msg {
		packages, err := stow.DiscoverPackages(repoDir, homeDir)
		return packagesMsg{packages: packages, err: err}
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

func (m Model) stowPackage() tea.Cmd {
	if m.selectedPkg >= len(m.packages) {
		return nil
	}
	pkg := m.packages[m.selectedPkg]
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
		msg := fmt.Sprintf("Resolved %d conflicts in %s", result.Linked, pkgName)
		return stowResultMsg{msg: msg}
	}
}

func (m Model) unstowPackage() tea.Cmd {
	if m.selectedPkg >= len(m.packages) {
		return nil
	}
	pkg := m.packages[m.selectedPkg]
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

		var totalLinked, totalSkipped int
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
			stowedNames = append(stowedNames, pkg.Name)
		}

		msg := fmt.Sprintf("Stowed %d packages (%d linked", len(stowedNames), totalLinked)
		if totalSkipped > 0 {
			msg += fmt.Sprintf(", %d skipped", totalSkipped)
		}
		msg += ")"
		return stowResultMsg{msg: msg}
	}
}
func (m Model) deletePackage() tea.Cmd {
	if m.selectedPkg >= len(m.packages) {
		return nil
	}
	pkg := m.packages[m.selectedPkg]
	repoDir := m.repoDir
	homeDir := m.homeDir
	logger := m.cfg.Logger
	return func() tea.Msg {
		result, err := stow.Unlink(repoDir, homeDir, pkg.Name)
		if err != nil {
			return stowResultMsg{err: fmt.Errorf("unstow before delete failed: %w", err)}
		}
		pkgDir := filepath.Join(repoDir, pkg.Name)
		if err := os.RemoveAll(pkgDir); err != nil {
			return stowResultMsg{err: fmt.Errorf("removing package directory: %w", err)}
		}
		if logger != nil {
			logger.Info("deleted package", "name", pkg.Name, "unlinked", result.Unlinked)
		}
		return stowResultMsg{msg: fmt.Sprintf("Deleted %s (%d unlinked, dir removed)", pkg.Name, result.Unlinked)}
	}
}
func (m Model) removeFileFromPackage() tea.Cmd {
	if m.selectedPkg >= len(m.packages) {
		return nil
	}
	pkg := m.packages[m.selectedPkg]
	if !m.expanded[m.selectedPkg] || m.selectedFile >= len(pkg.Files) {
		return nil
	}
	file := pkg.Files[m.selectedFile]
	repoDir := m.repoDir
	pkgName := pkg.Name
	logger := m.cfg.Logger

	return func() tea.Msg {
		pkgDir := filepath.Join(repoDir, pkgName)
		repoFilePath := filepath.Join(pkgDir, file.RelPath)

		if file.IsLinked {
			data, err := os.ReadFile(repoFilePath)
			if err != nil {
				return stowResultMsg{err: fmt.Errorf("reading repo file: %w", err)}
			}
			if err := os.Remove(file.TargetPath); err != nil {
				return stowResultMsg{err: fmt.Errorf("removing symlink: %w", err)}
			}
			if err := os.WriteFile(file.TargetPath, data, 0644); err != nil {
				return stowResultMsg{err: fmt.Errorf("restoring file: %w", err)}
			}
		}

		if err := os.Remove(repoFilePath); err != nil {
			return stowResultMsg{err: fmt.Errorf("removing from repo: %w", err)}
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
		if err := git.Sync(repoDir, logger); err != nil {
			return syncResultMsg{err: err}
		}
		return syncResultMsg{msg: "Synced"}
	}
}

func (m Model) pushRepo() tea.Cmd {
	repoDir := m.repoDir
	if !git.IsRepo(repoDir) {
		return func() tea.Msg {
			return syncResultMsg{err: fmt.Errorf("git not initialized — press i to set up git")}
		}
	}
	remoteURL, _ := git.GetRemoteURL(repoDir)
	if remoteURL == "" {
		return func() tea.Msg {
			return syncResultMsg{err: fmt.Errorf("no remote configured — press i to set up git")}
		}
	}
	return func() tea.Msg {
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

func loadLogs(level string) tea.Cmd {
	return func() tea.Msg {
		configDir, _ := config.GetConfigDir()
		logPath := filepath.Join(configDir, "logs", "dotcor.log")
		data, err := os.ReadFile(logPath)
		if err != nil {
			return logsLoadedMsg{lines: []string{"no logs found — run some commands first"}}
		}

		var filtered []string
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			if matchesLevel(line, level) {
				filtered = append(filtered, line)
			}
		}

		if len(filtered) == 0 {
			filtered = []string{"no log entries at level: " + level}
		}
		if len(filtered) > 1000 {
			filtered = filtered[len(filtered)-1000:]
		}
		return logsLoadedMsg{lines: filtered}
	}
}

func matchesLevel(line, level string) bool {
	switch level {
	case "debug":
		return true
	case "info":
		return !strings.Contains(line, "level=debug")
	case "warn":
		return strings.Contains(line, "level=warn") || strings.Contains(line, "level=error")
	case "error":
		return strings.Contains(line, "level=error")
	default:
		return true
	}
}
