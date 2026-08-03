package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincordova/dotcor/internal/stow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func samplePlan() *stow.ClassificationPlan {
	return &stow.ClassificationPlan{
		Packages: []stow.PackagePlan{{Name: "pkg", Files: []stow.ClassifiedFile{{RelPath: "a"}}}},
	}
}

// TestAddSession_StaleResultDoesNotEjectNewSession pins the fix for a guard
// that checked the view instead of the session.
//
// Escaping out of the wizard while ExecuteClassification is still running and
// then opening a fresh Add puts the user back in AddView, so the abandoned
// run's result still applied — throwing them straight back to the dashboard
// and discarding the new session's browse state.
func TestAddSession_StaleResultDoesNotEjectNewSession(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = AddView
	m.previewPlan = samplePlan()
	m.addStep = addStepConfirm
	m.repoDir = t.TempDir()

	// Execution starts for session N.
	started, _ := m.updateAdd(tea.KeyMsg{Type: tea.KeyEnter})
	running := started.(Model)
	staleSession := running.addSession

	// User escapes all the way out, then opens a fresh Add.
	running.resetAddState()
	running.activeView = AddView

	updated, _ := running.Update(classifyResultMsg{
		session: staleSession,
		result:  &stow.ClassificationResult{Added: 3},
	})

	assert.Equal(t, AddView, updated.(Model).activeView,
		"an abandoned run's result must not eject the new session")
}

// TestAddSession_StalePlanDoesNotRewindWizard pins the double-enter case.
// Pressing enter twice on the select step produced two plans; the later one
// rewound the wizard from confirm back to preview, rebuilding the toggles and
// resetting the scroll mid-review.
func TestAddSession_StalePlanDoesNotRewindWizard(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = AddView
	m.addStep = addStepConfirm
	m.addSession = 7

	updated, _ := m.Update(classifyPlanMsg{session: 6, plan: samplePlan()})

	assert.Equal(t, addStepConfirm, updated.(Model).addStep,
		"a superseded plan must not rewind the wizard")
	assert.Nil(t, updated.(Model).previewPlan, "a superseded plan must not be adopted")
}

// TestAddSession_CurrentPlanIsApplied keeps the normal flow working.
func TestAddSession_CurrentPlanIsApplied(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = AddView
	m.addStep = addStepSelect
	m.addSession = 7

	updated, _ := m.Update(classifyPlanMsg{session: 7, plan: samplePlan()})

	assert.Equal(t, addStepPreview, updated.(Model).addStep)
	assert.NotNil(t, updated.(Model).previewPlan)
}

// TestAddSession_EachDispatchStartsANewSession pins the invalidation point.
func TestAddSession_EachDispatchStartsANewSession(t *testing.T) {
	m := jumpModel(t)
	m.browserSelected[m.homeDir+"/b.conf"] = true

	first, cmd1 := m.confirmBrowserSelectionAndClassify()
	require.NotNil(t, cmd1)
	second, cmd2 := first.(Model).confirmBrowserSelectionAndClassify()
	require.NotNil(t, cmd2)

	assert.Greater(t, second.(Model).addSession, first.(Model).addSession,
		"each dispatch must supersede the previous one")
}

// TestAddView_RendersStatusMessage pins the fix for a report written to a
// field the view never drew. The conflict fallback — raised precisely because
// the Add view cannot host the confirm modal — was invisible and then wiped
// by its own timer.
func TestAddView_RendersStatusMessage(t *testing.T) {
	m := jumpModel(t)
	m.statusMsg = "nvim: 2 conflicts — press s on the dashboard to resolve"

	body := stripANSI(m.View())

	assert.Contains(t, body, "2 conflicts",
		"the Add view must surface status messages, not just errors")
}

// TestStowConflicts_ReachTheUserFromAddView is the end-to-end path.
func TestStowConflicts_ReachTheUserFromAddView(t *testing.T) {
	m := jumpModel(t)
	m.loading = false

	updated, _ := m.Update(stowResultMsg{pkgName: "nvim", conflicts: []string{"a", "b"}})
	body := stripANSI(updated.(Model).View())

	assert.Contains(t, body, "conflicts", "the user must be told the stow did nothing")
}

