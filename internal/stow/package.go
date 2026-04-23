package stow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PackageStatus int

const (
	StatusLinked PackageStatus = iota
	StatusPartial
	StatusUnlinked
)

type Package struct {
	Name    string
	Path    string
	Files   []FileEntry
	Status  PackageStatus
	ModTime time.Time
}

type FileEntry struct {
	RelPath    string
	TargetPath string
	IsLinked   bool
	Exists     bool
	IsSymlink  bool
	InRepo     bool
}

type LinkResult struct {
	Linked    int
	Skipped   int
	Conflicts []string
	// Foreign lists $HOME paths that are symlinks pointing outside repoDir.
	// Link deliberately does not adopt these — the user must confirm via the
	// explicit Add/Adopt flow so external symlinks are never silently rewritten.
	Foreign []string
}

type UnlinkResult struct {
	Unlinked int
	Removed  int
}

var excludedDirs = map[string]bool{
	".git":               true,
	"logs":               true,
	"backups":            true,
	".stow-local-ignore": true,
	".dotcorrc":          true,
}

func isExcluded(name string) bool {
	if excludedDirs[name] {
		return true
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	return false
}

func DiscoverPackages(repoDir, homeDir string) ([]Package, error) {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, fmt.Errorf("reading repo directory: %w", err)
	}

	var packages []Package
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if isExcluded(entry.Name()) {
			continue
		}

		pkgPath := filepath.Join(repoDir, entry.Name())
		files, err := discoverFiles(pkgPath, homeDir)
		if err != nil {
			return nil, fmt.Errorf("discovering files for package %s: %w", entry.Name(), err)
		}

		files = appendAutoDetected(files, pkgPath, homeDir)

		pkg := Package{
			Name:    entry.Name(),
			Path:    pkgPath,
			Files:   files,
			Status:  computeStatus(files),
			ModTime: packageModTime(pkgPath),
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}

func discoverFiles(pkgDir, homeDir string) ([]FileEntry, error) {
	var files []FileEntry

	err := filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(pkgDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}

		targetPath := filepath.Join(homeDir, relPath)

		entry := FileEntry{
			RelPath:    relPath,
			TargetPath: targetPath,
			InRepo:     true,
		}

		targetInfo, err := os.Lstat(targetPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("checking target path %s: %w", targetPath, err)
			}
		} else {
			entry.Exists = true
			entry.IsSymlink = targetInfo.Mode()&os.ModeSymlink != 0

			if entry.IsSymlink {
				linkTarget, err := os.Readlink(targetPath)
				if err == nil {
					var resolved string
					if filepath.IsAbs(linkTarget) {
						resolved = filepath.Clean(linkTarget)
					} else {
						resolved = filepath.Clean(filepath.Join(filepath.Dir(targetPath), linkTarget))
					}
					entry.IsLinked = resolved == filepath.Clean(path)
				}
			}
		}

		files = append(files, entry)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking package directory: %w", err)
	}

	return files, nil
}

func computeStatus(files []FileEntry) PackageStatus {
	if len(files) == 0 {
		return StatusUnlinked
	}

	linked := 0
	for _, f := range files {
		if f.IsLinked {
			linked++
		}
	}

	if linked == len(files) {
		return StatusLinked
	}
	if linked == 0 {
		return StatusUnlinked
	}
	return StatusPartial
}

func appendAutoDetected(files []FileEntry, pkgDir, homeDir string) []FileEntry {
	managedRoot := findManagedRoot(files)
	if managedRoot == "" {
		return files
	}

	tracked := make(map[string]bool, len(files))
	for _, f := range files {
		tracked[f.RelPath] = true
	}

	homeRoot := filepath.Join(homeDir, managedRoot)
	_ = filepath.Walk(homeRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, relErr := filepath.Rel(homeDir, path)
		if relErr != nil || tracked[relPath] {
			return nil
		}

		entry := FileEntry{
			RelPath:    relPath,
			TargetPath: path,
			Exists:     true,
			IsLinked:   false,
			IsSymlink:  false,
			InRepo:     false,
		}

		linkInfo, statErr := os.Lstat(path)
		if statErr == nil && linkInfo.Mode()&os.ModeSymlink != 0 {
			entry.IsSymlink = true
			linkTarget, readErr := os.Readlink(path)
			if readErr == nil {
				var resolved string
				if filepath.IsAbs(linkTarget) {
					resolved = filepath.Clean(linkTarget)
				} else {
					resolved = filepath.Clean(filepath.Join(filepath.Dir(path), linkTarget))
				}
				repoPath := filepath.Join(pkgDir, relPath)
				entry.IsLinked = resolved == filepath.Clean(repoPath)
			}
		}

		files = append(files, entry)
		return nil
	})

	return files
}

func findManagedRoot(files []FileEntry) string {
	var dirs []string
	for _, f := range files {
		if !f.InRepo {
			continue
		}
		dir := filepath.Dir(f.RelPath)
		if dir == "." {
			continue
		}
		dirs = append(dirs, dir)
	}
	if len(dirs) == 0 {
		return ""
	}

	common := dirs[0]
	for _, d := range dirs[1:] {
		for !strings.HasPrefix(d+string(filepath.Separator), common+string(filepath.Separator)) && d != common {
			parent := filepath.Dir(common)
			if parent == common {
				break
			}
			common = parent
		}
	}

	parts := strings.Split(common, string(filepath.Separator))
	if len(parts) <= 1 {
		return ""
	}

	return common
}

func packageModTime(pkgPath string) time.Time {
	info, err := os.Stat(pkgPath)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
