# GoReleaser Finalization Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Finalize GoReleaser configuration to support Linux, include shell completions, and use pre-built binaries for Homebrew

**Architecture:** GoReleaser for cross-platform builds, completion generation hooks, Homebrew formula optimization

**Tech Stack:** GoReleaser v2, GitHub Actions, Homebrew, shell completion scripts

**Pre-commit verification for ALL tasks:**
```bash
go build ./...
go test ./...
```

---

## Task 1: Add Linux support to builds

**Files:**
- Modify: `.goreleaser.yaml`

**Step 1: Add Linux to goos list**

Change the `builds` section from:
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

To:
```yaml
builds:
  - id: dotcor
    main: ./cmd/dotcor
    binary: dotcor
    ldflags:
      - -s -w -X main.version={{.Version}}
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    env:
      - CGO_ENABLED=0
```

---

## Task 2: Add completion script generation

**Files:**
- Modify: `.goreleaser.yaml`

**Step 1: Add post-build hooks for completions**

Add after the `env:` line in the builds section:
```yaml
    hooks:
      post:
        - cmd: ./dist/dotcor_darwin_arm64/dotcor completion bash
          output: dist/completions/dotcor.bash
        - cmd: ./dist/dotcor_darwin_arm64/dotcor completion zsh
          output: dist/completions/_dotcor
        - cmd: ./dist/dotcor_darwin_arm64/dotcor completion fish
          output: dist/completions/dotcor.fish
```

**Note:** Uses darwin_arm64 binary to generate completions (all binaries generate identical output)

---

## Task 3: Enhance archive contents

**Files:**
- Modify: `.goreleaser.yaml`

**Step 1: Update archives section**

Change from:
```yaml
archives:
  - id: default
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - README.md
```

To:
```yaml
archives:
  - id: default
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - README.md
      - LICENSE
      - src: dist/completions/*
        dst: completions
        strip_parent: true
```

---

## Task 4: Improve Homebrew formula

**Files:**
- Modify: `.goreleaser.yaml`

**Step 1: Update brews section to use pre-built binary**

Change from:
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
    dependencies:
      - name: go
        type: build
    install: |
      system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}"), "./cmd/dotcor"
    test: |
      assert_match "dotcor version #{version}", shell_output("#{bin}/dotcor --version")
```

To:
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
    url_template: "https://github.com/justincordova/dotcor/releases/download/{{ .Tag }}/{{ .ArtifactName }}"
    install: |
      bin.install "dotcor"
      bash_completion.install "completions/dotcor.bash" => "dotcor"
      zsh_completion.install "completions/_dotcor"
      fish_completion.install "completions/dotcor.fish"
    test: |
      assert_match "dotcor version #{version}", shell_output("#{bin}/dotcor --version")
```

**Why:** Pre-built binaries install faster and don't require Go on user's machine

---

## Task 5: Improve changelog grouping

**Files:**
- Modify: `.goreleaser.yaml`

**Step 1: Add Breaking Changes group**

Change from:
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

To:
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
    - title: Breaking Changes
      regexp: '^.*BREAKING CHANGE.*$'
      order: 0
    - title: Features
      regexp: '^feat:'
      order: 1
    - title: Bug Fixes
      regexp: '^fix:'
      order: 2
    - title: Improvements
      regexp: '^(refactor|perf|style):'
      order: 3
    - title: Others
      order: 999
```

---

## Task 6: Test GoReleaser locally

**Files:**
- None (manual testing)

**Step 1: Install GoReleaser**
```bash
brew install goreleaser
```

**Step 2: Run snapshot build**
```bash
# Clean previous builds
rm -rf dist/

# Run snapshot build (doesn't publish)
goreleaser release --snapshot --clean

# Check output
ls -lh dist/
ls -lh dist/completions/
```

**Step 3: Verify binaries**
```bash
# Test darwin binary
./dist/dotcor_darwin_arm64_v8.0/dotcor --version
./dist/dotcor_darwin_arm64_v8.0/dotcor completion bash | head -20

# Test linux binary (if on Linux or using Docker)
./dist/dotcor_linux_amd64_v1/dotcor --version
```

**Step 4: Check archive contents**
```bash
# List contents of darwin archive
tar tzf dist/dotcor_*_darwin_arm64.tar.gz

# Should show:
# - dotcor binary
# - README.md
# - LICENSE
# - completions/dotcor.bash
# - completions/_dotcor
# - completions/dotcor.fish
```

**Step 5: Check Homebrew formula**
```bash
# Review generated formula
cat dist/homebrew/Formula/dotcor.rb
```

---

## Task 7: Update RELEASING documentation

**Files:**
- Modify: `docs/RELEASING.md`

**Step 1: Update platform support**

Change line 38-42 from:
```markdown
1. **Builds binaries** for all platforms:
   - macOS: amd64, arm64
   - Linux: amd64, arm64
   - Windows: amd64, arm64
