package stow

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

type AdoptResult struct {
	Adopted int
	Skipped int
	// Ignored lists files skipped because they matched an ignore pattern,
	// so a user who expects one to be adopted can see why it was not.
	Ignored  []string
	Failures []string
}

// Adopt reparents foreign $HOME symlinks in a package into the repository.
//
// ignorePatterns is honoured here for the same reason as in Link, and it
// matters more: Link deliberately refuses foreign symlinks and tells the user
// to adopt them instead, so this is the path a secret reaches when it is
// reached through a symlink. Adopting ~/.config/nvim/prod.key → ~/secrets/…
// would otherwise copy the plaintext into a package directory, which — unlike
// logs/ and backups/ — is not gitignored, and push it on the next sync.
func Adopt(repoDir, homeDir, packageName string, ignorePatterns []string) (*AdoptResult, error) {
	pkgDir := filepath.Join(repoDir, packageName)

	if info, err := os.Stat(pkgDir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("package %q not found", packageName)
		}
		return nil, fmt.Errorf("checking package directory: %w", err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("package %q is not a directory", packageName)
	}

	packages, err := DiscoverPackages(repoDir, homeDir)
	if err != nil {
		return nil, fmt.Errorf("discovering packages: %w", err)
	}

	var pkg *Package
	for i := range packages {
		if packages[i].Name == packageName {
			pkg = &packages[i]
			break
		}
	}
	if pkg == nil {
		return nil, fmt.Errorf("package %q not found during discovery", packageName)
	}

	result := &AdoptResult{}

	for _, f := range pkg.Files {
		if f.InRepo || !f.IsSymlink || f.IsLinked {
			continue
		}

		if matched, pattern := matchIgnore(f.TargetPath, ignorePatterns); matched {
			result.Ignored = append(result.Ignored, f.RelPath)
			result.Skipped++
			slog.Default().Debug("adopt: skipping ignored file",
				"path", f.TargetPath, "pattern", pattern)
			continue
		}

		resolved, err := filepath.EvalSymlinks(f.TargetPath)
		if err != nil {
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		// Cap the read so adopting a multi-GB file via the foreign-symlink
		// path doesn't OOM the TUI. safeReadFile returns the perm too, so
		// the explicit Stat below is unnecessary in the common path.
		srcData, srcPerm, err := safeReadFile(resolved)
		if err != nil {
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		repoPath := filepath.Join(pkgDir, f.RelPath)
		if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		// Create the repo copy atomically with O_EXCL so an existing
		// destination is refused in a single syscall. The previous
		// stat-then-WriteFile sequence had a TOCTOU window, and
		// os.WriteFile would silently truncate a file that appeared
		// between the stat and the write — clobbering whatever was there.
		dstFile, err := os.OpenFile(repoPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, srcPerm)
		if err != nil {
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}
		if _, err := dstFile.Write(srcData); err != nil {
			_ = dstFile.Close()
			_ = os.Remove(repoPath)
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}
		if err := dstFile.Close(); err != nil {
			_ = os.Remove(repoPath)
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		relSymlink, err := relLinkTarget(f.TargetPath, repoPath)
		if err != nil {
			_ = os.Remove(repoPath)
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		// Atomic swap: stage the new symlink alongside the existing one,
		// then rename on top. The original Remove(f.TargetPath) before
		// Symlink+Rename opened a window where TargetPath had nothing
		// at all — a crash there left the user with no symlink AND no
		// file, recoverable only from the repo copy. POSIX rename
		// replaces the existing entry atomically in one syscall, so the
		// link is either the old foreign one or the new repo one,
		// never absent.
		tmpLink := f.TargetPath + tmpSuffix
		_ = os.Remove(tmpLink) // clean any leftover from a prior crashed run
		if err := os.Symlink(relSymlink, tmpLink); err != nil {
			_ = os.Remove(repoPath)
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		if err := os.Rename(tmpLink, f.TargetPath); err != nil {
			_ = os.Remove(tmpLink)
			_ = os.Remove(repoPath)
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		result.Adopted++
	}

	return result, nil
}
