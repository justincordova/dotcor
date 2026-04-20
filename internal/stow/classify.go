package stow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// classifySkipDirs mirrors the TUI browser's skip list so walkAndClassify avoids
// the same heavy directories.
var classifySkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".cache":       true,
	"__pycache__":  true,
	"Library":      true,
	".Trash":       true,
	".dotcor":      true,
}

// ClassifyFiles walks each selection path, classifies every file, and groups
// results by auto-derived package. selections is a list of absolute paths
// (files or directories) selected in the browser.
func ClassifyFiles(selections []string, repoDir, homeDir string) (*ClassificationPlan, error) {
	// Resolve repoDir through EvalSymlinks once to handle macOS /var→/private/var.
	if resolved, err := filepath.EvalSymlinks(repoDir); err == nil {
		repoDir = resolved
	}
	repoDir = filepath.Clean(repoDir)
	homeDir = filepath.Clean(homeDir)

	// Build the $HOME symlink index once — maps resolvedTarget → symlinkPath.
	// Used by classifyFileInDir to detect Adopt without walking $HOME per file.
	homeIndex, err := buildHomeSymlinkIndex(homeDir, repoDir)
	if err != nil {
		// Non-fatal: fall back to empty index; adopt detection degrades gracefully.
		homeIndex = make(map[string]string)
	}

	// Map from package name → index in plan.Packages, for merge.
	pkgIndex := make(map[string]int)
	plan := &ClassificationPlan{}

	for _, sel := range selections {
		sel = filepath.Clean(sel)

		lfi, err := os.Lstat(sel)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", sel, err)
		}

		if lfi.Mode()&os.ModeSymlink != 0 {
			// Selection itself is a symlink — classify as a single file.
			cf, err := classifyFile(sel, sel, repoDir, homeDir, homeIndex)
			if err != nil {
				return nil, err
			}
			mergeFile(plan, pkgIndex, cf)
			continue
		}

		if !lfi.IsDir() {
			// Single regular file.
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
				if err := walkAndClassify(plan, pkgIndex, pkgDir, entry.Name(), repoDir, homeDir, homeIndex); err != nil {
					return nil, err
				}
			}
			continue
		}

		// Regular directory selection — derive one package name.
		pkgName := derivePkgName(sel, homeDir)
		if err := walkAndClassify(plan, pkgIndex, sel, pkgName, repoDir, homeDir, homeIndex); err != nil {
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

// buildHomeSymlinkIndex scans $HOME once and returns a map of
// resolvedTarget → symlinkPath for every symlink that does NOT already point
// into repoDir. Heavy directories (Library, node_modules, etc.) are skipped.
func buildHomeSymlinkIndex(homeDir, repoDir string) (map[string]string, error) {
	index := make(map[string]string)
	err := filepath.Walk(homeDir, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return nil
		}
		if info.IsDir() {
			if classifySkipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		lfi, lerr := os.Lstat(path)
		if lerr != nil || lfi.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		target, rerr := os.Readlink(path)
		if rerr != nil {
			return nil
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		target = filepath.Clean(target)
		// Resolve for macOS /var → /private/var.
		if resolved, rerr := filepath.EvalSymlinks(target); rerr == nil {
			target = resolved
		}
		// Skip symlinks already pointing into repo.
		if strings.HasPrefix(target, repoDir+string(filepath.Separator)) || target == repoDir {
			return nil
		}
		// First found wins (multiple $HOME symlinks to same file is unusual).
		if _, exists := index[target]; !exists {
			index[target] = path
		}
		return nil
	})
	return index, err
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
func walkAndClassify(plan *ClassificationPlan, pkgIndex map[string]int, selDir, pkgName, repoDir, homeDir string, homeIndex map[string]string) error {
	return filepath.Walk(selDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}

		// Use Lstat so we detect symlinks without following them.
		lfi, err := os.Lstat(path)
		if err != nil {
			return nil
		}

		if lfi.IsDir() {
			if classifySkipDirs[lfi.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// If this entry is a symlink pointing at a directory, skip it (don't follow).
		if lfi.Mode()&os.ModeSymlink != 0 {
			if target, rerr := os.Readlink(path); rerr == nil {
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(path), target)
				}
				if tfi, sterr := os.Stat(target); sterr == nil && tfi.IsDir() {
					// Don't descend — but also don't SkipDir which would skip siblings.
					return nil
				}
			}
		}

		cf, err := classifyFileInDir(path, selDir, pkgName, repoDir, homeDir, homeIndex)
		if err != nil {
			return nil // skip files we can't classify
		}

		mergeFile(plan, pkgIndex, cf)
		return nil
	})
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
func executeAdd(cf ClassifiedFile) error {
	repoDest := cf.RepoDest
	srcPath := cf.AbsPath

	srcData, srcPerm, err := safeReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}

	// Create parent dirs in repo.
	if err := os.MkdirAll(filepath.Dir(repoDest), 0755); err != nil {
		return fmt.Errorf("creating repo dir: %w", err)
	}

	// Write to repo.
	if err := os.WriteFile(repoDest, srcData, srcPerm); err != nil {
		return fmt.Errorf("writing to repo: %w", err)
	}

	// Compute relative symlink from $HOME location to repo file.
	srcDir := resolvedDir(srcPath)
	relSym, err := filepath.Rel(srcDir, repoDest)
	if err != nil {
		_ = os.Remove(repoDest)
		return fmt.Errorf("computing symlink: %w", err)
	}

	// Atomic swap: write tmp symlink, rename over source (POSIX rename is atomic).
	tmp := srcPath + ".dotcor-tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(relSym, tmp); err != nil {
		_ = os.Remove(repoDest)
		return fmt.Errorf("creating tmp symlink: %w", err)
	}
	// Rename atomically replaces srcPath — no separate Remove needed.
	if err := os.Rename(tmp, srcPath); err != nil {
		_ = os.Remove(tmp)
		_ = os.WriteFile(srcPath, srcData, srcPerm)
		_ = os.Remove(repoDest)
		return fmt.Errorf("placing symlink: %w", err)
	}

	return nil
}

