package stow

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justincordova/dotcor/internal/core"
)

// Class represents how a file is classified relative to $HOME and repoDir.
type Class int

const (
	// ClassAdd: regular file directly in $HOME (depth 1). On execute: move to
	// repo and create a $HOME symlink.
	ClassAdd Class = iota

	// ClassAdopt: $HOME has a symlink pointing at this file. On execute: copy
	// to repo, repoint the $HOME symlink, replace source file with symlink into repo.
	ClassAdopt

	// ClassTrack: file is inside a non-$HOME folder with no $HOME connection.
	// On execute: copy to repo, replace source with a symlink into repo.
	// No new $HOME symlink is created.
	ClassTrack

	// ClassForeign: the file itself is a symlink pointing outside repo and
	// outside the selected folder. Default OFF. On execute (if toggled ON):
	// follow the symlink, adopt the resolved target.
	ClassForeign

	// ClassManaged: file is already a symlink pointing into repo.
	// Never modified; shown dim/locked in the preview.
	ClassManaged
)

func (c Class) String() string {
	switch c {
	case ClassAdd:
		return "ADD"
	case ClassAdopt:
		return "ADOPT"
	case ClassTrack:
		return "TRACK"
	case ClassForeign:
		return "FOREIGN"
	case ClassManaged:
		return "MANAGED"
	default:
		return "UNKNOWN"
	}
}

// ClassifiedFile holds the classification result for a single file.
type ClassifiedFile struct {
	// RelPath is the file's path relative to the package root (repo destination).
	RelPath string
	// AbsPath is the absolute path of the file on disk (the source).
	AbsPath string
	// Class is the classification for this file.
	Class Class
	// PackageName is the auto-derived package this file belongs to.
	PackageName string
	// RepoDest is the proposed repo destination path.
	RepoDest string
	// HomeSymlink is the existing $HOME symlink path that points to this file
	// (populated for ClassAdopt). Empty otherwise.
	HomeSymlink string
	// ForeignTarget is the resolved target of a foreign symlink (ClassForeign).
	ForeignTarget string
}

// PackagePlan groups all classified files for one package.
type PackagePlan struct {
	Name  string
	Files []ClassifiedFile
}

// ClassificationPlan is the top-level result returned by ClassifyFiles.
type ClassificationPlan struct {
	Packages []PackagePlan
	// Filtered lists files that were skipped because they matched an ignore
	// pattern. Surfaced in the preview so the user understands why expected
	// files are missing.
	Filtered []FilteredFile
	// Warnings lists non-fatal issues encountered while building the plan
	// (e.g. failure to walk $HOME for the symlink index, leading to degraded
	// adopt detection). Surfaced in the preview header.
	Warnings []string
}

// FilteredFile records a file skipped by ignore patterns and which pattern matched.
type FilteredFile struct {
	AbsPath string
	Pattern string
}

// ClassificationResult is returned by ExecuteClassification.
type ClassificationResult struct {
	Added    int
	Adopted  int
	Tracked  int
	Foreign  int
	Managed  int
	Failures []ClassificationFailure
}

// ClassificationFailure records a single file that failed during execution.
type ClassificationFailure struct {
	RelPath     string
	PackageName string
	Err         error
}

// maxFileSizeBytes is the maximum file size we'll copy in one shot.
// Files larger than this are refused with a clear error rather than OOMing.
const maxFileSizeBytes = 100 * 1024 * 1024 // 100 MiB

// fileID returns a stable string key for a file in a package, used as toggle key.
func fileID(pkgName, relPath string) string {
	return pkgName + "/" + relPath
}

// walkSkipDirs is a minimal skip list used when walking user-selected
// directories the user might legitimately want to manage (like .config, .ssh, .aws).
var walkSkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".cache":       true,
	"__pycache__":  true,
	".Trash":       true,
	".dotcor":      true,
}

