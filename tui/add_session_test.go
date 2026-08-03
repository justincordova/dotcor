package tui

import (
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
