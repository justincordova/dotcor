package stow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MigrationStep struct {
	Src  string
	Dst  string
	Type string
}

func DetectV1Layout(repoDir string) bool {
	filesDir := filepath.Join(repoDir, "files")
	info, err := os.Stat(filesDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func PlanMigration(repoDir string) ([]MigrationStep, error) {
	filesDir := filepath.Join(repoDir, "files")

	info, err := os.Stat(filesDir)
	if err != nil {
		return nil, fmt.Errorf("files directory not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("files path is not a directory")
	}

	var steps []MigrationStep

	err = filepath.Walk(filesDir, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relToFiles, err := filepath.Rel(filesDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}

		if !fi.IsDir() {
			parts := strings.SplitN(relToFiles, string(filepath.Separator), 2)
			if len(parts) < 2 {
				return nil
			}

			category := parts[0]
			filePart := parts[1]

			dstRel := filepath.Join(category, filePart)
			dst := filepath.Join(repoDir, dstRel)

			steps = append(steps, MigrationStep{
				Src:  path,
				Dst:  dst,
				Type: "file",
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking files directory: %w", err)
	}

	return steps, nil
}

// ExecuteMigration moves every step src→dst with rollback on failure.
//
// Failure semantics: if any step fails, every step that already completed is
// reversed (dst→src). The user is left in the v1 layout — the same state
// they started in — instead of stranded between v1 and v2.
//
// `cleanEmptyParents` is intentionally deferred until **all** moves succeed.
// Tearing down empty source directories mid-loop would prevent rollback from
// recreating those parents on the way back.
func ExecuteMigration(repoDir string, steps []MigrationStep) error {
	completed := make([]MigrationStep, 0, len(steps))

	for _, step := range steps {
		if step.Type != "file" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(step.Dst), 0755); err != nil {
			rollbackMigration(completed)
			return fmt.Errorf("creating destination directory %s: %w (rolled back %d step(s))",
				step.Dst, err, len(completed))
		}

		if err := os.Rename(step.Src, step.Dst); err != nil {
			rollbackMigration(completed)
			return fmt.Errorf("moving %s to %s: %w (rolled back %d step(s))",
				step.Src, step.Dst, err, len(completed))
		}

		completed = append(completed, step)
	}

	// Only after every move succeeds: clean up the now-empty source tree.
	for _, step := range completed {
		cleanEmptyParents(filepath.Dir(step.Src), repoDir)
	}

	filesDir := filepath.Join(repoDir, "files")
	entries, err := os.ReadDir(filesDir)
	if err == nil && len(entries) == 0 {
		_ = os.RemoveAll(filesDir)
	}

	return nil
}

// rollbackMigration reverses every completed step. Walks the slice in
// reverse so that nested directories are unwound in the same order they
// were created. Best-effort: rollback errors are not actionable from here
// (the user is mid-migration with disk issues), but each failure is logged
// and the function still attempts every remaining reversal.
func rollbackMigration(completed []MigrationStep) {
	for i := len(completed) - 1; i >= 0; i-- {
		step := completed[i]
		// Recreate the original source-side directory in case the dst-side
		// move emptied it. MkdirAll is a no-op if it still exists.
		if err := os.MkdirAll(filepath.Dir(step.Src), 0755); err != nil {
			// nothing more we can do — leave a breadcrumb in stderr-equivalent
			fmt.Fprintf(os.Stderr, "migration rollback: cannot recreate %s: %v\n",
				filepath.Dir(step.Src), err)
			continue
		}
		if err := os.Rename(step.Dst, step.Src); err != nil {
			fmt.Fprintf(os.Stderr, "migration rollback: cannot restore %s -> %s: %v\n",
				step.Dst, step.Src, err)
		}
	}
}

func cleanEmptyParents(dir, stopAt string) {
	for dir != "" && dir != stopAt {
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			break
		}
		if len(entries) > 0 {
			break
		}

		_ = os.Remove(dir)
		dir = parent
	}
}