```

To:
```markdown
1. **Builds binaries** for all platforms:
   - macOS: amd64, arm64
   - Linux: amd64, arm64
```

**Step 2: Add completion verification to checklist**

Add after line 84:
```markdown
- [ ] Completion scripts included in release archives
- [ ] Homebrew formula installs completions correctly
```

---

## Task 8: Update README installation section

**Files:**
- Modify: `README.md`

**Step 1: Add Linux installation instructions**

After the Homebrew section (around line 16), add:
```markdown
### Linux

Download the latest release:

```bash
# Using curl
curl -sL https://github.com/justincordova/dotcor/releases/latest/download/dotcor_Linux_x86_64.tar.gz | tar xz
sudo mv dotcor /usr/local/bin/

# Using wget
wget -qO- https://github.com/justincordova/dotcor/releases/latest/download/dotcor_Linux_x86_64.tar.gz | tar xz
sudo mv dotcor /usr/local/bin/
```

Install shell completions:

```bash
# Bash
sudo cp completions/dotcor.bash /usr/share/bash-completion/completions/dotcor

# Zsh
sudo cp completions/_dotcor /usr/share/zsh/vendor-completions/

# Fish
sudo cp completions/dotcor.fish /usr/share/fish/vendor_completions.d/
```
```

---

## Task 9: Create LICENSE file if missing

**Files:**
- Create: `LICENSE` (if doesn't exist)

**Step 1: Check if LICENSE exists**
```bash
ls -la LICENSE
```

**Step 2: If missing, create MIT LICENSE**
```
MIT License

Copyright (c) 2026 Justin Cordova

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

---

## Task 10: Pre-release verification

**Files:**
- None (manual verification)

**Step 1: Run all tests**
```bash
go test ./...
go test -race ./...
go vet ./...
gofmt -l .
```

**Step 2: Test build**
```bash
go build ./cmd/dotcor
./dotcor --version
./dotcor --help
```

**Step 3: Test GoReleaser snapshot**
```bash
goreleaser release --snapshot --clean --skip=publish
```

**Step 4: Verify snapshot outputs**
```bash
# Check all expected files exist
ls -lh dist/

# Verify completions
ls -lh dist/completions/
cat dist/completions/dotcor.bash | head -10

# Check archive contents
tar tzf dist/dotcor_*_darwin_arm64.tar.gz | grep -E "(LICENSE|README|completions)"
```

---

## Task 11: Create release

**Files:**
- None (git operations)

**Step 1: Commit all changes**
```bash
git add .
git commit -m "feat: add Linux support and shell completions to GoReleaser"
```

**Step 2: Create and push tag**
```bash
git tag -a v0.2.0 -m "Release v0.2.0: Linux support and shell completions"
git push origin main
git push origin v0.2.0
```

**Step 3: Monitor release**
- GitHub Actions: https://github.com/justincordova/dotcor/actions
- Releases: https://github.com/justincordova/dotcor/releases

---

## Task 12: Post-release verification

**Files:**
- None (manual verification)

**Step 1: Verify GitHub Release**
- Check release notes are correct
- Verify all archives are present (darwin, linux)
- Verify checksums file exists

**Step 2: Test Homebrew installation**
```bash
# Update tap
brew tap justincordova/dotcor

# Install
brew install dotcor

# Verify version
dotcor --version

# Test completions
dotcor <TAB>
```

**Step 3: Test Linux installation**
```bash
# Download and extract
curl -sL https://github.com/justincordova/dotcor/releases/latest/download/dotcor_Linux_x86_64.tar.gz | tar xz

# Test binary
./dotcor --version
```

---

## Success Criteria

- [ ] GoReleaser config includes Linux builds
- [ ] Completion scripts generated during build
- [ ] Archives include LICENSE, README, completions
- [ ] Homebrew uses pre-built binary (faster installs)
- [ ] Snapshot build succeeds locally
- [ ] All tests pass: `go test ./...`
- [ ] Documentation updated (README, RELEASING)
- [ ] v0.2.0 release created successfully
- [ ] Homebrew formula installs completions
- [ ] Linux binary available in release

---

## Notes

- Pre-built binaries are standard for Homebrew now (better UX)
- Linux support doubles potential user base
- Completions make the tool much easier to use
- This release is backward compatible (no breaking changes)
- Users with Go installed can still build from source if they want
