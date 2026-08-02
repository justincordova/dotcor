package stow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func Unlink(repoDir, homeDir, packageName string) (*UnlinkResult, error) {
	// Resolve repoDir/pkgDir through EvalSymlinks once. macOS aliases
	// /var → /private/var (and similar Library/Applications shims) cause
	// the resolved-target comparison below to fail when one side has the
	// alias and the other has the resolved form. classify.go already does
	// this for the same compare; mirror the pattern here so unlink doesn't
	// silently leave repo-owned symlinks in place.
	if resolved, err := filepath.EvalSymlinks(repoDir); err == nil {
		repoDir = resolved
	}
	pkgDir := filepath.Join(repoDir, packageName)
	if resolved, err := filepath.EvalSymlinks(pkgDir); err == nil {
		pkgDir = resolved
	}

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

		// Only remove links this package owns. symlinkTargetsPath resolves
		// both sides, so a /var → /private/var alias or a symlinked ancestor
		// on either side still matches.
		if !symlinkTargetsPath(targetPath, path) {
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
	// Resolve homeDir once so the stop comparison handles cases where
	// homeDir itself is a symlink (some SSO setups) or contains a /var
	// style alias. Same family as #8.
	resolvedHome := homeDir
	if r, err := filepath.EvalSymlinks(homeDir); err == nil {
		resolvedHome = r
	}

	for {
		resolvedDir := dir
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			resolvedDir = r
		}

		if dir == "" || resolvedDir == resolvedHome || dir == filepath.Dir(dir) {
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
