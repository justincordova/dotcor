package stow

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// fileTxn is a per-file transaction primitive used by the stow execute paths.
//
// Why this exists rather than just using internal/core.Transaction:
//
//   - core.Transaction takes a *config.Config so it can log via cfg.Logger
//     and call into internal/fs (which also takes a Config). The stow
//     package is intentionally Config-free so it can be tested and called
//     from tests without wiring a full app config. Threading Config through
//     ExecuteClassification + LinkWithBackup would ripple into every call
//     site and every test in the repo.
//
//   - Transaction semantics for a multi-file operation should be per-file,
//     not all-or-nothing. If file A succeeds and file B fails, file A
//     should remain in its new state (it was a complete unit of work) —
//     only file B's partial mutations should be unwound. core.Transaction
//     is designed for the all-or-nothing case.
//
// Each fileTxn captures pre-mutation state for the file it touches so any
// step that fails can roll back to a clean pre-state. Rollback walks
// completed steps in reverse order with a panic guard, mirroring
// core.Transaction's contract.
type fileTxn struct {
	steps []fileStep
}

// fileStep is a single reversible operation. do() performs the operation
// and undo() reverses it. undo() must be safe to call multiple times and
// must tolerate the operation's effects being absent (e.g. the file it
// would remove may already be gone).
type fileStep struct {
	desc string
	do   func() error
	undo func() error
}

// run executes the next step. On failure, every previously-completed step
// is unwound in reverse order. The returned error wraps the failing step's
// error and notes whether rollback also failed.
func (t *fileTxn) run(step fileStep) error {
	if err := step.do(); err != nil {
		rbErr := t.rollback()
		if rbErr != nil {
			return fmt.Errorf("%s: %w (rollback failed: %v)", step.desc, err, rbErr)
		}
		return fmt.Errorf("%s: %w", step.desc, err)
	}
	t.steps = append(t.steps, step)
	return nil
}

// commit clears the transaction's history. After commit, rollback is a
// no-op. Call this once the file is fully in its target state.
func (t *fileTxn) commit() {
	t.steps = nil
}

// rollback walks completed steps in reverse, calling undo() on each. A
// panic in any undo is converted to an error and stops the walk; remaining
// undos are skipped because the panic suggests the rollback path itself
// is unsafe.
func (t *fileTxn) rollback() error {
	var firstErr error
	for i := len(t.steps) - 1; i >= 0; i-- {
		step := t.steps[i]
		func() {
			defer func() {
				if r := recover(); r != nil && firstErr == nil {
					firstErr = fmt.Errorf("panic during undo of %q: %v", step.desc, r)
				}
			}()
			if err := step.undo(); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("undo %q: %w", step.desc, err)
				slog.Default().Warn("stow rollback step failed",
					"step", step.desc, "err", err)
			}
		}()
	}
	t.steps = nil
	return firstErr
}

// ─── Reusable steps ─────────────────────────────────────────────────────────

// stepWriteFile writes data+perm to dst.
//
// Undo restores dst to exactly what was there before the step ran: the prior
// bytes and mode if the file already existed, otherwise removal. Capturing
// the prior contents matters because dst is a repo path that may already be
// tracked — re-adding a file whose $HOME copy was replaced by a package
// installer overwrites the curated repo copy, and an undo that merely removed
// dst would leave the user with neither version. A rollback must never end in
// a worse state than not rolling back at all.
//
// Directories created by MkdirAll are removed back up to the deepest
// pre-existing ancestor, stopping at the first non-empty one. Removing only
// the immediate parent left orphaned directory chains behind, which
// DiscoverPackages then reports as empty packages.
func stepWriteFile(dst string, data []byte, perm os.FileMode) fileStep {
	parent := filepath.Dir(dst)
	deepestExisting := deepestExistingDir(parent)

	// Capture prior state at construction time, before any step runs.
	var priorData []byte
	var priorPerm os.FileMode
	priorExisted := false
	if info, err := os.Lstat(dst); err == nil && info.Mode().IsRegular() {
		if b, rerr := os.ReadFile(dst); rerr == nil {
			priorData = b
			priorPerm = info.Mode().Perm()
			priorExisted = true
		}
	}

	return fileStep{
		desc: "write " + dst,
		do: func() error {
			if err := os.MkdirAll(parent, 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", parent, err)
			}
			return os.WriteFile(dst, data, perm)
		},
		undo: func() error {
			if priorExisted {
				return os.WriteFile(dst, priorData, priorPerm)
			}
			if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
				return err
			}
			// Best-effort: unwind directories we created. Each Remove only
			// succeeds on an empty dir, so anything the user owns survives.
			for dir := parent; dir != deepestExisting && dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
				if err := os.Remove(dir); err != nil {
					break
				}
			}
			return nil
		},
	}
}

// deepestExistingDir walks up from dir and returns the first component that
// already exists, i.e. the boundary MkdirAll would not have created.
func deepestExistingDir(dir string) string {
	for dir != filepath.Dir(dir) {
		if pathExists(dir) {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return dir
}

// stepReplaceFileWithSymlink replaces the regular file at srcPath with a
// symlink whose target is linkTarget, atomically via tmp + rename. Undo
// restores the original file bytes/perm.
//
// The original bytes must be passed in (not re-read) so that undo works
// even if intermediate steps already modified the file.
func stepReplaceFileWithSymlink(srcPath, linkTarget string, origData []byte, origPerm os.FileMode) fileStep {
	return fileStep{
		desc: "replace " + srcPath + " with symlink",
		do: func() error {
			tmp := srcPath + ".dotcor-tmp"
			_ = os.Remove(tmp) // clean any leftover from a crashed prior run
			if err := os.Symlink(linkTarget, tmp); err != nil {
				return fmt.Errorf("create tmp symlink: %w", err)
			}
			if err := os.Rename(tmp, srcPath); err != nil {
				_ = os.Remove(tmp)
				return fmt.Errorf("rename tmp over source: %w", err)
			}
			return nil
		},
		undo: func() error {
			// Remove whatever is at srcPath (the symlink we just placed)
			// and write the original bytes back. WriteFile truncates so
			// it works even if the symlink wasn't fully removed.
			_ = os.Remove(srcPath)
			return os.WriteFile(srcPath, origData, origPerm)
		},
	}
}

// stepRepointSymlink atomically replaces the symlink at linkPath so that
// it points to newTarget. Undo restores the previous target.
//
// origTarget must be captured by the caller before invoking this step
// (typically via os.Readlink). If the link did not previously exist,
// pass an empty origTarget — undo will then remove the new link.
func stepRepointSymlink(linkPath, newTarget, origTarget string) fileStep {
	return fileStep{
		desc: "repoint symlink " + linkPath,
		do: func() error {
			return atomicSymlink(newTarget, linkPath)
		},
		undo: func() error {
			if origTarget == "" {
				if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
					return err
				}
				return nil
			}
			return atomicSymlink(origTarget, linkPath)
		},
	}
}

// pathExists reports whether path exists (regular file, dir, or symlink).
// It uses Lstat so it doesn't follow symlinks.
func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
