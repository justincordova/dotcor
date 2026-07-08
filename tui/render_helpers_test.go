package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// stripANSI removes lipgloss color/style escape codes so tests can assert on
// the underlying text without coupling to exact styling.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestHumanSize_Thresholds(t *testing.T) {
	cases := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"just under KB", 1023, "1023 B"},
		{"exactly KB", 1024, "1.0 KB"},
		{"mid KB (regression for 0.0MB bug)", 340 * 1024, "340.0 KB"},
		{"just under MB", 1024*1024 - 1, "1024.0 KB"},
		{"exactly MB", 1024 * 1024, "1.0 MB"},
		{"multi MB", 5 * 1024 * 1024, "5.0 MB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, humanSize(tc.bytes))
		})
	}
}

func TestJoinSubview_DropsEmptySections(t *testing.T) {
	// An absent status row (empty string) must not introduce a blank line.
	withEmpty := joinSubview("header", "body", "", "footer")
	withoutEmpty := joinSubview("header", "body", "footer")
	assert.Equal(t, withoutEmpty, withEmpty,
		"empty sections should be dropped, yielding the same layout as omitting them")
}

func TestJoinSubview_KeepsNonEmptySections(t *testing.T) {
	out := joinSubview("a", "b", "c")
	lines := strings.Split(out, "\n")
	assert.Equal(t, []string{"a", "b", "c"}, lines)
}

func TestSubviewStatusRow_Precedence(t *testing.T) {
	// Error takes precedence over status message.
	m := Model{err: fmt.Errorf("boom"), statusMsg: "saved"}
	row := stripANSI(subviewStatusRow(m))
	assert.Contains(t, row, "boom")
	assert.Contains(t, row, "✗")
	assert.NotContains(t, row, "saved")

	// Status message shown when there is no error.
	m = Model{statusMsg: "saved"}
	row = stripANSI(subviewStatusRow(m))
	assert.Contains(t, row, "saved")
	assert.Contains(t, row, "✓")

	// Nothing to report yields an empty row (so joinSubview drops it).
	m = Model{}
	assert.Equal(t, "", subviewStatusRow(m))
}

func TestSelectableRow_MarkerOnlyWhenSelected(t *testing.T) {
	selected := stripANSI(selectableRow("content", true, 40))
	unselected := stripANSI(selectableRow("content", false, 40))

	assert.Contains(t, selected, selectionMarker, "selected row carries the canonical marker")
	assert.NotContains(t, unselected, selectionMarker, "unselected row has no marker")
	assert.Contains(t, unselected, "content")
}

func TestRenderLogo_AllRunesStyled(t *testing.T) {
	// The gradient is indexed by rune, not byte. "◆" is multi-byte, so a
	// byte-indexed loop would skip stops; assert every source rune survives
	// (i.e. the visible text is intact after styling).
	got := stripANSI(renderLogo())
	assert.Equal(t, "◆ dotcor", got)
}

func TestMetaSep_IsNonEmpty(t *testing.T) {
	// The separator must render a visible middle dot between metadata facts.
	assert.Contains(t, stripANSI(metaSep()), "·")
}

// Guard: humanSize output width stays compact so it fits inline in the stats
// strip without wrapping the row.
func TestHumanSize_CompactWidth(t *testing.T) {
	for _, b := range []int64{0, 999, 1024, 340 * 1024, 5 * 1024 * 1024} {
		assert.LessOrEqual(t, lipgloss.Width(humanSize(b)), 10)
	}
}
