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

			dstDir := filepath.Dir(dst)
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				return fmt.Errorf("creating destination directory %s: %w", dstDir, err)
			}

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

func ExecuteMigration(repoDir string, steps []MigrationStep) error {
	for _, step := range steps {
		if step.Type == "file" {
			srcDir := filepath.Dir(step.Src)
			if err := os.MkdirAll(filepath.Dir(step.Dst), 0755); err != nil {
				return fmt.Errorf("creating destination directory %s: %w", step.Dst, err)
			}

			if err := os.Rename(step.Src, step.Dst); err != nil {
				return fmt.Errorf("moving %s to %s: %w", step.Src, step.Dst, err)
			}

			cleanEmptyParents(srcDir, repoDir)
		}
	}

	filesDir := filepath.Join(repoDir, "files")
	entries, err := os.ReadDir(filesDir)
	if err == nil && len(entries) == 0 {
		_ = os.RemoveAll(filesDir)
	}

	return nil
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
