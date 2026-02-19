# GoReleaser Integration Plan

## Overview

This document outlines the plan for integrating GoReleaser to automate macOS releases for DotCor, distributing exclusively via Homebrew.

## Platform

**DotCor is distributed exclusively via Homebrew for macOS.**

- **Supported platforms:** macOS (Intel amd64, Apple Silicon arm64)
- **Package manager:** Homebrew tap at `justincordova/homebrew-dotcor`
- **Why macOS only:** DotCor is a symlink-based dotfile manager designed specifically for macOS, where symlinks are first-class citizens with full system support

## Current State

- **Version:** v1.0.0
- **Distribution:** Manual Homebrew tap at `justincordova/homebrew-dotcor`
- **Build:** Manual with `go build ./cmd/dotcor`
- **Platforms:** macOS amd64, arm64
- **Release Process:** Manual binary creation, SHA256 calculation, formula updates

## Goals

1. Automate binary builds for both macOS architectures
2. Automatically update Homebrew tap on release
3. Streamline release workflow via GitHub Actions
4. Maintain backward compatibility with existing Homebrew installations

## Implementation Plan

### Phase 1: GoReleaser Configuration

**Create `.goreleaser.yaml` in project root**

Key configuration sections:

**Builds:**
- Target platforms: macOS (darwin) only
- Architectures: amd64 (Intel), arm64 (Apple Silicon)
- Binary name: `dotcor`
- Version injection via ldflags

```yaml
builds:
  - id: dotcor
    main: ./cmd/dotcor
    binary: dotcor
    ldflags:
      - -s -w -X main.version={{.Version}}
    goos:
      - darwin
    goarch:
      - amd64
      - arm64
    env:
      - CGO_ENABLED=0
```

**Archives:**
- Format: `.tar.gz` (standard for Unix)
- Naming: `dotcor_{version}_{os}_{arch}.tar.gz`
- Examples:
  - `dotcor_1.0.0_darwin_amd64.tar.gz`
  - `dotcor_1.0.0_darwin_arm64.tar.gz`
- Files to include:
  - Binary (`dotcor`)
  - `README.md`

```yaml
archives:
  - id: default
    format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - README.md
```

**Checksums:**
- Algorithm: SHA256
- File: `checksums.txt`

```yaml
checksum:
  name_template: 'checksums.txt'
  algorithm: sha256
```

**Changelog:**
- Auto-generate from GitHub commits
- Filter out docs, test, chore, ci commits
- Group by type (Features, Bug Fixes, Others)

```yaml
changelog:
  sort: asc
  use: github
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^chore:'
      - '^ci:'
  groups:
    - title: Features
      regexp: '^feat:'
      order: 0
    - title: Bug Fixes
      regexp: '^fix:'
      order: 1
    - title: Others
      order: 999
```

**Verify `cmd/dotcor/main.go` has version variable:**
```go
var version = "dev" // Will be overridden by ldflags during build
```

### Phase 2: Homebrew Tap Integration

**Configure GoReleaser to update `justincordova/homebrew-dotcor`**

Requirements:
- Repository reference: `justincordova/homebrew-dotcor`
- Formula location: `Formula/dotcor.rb`
- Maintain existing formula structure
- Test block: Validate version output

**GoReleaser homebrew configuration:**
```yaml
brews:
  - name: dotcor
    repository:
      owner: justincordova
      name: homebrew-dotcor
    directory: Formula
    homepage: https://github.com/justincordova/dotcor
    description: "Symlink-based dotfile manager with Git integration"
    license: MIT
    skip_upload: false
    install: |
      bin.install "dotcor"
    test: |
      assert_match "dotcor version #{version}", shell_output("#{bin}/dotcor --version")
```

**Authentication:**
- Uses `GITHUB_TOKEN` for pushing formula updates
- Token automatically provided by GitHub Actions
- Requires write access to homebrew-dotcor repository

**Formula structure (auto-generated):**
```ruby
class Dotcor < Formula
  desc "Symlink-based dotfile manager with Git integration"
  homepage "https://github.com/justincordova/dotcor"
  url "https://github.com/justincordova/dotcor/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "<checksum>"
  license "MIT"

  depends_on "git"

  def install
    bin.install "dotcor"
  end

  test do
    assert_match "dotcor version #{version}", shell_output("#{bin}/dotcor --version")
  end
end
```

