# GoReleaser Integration Plan

## Overview

This document outlines the plan for integrating GoReleaser to automate multi-platform releases for DotCor, supporting both Homebrew and Winget package managers.

## Current State

- **Version:** v0.1.1
- **Distribution:** Manual Homebrew tap at `justincordova/homebrew-dotcor`
- **Build:** Manual with `go build`
- **Platforms:** macOS, Linux, Windows (symlink support required)
- **Release Process:** Manual tarball creation, SHA256 calculation, formula updates

## Goals

1. Automate binary builds for multiple platforms and architectures
2. Automatically update Homebrew tap on release
3. Publish to Winget package manager
4. Streamline release workflow via GitHub Actions
5. Maintain backward compatibility with existing installations

## Implementation Phases

### Phase 1: GoReleaser Configuration

**Create `.goreleaser.yaml` in project root**

Key configuration sections:
- **Builds:** Configure build matrix for all platforms
  - macOS: amd64, arm64 (Apple Silicon)
  - Linux: amd64, arm64
  - Windows: amd64, arm64
- **Archives:** Generate platform-appropriate archives
  - Unix: `.tar.gz`
  - Windows: `.zip`
- **Checksums:** Generate SHA256 checksums for all artifacts
- **Changelog:** Auto-generate from git commits
- **Version injection:** Use ldflags to inject version at build time

**Verify `cmd/dotcor/main.go` has version variable:**
```go
var version = "dev" // Will be overridden by ldflags during build
```

**Archive naming convention:**
```
dotcor_{version}_{os}_{arch}.{ext}
Examples:
  dotcor_0.2.0_darwin_amd64.tar.gz
  dotcor_0.2.0_linux_arm64.tar.gz
  dotcor_0.2.0_windows_amd64.zip
```

**Files to include in archives:**
- Binary (`dotcor` or `dotcor.exe`)
- `README.md`
- `LICENSE`
- Shell completion scripts (if implemented)

**GoReleaser build configuration:**
```yaml
builds:
  - main: ./cmd/dotcor
    binary: dotcor
    ldflags:
      - -s -w -X main.version={{.Version}}
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
```

### Phase 2: Homebrew Tap Integration

**Configure GoReleaser to update `justincordova/homebrew-dotcor`**

Requirements:
- Repository reference: `justincordova/homebrew-dotcor`
- Formula location: `Formula/dotcor.rb`
- Maintain existing formula structure
- Dependencies: `go` as build dependency
- Test block: Validate version output

**GoReleaser homebrew configuration:**
```yaml
brews:
  - name: dotcor
    tap:
      owner: justincordova
      name: homebrew-dotcor
    folder: Formula
    homepage: https://github.com/justincordova/dotcor
    description: "Symlink-based dotfile manager with Git integration"
    license: MIT
    dependencies:
      - name: go
        type: build
    install: |
      system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}"), "./cmd/dotcor"
    test: |
      assert_match "dotcor version #{version}", shell_output("#{bin}/dotcor --version")
```

**Authentication:**
- Uses `GITHUB_TOKEN` for pushing formula updates
- Token automatically provided by GitHub Actions
- Requires write access to homebrew-dotcor repository

### Phase 3: Winget Integration

**Configure GoReleaser to generate Winget manifest and submit PR**

Package details:
- **Package identifier:** `JustinCordova.DotCor`
- **Publisher:** `JustinCordova`
- **Installer type:** Portable (ZIP extraction)
- **Target repository:** `microsoft/winget-pkgs`

**GoReleaser winget configuration:**
```yaml
winget:
  - name: DotCor
    publisher: JustinCordova
    license: MIT
    homepage: https://github.com/justincordova/dotcor
    short_description: "Symlink-based dotfile manager with Git integration"
    repository:
      owner: microsoft
      name: winget-pkgs
      branch: "{{.ProjectName}}-{{.Version}}"
    package_identifier: JustinCordova.DotCor
    install_modes:
      - interactive
      - silent
    tags:
      - dotfiles
      - configuration
      - symlinks
      - git
```

