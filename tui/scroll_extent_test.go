package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/stow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func planWithFiles(n int) *stow.ClassificationPlan {
	files := make([]stow.ClassifiedFile, 0, n)
	for i := 0; i < n; i++ {
		files = append(files, stow.ClassifiedFile{
			RelPath: fmt.Sprintf("file-%03d.conf", i),
			AbsPath: fmt.Sprintf("/home/u/file-%03d.conf", i),
			Class:   stow.ClassAdd,
		})
	}
	return &stow.ClassificationPlan{
		Packages: []stow.PackagePlan{{Name: "pkg", Files: files}},
	}
}

func addModelWithPlan(n, height int) Model {
	m := NewModel(&config.Config{IgnorePatterns: []string{"*.key"}}, "test")
	m.loading = false
	m.activeView = AddView
	m.width = 100
	m.height = height
	m.previewPlan = planWithFiles(n)
	m.previewToggles = stow.BuildDefaultToggles(m.previewPlan)
	m.previewRows = buildPreviewRows(m.previewPlan)
	return m
}

// contentRowCount counts the body rows a step renderer emitted, excluding the
// dialog chrome (border, stepper, blank lines, hints, footer).
func contentRowCount(rendered string, marker string) int {
	n := 0
	for _, line := range strings.Split(stripANSI(rendered), "\n") {
		if strings.Contains(line, marker) {
			n++
		}
	}
	return n
}

// TestConfirmContentHeight_MatchesWhatIsRendered pins the fix for the key
// handler and the renderer disagreeing about the viewport size.
//
// The handler used a hardcoded `height - 14` while the renderer measured the
// chrome it actually drew. Measured across terminal sizes the handler was 6
// rows short at every height, so maxScroll was computed against the wrong
// number: pressing down at the bottom did nothing for six keypresses, and
// pgdown/ctrl+d paged by less than a screenful.
func TestConfirmContentHeight_MatchesWhatIsRendered(t *testing.T) {
	for _, height := range []int{24, 30, 40, 60} {
		t.Run(fmt.Sprintf("height=%d", height), func(t *testing.T) {
			m := addModelWithPlan(200, height)
			m.addStep = addStepConfirm

			cw := contentWidth(m.width)
			rendered := renderConfirmStep(m, cw-4, cw)

			// The window mixes package/class header rows with file rows, so
			// derive how many file rows the first screenful should contain.
			want := confirmContentHeight(m)
			lines := buildConfirmLines(m.previewPlan, m.previewToggles, bodyWidth(m.width))
			require.Greater(t, len(lines), want, "the plan must overflow the viewport")

			expected := 0
			for _, line := range lines[:want] {
				if strings.Contains(stripANSI(line), "file-") {
					expected++
				}
			}

			got := contentRowCount(rendered, "file-")
			assert.Equal(t, expected, got,
				"the scroll handler's viewport height must equal the number of rows actually drawn")
		})
	}
}

// TestPreviewContentHeight_MatchesWhatIsRendered is the same invariant for
// the preview step.
func TestPreviewContentHeight_MatchesWhatIsRendered(t *testing.T) {
	for _, height := range []int{24, 30, 40, 60} {
		t.Run(fmt.Sprintf("height=%d", height), func(t *testing.T) {
			m := addModelWithPlan(200, height)
			m.addStep = addStepPreview

			cw := contentWidth(m.width)
			rendered := renderPreviewStep(m, cw-4, cw)

			// The preview window mixes package/class header rows with file
			// rows, so derive how many file rows the window should contain.
			want := previewContentHeight(m)
			require.Greater(t, len(m.previewRows), want, "the plan must overflow the viewport")
			headers := 0
			for _, row := range m.previewRows[:want] {
				if row.isHeader {
					headers++
				}
			}

			got := contentRowCount(rendered, "file-")
			assert.Equal(t, want-headers, got,
				"the scroll handler's viewport height must equal the number of rows actually drawn")
		})
	}
}

// TestConfirmScroll_EndThenDownIsIdempotent asserts there are no dead
// keypresses at the bottom of the list.
func TestConfirmScroll_EndThenDownIsIdempotent(t *testing.T) {
	m := addModelWithPlan(200, 40)
	m.addStep = addStepConfirm

	atEnd, _ := m.confirmHandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	end := atEnd.(Model)
	require.Positive(t, end.confirmScroll)

	afterDown, _ := end.confirmHandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, end.confirmScroll, afterDown.(Model).confirmScroll,
		"pressing down at the bottom must not advance a scroll offset the renderer will ignore")
}

// TestScrollHandlers_ClampStoredOffsetAfterResize pins the fix for dead
// keypresses. Only the renderer's local copy was clamped, so after a resize
// shrank the viewport the model kept a larger offset: "down" was already at
// its limit and did nothing, while "up" had to be pressed several times
// before anything moved.
func TestScrollHandlers_ClampStoredOffsetAfterResize(t *testing.T) {
	t.Run("confirm", func(t *testing.T) {
		m := addModelWithPlan(200, 24)
		m.addStep = addStepConfirm
		atEnd, _ := m.confirmHandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
		big := atEnd.(Model)
		require.Positive(t, big.confirmScroll)

		// The window GROWS, which lowers maxScroll below the stored offset.
		big.height = 60
		lines := buildConfirmLines(big.previewPlan, big.previewToggles, bodyWidth(big.width))
		maxScroll := len(lines) - confirmContentHeight(big)

		clamped, _ := big.confirmHandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		got := clamped.(Model).confirmScroll

		assert.LessOrEqual(t, got, maxScroll, "the stored offset must be clamped to the new viewport")
	})

	t.Run("preview", func(t *testing.T) {
		m := addModelWithPlan(200, 24)
		m.addStep = addStepPreview
		atEnd, _ := m.previewHandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
		big := atEnd.(Model)
		require.Positive(t, big.previewScroll)

		big.height = 60
		maxScroll := len(big.previewRows) - previewContentHeight(big)

		clamped, _ := big.previewHandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		got := clamped.(Model).previewScroll

		assert.LessOrEqual(t, got, maxScroll, "the stored offset must be clamped to the new viewport")
	})
}

// TestClampDialogHeight_ShortDialog pins the guard against a panic on a
// dialog smaller than its own chrome.
func TestClampDialogHeight_ShortDialog(t *testing.T) {
	assert.NotPanics(t, func() {
		got := clampDialogHeight("a\nb\nc", 1)
		assert.Equal(t, "a\nb\nc", got, "too short to trim: return it unchanged")
	})
	assert.NotPanics(t, func() { _ = clampDialogHeight("", 1) })
}
