package stow

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
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

		result.linkFile(path, relPath, targetPath, relSymlink)
		return nil
	})

	if err != nil {
		return nil, err
	}

	packages, discoverErr := DiscoverPackages(repoDir, homeDir)
	if discoverErr != nil {
		return result, nil
	}

	for _, pkg := range packages {
		if pkg.Name != packageName {
			continue
		}
		for _, f := range pkg.Files {
			if f.InRepo {
				continue
			}
			linkAutoDetectedFile(result, pkgDir, homeDir, f)
		}
		break
	}

	return result, nil
}

func linkAutoDetectedFile(result *LinkResult, pkgDir, homeDir string, f FileEntry) {
	repoPath := filepath.Join(pkgDir, f.RelPath)

	targetInfo, err := os.Lstat(f.TargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		result.Skipped++
		return
	}

	if targetInfo.Mode()&os.ModeSymlink != 0 {
		linkTarget, readErr := os.Readlink(f.TargetPath)
		if readErr == nil {
			var resolved string
			if filepath.IsAbs(linkTarget) {
				resolved = filepath.Clean(linkTarget)
			} else {
				resolved = filepath.Clean(filepath.Join(filepath.Dir(f.TargetPath), linkTarget))
			}
			if resolved == filepath.Clean(repoPath) {
				result.Skipped++
				return
			}
		}
		result.Conflicts = append(result.Conflicts, f.RelPath)
		result.Skipped++
		return
	}

	srcData, readErr := os.ReadFile(f.TargetPath)
	if readErr != nil {
		result.Conflicts = append(result.Conflicts, f.RelPath)
		result.Skipped++
		return
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(repoPath), 0755); mkdirErr != nil {
		result.Conflicts = append(result.Conflicts, f.RelPath)
		result.Skipped++
		return
	}

	srcPerm := targetInfo.Mode().Perm()
	if writeErr := os.WriteFile(repoPath, srcData, srcPerm); writeErr != nil {
		result.Conflicts = append(result.Conflicts, f.RelPath)
		result.Skipped++
		return
	}

	relSymlink, symErr := filepath.Rel(filepath.Dir(f.TargetPath), repoPath)
	if symErr != nil {
		result.Skipped++
		return
	}

	if symErr = os.Symlink(relSymlink, f.TargetPath+".dotcor-tmp"); symErr != nil {
		result.Skipped++
		return
	}

	if removeErr := os.Remove(f.TargetPath); removeErr != nil {
		_ = os.Remove(f.TargetPath + ".dotcor-tmp")
		result.Conflicts = append(result.Conflicts, f.RelPath)
		result.Skipped++
		return
	}

	if renameErr := os.Rename(f.TargetPath+".dotcor-tmp", f.TargetPath); renameErr != nil {
		_ = os.Remove(f.TargetPath + ".dotcor-tmp")
		if writeErr := os.WriteFile(f.TargetPath, srcData, srcPerm); writeErr != nil {
			result.Conflicts = append(result.Conflicts, f.RelPath)
		}
		result.Skipped++
		return
	}

	result.Linked++
}

func (r *LinkResult) linkFile(path, relPath, targetPath, relSymlink string) {
	targetInfo, err := os.Lstat(targetPath)
	if err != nil {
		if !os.IsNotExist(err) {
			r.Conflicts = append(r.Conflicts, relPath)
			r.Skipped++
			return
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			r.Conflicts = append(r.Conflicts, relPath)
			r.Skipped++
			return
		}

		if err := os.Symlink(relSymlink, targetPath); err != nil {
			r.Conflicts = append(r.Conflicts, relPath)
			r.Skipped++
			return
		}
		r.Linked++
		return
	}

	if targetInfo.Mode()&os.ModeSymlink != 0 {
		existingTarget, err := os.Readlink(targetPath)
		if err != nil {
			r.Conflicts = append(r.Conflicts, relPath)
			r.Skipped++
			return
		}

		var resolved string
		if filepath.IsAbs(existingTarget) {
			resolved = filepath.Clean(existingTarget)
		} else {
			resolved = filepath.Clean(filepath.Join(filepath.Dir(targetPath), existingTarget))
		}

		if resolved == filepath.Clean(path) {
			r.Skipped++
			return
		}

		r.Conflicts = append(r.Conflicts, relPath)
		r.Skipped++
		return
	}

	r.Conflicts = append(r.Conflicts, relPath)
	r.Skipped++
}

func LinkWithBackup(repoDir, homeDir, packageName, backupDir string) (*LinkResult, error) {
	result, err := Link(repoDir, homeDir, packageName)
	if err != nil {
		return nil, err
	}

	if len(result.Conflicts) == 0 {
		return result, nil
	}

	pkgDir := filepath.Join(repoDir, packageName)
	ts := time.Now().Format("2006-01-02_15-04-05")

	var remaining []string
	for _, relPath := range result.Conflicts {
		targetPath := filepath.Join(homeDir, relPath)
		repoPath := filepath.Join(pkgDir, relPath)

		srcData, readErr := os.ReadFile(targetPath)
		if readErr != nil {
			remaining = append(remaining, relPath)
			continue
		}

		srcInfo, statErr := os.Lstat(targetPath)
		srcPerm := os.FileMode(0644)
		if statErr == nil {
			srcPerm = srcInfo.Mode().Perm()
		}

		backupPath := filepath.Join(backupDir, ts, relPath)
		if mkdirErr := os.MkdirAll(filepath.Dir(backupPath), 0755); mkdirErr != nil {
			remaining = append(remaining, relPath)
			continue
		}
		if writeErr := os.WriteFile(backupPath, srcData, srcPerm); writeErr != nil {
			remaining = append(remaining, relPath)
			continue
		}

		if removeErr := os.Remove(targetPath); removeErr != nil {
			remaining = append(remaining, relPath)
			continue
		}

		relSymlink, symErr := filepath.Rel(filepath.Dir(targetPath), repoPath)
		if symErr != nil {
			remaining = append(remaining, relPath)
			continue
		}
		if symErr = os.Symlink(relSymlink, targetPath); symErr != nil {
			_ = os.WriteFile(targetPath, srcData, srcPerm)
			remaining = append(remaining, relPath)
			continue
		}

		result.Linked++
		result.Skipped--
	}

	result.Conflicts = remaining
	return result, nil
}