// ClassifyFiles walks each selection path, classifies every file, and groups
// results by auto-derived package. selections is a list of absolute paths
// (files or directories) selected in the browser. ignorePatterns, if non-nil,
// causes any file matching a pattern (via core.ShouldIgnore) to be recorded in
// plan.Filtered and omitted from every package.
func ClassifyFiles(selections []string, repoDir, homeDir string, ignorePatterns []string) (*ClassificationPlan, error) {
	// Resolve repoDir through EvalSymlinks once to handle macOS /var→/private/var.
	if resolved, err := filepath.EvalSymlinks(repoDir); err == nil {
		repoDir = resolved
	}
	repoDir = filepath.Clean(repoDir)
	homeDir = filepath.Clean(homeDir)

	// Map from package name → index in plan.Packages, for merge.
	pkgIndex := make(map[string]int)
	plan := &ClassificationPlan{}

	// Build the $HOME symlink index once — maps resolvedTarget → symlinkPath.
	// Used by classifyFileInDir to detect Adopt without walking $HOME per file.
	// On error, fall back to empty index AND surface a warning so the user
	// knows adopt detection is degraded — without this, files that should
	// classify as Adopt silently become Add (which moves the source file
	// and breaks any external tool owning the original target).
	homeIndex, err := buildHomeSymlinkIndex(homeDir, repoDir, selections)
	if err != nil {
		homeIndex = make(map[string]string)
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("$HOME symlink index unavailable (%v) — adopt detection disabled; files pointing into the repo via existing $HOME symlinks may be classified as Add instead", err))
		slog.Default().Warn("classify: home symlink index unavailable",
			"err", err, "homeDir", homeDir)
	}

	for _, sel := range selections {
		sel = filepath.Clean(sel)

		lfi, err := os.Lstat(sel)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", sel, err)
		}

		if lfi.Mode()&os.ModeSymlink != 0 {
			// Selection itself is a symlink — classify as a single file.
			if matched, pattern := matchIgnore(sel, ignorePatterns); matched {
				plan.Filtered = append(plan.Filtered, FilteredFile{AbsPath: sel, Pattern: pattern})
				continue
			}
			cf, err := classifyFile(sel, sel, repoDir, homeDir, homeIndex)
			if err != nil {
				return nil, err
			}
			mergeFile(plan, pkgIndex, cf)
			continue
		}

		if !lfi.IsDir() {
			// Single regular file.
			if matched, pattern := matchIgnore(sel, ignorePatterns); matched {
				plan.Filtered = append(plan.Filtered, FilteredFile{AbsPath: sel, Pattern: pattern})
				continue
			}
			cf, err := classifyFile(sel, sel, repoDir, homeDir, homeIndex)
			if err != nil {
				return nil, err
			}
			mergeFile(plan, pkgIndex, cf)
			continue
		}

		// It's a directory. Check for Stow-parent split.
		if isStowParent(sel) {
			entries, err := os.ReadDir(sel)
			if err != nil {
				return nil, fmt.Errorf("reading %q: %w", sel, err)
			}
			for _, entry := range entries {
				if !entry.IsDir() || isExcluded(entry.Name()) {
					continue
				}
				pkgDir := filepath.Join(sel, entry.Name())
				if err := walkAndClassify(plan, pkgIndex, pkgDir, entry.Name(), repoDir, homeDir, homeIndex, ignorePatterns); err != nil {
					return nil, err
				}
			}
			continue
		}

		// Regular directory selection — derive one package name.
		pkgName := derivePkgName(sel, homeDir)
		if err := walkAndClassify(plan, pkgIndex, sel, pkgName, repoDir, homeDir, homeIndex, ignorePatterns); err != nil {
			return nil, err
		}
	}

	// Sort packages by name for stable rendering.
	sort.Slice(plan.Packages, func(i, j int) bool {
		return plan.Packages[i].Name < plan.Packages[j].Name
	})
	// Sort files within each package by RelPath.
	for i := range plan.Packages {
		sort.Slice(plan.Packages[i].Files, func(a, b int) bool {
			return plan.Packages[i].Files[a].RelPath < plan.Packages[i].Files[b].RelPath
		})
	}

	return plan, nil
}

