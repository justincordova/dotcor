package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
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
