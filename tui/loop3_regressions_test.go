package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincordova/dotcor/internal/stow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfirmStep_NoRowHiddenAtAnyScroll pins the fix for a file silently
// removed from the approval list.
//
// The renderer emits TWO sticky bottom rows once the list overflows (the
// scroll counter and the execute hint) but the height model counted one, so
// the dialog was always one line too tall and clampDialogHeight deleted the
// third visible body row. At any non-zero scroll that is a file row — and the
// confirm step is the last gate before files are moved out of $HOME.
func TestConfirmStep_NoRowHiddenAtAnyScroll(t *testing.T) {
	for _, height := range []int{24, 30, 40, 50} {
		for _, scroll := range []int{0, 1, 5, 20, 100} {
			t.Run(fmt.Sprintf("h=%d/scroll=%d", height, scroll), func(t *testing.T) {
				m := addModelWithPlan(200, height)
				m.addStep = addStepConfirm
				m.confirmScroll = scroll

				lines := buildConfirmLines(m.previewPlan, m.previewToggles, bodyWidth(m.width))
				ch := confirmContentHeight(m)
				maxScroll := len(lines) - ch
				if scroll > maxScroll {
					t.Skip("scroll beyond the end for this height")
				}

				body := stripANSI(m.View())
				// Every row the viewport claims to show must actually appear.
				for i := scroll; i < scroll+ch && i < len(lines); i++ {
					want := strings.TrimSpace(stripANSI(lines[i]))
					if want == "" {
						continue
					}
					assert.Contains(t, body, want,
						"row %d is inside the viewport but was dropped from the render", i)
				}
			})
		}
	}
}

// TestBrowserMemo_WarmAfterExpandAndJump pins the fix for the memo being left
// cold. View would then re-walk $HOME on a value copy whose memo is thrown
// away — and the program enables mouse cell motion, so every pixel of mouse
// travel produced a frame.
func TestBrowserMemo_WarmAfterExpandAndJump(t *testing.T) {
	m := browserModel(t, 30, 40)

	// Put the cursor on a directory and expand it.
	items := m.buildBrowserItems()
	dirIdx := -1
	for i, item := range items {
		if item.isDir {
			dirIdx = i
			break
		}
	}
	if dirIdx < 0 {
		t.Skip("no directory in the fixture")
	}
	m.browserCursor = dirIdx

	updated, _ := m.updateAdd(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, updated.(Model).browserItems,
		"expanding a directory must leave the browser memo populated")
}

// TestStowConflicts_NotArmedDuringInitWizard pins the fix for enter running
// the wrong action. Update routes keys to updateInit before the view switch,
// so the dashboard renders the modal while the wizard consumes enter — and
// pressing it ran `git init` instead of resolving the conflicts.
func TestStowConflicts_NotArmedDuringInitWizard(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = DashboardView
	m.initStep = 1

	updated, _ := m.Update(stowResultMsg{pkgName: "zsh", conflicts: []string{".zshrc"}})
	asModel := updated.(Model)

	assert.False(t, asModel.confirmOpen,
		"no modal may be armed while the init wizard owns the keyboard")
	assert.Contains(t, asModel.statusMsg, "conflicts")
}

// TestStowConflicts_NotArmedWhileSearching is the same shape for the search box.
func TestStowConflicts_NotArmedWhileSearching(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = DashboardView
	m.searching = true

	updated, _ := m.Update(stowResultMsg{pkgName: "zsh", conflicts: []string{".zshrc"}})

	assert.False(t, updated.(Model).confirmOpen)
}

// TestDiffMsg_DoesNotClobberLogsView pins the fix for a shared viewport. A
// slow `git diff` landing after the user navigated away painted a colourised
// diff into the logs view, with the log header and line count still shown.
func TestDiffMsg_DoesNotClobberLogsView(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = LogsView
	m.logs = []string{"LOGLINE-ONE", "LOGLINE-TWO"}
	m.viewport.SetContent(strings.Join(m.logs, "\n"))

	updated, _ := m.Update(diffMsg{content: "DIFFCONTENT"})
	body := stripANSI(updated.(Model).View())

	assert.Contains(t, body, "LOGLINE-ONE", "the logs view must keep its content")
	assert.NotContains(t, body, "DIFFCONTENT", "a late diff must not paint into the logs view")
}