// buildHomeSymlinkIndex builds an index of symlinks in $HOME that point
// outside repoDir. Instead of walking the entire $HOME tree (which is
// extremely slow on real machines), it only scans the depth-1 entries
// under $HOME plus depth-1 entries under each selection's parent. This
// catches the common Adopt case ($HOME symlink → external file) without
// scanning .npm, .nvm, .local, etc.
func buildHomeSymlinkIndex(homeDir, repoDir string, selections []string) (map[string]string, error) {
	index := make(map[string]string)

	dirs := map[string]bool{homeDir: true}
	for _, sel := range selections {
		sel = filepath.Clean(sel)
		rel, err := filepath.Rel(homeDir, sel)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) >= 1 && parts[0] != "." {
			dirs[filepath.Join(homeDir, parts[0])] = true
		}
	}

	for dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			lfi, lerr := os.Lstat(path)
			if lerr != nil || lfi.Mode()&os.ModeSymlink == 0 {
				continue
			}
			target, rerr := os.Readlink(path)
			if rerr != nil {
				continue
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			target = filepath.Clean(target)
			if resolved, rerr := filepath.EvalSymlinks(target); rerr == nil {
				target = resolved
			}
			if strings.HasPrefix(target, repoDir+string(filepath.Separator)) || target == repoDir {
				continue
			}
			if _, exists := index[target]; !exists {
				index[target] = path
			}
		}
	}

	return index, nil
}

// isStowParent returns true when every non-excluded direct child of dir is
// itself a directory (GNU Stow parent layout).
func isStowParent(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	hasDirs := false
	for _, e := range entries {
		if isExcluded(e.Name()) {
			continue
		}
		lfi, err := os.Lstat(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if lfi.Mode()&os.ModeSymlink != 0 {
			// Symlinks at top level → not a pure Stow parent.
			return false
		}
		if !e.IsDir() {
			return false
		}
		hasDirs = true
	}
	return hasDirs
}

// derivePkgName auto-derives a package name from a selection path.
func derivePkgName(sel, homeDir string) string {
	rel, err := filepath.Rel(homeDir, sel)
	if err != nil || strings.HasPrefix(rel, "..") {
		// Outside $HOME — use leaf name.
		return filepath.Base(sel)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) == 1 && parts[0] != "." {
		// Direct child of $HOME — use the name without leading dot.
		name := strings.TrimPrefix(parts[0], ".")
		if name == "" {
			return parts[0]
		}
		return name
	}
	if len(parts) == 0 || parts[0] == "." {
		return "home"
	}
	// Loose $HOME file → "home"; deep path → leaf name.
	return filepath.Base(sel)
}

// derivePkgNameForFile returns the package name for a single file in $HOME.
// Files directly in $HOME go to "home".
func derivePkgNameForFile(absPath, homeDir string) string {
	rel, err := filepath.Rel(homeDir, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.Base(filepath.Dir(absPath))
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) == 1 {
		// Direct child of $HOME (e.g. ~/.bashrc) → package "home".
		return "home"
	}
	// Inside a subdirectory — use the first directory component.
	name := strings.TrimPrefix(parts[0], ".")
	if name == "" {
		return parts[0]
	}
	return name
}

// walkAndClassify walks selDir and classifies every file into pkgName.
// Files matching any pattern in ignorePatterns are recorded in plan.Filtered
// and skipped.
func walkAndClassify(plan *ClassificationPlan, pkgIndex map[string]int, selDir, pkgName, repoDir, homeDir string, homeIndex map[string]string, ignorePatterns []string) error {
	return filepath.WalkDir(selDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if d.IsDir() {
			if walkSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		lfi, err := os.Lstat(path)
		if err != nil {
			return nil
		}

		if lfi.Mode()&os.ModeSymlink != 0 {
			if target, rerr := os.Readlink(path); rerr == nil {
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(path), target)
				}
				if tfi, sterr := os.Stat(target); sterr == nil && tfi.IsDir() {
					return nil
				}
			}
		}

		if matched, pattern := matchIgnore(path, ignorePatterns); matched {
			plan.Filtered = append(plan.Filtered, FilteredFile{AbsPath: path, Pattern: pattern})
			return nil
		}

		cf, err := classifyFileInDir(path, selDir, pkgName, repoDir, homeDir, homeIndex)
		if err != nil {
			return nil
		}

		mergeFile(plan, pkgIndex, cf)
		return nil
	})
}

