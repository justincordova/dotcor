package stow

import (
	"fmt"
	"os"
	"path/filepath"
)

type AdoptResult struct {
	Adopted  int
	Skipped  int
	Failures []string
}

func Adopt(repoDir, homeDir, packageName string) (*AdoptResult, error) {
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

		resolved, err := filepath.EvalSymlinks(f.TargetPath)
		if err != nil {
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		srcData, err := os.ReadFile(resolved)
		if err != nil {
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		srcInfo, err := os.Stat(resolved)
		srcPerm := os.FileMode(0644)
		if err == nil {
			srcPerm = srcInfo.Mode().Perm()
		}

		repoPath := filepath.Join(pkgDir, f.RelPath)
		if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		if _, err := os.Stat(repoPath); err == nil {
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		if err := os.WriteFile(repoPath, srcData, srcPerm); err != nil {
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		if err := os.Remove(f.TargetPath); err != nil {
			_ = os.Remove(repoPath)
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		relSymlink, err := filepath.Rel(filepath.Dir(f.TargetPath), repoPath)
		if err != nil {
			_ = os.WriteFile(f.TargetPath, srcData, srcPerm)
			_ = os.Remove(repoPath)
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		tmpLink := f.TargetPath + ".dotcor-tmp"
		if err := os.Symlink(relSymlink, tmpLink); err != nil {
			_ = os.WriteFile(f.TargetPath, srcData, srcPerm)
			_ = os.Remove(repoPath)
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		if err := os.Rename(tmpLink, f.TargetPath); err != nil {
			_ = os.Remove(tmpLink)
			_ = os.WriteFile(f.TargetPath, srcData, srcPerm)
			_ = os.Remove(repoPath)
			result.Failures = append(result.Failures, f.RelPath)
			result.Skipped++
			continue
		}

		result.Adopted++
	}

	return result, nil
}
