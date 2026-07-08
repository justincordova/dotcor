package tui

import "testing"

// Sample lines in the exact format charmlog's file handler emits:
// "<timestamp> <LEVEL-abbrev> <message> [key=val ...]". These are the
// abbreviated, uppercase level tokens (DEBU/INFO/WARN/ERRO) — NOT the
// logfmt "level=warn" form. The filter must key off these tokens.
const (
	debugLine = "2026/07/08 08:04:44 DEBU debug msg k=v"
	infoLine  = "2026/07/08 08:04:44 INFO info msg"
	warnLine  = "2026/07/08 08:04:44 WARN warn msg"
	errorLine = "2026/07/08 08:04:44 ERRO error msg"
)

func TestMatchesLevel_Hierarchy(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		level  string
		expect bool
	}{
		// debug filter shows everything
		{"debug shows debug", debugLine, "debug", true},
		{"debug shows info", infoLine, "debug", true},
		{"debug shows warn", warnLine, "debug", true},
		{"debug shows error", errorLine, "debug", true},

		// info filter shows info and above, hides debug
		{"info hides debug", debugLine, "info", false},
		{"info shows info", infoLine, "info", true},
		{"info shows warn", warnLine, "info", true},
		{"info shows error", errorLine, "info", true},

		// warn filter shows warn and error only
		{"warn hides debug", debugLine, "warn", false},
		{"warn hides info", infoLine, "warn", false},
		{"warn shows warn", warnLine, "warn", true},
		{"warn shows error", errorLine, "warn", true},

		// error filter shows error only
		{"error hides debug", debugLine, "error", false},
		{"error hides info", infoLine, "error", false},
		{"error hides warn", warnLine, "error", false},
		{"error shows error", errorLine, "error", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesLevel(tt.line, tt.level); got != tt.expect {
				t.Errorf("matchesLevel(%q, %q) = %v, want %v", tt.line, tt.level, got, tt.expect)
			}
		})
	}
}

// A message body that merely contains a level word must not be
// misclassified: the token match is space-anchored to the leading level
// column, not any occurrence in the free-text message.
func TestMatchesLevel_MessageBodyNotMisclassified(t *testing.T) {
	// An INFO line whose message mentions the word "error" should still
	// be treated as info, and thus hidden by the error filter.
	line := "2026/07/08 08:04:44 INFO retrying after error k=v"
	if matchesLevel(line, "error") {
		t.Errorf("info line with 'error' in body should not pass the error filter: %q", line)
	}
	if !matchesLevel(line, "info") {
		t.Errorf("info line should pass the info filter: %q", line)
	}
}