// matchIgnore returns (true, pattern) if path matches any ignore pattern.
// Empty/nil patterns means nothing is filtered.
func matchIgnore(path string, patterns []string) (bool, string) {
	if len(patterns) == 0 {
		return false, ""
	}
	return core.ShouldIgnore(path, patterns)
}

// classifyFile classifies a single file (not inside a walked directory).
// selPath is used to derive the package name.
func classifyFile(path, selPath, repoDir, homeDir string, homeIndex map[string]string) (ClassifiedFile, error) {
	pkgName := derivePkgNameForFile(path, homeDir)
	return classifyFileInDir(path, filepath.Dir(selPath), pkgName, repoDir, homeDir, homeIndex)
}

// classifyFileInDir classifies a file given the walk root (selDir) and package name.
// relPath within the package is computed relative to selDir.
func classifyFileInDir(absPath, selDir, pkgName, repoDir, homeDir string, homeIndex map[string]string) (ClassifiedFile, error) {
	lfi, err := os.Lstat(absPath)
	if err != nil {
		return ClassifiedFile{}, fmt.Errorf("lstat %q: %w", absPath, err)
	}

	// Resolve absPath and selDir via EvalSymlinks for consistent path comparison.
	// We keep the raw absPath for file removal operations (Remove needs the original
	// symlink path when the entry is a symlink) but use resolvedAbs for computing
	// relative symlink targets and repo paths.
	resolvedAbs := absPath
	if resolved, rerr := filepath.EvalSymlinks(filepath.Dir(absPath)); rerr == nil {
		resolvedAbs = filepath.Join(resolved, filepath.Base(absPath))
	}
	resolvedSelDir := selDir
	if resolved, rerr := filepath.EvalSymlinks(selDir); rerr == nil {
		resolvedSelDir = resolved
	}

	relPath, relErr := filepath.Rel(resolvedSelDir, resolvedAbs)
	if relErr != nil || strings.HasPrefix(relPath, "..") {
		// Fall back using unresolved paths.
		relPath, relErr = filepath.Rel(selDir, absPath)
		if relErr != nil || strings.HasPrefix(relPath, "..") {
			// Last resort: use basename with a content hash prefix to avoid collisions
			// when multiple files share the same name across subdirs.
			relPath = filepath.Base(absPath)
		}
	}

	repoDest := filepath.Join(repoDir, pkgName, relPath)

	cf := ClassifiedFile{
		RelPath:     relPath,
		AbsPath:     absPath,
		PackageName: pkgName,
		RepoDest:    repoDest,
	}

	isSymlink := lfi.Mode()&os.ModeSymlink != 0

	if isSymlink {
		target, err := os.Readlink(absPath)
		if err != nil {
			return cf, fmt.Errorf("readlink %q: %w", absPath, err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(absPath), target)
		}
		target = filepath.Clean(target)

		// Resolve through EvalSymlinks to handle macOS /var → /private/var.
		if resolved, rerr := filepath.EvalSymlinks(target); rerr == nil {
			target = resolved
		}

		// Check if it points into repo → Managed.
		if strings.HasPrefix(target, repoDir+string(filepath.Separator)) || target == repoDir {
			cf.Class = ClassManaged
			return cf, nil
		}

		// Foreign symlink (points outside repo and outside the walked tree).
		cf.Class = ClassForeign
		cf.ForeignTarget = target
		return cf, nil
	}

	// Regular file. Determine classification by checking $HOME for a symlink
	// that points to this file.
	homeRel, err := filepath.Rel(homeDir, absPath)
	if err == nil && !strings.HasPrefix(homeRel, "..") {
		// File is inside $HOME.
		homePath := filepath.Join(homeDir, homeRel)
		if homePath == absPath {
			// File IS a direct $HOME file.
			cf.Class = ClassAdd
			return cf, nil
		}
	}

	// Check the pre-built index: does any $HOME symlink point at this file?
	resolvedForIndex := resolvedAbs
	if homeSymlink, found := homeIndex[resolvedForIndex]; found {
		cf.Class = ClassAdopt
		cf.HomeSymlink = homeSymlink
		return cf, nil
	}

	// No $HOME connection → Track.
	cf.Class = ClassTrack
	return cf, nil
}