### Phase 3: GitHub Actions Workflow

**Create `.github/workflows/release.yml`**

Trigger conditions:
- On push of tags matching `v*` (e.g., v1.0.0, v1.1.0)

Workflow steps:
1. Checkout repository with full git history
2. Set up Go environment (version from go.mod)
3. Install GoReleaser
4. Run GoReleaser with release configuration
5. Upload artifacts to GitHub Releases
6. Update Homebrew tap (automatic)

**Workflow configuration:**
```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: 'go.mod'

      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Required secrets (GitHub repository settings):**
- `GITHUB_TOKEN` - Automatically provided by GitHub Actions (no manual setup needed)

### Phase 4: Documentation and Release Process

**Update `README.md`:**
- Verify Homebrew installation instructions are clear
- Add note about macOS-only distribution
- Update version examples

**Update `docs/RELEASING.md`:**
- Document automated release process
- Focus on macOS and Homebrew
- Include troubleshooting steps
- Document rollback procedure

**Create/update `CHANGELOG.md`:**
- Track release notes
- Follow conventional commit format
- Group by Features, Bug Fixes, Others

**Update `CLAUDE.md`:**
- Document automated release workflow in Development Guide
- Update version injection notes

## Dependencies

**External Tools:**
- GoReleaser (v2.0+)
- GitHub Actions (included with GitHub)
- Git tags for versioning

**Repository Access:**
- Write access to `justincordova/dotcor` ✓ (already have)
- Write access to `justincordova/homebrew-dotcor` ✓ (already have)

**GitHub Secrets Required:**
- `GITHUB_TOKEN` - Automatic, no setup needed

## Testing Strategy

### Local Testing (Before First Release)

```bash
# Install GoReleaser
brew install goreleaser/tap/goreleaser

# Test build without publishing (creates dist/ directory)
goreleaser release --snapshot --clean --skip=publish

# Verify artifacts in dist/
ls -lh dist/

# Expected output:
# dist/dotcor_0.0.0-snapshot_darwin_amd64.tar.gz
# dist/dotcor_0.0.0-snapshot_darwin_arm64.tar.gz
# dist/checksums.txt
# dist/homebrew/Formula/dotcor.rb

# Test Intel binary
./dist/dotcor_0.0.0-snapshot_darwin_amd64/dotcor --version

# Test Apple Silicon binary
./dist/dotcor_0.0.0-snapshot_darwin_arm64/dotcor --version

# Test Homebrew formula locally
brew install --build-from-source ./dist/homebrew/Formula/dotcor.rb
```

### Pre-Release Testing (First Automated Release)

1. Create pre-release tag: `v1.0.0-rc1`
2. Monitor GitHub Actions workflow execution
3. Download and test both macOS binaries
4. Install from Homebrew tap: `brew install justincordova/dotcor/dotcor`
5. Verify version command output matches release
6. Review generated changelog for accuracy
7. If all tests pass, create final release: `v1.0.0`

### Ongoing Release Testing

1. Tag release version: `v1.1.0`
2. Verify GitHub Actions completes successfully
3. Test Homebrew update: `brew upgrade dotcor`
4. Verify both architectures work (test on both Intel and Apple Silicon if possible)
5. Check generated changelog

## Release Workflow (Post-Implementation)

### Standard Release Process

```bash
# 1. Prepare release
git checkout main
git pull origin main

# 2. Verify everything is ready
go test ./...
go build ./cmd/dotcor
./dotcor --version

# 3. Create release tag
git tag -a v1.0.0 -m "Release v1.0.0: Feature summary"
git push origin v1.0.0

# 4. GitHub Actions automatically runs
# Monitor at: https://github.com/justincordova/dotcor/actions

# 5. Verify release
# - Check GitHub Releases page
# - Test Homebrew: brew upgrade dotcor
# - Verify version: dotcor --version
```

### Emergency Rollback

```bash
# Delete tag locally and remotely
git tag -d v1.0.0
git push --delete origin v1.0.0

# Delete GitHub Release (via web UI)
# https://github.com/justincordova/dotcor/releases