// TestLogsMsg_DoesNotClobberDiffView is the symmetric case.
func TestLogsMsg_DoesNotClobberDiffView(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = DiffView
	m.viewport.SetContent("DIFFCONTENT")

	updated, _ := m.Update(logsLoadedMsg{lines: []string{"LOGLINE-ONE"}})
	body := stripANSI(updated.(Model).View())

	assert.Contains(t, body, "DIFFCONTENT", "the diff view must keep its content")
	assert.NotContains(t, body, "LOGLINE-ONE")
}

// TestBrowserContentHeight_MatchesWhatIsRendered pins the shared measurement.
func TestBrowserContentHeight_MatchesWhatIsRendered(t *testing.T) {
	for _, height := range []int{24, 30, 40, 60} {
		t.Run(fmt.Sprintf("height=%d", height), func(t *testing.T) {
			m := browserModel(t, 300, height)

			body := stripANSI(m.View())
			rows := 0
			for _, line := range strings.Split(body, "\n") {
				if strings.Contains(line, "○ f") {
					rows++
				}
			}

			assert.Equal(t, browserContentHeight(m), rows,
				"the scroll maths must use the number of rows actually drawn")
		})
	}
}

// TestSettingsMsg_ReportsConfigSaveFailure pins the fix for a success message
// covering a failed write. The error set when SaveConfig failed was cleared
// two lines later, so the user saw a green "Remote saved" while .git/config
// held the new remote and .dotcorrc still held the old one.
func TestSettingsMsg_ReportsConfigSaveFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}

	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
	t.Setenv("DOTCOR_DIR", dir)

	m := NewModel(testCfg(), "test")
	m.loading = false

	saved := "https://github.com/new/repo.git"
	updated, _ := m.Update(settingsMsg{msg: "Remote saved", gitRemote: &saved})
	asModel := updated.(Model)

	if _, err := os.Stat(filepath.Join(dir, ".dotcorrc")); err == nil {
		t.Skip("filesystem allowed the write; the failure path was not exercised")
	}

	require.Error(t, asModel.err, "a failed config write must be reported")
	assert.Contains(t, asModel.err.Error(), "saving config")
	assert.Empty(t, asModel.statusMsg, "a failed write must not also claim success")
}

// TestClassifyResult_DoesNotYankUserOutOfAnotherView pins the guard added for
// diffMsg and logsLoadedMsg but missed here. ExecuteClassification takes
// seconds on a large tree and the confirm step shows no in-flight indicator,
// so the user may well have navigated away.
func TestClassifyResult_DoesNotYankUserOutOfAnotherView(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = SettingsView

	updated, _ := m.Update(classifyResultMsg{result: &stow.ClassificationResult{Added: 2}})

	assert.Equal(t, SettingsView, updated.(Model).activeView,
		"a late classification result must not change the active view")
}

// TestClassifyResult_ReturnsToDashboardFromAddView keeps the normal flow.
func TestClassifyResult_ReturnsToDashboardFromAddView(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = AddView

	updated, _ := m.Update(classifyResultMsg{result: &stow.ClassificationResult{Added: 2}})

	assert.Equal(t, DashboardView, updated.(Model).activeView)
}

// TestConfirmStep_SecondEnterDoesNotStartConcurrentRun pins the in-flight
// guard. Two concurrent ExecuteClassification runs over the same plan would
// race on the same *.dotcor-tmp staging paths and on each other's symlink
// swaps.
func TestConfirmStep_SecondEnterDoesNotStartConcurrentRun(t *testing.T) {
	m := addModelWithPlan(5, 40)
	m.addStep = addStepConfirm
	m.repoDir = t.TempDir()

	first, cmd1 := m.updateAdd(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd1, "the first enter must start the run")
	assert.True(t, first.(Model).classifying)

	_, cmd2 := first.(Model).updateAdd(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd2, "a second enter must not start a concurrent run")

	// The flag clears when the result lands, so a later Add works.
	done, _ := first.(Model).Update(classifyResultMsg{result: &stow.ClassificationResult{}})
	assert.False(t, done.(Model).classifying)
}
