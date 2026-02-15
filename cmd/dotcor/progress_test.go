package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProgress(t *testing.T) {
	tests := []struct {
		name  string
		total int
		width int
	}{
		{
			name:  "default values",
			total: 10,
			width: 20,
		},
		{
			name:  "custom width",
			total: 5,
			width: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewProgress(tt.total, tt.width)
			assert.Equal(t, tt.total, got.Total)
			assert.Equal(t, tt.width, got.Width)
			assert.Equal(t, 0, got.Current)
		})
	}
}

func TestProgress_Increment(t *testing.T) {
	tests := []struct {
		name        string
		total       int
		width       int
		increments  int
		wantCurrent int
	}{
		{
			name:        "single increment",
			total:       5,
			width:       20,
			increments:  1,
			wantCurrent: 1,
		},
		{
			name:        "multiple increments",
			total:       10,
			width:       20,
			increments:  3,
			wantCurrent: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(tt.total, tt.width)
			for i := 0; i < tt.increments; i++ {
				p.Increment()
			}
			assert.Equal(t, tt.wantCurrent, p.Current)
		})
	}
}

func TestProgress_IsComplete(t *testing.T) {
	tests := []struct {
		name        string
		total       int
		width       int
		incrementTo int
		want        bool
	}{
		{
			name:        "not complete",
			total:       5,
			width:       20,
			incrementTo: 2,
			want:        false,
		},
		{
			name:        "complete",
			total:       5,
			width:       20,
			incrementTo: 5,
			want:        true,
		},
		{
			name:        "not started",
			total:       5,
			width:       20,
			incrementTo: 0,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(tt.total, tt.width)
			for i := 0; i < tt.incrementTo; i++ {
				p.Increment()
			}
			assert.Equal(t, tt.want, p.IsComplete())
		})
	}
}

func TestProgress_Render(t *testing.T) {
	tests := []struct {
		name    string
		total   int
		width   int
		current int
		wantIn  []string
	}{
		{
			name:    "0% progress",
			total:   10,
			width:   20,
			current: 0,
			wantIn:  []string{"0%", "(0/10"},
		},
		{
			name:    "50% progress",
			total:   10,
			width:   20,
			current: 5,
			wantIn:  []string{"50%", "(5/10"},
		},
		{
			name:    "100% progress",
			total:   10,
			width:   20,
			current: 10,
			wantIn:  []string{"100%", "(10/10"},
		},
		{
			name:    "33% progress",
			total:   3,
			width:   20,
			current: 1,
			wantIn:  []string{"33%", "(1/3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(tt.total, tt.width)
			for i := 0; i < tt.current; i++ {
				p.Increment()
			}
			got := p.Render()
			for _, substr := range tt.wantIn {
				assert.Contains(t, got, substr, "rendered progress should contain %s", substr)
			}
		})
	}
}

func TestProgress_Update(t *testing.T) {
	tests := []struct {
		name        string
		total       int
		width       int
		updateCount int
	}{
		{
			name:        "multiple updates",
			total:       5,
			width:       20,
			updateCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(tt.total, tt.width)
			for i := 0; i < tt.updateCount; i++ {
				p.Update()
				p.Increment()
			}
			assert.Equal(t, tt.updateCount, p.Current)
		})
	}
}

func TestProgress_Done(t *testing.T) {
	tests := []struct {
		name  string
		total int
		width int
	}{
		{
			name:  "complete progress",
			total: 5,
			width: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(tt.total, tt.width)
			for i := 0; i < tt.total; i++ {
				p.Increment()
			}
			rendered := p.Render()
			assert.Contains(t, rendered, "100%", "done should show 100%")
			assert.True(t, p.IsComplete())
		})
	}
}

func TestShouldUseProgress(t *testing.T) {
	tests := []struct {
		name  string
		total int
		batch bool
		want  bool
	}{
		{
			name:  "single file batch mode",
			total: 1,
			batch: true,
			want:  false,
		},
		{
			name:  "single file non-batch",
			total: 1,
			batch: false,
			want:  false,
		},
		{
			name:  "multiple files batch mode",
			total: 5,
			batch: true,
			want:  true,
		},
		{
			name:  "multiple files non-batch",
			total: 5,
			batch: false,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseProgress(tt.total, tt.batch)
			assert.Equal(t, tt.want, got)
		})
	}
}