**Important notes:**
- First-time Winget submission requires manual review by Microsoft
- Subsequent updates are automated via PR
- Requires personal access token (PAT) with repo scope
- Windows users need Developer Mode or Admin rights for symlinks

**Required secret:**
- `WINGET_TOKEN` - Personal access token for winget-pkgs repository

### Phase 4: GitHub Actions Workflow

**Create `.github/workflows/release.yml`**

Trigger conditions:
- On push of tags matching `v*` (e.g., v0.2.0, v1.0.0)

Workflow steps:
1. Checkout repository with full git history
2. Set up Go environment (version from go.mod)
3. Install GoReleaser
4. Run GoReleaser with release configuration
5. Upload artifacts to GitHub Releases
6. Update Homebrew tap (automatic)
7. Submit Winget package PR (automatic)

**Workflow configuration:**
```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write
  packages: write

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
          WINGET_TOKEN: ${{ secrets.WINGET_TOKEN }}
```

**Required secrets (GitHub repository settings):**
- `GITHUB_TOKEN` - Automatically provided by GitHub Actions
- `WINGET_TOKEN` - Must be manually created and added

### Phase 5: Documentation and Release Process

**Update `README.md`:**
- Add Winget installation instructions
- Update Homebrew installation (no changes, but verify clarity)
- Add note about Windows Developer Mode for symlinks

**Create `RELEASING.md`:**
```markdown
# Release Process

## Pre-Release Checklist
- [ ] All tests passing
- [ ] CHANGELOG.md updated
- [ ] Version bumped in relevant files
- [ ] Documentation updated
- [ ] Local build tested: `go build ./cmd/dotcor`

## Creating a Release
1. Create and push version tag:
   ```bash
   git tag -a v0.2.0 -m "Release v0.2.0: Description"
   git push origin v0.2.0
   ```

2. GitHub Actions automatically:
   - Builds binaries for all platforms
   - Creates GitHub Release with artifacts
   - Updates Homebrew tap
   - Submits Winget package PR

3. Monitor:
   - GitHub Actions: https://github.com/justincordova/dotcor/actions
   - GitHub Releases: https://github.com/justincordova/dotcor/releases
   - Homebrew tap: https://github.com/justincordova/homebrew-dotcor
   - Winget PR: https://github.com/microsoft/winget-pkgs/pulls

## Post-Release Verification
- [ ] GitHub Release created with all artifacts
- [ ] Homebrew formula updated successfully
- [ ] Test Homebrew installation: `brew upgrade dotcor`
- [ ] Winget PR submitted (wait for Microsoft review)
- [ ] Test Windows binary download and execution

## Rolling Back
If release has critical issues:
1. Delete the git tag: `git push --delete origin v0.2.0`
2. Delete the GitHub Release
3. Revert Homebrew tap commit
4. Close Winget PR
```

**Update `CLAUDE.md`:**
Add release workflow section documenting the automated process.

## Dependencies

**External Tools:**
- GoReleaser (v2.0+)
- GitHub Actions (included with GitHub)
- Git tags for versioning

**Repository Access:**
- Write access to `justincordova/dotcor` ✓ (already have)
- Write access to `justincordova/homebrew-dotcor` ✓ (already have)
- Fork and PR access to `microsoft/winget-pkgs` ⚠ (need PAT)

**GitHub Secrets Required:**
- `GITHUB_TOKEN` - Automatic, no setup needed
- `WINGET_TOKEN` - Manual setup required

## Risks and Mitigations

### HIGH: Winget Submission Complexity
**Risk:** Winget has specific manifest format requirements and first-time submissions require manual review.

**Mitigation:**
- Research Winget manifest format before implementation
- Create manual test manifest first to validate
- Be prepared for PR feedback from Microsoft maintainers
- Consider manual first submission, automate subsequent updates

### MEDIUM: Breaking Existing Homebrew Installations
**Risk:** GoReleaser-generated formula might differ from current manual formula, breaking installations.

**Mitigation:**
- Carefully replicate existing formula structure in GoReleaser config
- Test formula locally before pushing: `brew install --build-from-source ./Formula/dotcor.rb`
- Maintain identical ldflags pattern: `-X main.version=#{version}`
- Verify test block passes

