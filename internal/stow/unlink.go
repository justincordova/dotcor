package stow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func Unlink(repoDir, homeDir, packageName string) (*UnlinkResult, error) {
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

	result := &UnlinkResult{}

	var targetDirs []string

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

		targetInfo, err := os.Lstat(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("checking target %s: %w", targetPath, err)
		}

		if targetInfo.Mode()&os.ModeSymlink == 0 {
			return nil
		}

		existingTarget, err := os.Readlink(targetPath)
		if err != nil {
			return fmt.Errorf("reading symlink %s: %w", targetPath, err)
		}

		var resolved string
		if filepath.IsAbs(existingTarget) {
			resolved = filepath.Clean(existingTarget)
		} else {
			resolved = filepath.Clean(filepath.Join(filepath.Dir(targetPath), existingTarget))
		}

		if resolved != filepath.Clean(path) {
			return nil
		}

		if err := os.Remove(targetPath); err != nil {
			return fmt.Errorf("removing symlink %s: %w", targetPath, err)
		}

		result.Unlinked++
		targetDirs = append(targetDirs, filepath.Dir(targetPath))
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Sort(sort.Reverse(sort.StringSlice(targetDirs)))

	seen := make(map[string]bool)
	for _, dir := range targetDirs {
		cleanDirChain(dir, homeDir, seen, result)
	}

	return result, nil
}

func cleanDirChain(dir, homeDir string, seen map[string]bool, result *UnlinkResult) {
	for {
		if dir == "" || dir == homeDir || dir == filepath.Dir(dir) {
			return
		}
		if seen[dir] {
			return
		}
		seen[dir] = true

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		if len(entries) > 0 {
			return
		}

		if err := os.Remove(dir); err != nil {
			return
		}
		result.Removed++

		dir = filepath.Dir(dir)
	}
}