// mergeFile inserts cf into the correct PackagePlan, creating one if needed.
func mergeFile(plan *ClassificationPlan, pkgIndex map[string]int, cf ClassifiedFile) {
	idx, ok := pkgIndex[cf.PackageName]
	if !ok {
		plan.Packages = append(plan.Packages, PackagePlan{Name: cf.PackageName})
		idx = len(plan.Packages) - 1
		pkgIndex[cf.PackageName] = idx
	}
	plan.Packages[idx].Files = append(plan.Packages[idx].Files, cf)
}

// ExecuteClassification runs the per-file operation for each ON row in the plan.
// toggles is keyed by fileID(pkgName, relPath). A missing key means "use default".
// Default for ClassManaged is always OFF (never executed).
// Default for ClassForeign is OFF. All others default ON.
func ExecuteClassification(plan *ClassificationPlan, toggles map[string]bool, repoDir, homeDir string) (*ClassificationResult, error) {
	// repoDir and homeDir are not used directly in execute functions — all path
	// information is already embedded in the ClassifiedFile fields populated by
	// ClassifyFiles (which resolves both dirs itself). The parameters are kept
	// in the signature for forward-compatibility.
	_, _ = repoDir, homeDir

	result := &ClassificationResult{}

	for _, pkg := range plan.Packages {
		// Sort for stable execution order.
		files := make([]ClassifiedFile, len(pkg.Files))
		copy(files, pkg.Files)
		sort.Slice(files, func(i, j int) bool {
			return files[i].RelPath < files[j].RelPath
		})

		for _, cf := range files {
			id := fileID(pkg.Name, cf.RelPath)

			// Determine effective toggle state.
			var on bool
			switch cf.Class {
			case ClassManaged:
				// Never executed.
				result.Managed++
				continue
			case ClassForeign:
				on = toggles[id] // default false
			default:
				if v, exists := toggles[id]; exists {
					on = v
				} else {
					on = true // default ON
				}
			}

			if !on {
				continue
			}

			var execErr error
			switch cf.Class {
			case ClassAdd:
				execErr = executeAdd(cf)
				if execErr == nil {
					result.Added++
				}
			case ClassAdopt:
				execErr = executeAdopt(cf)
				if execErr == nil {
					result.Adopted++
				}
			case ClassTrack:
				execErr = executeTrack(cf)
				if execErr == nil {
					result.Tracked++
				}
			case ClassForeign:
				execErr = executeForeign(cf)
				if execErr == nil {
					result.Foreign++
				}
			}

			if execErr != nil {
				result.Failures = append(result.Failures, ClassificationFailure{
					RelPath:     cf.RelPath,
					PackageName: pkg.Name,
					Err:         execErr,
				})
			}
		}
	}

	return result, nil
}

// resolvedDir returns the EvalSymlinks-resolved directory of a path, or the
// original if resolution fails.
func resolvedDir(path string) string {
	dir := filepath.Dir(path)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return dir
}

// safeReadFile reads a file, refusing to load anything larger than maxFileSizeBytes.
func safeReadFile(path string) ([]byte, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("stat %q: %w", path, err)
	}
	if info.Size() > maxFileSizeBytes {
		return nil, 0, fmt.Errorf("file too large (%d bytes, max %d): %q", info.Size(), maxFileSizeBytes, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("reading %q: %w", path, err)
	}
	return data, info.Mode().Perm(), nil
}

