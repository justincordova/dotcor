package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincordova/dotcor/internal/stow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jumpModel(t *testing.T) Model {
	t.Helper()
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(home, "sub", "a.conf"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(home, "b.conf"), []byte("x"), 0644))

	m := NewModel(testCfg(), "test")
	m.loading = false
	m.homeDir = home
	m.activeView = AddView
	m.width, m.height = 100, 40
	m.buildBrowserItems()
	return m
}

// TestJumpPrompt_EscClosesPromptAndKeepsSelection pins the fix for a wiped
// selection. updateAdd matched Esc before delegating to the browser, so the
// jump prompt's own esc handler was unreachable: pressing esc to dismiss a
// text prompt reset the whole wizard and dropped every selected file — while
// the footer still advertised "esc cancel".
func TestJumpPrompt_EscClosesPromptAndKeepsSelection(t *testing.T) {
	m := jumpModel(t)
	m.browserSelected[filepath.Join(m.homeDir, "b.conf")] = true
	m.browserJumping = true
	m.browserJumpInput.SetValue("sub")

	updated, _ := m.updateAdd(tea.KeyMsg{Type: tea.KeyEsc})
	asModel := updated.(Model)

	assert.False(t, asModel.browserJumping, "esc must close the jump prompt")
	assert.Equal(t, AddView, asModel.activeView, "esc must not leave the Add view")
	assert.Len(t, asModel.browserSelected, 1, "the file selection must survive")
}

// TestJumpPrompt_EnterJumpsInsteadOfSubmittingSelection pins the other half.
// enter submitted the selection for classification with the prompt still on
// screen and the typed path thrown away, so the jump feature had no working
// submit key.
func TestJumpPrompt_EnterJumpsInsteadOfSubmittingSelection(t *testing.T) {
	m := jumpModel(t)
	m.browserSelected[filepath.Join(m.homeDir, "b.conf")] = true
	m.browserJumping = true
	m.browserJumpInput.SetValue("sub")

	updated, _ := m.updateAdd(tea.KeyMsg{Type: tea.KeyEnter})
	asModel := updated.(Model)

	assert.False(t, asModel.browserJumping, "enter must close the jump prompt")
	assert.Equal(t, addStepSelect, asModel.addStep,
		"enter in the jump prompt must not advance the wizard")
}

// TestAddEsc_StillLeavesWizardWhenPromptClosed guards the normal behaviour.
func TestAddEsc_StillLeavesWizardWhenPromptClosed(t *testing.T) {
	m := jumpModel(t)

	updated, _ := m.updateAdd(tea.KeyMsg{Type: tea.KeyEsc})

	assert.Equal(t, DashboardView, updated.(Model).activeView)
}

// TestClassifyPlanMsg_IgnoredOutsideAddView pins the guard its siblings have.
// A plan arriving after the user cancelled belonged to an abandoned session;
// applying it dropped them into the preview of a selection they backed out
// of, two enters away from executing it.
func TestClassifyPlanMsg_IgnoredOutsideAddView(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = DashboardView
	m.addStep = addStepSelect

	plan := &stow.ClassificationPlan{
		Packages: []stow.PackagePlan{{Name: "pkg", Files: []stow.ClassifiedFile{{RelPath: "a"}}}},
	}
	updated, _ := m.Update(classifyPlanMsg{plan: plan})
	asModel := updated.(Model)

	assert.Equal(t, DashboardView, asModel.activeView)
	assert.Equal(t, addStepSelect, asModel.addStep, "a stale plan must not move the wizard")
	assert.Nil(t, asModel.previewPlan, "a stale plan must not be adopted")
}

// TestClassifyPlanMsg_AppliedInsideAddView keeps the normal flow working.
func TestClassifyPlanMsg_AppliedInsideAddView(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = AddView
	m.addStep = addStepSelect

	plan := &stow.ClassificationPlan{
		Packages: []stow.PackagePlan{{Name: "pkg", Files: []stow.ClassifiedFile{{RelPath: "a"}}}},
	}
	updated, _ := m.Update(classifyPlanMsg{plan: plan})
	asModel := updated.(Model)

	assert.Equal(t, addStepPreview, asModel.addStep)
	assert.NotNil(t, asModel.previewPlan)
}

// TestClassifyResultErr_IgnoredOutsideAddView covers the error branch, which
// otherwise threw a fresh session to a confirm step with no plan to render.
func TestClassifyResultErr_IgnoredOutsideAddView(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = SettingsView
	m.addStep = addStepSelect

	updated, _ := m.Update(classifyResultMsg{err: assert.AnError})
	asModel := updated.(Model)

	assert.Equal(t, addStepSelect, asModel.addStep, "a stale failure must not move the wizard")
	assert.Equal(t, SettingsView, asModel.activeView)
	assert.Error(t, asModel.err, "the error must still be surfaced")
}
