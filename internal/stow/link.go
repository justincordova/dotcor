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

	// Previously this re-ran a full DiscoverPackages to find auto-detected
	// files inside the managed $HOME tree for this one package. That
	// walked every other package in the repo AND every package's managed
	// $HOME root — wasteful when we only care about one package.
	//
	// Instead, rebuild just this package's file list (discoverFiles +
	// appendAutoDetected) and iterate its non-InRepo entries.
	pkgFiles, err := discoverFiles(pkgDir, homeDir)
	if err != nil {
		return result, nil
	}
	pkgFiles = appendAutoDetected(pkgFiles, pkgDir, homeDir)
	for _, f := range pkgFiles {
		if f.InRepo {
			continue
		}
		linkAutoDetectedFile(result, pkgDir, homeDir, f)
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
		// Foreign symlink in $HOME — do NOT silently rewrite it. The user
		// must confirm via the explicit Add/Adopt flow (the `o` key), where
		// the actual owner of the target is visible in the preview.
		result.Foreign = append(result.Foreign, f.RelPath)
		result.Skipped++
		return
	}

	// Cap the read so a multi-GB file in $HOME (font cache, media file)
	// doesn't OOM the TUI. Skips with a conflict marker so the user sees
	// the file and can deal with it manually.
	srcData, _, readErr := safeReadFile(f.TargetPath)
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

	// Atomic swap: write the new symlink to a temp path in the same
	// directory, then rename it on top of the existing target. POSIX
	// rename replaces the destination in a single syscall — there is no
	// window where TargetPath is missing, so a crash mid-swap leaves
	// the user's $HOME with either the original file (if rename hadn't
	// happened) or the new symlink (if it had). Removing the target
	// first, as the previous implementation did, opened a window where
	// $HOME had no file at all; a crash there left the file gone with
	// the bytes only present in the just-written repo copy.
	tmpLink := f.TargetPath + ".dotcor-tmp"
	_ = os.Remove(tmpLink) // clean any leftover from a prior crashed run
	if symErr = os.Symlink(relSymlink, tmpLink); symErr != nil {
		result.Skipped++
		return
	}

	if renameErr := os.Rename(tmpLink, f.TargetPath); renameErr != nil {
		_ = os.Remove(tmpLink)
		result.Conflicts = append(result.Conflicts, f.RelPath)
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

// LinkWithBackup resolves conflicts from a prior Link() by backing up the
// existing $HOME file and replacing it with a symlink into the repo.
//
// Per-conflict atomicity is provided by fileTxn — if any step fails for a
// given file, every prior step for that file is unwound. Successful files
// remain in their new state; the loop continues to the next conflict.
//
// Issue #45 fix: when the symlink swap fails AND the restore to the
// original file also fails, the file is recorded in result.RestoreFailures.
// Previously the restore error was silently dropped (`_ = os.WriteFile`),
// so a user could end up with a missing $HOME file and no warning.
//
// Issue #22 fix: result.Resolved is bumped (not just Linked) so the UI
// can distinguish "linked cleanly" from "linked after backup".
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

		// Never follow a symlink at the conflict target — doing so would
		// read+copy whatever is on the other end, silently adopting a
		// foreign file. Foreign symlinks should be handled by the explicit
		// Add/Adopt flow, not by conflict resolution during stow.
		srcInfo, statErr := os.Lstat(targetPath)
		if statErr != nil {
			remaining = append(remaining, relPath)
			continue
		}
		if srcInfo.Mode()&os.ModeSymlink != 0 {
			result.Foreign = append(result.Foreign, relPath)
			remaining = append(remaining, relPath)
			continue
		}

		// Cap the read at maxFileSizeBytes so a giant conflict file doesn't
		// OOM the TUI mid-resolution.
		srcData, _, readErr := safeReadFile(targetPath)
		if readErr != nil {
			remaining = append(remaining, relPath)
			continue
		}
		srcPerm := srcInfo.Mode().Perm()

		relSymlink, symErr := filepath.Rel(filepath.Dir(targetPath), repoPath)
		if symErr != nil {
			remaining = append(remaining, relPath)
			continue
		}

		backupPath := filepath.Join(backupDir, ts, relPath)

		if err := resolveOneConflict(result, targetPath, backupPath, relSymlink, srcData, srcPerm); err != nil {
			remaining = append(remaining, relPath)
			continue
		}

		result.Linked++
		result.Resolved++
		result.Skipped--
	}

	result.Conflicts = remaining
	return result, nil
}

// resolveOneConflict performs one conflict resolution as a per-file
// transaction: backup → replace with symlink. On step-2 failure the
// backup is preserved (it's the user's only recovery copy) but the
// rollback restores the original file. If the restore itself fails, the
// file is appended to result.RestoreFailures so the caller can warn.
func resolveOneConflict(result *LinkResult, targetPath, backupPath, relSymlink string, srcData []byte, srcPerm os.FileMode) error {
	// Step 1: write backup. We DO NOT roll this back on later failure —
	// the backup is the user's only safety net if anything else goes
	// wrong, and removing it on failure would leave them stranded.
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(backupPath, srcData, srcPerm); err != nil {
		return err
	}

	// Step 2: replace target with symlink. If this fails, write the
	// original bytes back to targetPath. If THAT fails, the file is
	// missing — record in RestoreFailures so the user is told.
	step := stepReplaceFileWithSymlink(targetPath, relSymlink, srcData, srcPerm)
	if err := step.do(); err != nil {
		// The do() in stepReplaceFileWithSymlink does tmp + rename, which
		// leaves the original file intact on failure (rename is atomic).
		// But to be defensive, attempt restore and surface any failure.
		if _, statErr := os.Lstat(targetPath); statErr != nil && os.IsNotExist(statErr) {
			if writeErr := os.WriteFile(targetPath, srcData, srcPerm); writeErr != nil {
				rel, _ := filepath.Rel(filepath.Dir(filepath.Dir(backupPath)), targetPath)
				if rel == "" {
					rel = targetPath
				}
				result.RestoreFailures = append(result.RestoreFailures, fmt.Sprintf("%s (backup at %s)", rel, backupPath))
			}
		}
		return err
	}

	return nil
}