// atomicSymlink creates a symlink at dst atomically using a tmp+rename.
// Any pre-existing .dotcor-tmp file is removed first to handle crashed prior runs.
func atomicSymlink(target, dst string) error {
	tmp := dst + ".dotcor-tmp"
	_ = os.Remove(tmp) // clean up any leftover from a prior crashed run
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("creating tmp symlink: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("placing symlink: %w", err)
	}
	return nil
}

// executeAdd: file lives in $HOME; move to repo, create $HOME symlink.
// The file IS already at its $HOME location (AbsPath is in $HOME).
//
// Sequenced as a fileTxn so any failure rolls every prior step back: if
// the symlink swap fails, the repo copy is removed and the original file
// remains in place. If the WriteFile to repo fails, no $HOME mutation has
// happened yet so there's nothing to undo on the source side.
func executeAdd(cf ClassifiedFile) error {
	repoDest := cf.RepoDest
	srcPath := cf.AbsPath

	srcData, srcPerm, err := safeReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}

	relSym, err := filepath.Rel(resolvedDir(srcPath), repoDest)
	if err != nil {
		return fmt.Errorf("computing symlink: %w", err)
	}

	txn := &fileTxn{}
	if err := txn.run(stepWriteFile(repoDest, srcData, srcPerm)); err != nil {
		return err
	}
	if err := txn.run(stepReplaceFileWithSymlink(srcPath, relSym, srcData, srcPerm)); err != nil {
		return err
	}
	txn.commit()
	return nil
}

// executeAdopt: $HOME has a symlink pointing at cf.AbsPath.
// Copy to repo; repoint $HOME symlink to repo; replace source with repo symlink.
//
// Issue #11 background: the previous implementation returned nil silently
// when the post-home-repoint steps (Rel, Symlink, Rename of source) failed,
// leaving srcPath as a regular file disconnected from $HOME. The fix:
//
//   - Capture the original $HOME symlink target so a failed repoint can be
//     undone (transaction rollback).
//   - Surface the "home repointed but source not relinked" case as an
//     error rather than swallowing it. The caller records this in
//     ClassificationFailure so the user sees what happened.
//   - Do not roll back the $HOME repoint on source-relink failure — that's
//     the intended end state ($HOME → repo). Only the source-side mutation
//     is partial.
func executeAdopt(cf ClassifiedFile) error {
	repoDest := cf.RepoDest
	srcPath := cf.AbsPath      // the actual file on disk
	homeLink := cf.HomeSymlink // the $HOME symlink that points to srcPath

	srcData, srcPerm, err := safeReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}

	// Capture the pre-adopt $HOME symlink target so a failed repoint can
	// be unwound. Empty origHomeTarget signals "no link existed" — undo
	// will remove the new link in that case.
	var origHomeTarget string
	if homeLink != "" {
		if t, rerr := os.Readlink(homeLink); rerr == nil {
			origHomeTarget = t
		}
	}

	txn := &fileTxn{}

	// Step 1: copy to repo.
	if err := txn.run(stepWriteFile(repoDest, srcData, srcPerm)); err != nil {
		return err
	}

	// Step 2: repoint the $HOME symlink → repo. Failure unwinds step 1.
	if homeLink != "" {
		relHomeToRepo, err := filepath.Rel(resolvedDir(homeLink), repoDest)
		if err != nil {
			_ = txn.rollback()
			return fmt.Errorf("computing home symlink: %w", err)
		}
		if err := txn.run(stepRepointSymlink(homeLink, relHomeToRepo, origHomeTarget)); err != nil {
			return err
		}
	}

	// At this point $HOME → repo is the desired end state. The source-side
	// relink (replacing srcPath with a symlink to repo) is a separate unit
	// of work — its failure is reported but we do NOT undo the home
	// repoint, because $HOME → repo is what we wanted.
	relSrcToRepo, err := filepath.Rel(resolvedDir(srcPath), repoDest)
	if err != nil {
		txn.commit()
		return fmt.Errorf("home repointed but source not relinked at %s: %w", srcPath, err)
	}
	if err := stepReplaceFileWithSymlink(srcPath, relSrcToRepo, srcData, srcPerm).do(); err != nil {
		txn.commit()
		return fmt.Errorf("home repointed but source not relinked at %s: %w", srcPath, err)
	}

	txn.commit()
	return nil
}