### MEDIUM: Version Injection Compatibility
**Risk:** Current formula uses `main.version` variable; GoReleaser must use same pattern.

**Mitigation:**
- Verify `cmd/dotcor/main.go` has `var version` declared
- Test local build with ldflags: `go build -ldflags="-X main.version=test"`
- Run GoReleaser in snapshot mode before first real release

### LOW: GitHub Token Permissions
**Risk:** Default GITHUB_TOKEN might lack permissions for tap updates or package publishing.

**Mitigation:**
- Grant workflow write permissions in repository settings
- Use fine-grained PAT if default token fails
- Required scopes: `contents:write`, `packages:write`

### LOW: Archive Size
**Risk:** Including unnecessary files increases download size and times.

**Mitigation:**
- Configure GoReleaser file filters to exclude:
  - `.git/` directory
  - Test files and test data
  - Development-only files
  - `.DS_Store`, IDE files

## Testing Strategy

### Local Testing (Before First Release)
```bash
# Install GoReleaser
brew install goreleaser/tap/goreleaser

# Test build without publishing (creates dist/ directory)
goreleaser release --snapshot --clean

# Verify artifacts in dist/
ls -lh dist/

# Test binary execution
./dist/dotcor_darwin_amd64_v1/dotcor --version

# Test Homebrew formula locally
brew install --build-from-source ./dist/homebrew/Formula/dotcor.rb
```

### Pre-Release Testing (First Automated Release)
1. Create pre-release tag: `v0.1.2-rc1`
2. Monitor GitHub Actions workflow execution
3. Download and test binaries for each platform
4. Install from Homebrew tap: `brew install justincordova/dotcor/dotcor`
5. Verify version command output matches release
6. Review generated changelog for accuracy
7. If all tests pass, create final release: `v0.1.2`

### Ongoing Release Testing
1. Tag release version: `v0.2.0`
2. Verify GitHub Actions completes successfully
3. Test Homebrew update: `brew upgrade dotcor`
4. Check Winget PR created and CI passes
5. Test one platform binary manually (rotate platforms)

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
git tag -a v0.2.0 -m "Release v0.2.0: Feature summary"
git push origin v0.2.0

# 4. GitHub Actions automatically runs
# Monitor at: https://github.com/justincordova/dotcor/actions

# 5. Verify release
# - Check GitHub Releases page
# - Test Homebrew: brew upgrade dotcor
# - Verify Winget PR submitted
# - Test Windows binary (on Windows machine)
```

### Emergency Rollback
```bash
# Delete tag locally and remotely
git tag -d v0.2.0
git push --delete origin v0.2.0

# Delete GitHub Release (via web UI)
# Revert Homebrew tap commit
cd ~/cs/homebrew-dotcor
git revert HEAD
git push origin main

