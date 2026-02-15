package utils

import (
	"fmt"
	"strings"
	"time"
)

const (
	// Size constants for human-readable format
	KB = 1024
	MB = KB * 1024
	GB = MB * 1024

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
	switch {
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

// ParseDuration parses a human-friendly duration string
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	if s == "" {
		return 30 * 24 * time.Hour, nil // Default: 30 days
	}

	// Handle common formats
	var multiplier time.Duration
	var value int

	if strings.HasSuffix(s, "d") {
		multiplier = 24 * time.Hour
		_, err := fmt.Sscanf(s, "%dd", &value)
		if err != nil {
			return 0, fmt.Errorf("invalid format: %s", s)
		}
		if value <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
	} else if strings.HasSuffix(s, "w") {
		multiplier = 7 * 24 * time.Hour
		_, err := fmt.Sscanf(s, "%dw", &value)
		if err != nil {
			return 0, fmt.Errorf("invalid format: %s", s)
		}
		if value <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
	} else if strings.HasSuffix(s, "m") {
		multiplier = 30 * 24 * time.Hour // Approximate month
		_, err := fmt.Sscanf(s, "%dm", &value)
		if err != nil {
			return 0, fmt.Errorf("invalid format: %s", s)
		}
		if value <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
	} else if strings.HasSuffix(s, "h") {
		multiplier = time.Hour
		_, err := fmt.Sscanf(s, "%dh", &value)
		if err != nil {
			return 0, fmt.Errorf("invalid format: %s", s)
		}
		if value <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
	} else {
		// Try standard Go duration
		return time.ParseDuration(s)
	}

	return time.Duration(value) * multiplier, nil
}