// executeTrack: file inside a non-$HOME folder. Copy to repo; replace
// source with symlink. Same shape as executeAdd; both go through fileTxn
// so any step failing rolls back to the pre-state.
func executeTrack(cf ClassifiedFile) error {
	repoDest := cf.RepoDest
	srcPath := cf.AbsPath

	srcData, srcPerm, err := safeReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}

	relSym, err := filepath.Rel(resolvedDir(srcPath), repoDest)
	if err != nil {
		return fmt.Errorf("computing symlink: %w", err)
	}

	txn := &fileTxn{}
	if err := txn.run(stepWriteFile(repoDest, srcData, srcPerm)); err != nil {
		return err
	}
	if err := txn.run(stepReplaceFileWithSymlink(srcPath, relSym, srcData, srcPerm)); err != nil {
		return err
	}
	txn.commit()
	return nil
}

// executeForeign: follow the symlink, copy resolved target to repo, repoint
// chain. cf.AbsPath is the symlink (in $HOME or a tracked folder); the
// resolved target is somewhere external. The original symlink target is
// captured for rollback so a failed repoint restores the link unchanged.
func executeForeign(cf ClassifiedFile) error {
	repoDest := cf.RepoDest
	resolvedTarget := cf.ForeignTarget

	if resolvedTarget == "" {
		return fmt.Errorf("no resolved target for foreign symlink")
	}

	srcData, srcPerm, err := safeReadFile(resolvedTarget)
	if err != nil {
		return fmt.Errorf("reading foreign target: %w", err)
	}

	relSym, err := filepath.Rel(resolvedDir(cf.AbsPath), repoDest)
	if err != nil {
		return fmt.Errorf("computing symlink: %w", err)
	}

	// Capture the original symlink target so rollback can restore it
	// untouched. The link is guaranteed to exist (we classified it as a
	// symlink) — but a Readlink failure shouldn't abort the whole op,
	// just leave undo unable to restore the prior state.
	var origTarget string
	if t, rerr := os.Readlink(cf.AbsPath); rerr == nil {
		origTarget = t
	}

	txn := &fileTxn{}
	if err := txn.run(stepWriteFile(repoDest, srcData, srcPerm)); err != nil {
		return err
	}
	if err := txn.run(stepRepointSymlink(cf.AbsPath, relSym, origTarget)); err != nil {
		return err
	}
	txn.commit()
	return nil
}

// DefaultToggle returns the default toggle state for a classified file.
func DefaultToggle(class Class) bool {
	switch class {
	case ClassForeign, ClassManaged:
		return false
	default:
		return true
	}
}

// BuildDefaultToggles builds the default toggles map from a ClassificationPlan.
func BuildDefaultToggles(plan *ClassificationPlan) map[string]bool {
	toggles := make(map[string]bool)
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			toggles[fileID(pkg.Name, cf.RelPath)] = DefaultToggle(cf.Class)
		}
	}
	return toggles
}

// FileID returns the stable string key for a file, for use as a toggle map key.
func FileID(pkgName, relPath string) string {
	return fileID(pkgName, relPath)
}

// copyToggles returns a shallow copy of a toggles map. Use before passing to a
// goroutine to avoid concurrent map read/write races.
func copyToggles(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// CopyToggles is the exported wrapper for use from the tui package.
func CopyToggles(src map[string]bool) map[string]bool {
	return copyToggles(src)
}