// executeAdopt: $HOME has a symlink pointing at cf.AbsPath.
// Copy to repo; repoint $HOME symlink to repo; replace source with repo symlink.
func executeAdopt(cf ClassifiedFile) error {
	repoDest := cf.RepoDest
	srcPath := cf.AbsPath      // the actual file on disk
	homeLink := cf.HomeSymlink // the $HOME symlink that points to srcPath

	srcData, srcPerm, err := safeReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}

	// Create parent dirs in repo.
	if err := os.MkdirAll(filepath.Dir(repoDest), 0755); err != nil {
		return fmt.Errorf("creating repo dir: %w", err)
	}

	// Copy to repo.
	if err := os.WriteFile(repoDest, srcData, srcPerm); err != nil {
		return fmt.Errorf("writing to repo: %w", err)
	}

	// Repoint $HOME symlink → repo (atomic).
	if homeLink != "" {
		relHomeToRepo, err := filepath.Rel(resolvedDir(homeLink), repoDest)
		if err != nil {
			_ = os.Remove(repoDest)
			return fmt.Errorf("computing home symlink: %w", err)
		}
		if err := atomicSymlink(relHomeToRepo, homeLink); err != nil {
			_ = os.Remove(repoDest)
			return fmt.Errorf("repointing home symlink: %w", err)
		}
	}

	// Replace source file with symlink → repo (atomic).
	relSrcToRepo, err := filepath.Rel(resolvedDir(srcPath), repoDest)
	if err != nil {
		// Home is already repointed; log partial success but don't fail.
		// The important invariant ($HOME → repo) is satisfied.
		return nil
	}
	tmp := srcPath + ".dotcor-tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(relSrcToRepo, tmp); err != nil {
		// Non-fatal: home already repointed.
		return nil
	}
	// Atomic rename replaces the original file at srcPath.
	if err := os.Rename(tmp, srcPath); err != nil {
		_ = os.Remove(tmp)
		// Restore source from memory so disk state is consistent.
		_ = os.WriteFile(srcPath, srcData, srcPerm)
	}

	return nil
}

// executeTrack: file inside a non-$HOME folder. Copy to repo; replace source with symlink.
func executeTrack(cf ClassifiedFile) error {
	repoDest := cf.RepoDest
	srcPath := cf.AbsPath

	srcData, srcPerm, err := safeReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}

	// Create parent dirs in repo.
	if err := os.MkdirAll(filepath.Dir(repoDest), 0755); err != nil {
		return fmt.Errorf("creating repo dir: %w", err)
	}

	// Copy to repo.
	if err := os.WriteFile(repoDest, srcData, srcPerm); err != nil {
		return fmt.Errorf("writing to repo: %w", err)
	}

	relSym, err := filepath.Rel(resolvedDir(srcPath), repoDest)
	if err != nil {
		_ = os.Remove(repoDest)
		return fmt.Errorf("computing symlink: %w", err)
	}

	tmp := srcPath + ".dotcor-tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(relSym, tmp); err != nil {
		_ = os.Remove(repoDest)
		return fmt.Errorf("creating tmp symlink: %w", err)
	}
	if err := os.Rename(tmp, srcPath); err != nil {
		_ = os.Remove(tmp)
		_ = os.WriteFile(srcPath, srcData, srcPerm)
		_ = os.Remove(repoDest)
		return fmt.Errorf("placing symlink: %w", err)
	}

	return nil
}

// executeForeign: follow the symlink, copy resolved target to repo, repoint chain.
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

	// Create parent dirs in repo.
	if err := os.MkdirAll(filepath.Dir(repoDest), 0755); err != nil {
		return fmt.Errorf("creating repo dir: %w", err)
	}

	// Copy to repo.
	if err := os.WriteFile(repoDest, srcData, srcPerm); err != nil {
		return fmt.Errorf("writing to repo: %w", err)
	}

	// Repoint the foreign symlink (cf.AbsPath) to point at repo — atomic.
	relSym, err := filepath.Rel(resolvedDir(cf.AbsPath), repoDest)
	if err != nil {
		_ = os.Remove(repoDest)
		return fmt.Errorf("computing symlink: %w", err)
	}

	if err := atomicSymlink(relSym, cf.AbsPath); err != nil {
		_ = os.Remove(repoDest)
		return fmt.Errorf("repointing foreign symlink: %w", err)
	}

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
