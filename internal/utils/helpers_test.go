package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatSizeEdgeCases(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 bytes"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatSize(tt.bytes)
			assert.Equal(t, tt.expected, result, "FormatSize(%d)", tt.bytes)
		})
	}
}

func TestParseDuration(t *testing.T) {
	// Critical regression test: `5m` must mean 5 minutes (matching
	// time.ParseDuration), NOT 150 days. The previous implementation
	// remapped `m` to "month" (30 days) and silently turned `5m` into
	// 5*30*24h = 3600 hours.
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"empty defaults to 30 days", "", 30 * 24 * time.Hour, false},
		{"5 minutes (stdlib semantics)", "5m", 5 * time.Minute, false},
		{"3 hours", "3h", 3 * time.Hour, false},
		{"30 seconds", "30s", 30 * time.Second, false},
		{"7 days", "7d", 7 * 24 * time.Hour, false},
		{"2 weeks", "2w", 2 * 7 * 24 * time.Hour, false},
		{"1 month (mo suffix)", "1mo", 30 * 24 * time.Hour, false},
		{"6 months", "6mo", 6 * 30 * 24 * time.Hour, false},
		{"negative rejected", "-3d", 0, true},
		{"zero rejected", "0d", 0, true},
		{"garbage rejected", "abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