# Close Winget PR (via web UI)
```

## Files Modified Summary

### New Files
- `.goreleaser.yaml` - GoReleaser configuration
- `.github/workflows/release.yml` - CI/CD automation
- `RELEASING.md` - Release process documentation

### Modified Files
- `README.md` - Add Winget installation instructions
- `CLAUDE.md` - Document automated release workflow
- `cmd/dotcor/main.go` - Ensure version variable exists (verify only)

## Open Questions

### 1. Winget Account Setup
**Question:** Do you have a GitHub account configured for Winget contributions?

**Action needed:**
- Create GitHub Personal Access Token (PAT)
- Grant `public_repo` scope (for submitting PRs to winget-pkgs)
- Add as `WINGET_TOKEN` secret in dotcor repository settings

### 2. Code Signing
**Question:** Should binaries be code-signed for better platform trust?

**Options:**
- **macOS:** Apple Developer certificate for notarization (reduces Gatekeeper warnings)
- **Windows:** Authenticode certificate (reduces SmartScreen warnings)
- **Cost:** ~$99/year for Apple, ~$200/year for Windows certificate
- **Benefit:** Professional appearance, better user trust
- **Skip for now:** Users can still use unsigned binaries with manual approval

**Recommendation:** Start without signing, add later if adoption grows.

### 3. Auto-Update Mechanism
**Question:** Should DotCor include a self-update command?

**Options:**
- Add `dotcor update` command to download latest release
- Rely on package managers (brew upgrade, winget upgrade)
- Hybrid: self-update for direct downloads, package manager for installed versions

**Recommendation:** Rely on package managers for v1.0, consider self-update in v2.0.

### 4. Changelog Generation
**Question:** Automate changelog from commits, or manually curate?

**Options:**
- **Automated:** GoReleaser generates from conventional commits
  - Requires commit format: `feat:`, `fix:`, `chore:`, etc.
  - Fast, consistent, but may lack context
- **Manual:** Hand-write release notes
  - More control, better storytelling
  - Time-consuming, easy to forget details
- **Hybrid:** Generate draft, manually refine

**Recommendation:** Start with automated, refine manually for major releases.

### 5. Pre-Release Channel
**Question:** Need staging/beta releases for testing?

**Options:**
- Pre-release tags: `v0.2.0-beta.1`
- Separate Homebrew tap: `homebrew-dotcor-beta`
- GitHub pre-release flag

**Recommendation:** Use GitHub pre-releases (`-rc1`, `-beta`) for major versions.

## Estimated Complexity: MEDIUM

**Time breakdown:**
- Phase 1 (GoReleaser config): 1-2 hours
- Phase 2 (Homebrew integration): 1 hour
- Phase 3 (Winget setup): 2-3 hours
- Phase 4 (GitHub Actions): 1 hour
- Phase 5 (Documentation): 1 hour
- Testing and validation: 2-3 hours
- **Total: 8-11 hours**

**Complexity factors:**
- GoReleaser documentation is excellent (low complexity)
- Homebrew tap integration is straightforward (low complexity)
- Winget has specific requirements and learning curve (high complexity)
- GitHub Actions workflow is standard (low complexity)
- Testing requires access to multiple platforms (medium complexity)

## Next Steps (When Ready to Implement)

1. **Create PAT for Winget:**
   - Go to GitHub Settings → Developer settings → Personal access tokens
   - Create token with `public_repo` scope
   - Add as `WINGET_TOKEN` secret in dotcor repository

2. **Verify version variable:**
   ```bash
   grep "var version" cmd/dotcor/main.go
   # Should see: var version = "dev" or similar
   ```

3. **Install GoReleaser locally:**
   ```bash
   brew install goreleaser
   ```

4. **Start with Phase 1:**
   - Create `.goreleaser.yaml`
   - Test with `goreleaser release --snapshot --clean`
   - Verify binaries build successfully

5. **Proceed through phases:**
   - Test each phase independently
   - Commit after each working phase
   - Use pre-release tag for first real test

## References

- [GoReleaser Documentation](https://goreleaser.com/intro/)
- [GoReleaser Homebrew Integration](https://goreleaser.com/customization/homebrew/)
- [GoReleaser Winget Integration](https://goreleaser.com/customization/winget/)
- [GitHub Actions for Go](https://docs.github.com/en/actions/automating-builds-and-tests/building-and-testing-go)
- [Winget Package Manifest](https://docs.microsoft.com/en-us/windows/package-manager/package/manifest)
- [Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook)

## Success Criteria

**Phase 1 complete when:**
- GoReleaser builds binaries for all platforms
- Version injection works correctly
- Archives contain correct files
- Checksums generated

**Phase 2 complete when:**
- Homebrew formula auto-updates on release
- Formula test passes
- Installation via tap works

**Phase 3 complete when:**
- Winget manifest generated correctly
- PR submitted to winget-pkgs
- Windows binary installs successfully

**Phase 4 complete when:**
- GitHub Actions workflow triggers on tag
- All platforms build successfully
- Artifacts uploaded to release
- No workflow errors

**Phase 5 complete when:**
- Documentation updated
- Release process documented
- Team can execute releases independently

**Overall success:**
- Tag release → Binaries available within 10 minutes
- Homebrew users can `brew upgrade dotcor` immediately
- Winget PR submitted (merge time depends on Microsoft)
- Zero manual steps required for releases
