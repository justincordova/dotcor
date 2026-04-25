package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// Size constants for human-readable format
	KB = 1024
	MB = KB * 1024
	GB = MB * 1024
	TB = GB * 1024

	// Diff command constants
	DiffBinary      = "diff"
	DiffUnifiedFlag = "-u"
	DiffStatFlag    = "--stat"

	// Time format constants
	BackupTimestampFormat = "2006-01-02_15-04-05"
	GitTimestampFormat    = "2006-01-02 15:04:05"
)

// FormatSize formats file size in human-readable format
func FormatSize(bytes int64) string {
	if bytes < 0 {
		return "0 bytes"
	}

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// FormatAge formats a duration into a human-readable string
func FormatAge(d time.Duration) string {
	if d < time.Minute {
		seconds := int(d.Seconds())
		return fmt.Sprintf("%d second%s", seconds, pluralize(seconds))
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		return fmt.Sprintf("%d minute%s", minutes, pluralize(minutes))
	} else {
		hours := d.Hours()
		return fmt.Sprintf("%.1f hour%s", hours, pluralize(int(hours)))
	}
}

// pluralize returns 's' if n != 1
func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// ParseDuration parses a human-friendly duration string with extra
// suffixes that time.ParseDuration doesn't support: `d` for days, `w`
// for weeks, and `mo` for months (approximated as 30 days).
//
// `m` is intentionally NOT remapped to months: time.ParseDuration uses
// `m` for minutes, and silently turning a user's `5m` into 150 days
// (the previous behaviour) was a 4-order-of-magnitude surprise. Use
// `mo` for months.
//
// Anything that isn't `Nd`/`Nw`/`Nmo` falls through to time.ParseDuration,
// which keeps stdlib semantics for s/m/h/ms/us/ns.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	if s == "" {
		return 30 * 24 * time.Hour, nil // Default: 30 days
	}

	// Order matters: check `mo` before `m` (which we no longer remap)
	// so `5mo` doesn't get parsed by time.ParseDuration as nonsense.
	type rule struct {
		suffix     string
		multiplier time.Duration
	}
	rules := []rule{
		{"mo", 30 * 24 * time.Hour}, // approximate month
		{"d", 24 * time.Hour},
		{"w", 7 * 24 * time.Hour},
	}
	for _, r := range rules {
		if !strings.HasSuffix(s, r.suffix) {
			continue
		}
		num := strings.TrimSuffix(s, r.suffix)
		value, err := strconv.Atoi(num)
		if err != nil {
			return 0, fmt.Errorf("invalid format: %s", s)
		}
		if value <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
		return time.Duration(value) * r.multiplier, nil
	}

	// Fall through to stdlib: handles s/m/h/ms/us/ns.
	return time.ParseDuration(s)
}
