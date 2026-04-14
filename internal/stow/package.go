package stow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PackageStatus int

const (
	StatusLinked PackageStatus = iota
	StatusPartial
	StatusUnlinked
)

type Package struct {
	Name   string
	Path   string
	Files  []FileEntry
	Status PackageStatus
}

type FileEntry struct {
	RelPath    string
	TargetPath string
	IsLinked   bool
	Exists     bool
	IsSymlink  bool
}

type LinkResult struct {
	Linked    int
	Skipped   int
	Conflicts []string
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
	if strings.HasPrefix(name, ".") && name != ".dotcorrc" {
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

		pkg := Package{
			Name:   entry.Name(),
			Path:   pkgPath,
			Files:  files,
			Status: computeStatus(files),
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
