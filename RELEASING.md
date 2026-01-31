# Release Process

This document outlines the automated release process for DotCor using GoReleaser and GitHub Actions.

## Pre-Release Checklist

Before creating a release, ensure:

- [ ] All tests passing: `go test ./...`
- [ ] Local build successful: `go build ./cmd/dotcor`
- [ ] Documentation updated (README.md, CLAUDE.md, etc.)
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
git tag -a v0.2.0 -m "Release v0.2.0: Brief description of changes"
git push origin v0.2.0
```

### What Happens Automatically

Once the tag is pushed, GitHub Actions automatically:

1. **Builds binaries** for all platforms:
   - macOS: amd64, arm64
   - Linux: amd64, arm64
   - Windows: amd64, arm64

2. **Creates archives**:
   - Unix platforms: `.tar.gz`
   - Windows: `.zip`

3. **Generates checksums** (SHA256) for all artifacts

4. **Creates GitHub Release** with:
   - Release notes (generated from commit messages)
   - Binary archives
   - Checksums file

5. **Updates Homebrew tap** (justincordova/homebrew-dotcor):
   - Automatically updates Formula/dotcor.rb
   - Commits and pushes to tap repository

## Monitoring the Release

Monitor the release process at:

- **GitHub Actions**: https://github.com/justincordova/dotcor/actions
- **GitHub Releases**: https://github.com/justincordova/dotcor/releases
- **Homebrew Tap**: https://github.com/justincordova/homebrew-dotcor

The entire process typically completes in 5-10 minutes.

## Post-Release Verification

After the release completes:

- [ ] GitHub Release created with all artifacts
- [ ] Homebrew formula updated in tap repository
- [ ] Test Homebrew installation:
  ```bash
  brew upgrade dotcor
  # or for first install:
  brew install justincordova/dotcor/dotcor
  ```
- [ ] Verify version:
  ```bash
  dotcor --version  # Should show the new version
  ```
- [ ] Download and test one binary manually (optional)

## Version Numbering

Follow semantic versioning (semver):

- **v1.0.0**: Major release (breaking changes)
- **v0.2.0**: Minor release (new features, backward compatible)
- **v0.1.1**: Patch release (bug fixes)

Use pre-release tags for testing:
- **v0.2.0-rc1**: Release candidate
- **v0.2.0-beta.1**: Beta release

## Rolling Back a Release

If a release has critical issues:

### 1. Delete the Git Tag

```bash
# Delete locally
git tag -d v0.2.0

# Delete remotely
git push --delete origin v0.2.0
```

### 2. Delete the GitHub Release

1. Go to https://github.com/justincordova/dotcor/releases
2. Find the release
3. Click "Delete release"

### 3. Revert Homebrew Tap

```bash
cd ~/cs/homebrew-dotcor
git log  # Find the commit before the auto-update
git revert <commit-hash>
git push origin main
```

### 4. Create Fixed Release

Fix the issues, then create a new release with an incremented version (e.g., v0.2.1).

## Testing Releases Locally

Before pushing a real tag, test the build locally:

```bash
# Run snapshot build (doesn't publish)
goreleaser release --snapshot --clean --skip=publish

# Check generated artifacts
ls -lh dist/

# Test a binary
./dist/dotcor_darwin_arm64_v8.0/dotcor --version

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

1. Check GitHub Actions logs for goreleaser step
2. Verify GITHUB_TOKEN has permissions
3. Check that homebrew-dotcor repository is accessible

### Version Not Injected Correctly

If binaries show wrong version:
1. Verify ldflags in `.goreleaser.yaml`
2. Check `main.go` has `var version` declared
3. Test locally: `go build -ldflags="-X main.version=test" ./cmd/dotcor`

## Future Enhancements

Not yet implemented (planned for future):

- **Winget integration**: Requires setting up WINGET_TOKEN secret
- **Code signing**: macOS notarization and Windows Authenticode
- **Auto-update command**: `dotcor update` to download latest release

## Additional Resources

- [GoReleaser Documentation](https://goreleaser.com/)
- [Semantic Versioning](https://semver.org/)
- [GitHub Actions](https://docs.github.com/en/actions)
- [Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook)
