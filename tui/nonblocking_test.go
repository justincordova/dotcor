package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSettingsRemote_DoesNotBlockUpdate pins the fix for a frozen UI.
//
// Saving a remote used to call git.GetRemoteURL / RemoveRemote / SetRemote
// synchronously inside Update. Each carries a 30s timeout and SetRemote forks
// two processes, so on a slow or contended repo the whole event loop stalled
// with no spinner and no way to cancel. The work must be handed to a tea.Cmd.
func TestSettingsRemote_DoesNotBlockUpdate(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.repoDir = t.TempDir()
	m.activeView = SettingsView
	m.settingsStep = settingsStepEditRemote
	m.settingsInput.SetValue("https://github.com/u/dots.git")

	done := make(chan tea.Cmd, 1)
	go func() {
		_, cmd := updateSettingsEditRemote(m, tea.KeyMsg{Type: tea.KeyEnter})
		done <- cmd
	}()

	select {
	case cmd := <-done:
		require.NotNil(t, cmd, "the git work must be handed to a command")
	case <-time.After(2 * time.Second):
		t.Fatal("updateSettingsEditRemote blocked — git work is still running on the event loop")
	}
}

// TestSettingsRemote_RejectsBadURLImmediately keeps the fast-feedback path:
// URL validation is pure string work and stays inline.
func TestSettingsRemote_RejectsBadURLImmediately(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.repoDir = t.TempDir()
	m.activeView = SettingsView
	m.settingsStep = settingsStepEditRemote
	m.settingsInput.SetValue("ext::sh -c 'touch /tmp/pwned'")

	updated, cmd := updateSettingsEditRemote(m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Nil(t, cmd, "a rejected URL must not dispatch any git work")
	require.Error(t, updated.(Model).err)
	assert.Contains(t, updated.(Model).err.Error(), "invalid remote URL")
}

// TestUpdateHandlersDoNotCallGitDirectly is a structural guard against
// reintroducing blocking exec calls on the event loop. Update handlers may
// build commands that call git; they must not call it themselves.
func TestUpdateHandlersDoNotCallGitDirectly(t *testing.T) {
	// Cheap, non-exec helpers that are safe to call inline.
	allowed := map[string]bool{
		"IsRepo":               true,
		"IsGitInstalled":       true,
		"ValidateRemoteURL":    true,
		"RedactURLCredentials": true,
	}

	for _, file := range []string{"settings_view.go", "app.go"} {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		require.NoError(t, err)

		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isUpdateHandler(fn.Name.Name) {
				continue
			}
			assertNoDirectGitCall(t, fset, fn, allowed)
		}
	}
}

func isUpdateHandler(name string) bool {
	switch name {
	case "updateSettingsEditRemote", "updateSettingsMain", "updateSettingsBackups",
		"updateSettingsAddPattern", "updateDashboard", "updateLogs":
		return true
	}
	return false
}

// assertNoDirectGitCall reports git.X() calls that are not nested inside a
// function literal (a tea.Cmd closure).
func assertNoDirectGitCall(t *testing.T, fset *token.FileSet, fn *ast.FuncDecl, allowed map[string]bool) {
	t.Helper()

	ast.Inspect(fn, func(node ast.Node) bool {
		if node == ast.Node(fn) {
			return true
		}
		// Anything inside a closure runs off the event loop.
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "git" || allowed[sel.Sel.Name] {
			return true
		}
		t.Errorf("%s calls git.%s inline at %s — this blocks the event loop; move it into a tea.Cmd",
			fn.Name.Name, sel.Sel.Name, fset.Position(call.Pos()))
		return true
	})
}
