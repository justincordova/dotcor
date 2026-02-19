# Release Process

This document outlines the automated release process for DotCor using GoReleaser and Homebrew.

## Platform

**DotCor is distributed exclusively via Homebrew for macOS.**

- **Supported platforms:** macOS (Intel amd64, Apple Silicon arm64)
- **Package manager:** Homebrew tap at `justincordova/homebrew-dotcor`
- **Installation:** `brew install justincordova/dotcor/dotcor`

## Pre-Release Checklist

Before creating a release, ensure:

- [ ] All tests passing: `go test ./...`
- [ ] Local build successful: `go build ./cmd/dotcor`
- [ ] Documentation updated (README.md, etc.)
- [ ] All changes committed to main branch
- [ ] Working tree is clean: `git status`

## Creating a Release

Releases are triggered by pushing a version tag to GitHub:

```bash
# 1. Prepare release
git checkout main
git pull origin main

# 2. Verify everything is ready
go test ./...
go build ./cmd/dotcor
./dotcor --version

# 3. Create and push version tag
git tag -a v1.0.0 -m "Release v1.0.0: Brief description of changes"
git push origin v1.0.0
```

### What Happens Automatically

Once the tag is pushed, GitHub Actions automatically:

1. **Builds binaries** for macOS:
   - macOS Intel (amd64)
   - macOS Apple Silicon (arm64)

2. **Creates archives**:
   - `.tar.gz` archives for both architectures
   - Includes binary and README.md

3. **Generates checksums** (SHA256) for all artifacts

4. **Creates GitHub Release** with:
   - Release notes (generated from commit messages)
   - Binary archives
   - Checksums file

5. **Updates Homebrew tap** (justincordova/homebrew-dotcor):
   - Automatically updates Formula/dotcor.rb
   - Commits and pushes to tap repository

## Monitoring Release

Monitor the release process at:

- **GitHub Actions**: https://github.com/justincordova/dotcor/actions
- **GitHub Releases**: https://github.com/justincordova/dotcor/releases
- **Homebrew Tap**: https://github.com/justincordova/homebrew-dotcor

The entire process typically completes in 3-5 minutes.

## Post-Release Verification

After release completes:

- [ ] GitHub Release created with both macOS binaries
- [ ] Homebrew formula updated in tap repository
- [ ] Test Homebrew installation:
  ```bash
  brew upgrade dotcor
  # or for first install:
  brew install justincordova/dotcor/dotcor
  ```
- [ ] Verify version:
  ```bash
  dotcor --version  # Should show new version
  ```
- [ ] Test on both Intel and Apple Silicon (if available)

## Version Numbering

Follow semantic versioning (semver):

- **v1.0.0**: Major release (breaking changes)
- **v1.1.0**: Minor release (new features, backward compatible)
- **v1.0.1**: Patch release (bug fixes)

Use pre-release tags for testing:
- **v1.0.0-rc1**: Release candidate
- **v1.0.0-beta.1**: Beta release

## Rolling Back a Release

If a release has critical issues:

### 1. Delete Git Tag

```bash
# Delete locally
git tag -d v1.0.0

# Delete remotely
git push --delete origin v1.0.0
```

### 2. Delete GitHub Release

1. Go to https://github.com/justincordova/dotcor/releases
2. Find the release
3. Click "Delete release"

### 3. Revert Homebrew Tap

```bash
cd ~/cs/homebrew-dotcor
git log  # Find the commit before auto-update
git revert <commit-hash>
git push origin main
```

### 4. Create Fixed Release

Fix issues, then create a new release with an incremented version (e.g., v1.0.1).

## Testing Releases Locally

Before pushing a real tag, test the build locally:

```bash
# Install GoReleaser
brew install goreleaser/tap/goreleaser

# Run snapshot build (doesn't publish)
goreleaser release --snapshot --clean --skip=publish

# Check generated artifacts
ls -lh dist/

# Test Intel binary
./dist/dotcor_0.0.0-snapshot_darwin_amd64/dotcor --version

# Test Apple Silicon binary
./dist/dotcor_0.0.0-snapshot_darwin_arm64/dotcor --version

# Review generated Homebrew formula
cat dist/homebrew/Formula/dotcor.rb
```

## Troubleshooting

### Build Fails in GitHub Actions

1. Check the Actions tab for error details
2. Common issues:
   - Go version mismatch (check go.mod)
   - Missing dependencies (run `go mod tidy`)
   - Test failures (run `go test ./...` locally)

### Homebrew Formula Not Updated

1. Check GitHub Actions logs for the goreleaser step
2. Verify that GITHUB_TOKEN has write permissions
3. Check that the homebrew-dotcor repository is accessible

### Version Not Injected Correctly

If binaries show the wrong version:
1. Verify ldflags in `.goreleaser.yaml`:
   ```yaml
   ldflags:
     - -s -w -X main.version={{.Version}}
   ```
2. Check that `cmd/dotcor/main.go` has `var version` declared
3. Test locally: `go build -ldflags="-X main.version=test" ./cmd/dotcor`

### Homebrew Installation Fails

1. Check the Homebrew formula for errors
2. Verify that the binary URLs are accessible
3. Check SHA256 checksums match the release
4. Test formula locally:
   ```bash
   brew install --build-from-source ./dist/homebrew/Formula/dotcor.rb
   ```

## Additional Resources

- [GoReleaser Documentation](https://goreleaser.com/)
- [GoReleaser Homebrew Integration](https://goreleaser.com/customization/homebrew/)
- [Semantic Versioning](https://semver.org/)
- [GitHub Actions](https://docs.github.com/en/actions)
- [Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook)