// TestAddSession_GateReleasedOnlyByItsOwner pins the fix for a gate handed
// away while another execution was still running.
//
// classifyingRun is what stops a second enter starting a concurrent
// ExecuteClassification. Two runs share the same *.dotcor-tmp staging paths,
// and an interleaved rollback can delete the repo copy the other run's $HOME
// symlink already points at — destroying the file's only copy. Releasing on
// any arriving result let an abandoned run open the gate for a live one.
func TestAddSession_GateReleasedOnlyByItsOwner(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = AddView
	m.addStep = addStepConfirm
	m.repoDir = t.TempDir()
	m.addSession = 5

	// Run 2 currently holds the gate; run 1 was abandoned earlier.
	m.classifyRunSeq = 2
	m.classifyingRun = 2

	stale, _ := m.Update(classifyResultMsg{session: 4, run: 1, result: &stow.ClassificationResult{Added: 1}})
	assert.Equal(t, 2, stale.(Model).classifyingRun,
		"an abandoned run must not release a gate held by a live execution")

	owner, _ := stale.(Model).Update(classifyResultMsg{session: 5, run: 2, result: &stow.ClassificationResult{Added: 1}})
	assert.Zero(t, owner.(Model).classifyingRun,
		"the owning run's result must release the gate")
}

// TestAddSession_ConfirmEnterBlockedWhileRunning is the safety half.
func TestAddSession_ConfirmEnterBlockedWhileRunning(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = AddView
	m.addStep = addStepConfirm
	m.previewPlan = samplePlan()
	m.repoDir = t.TempDir()

	started, cmd := m.updateAdd(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	running := started.(Model)
	require.NotZero(t, running.classifyingRun)

	_, second := running.updateAdd(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, second, "a second enter must not start a concurrent execution")

	// Abandoning the wizard must NOT release the gate — the run is still
	// touching the filesystem.
	running.resetAddState()
	assert.NotZero(t, running.classifyingRun,
		"abandoning the wizard must not hand the gate to the next session")
}

// TestAddSession_ConfirmEnterWorksAfterOwningResult confirms the step is
// usable again once the run that held the gate reports back.
func TestAddSession_ConfirmEnterWorksAfterOwningResult(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = AddView
	m.addStep = addStepConfirm
	m.previewPlan = samplePlan()
	m.repoDir = t.TempDir()

	started, _ := m.updateAdd(tea.KeyMsg{Type: tea.KeyEnter})
	running := started.(Model)

	done, _ := running.Update(classifyResultMsg{
		session: running.addSession,
		run:     running.classifyingRun,
		result:  &stow.ClassificationResult{},
	})
	ready := done.(Model)
	require.Zero(t, ready.classifyingRun)

	ready.addStep = addStepConfirm
	ready.previewPlan = samplePlan()
	_, cmd := ready.updateAdd(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd, "the confirm step must be usable again")
}

// TestAddNoticeLines_BudgetedByEveryContentHeight pins the fix for rows
// silently deleted off the top of every Add step.
//
// viewAdd emits a two-line notice for statusMsg as well as err, but only err
// was budgeted — so the dialog overflowed and clampDialogHeight removed the
// first two body rows: the browser's first entries (with the cursor sitting
// invisibly on one), or files from the confirm approval list.
func TestAddNoticeLines_BudgetedByEveryContentHeight(t *testing.T) {
	for _, height := range []int{24, 30, 40, 60} {
		t.Run(fmt.Sprintf("height=%d", height), func(t *testing.T) {
			plain := browserModel(t, 200, height)
			withStatus := browserModel(t, 200, height)
			withStatus.statusMsg = "Added: 3 added"

			assert.Equal(t, browserContentHeight(plain)-2, browserContentHeight(withStatus),
				"the status notice must be budgeted like an error")

			body := stripANSI(withStatus.View())
			assert.Contains(t, body, "f000", "the first entry must stay visible with a status message")
			assert.Contains(t, body, selectionMarker, "the cursor must stay visible")
			assert.LessOrEqual(t, len(strings.Split(withStatus.View(), "\n")), height,
				"the view must still fit the terminal")
		})
	}
}

// TestAddNoticeLines_ConfirmAndPreviewBudgeted covers the other two steps.
func TestAddNoticeLines_ConfirmAndPreviewBudgeted(t *testing.T) {
	for _, step := range []int{addStepPreview, addStepConfirm} {
		plain := addModelWithPlan(200, 40)
		plain.addStep = step
		withStatus := addModelWithPlan(200, 40)
		withStatus.addStep = step
		withStatus.statusMsg = "Added: 3 added"

		if step == addStepPreview {
			assert.Equal(t, previewContentHeight(plain)-2, previewContentHeight(withStatus))
		} else {
			assert.Equal(t, confirmContentHeight(plain)-2, confirmContentHeight(withStatus))
		}

		assert.LessOrEqual(t, len(strings.Split(withStatus.View(), "\n")), withStatus.height,
			"step %d must fit the terminal with a status message", step)
	}
}