# Revert Homebrew tap commit
cd ~/cs/homebrew-dotcor
git revert HEAD
git push origin main
```

## Files Modified Summary

### New Files
- `.goreleaser.yaml` - GoReleaser configuration
- `.github/workflows/release.yml` - CI/CD automation
- `CHANGELOG.md` - Release notes (optional but recommended)

### Modified Files
- `README.md` - Verify Homebrew instructions
- `docs/RELEASING.md` - Update release process documentation
- `cmd/dotcor/main.go` - Ensure version variable exists (verify only)

## Risks and Mitigations

### MEDIUM: Breaking Existing Homebrew Installations

**Risk:** GoReleaser-generated formula might differ from current manual formula, breaking installations.

**Mitigation:**
- Carefully replicate existing formula structure in GoReleaser config
- Test formula locally before pushing: `brew install --build-from-source ./Formula/dotcor.rb`
- Verify test block passes
- Keep install method simple: `bin.install "dotcor"`

### LOW: Version Injection Compatibility

**Risk:** Current formula uses `main.version` variable; GoReleaser must use same pattern.

**Mitigation:**
- Verify `cmd/dotcor/main.go` has `var version` declared
- Test local build with ldflags: `go build -ldflags="-X main.version=test"`
- Run GoReleaser in snapshot mode before first real release

### LOW: GitHub Token Permissions

**Risk:** Default GITHUB_TOKEN might lack permissions for tap updates or package publishing.

**Mitigation:**
- Grant workflow write permissions in repository settings
- Required scopes: `contents:write`

### LOW: Archive Size

**Risk:** Including unnecessary files increases download size and times.

**Mitigation:**
- Configure GoReleaser to include only necessary files
- Only include: binary and README.md

## Estimated Complexity: LOW

**Time breakdown:**
- Phase 1 (GoReleaser config): 1-2 hours
- Phase 2 (Homebrew integration): 30 minutes
- Phase 3 (GitHub Actions): 30 minutes
- Phase 4 (Documentation): 30 minutes
- Testing and validation: 1-2 hours
- **Total: 3.5-5.5 hours**

**Complexity factors:**
- GoReleaser documentation is excellent (low complexity)
- Homebrew tap integration is straightforward (low complexity)
- GitHub Actions workflow is standard (low complexity)
- macOS-only focus reduces complexity significantly (no Windows/Linux considerations)
- Testing requires access to both Intel and Apple Silicon (medium complexity)

## Success Criteria

**Phase 1 complete when:**
- GoReleaser builds binaries for both macOS architectures
- Version injection works correctly
- Archives contain correct files (binary + README)
- Checksums generated

**Phase 2 complete when:**
- Homebrew formula auto-updates on release
- Formula test passes
- Installation via tap works

**Phase 3 complete when:**
- GitHub Actions workflow triggers on tag
- Both architectures build successfully
- Artifacts uploaded to release
- Homebrew formula updated
- No workflow errors

**Phase 4 complete when:**
- Documentation updated
- Release process documented
- Team can execute releases independently

**Overall success:**
- Tag release → Binaries available within 5 minutes
- Homebrew users can `brew upgrade dotcor` immediately
- Zero manual steps required for releases

## Next Steps (When Ready to Implement)

1. **Verify version variable:**
   ```bash
   grep "var version" cmd/dotcor/main.go
   # Should see: var version = "dev" or similar
   ```

2. **Install GoReleaser locally:**
   ```bash
   brew install goreleaser/tap/goreleaser
   ```

3. **Start with Phase 1:**
   - Create `.goreleaser.yaml`
   - Test with `goreleaser release --snapshot --clean --skip=publish`
   - Verify binaries build successfully for both architectures

4. **Proceed through phases:**
   - Test each phase independently
   - Commit after each working phase
   - Use pre-release tag for first real test (v1.0.0-rc1)

5. **First release:**
   - Ensure all tests pass
   - Create and push v1.0.0 tag
   - Monitor GitHub Actions
   - Verify Homebrew tap updates
   - Test installation from tap

## References

- [GoReleaser Documentation](https://goreleaser.com/intro/)
- [GoReleaser Homebrew Integration](https://goreleaser.com/customization/homebrew/)
- [GitHub Actions for Go](https://docs.github.com/en/actions/automating-builds-and-tests/building-and-testing-go)
- [Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook)
- [Semantic Versioning](https://semver.org/)
- [Conventional Commits](https://www.conventionalcommits.org/)
