package stow

import (
	"fmt"
	"os"
	"path/filepath"
)

func Link(repoDir, homeDir, packageName string) (*LinkResult, error) {
	pkgDir := filepath.Join(repoDir, packageName)

	info, err := os.Stat(pkgDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("package %q not found", packageName)
		}
		return nil, fmt.Errorf("checking package directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("package %q is not a directory", packageName)
	}

	result := &LinkResult{}

	err = filepath.Walk(pkgDir, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(pkgDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}

		targetPath := filepath.Join(homeDir, relPath)
		relSymlink, err := filepath.Rel(filepath.Dir(targetPath), path)
		if err != nil {
			return fmt.Errorf("computing relative symlink: %w", err)
		}

		targetInfo, err := os.Lstat(targetPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("checking target %s: %w", targetPath, err)
			}

			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("creating parent directory for %s: %w", targetPath, err)
			}

			if err := os.Symlink(relSymlink, targetPath); err != nil {
				return fmt.Errorf("creating symlink %s: %w", targetPath, err)
			}
			result.Linked++
			return nil
		}

		if targetInfo.Mode()&os.ModeSymlink != 0 {
			existingTarget, err := os.Readlink(targetPath)
			if err != nil {
				return fmt.Errorf("reading existing symlink %s: %w", targetPath, err)
			}

			var resolved string
			if filepath.IsAbs(existingTarget) {
				resolved = filepath.Clean(existingTarget)
			} else {
				resolved = filepath.Clean(filepath.Join(filepath.Dir(targetPath), existingTarget))
			}

			if resolved == filepath.Clean(path) {
				result.Skipped++
				return nil
			}

			result.Conflicts = append(result.Conflicts, relPath)
			result.Skipped++
			return nil
		}

		result.Conflicts = append(result.Conflicts, relPath)
		result.Skipped++
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
