package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/justincordova/dotcor/internal/config"
)

func TestConfigConcurrentWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race condition test in short mode")
	}

	tmpDir := t.TempDir()
	cfg, _ := config.NewDefaultConfig()
	cfg.RepoPath = tmpDir

	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 3)

	// Try to add same file 3 times concurrently
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mf := config.ManagedFile{
				SourcePath: testFile,
				RepoPath:   fmt.Sprintf("test%d.txt", idx),
				AddedAt:    time.Now(),
			}
			errors <- cfg.AddManagedFile(mf)
		}(i)
	}

	wg.Wait()
	close(errors)

	// Count successes - should be at most 1
	successCount := 0
	for err := range errors {
		if err == nil {
			successCount++
		}
	}

	if successCount > 1 {
		t.Errorf("concurrent adds should be serialized, got %d successes", successCount)
	}
}
