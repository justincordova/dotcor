package tui

import (
	"fmt"
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

func (e errMsg) Error() string {
	return e.err.Error()
}

type tickMsg time.Time

type stowResultMsg struct {
	msg string
	err error
}

type syncResultMsg struct {
	msg string
	err error
}

type Model struct {
	cfg          *config.Config
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
	showHelp     bool
	searchInput  textinput.Model
	searching    bool
	searchQuery  string
	viewport     viewport.Model
	logs         []string
	logLevel     string
	loading      bool
	expanded     map[int]bool

	addInput   textinput.Model
	addStep    int
	addPkgName string
	addPreview string
	addSecrets []string

	commits        []git.CommitInfo
	selectedCommit int

	settingsStep  int
	settingsInput textinput.Model
	backups       []core.BackupInfo
}

func NewModel(cfg *config.Config) Model {
	homeDir, _ := os.UserHomeDir()
	repoDir := homeDir

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(iris))

	si := textinput.New()
	si.Placeholder = "search packages..."
	si.CharLimit = 50

	ai := textinput.New()
	ai.Placeholder = "~/.config/app/config"
	ai.CharLimit = 200

	sti := textinput.New()
	sti.Placeholder = "https://github.com/..."
	sti.CharLimit = 200

	vp := viewport.New(80, 20)

	keys := newKeyMap()

	return Model{
		cfg:           cfg,
		repoDir:       repoDir,
		homeDir:       homeDir,
		spinner:       sp,
		help:          newHelpModel(),
		keys:          keys,
		searchInput:   si,
		addInput:      ai,
		settingsInput: sti,
		viewport:      vp,
		expanded:      make(map[int]bool),
		logLevel:      "info",
		loading:       true,
		width:         80,
		height:        24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		discoverPackages(m.repoDir, m.homeDir),
		fetchGitStatus(m.repoDir),
	)
}

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
		if m.statusMsg != "" {
			m.statusMsg = ""
		}
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
		cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))
		return m, tea.Batch(cmds...)

	case errMsg:
		m.err = msg.err
		return m, nil

	case stowResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.statusMsg = msg.msg
			cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return tickMsg(t)
			}))
		}
		cmds = append(cmds, m.refreshAll())
		return m, tea.Batch(cmds...)

	case syncResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.statusMsg = msg.msg
			cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return tickMsg(t)
			}))
		}
		cmds = append(cmds, fetchGitStatus(m.repoDir))
		return m, tea.Batch(cmds...)

	case addResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.statusMsg = msg.msg
			m.addStep = 0
			m.addInput.SetValue("")
			m.addInput.Blur()
			m.addPkgName = ""
			m.addPreview = ""
			m.addSecrets = nil
			m.activeView = DashboardView
			cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return tickMsg(t)
			}))
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
			cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return tickMsg(t)
			}))
		}
		return m, tea.Batch(cmds...)

	case backupsMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.backups = msg.backups
		return m, nil
	}

	if m.searching {
		return m.updateSearch(msg)
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
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Up):
			if m.selectedPkg > 0 {
				m.selectedPkg--
				m.selectedFile = 0
			}

		case key.Matches(msg, m.keys.Down):
			if m.selectedPkg < len(m.packages)-1 {
				m.selectedPkg++
				m.selectedFile = 0
			}

		case key.Matches(msg, m.keys.Enter):
			m.expanded[m.selectedPkg] = !m.expanded[m.selectedPkg]

		case key.Matches(msg, m.keys.Stow):
			return m, m.stowPackage()

		case key.Matches(msg, m.keys.Unstow):
			return m, m.unstowPackage()

		case key.Matches(msg, m.keys.Sync):
			return m, m.syncRepo()

		case key.Matches(msg, m.keys.Help):
			m.activeView = HelpView
			m.help.ShowAll = true

		case key.Matches(msg, m.keys.Logs):
			m.activeView = LogsView
			m.loadLogs()
			m.viewport.SetContent(strings.Join(m.logs, "\n"))
			m.viewport.GotoBottom()

		case key.Matches(msg, m.keys.Search):
			m.searching = true
			m.searchInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, m.keys.Push):
			return m, m.pushRepo()

		case key.Matches(msg, m.keys.Pull):
			return m, m.pullRepo()

		case key.Matches(msg, m.keys.Add):
			m.activeView = AddView
			m.addStep = 0
			m.addInput.SetValue("")
			m.addInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, m.keys.Diff):
			m.activeView = DiffView
			return m, getDiff(m)

		case key.Matches(msg, m.keys.History):
			m.activeView = HistoryView
			m.commits = nil
			m.selectedCommit = 0
			return m, getFileHistory(m)

		case key.Matches(msg, m.keys.Settings):
			m.activeView = SettingsView
			m.settingsStep = 0
			return m, nil
		}
	}

	return m, nil
}

