package stow

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Link creates the $HOME symlinks for a package.
//
// ignorePatterns filters the auto-detect pass, which sweeps untracked files
// found under the package's managed $HOME subtree into the repo. Without it a
// plain "stow this package" keypress copied ~/.ssh/id_rsa, .env and *.pem
// into a repository that is then committed and pushed. Pass nil only when
// there is genuinely no configuration to honour.
func Link(repoDir, homeDir, packageName string, ignorePatterns []string) (*LinkResult, error) {
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

		// The relative link target is computed inside linkFile, after the
		// parent directory exists — resolving the physical parent requires it.
		result.linkFile(path, relPath, filepath.Join(homeDir, relPath))
		return nil
	})

	if err != nil {
		// Return what was actually done alongside the error. filepath.Walk
		// aborts on the first callback failure, but every file before that
		// point has already been linked — discarding the result left the
		// caller reporting a bare failure for a package that is now
		// half-linked, with no record of which half.
		return result, err
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
		// The repo-side links are already in place; only the auto-detect
		// pass is lost. Log it rather than returning success silently — the
		// previous `return result, nil` made a skipped phase indistinguishable
		// from a clean run.
		slog.Default().Warn("link: auto-detect pass skipped",
			"package", packageName, "err", err)
		return result, nil
	}
	pkgFiles = appendAutoDetected(pkgFiles, pkgDir, homeDir)
	for _, f := range pkgFiles {
		if f.InRepo {
			continue
		}
		// Honour the ignore list here, not just on the Add/Adopt path. This
		// pass pulls arbitrary untracked $HOME files into the repo, so it is
		// the one place a secret is most likely to slip in unnoticed.
		if matched, pattern := matchIgnore(f.TargetPath, ignorePatterns); matched {
			result.Ignored = append(result.Ignored, f.RelPath)
			slog.Default().Debug("link: skipping ignored file",
				"path", f.TargetPath, "pattern", pattern)
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
		if symlinkTargetsPath(f.TargetPath, repoPath) {
			result.Skipped++
			return
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
	// open(2) applies perm only when it creates the file, so an existing
	// repo copy would keep a looser mode than the $HOME file it mirrors.
	if chmodErr := os.Chmod(repoPath, srcPerm); chmodErr != nil {
		result.Conflicts = append(result.Conflicts, f.RelPath)
		result.Skipped++
		return
	}

	relSymlink, symErr := relLinkTarget(f.TargetPath, repoPath)
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

func (r *LinkResult) linkFile(path, relPath, targetPath string) {
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

		relSymlink, relErr := relLinkTarget(targetPath, path)
		if relErr != nil {
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
		if symlinkTargetsPath(targetPath, path) {
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
func LinkWithBackup(repoDir, homeDir, packageName, backupDir string, ignorePatterns []string) (*LinkResult, error) {
	result, err := Link(repoDir, homeDir, packageName, ignorePatterns)
	if err != nil {
		// Link returns what it managed to do alongside the error; pass that
		// through rather than reinstating a bare failure with no counts.
		return result, err
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

		// The repo copy must already exist before we point $HOME at it.
		// Not every conflict comes from the repo walk: linkAutoDetectedFile
		// records a conflict precisely when it FAILED to create the repo
		// copy (read-only mount, ENOSPC, a root-owned package dir). Swapping
		// in a symlink then succeeds — creating a link needs no repo write —
		// and leaves the user with a dangling symlink whose content survives
		// only in the backup tree, reported as a success.
		if _, statErr := os.Lstat(repoPath); statErr != nil {
			remaining = append(remaining, relPath)
			continue
		}

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

		relSymlink, symErr := relLinkTarget(targetPath, repoPath)
		if symErr != nil {
			remaining = append(remaining, relPath)
			continue
		}

		backupPath := filepath.Join(backupDir, ts, relPath)

		if err := resolveOneConflict(result, homeDir, targetPath, backupPath, relSymlink, srcData, srcPerm); err != nil {
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
func resolveOneConflict(result *LinkResult, homeDir, targetPath, backupPath, relSymlink string, srcData []byte, srcPerm os.FileMode) error {
	// Step 1: write backup. We DO NOT roll this back on later failure —
	// the backup is the user's only safety net if anything else goes
	// wrong, and removing it on failure would leave them stranded.
	// 0700, not 0755. The backup tree mirrors $HOME, so backing up
	// ~/.ssh/config creates a .ssh directory inside it. The file itself keeps
	// its own mode, but a world-traversable directory still leaks the
	// filenames under ~/.ssh, ~/.gnupg and ~/.aws on a shared host. A backup
	// must never be more exposed than what it copied.
	if err := os.MkdirAll(filepath.Dir(backupPath), 0700); err != nil {
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
				// Report the path relative to $HOME. The previous
				// computation used the backup directory as the base, which
				// produced a meaningless "../../../.." string for a path
				// that lives under $HOME.
				rel, relErr := filepath.Rel(homeDir, targetPath)
				if relErr != nil {
					rel = targetPath
				}
				result.RestoreFailures = append(result.RestoreFailures, fmt.Sprintf("%s (backup at %s)", rel, backupPath))
			}
		}
		return err
	}

	return nil
}