func (m Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
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
				if strings.Contains(strings.ToLower(pkg.Name), strings.ToLower(m.searchQuery)) {
					m.selectedPkg = i
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

func (m Model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Esc), key.Matches(msg, m.keys.Help),
			key.Matches(msg, m.keys.Quit):
			m.activeView = DashboardView
			m.help.ShowAll = false
			return m, nil
		}
	}
	return m, nil
}

func (m Model) updateLogs(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Esc), key.Matches(msg, m.keys.Logs),
			key.Matches(msg, m.keys.Quit):
			m.activeView = DashboardView
			return m, nil
		case msg.String() == "1":
			m.logLevel = "debug"
		case msg.String() == "2":
			m.logLevel = "info"
		case msg.String() == "3":
			m.logLevel = "warn"
		case msg.String() == "4":
			m.logLevel = "error"
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		m.loadLogs()
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s Loading packages...\n\n", m.spinner.View())
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
	)
}

func (m Model) stowPackage() tea.Cmd {
	if m.selectedPkg >= len(m.packages) {
		return nil
	}
	pkg := m.packages[m.selectedPkg]
	repoDir := m.repoDir
	homeDir := m.homeDir
	return func() tea.Msg {
		result, err := stow.Link(repoDir, homeDir, pkg.Name)
		if err != nil {
			return stowResultMsg{err: err}
		}
		msg := fmt.Sprintf("Stowed %s: %d linked", pkg.Name, result.Linked)
		if result.Skipped > 0 {
			msg += fmt.Sprintf(", %d skipped", result.Skipped)
		}
		return stowResultMsg{msg: msg}
	}
}

func (m Model) unstowPackage() tea.Cmd {
	if m.selectedPkg >= len(m.packages) {
		return nil
	}
	pkg := m.packages[m.selectedPkg]
	repoDir := m.repoDir
	homeDir := m.homeDir
	return func() tea.Msg {
		result, err := stow.Unlink(repoDir, homeDir, pkg.Name)
		if err != nil {
			return stowResultMsg{err: err}
		}
		msg := fmt.Sprintf("Unstowed %s: %d unlinked", pkg.Name, result.Unlinked)
		return stowResultMsg{msg: msg}
	}
}

func (m Model) syncRepo() tea.Cmd {
	repoDir := m.repoDir
	logger := m.cfg.Logger
	return func() tea.Msg {
		err := git.Sync(repoDir, logger)
		if err != nil {
			return syncResultMsg{err: err}
		}
		return syncResultMsg{msg: "Synced"}
	}
}

func (m Model) pushRepo() tea.Cmd {
	repoDir := m.repoDir
	return func() tea.Msg {
		err := git.PushWithProgress(repoDir)
		if err != nil {
			return syncResultMsg{err: err}
		}
		return syncResultMsg{msg: "Pushed"}
	}
}

func (m Model) pullRepo() tea.Cmd {
	repoDir := m.repoDir
	return func() tea.Msg {
		err := git.Pull(repoDir)
		if err != nil {
			return stowResultMsg{err: err}
		}
		return stowResultMsg{msg: "Pulled"}
	}
}

func (m *Model) loadLogs() {
	home, err := os.UserHomeDir()
	if err != nil {
		m.logs = []string{"error: cannot determine home directory"}
		return
	}

	logPath := filepath.Join(home, ".dotcor", "logs", "dotcor.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		m.logs = []string{"no logs found"}
		return
	}

	lines := strings.Split(string(data), "\n")
	var filtered []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		if matchesLevel(line, m.logLevel) {
			filtered = append(filtered, line)
		}
	}

	if len(filtered) == 0 {
		filtered = []string{"no log entries matching level: " + m.logLevel}
	}

	if len(filtered) > 1000 {
		filtered = filtered[len(filtered)-1000:]
	}

	m.logs = filtered
}

func matchesLevel(line, level string) bool {
	switch level {
	case "debug":
		return true
	case "info":
		return !strings.Contains(line, "level=debug")
	case "warn":
		return strings.Contains(line, "level=warn") ||
			strings.Contains(line, "level=error")
	case "error":
		return strings.Contains(line, "level=error")
	default:
		return true
	}
}
