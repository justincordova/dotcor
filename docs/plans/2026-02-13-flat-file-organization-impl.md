# Flat File Organization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove automatic categorization and `--category` flag, storing all files in `/files/` with full paths preserved

**Architecture:** Simplify `GenerateRepoPath()` to just strip `~/` prefix, remove category mapping and prefix matching, update all callers

**Tech Stack:** Go 1.21+, cobra (CLI), testify (testing)

---

### Task 1: Update GenerateRepoPath signature and implementation

**Files:**
- Modify: `internal/config/paths.go:152-228`

**Step 1: Read current implementation**

Read the file to understand the current logic.

**Step 2: Replace GenerateRepoPath function**

Remove the entire `GenerateRepoPath()` function and replace with simplified version:

```go
// GenerateRepoPath generates the repository path for a source file
// Returns the path relative to the repository's files directory
// Example: ~/.zshrc -> .zshrc
func GenerateRepoPath(sourcePath string, cfg *Config) (string, error) {
	cfg.Logger.Debug("generating repo path", "source", sourcePath)

	// Expand source path to absolute
	expanded, err := ExpandPath(sourcePath, cfg)
	if err != nil {
		return "", fmt.Errorf("expanding path: %w", err)
	}

	// Strip home directory prefix
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	relPath := strings.TrimPrefix(expanded, home)
	relPath = strings.TrimPrefix(relPath, string(filepath.Separator))

	cfg.Logger.Debug("repo path generated", "source", sourcePath, "repo", relPath)
	return relPath, nil
}
```

**Step 3: Remove unused functions**

Delete `categoryMap` (lines 11-42) and `getCategoryByPrefix()` function (lines 230-251).

**Step 4: Run tests to see failures**

Run: `go test ./internal/config/...`
Expected: FAIL - tests for category-based paths will fail

**Step 5: Commit**

```bash
git add internal/config/paths.go
git commit -m "refactor: simplify GenerateRepoPath to use flat structure"
```

---

### Task 2: Update GenerateRepoPath tests

**Files:**
- Modify: `internal/config/paths_test.go`

**Step 1: Find and read TestGenerateRepoPath**

Read the file to understand current test structure.

**Step 2: Replace all category tests with flat path tests**

Remove all test cases that test categorization, add these instead:

```go
func TestGenerateRepoPath(t *testing.T) {
	tests := []struct {
		name       string
		sourcePath string
		wantRepo   string
		wantErr    bool
	}{
		{
			name:       "flat dotfile in home",
			sourcePath: "~/.zshrc",
			wantRepo:   ".zshrc",
			wantErr:    false,
		},
		{
			name:       "nested config file",
			sourcePath: "~/.config/nvim/init.vim",
			wantRepo:   ".config/nvim/init.vim",
			wantErr:    false,
		},
		{
			name:       "ssh config file",
			sourcePath: "~/.ssh/config",
			wantRepo:   "ssh/config",
			wantErr:    false,
		},
		{
			name:       "system file outside home",
			sourcePath: "/etc/hosts",
			wantRepo:   "/etc/hosts",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			got, err := GenerateRepoPath(tt.sourcePath, cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateRepoPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantRepo {
				t.Errorf("GenerateRepoPath() = %v, want %v", got, tt.wantRepo)
			}
		})
	}
}
```

**Step 3: Run tests to verify they pass**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/config/paths_test.go
git commit -m "test: update GenerateRepoPath tests for flat structure"
```

---

### Task 3: Remove --category flag from add command

**Files:**
- Modify: `cmd/dotcor/add.go:38-45`

**Step 1: Read the init() function**

Read to see current flag definitions.

**Step 2: Remove --category flag**

Remove the category flag line from the init function:

```go
func init() {
	addCmd.Flags().BoolP("force", "f", false, "Force add, ignoring warnings (not errors)")
	addCmd.Flags().Bool("template", false, "Treat file as template (adds .template extension)")
	addCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	rootCmd.AddCommand(addCmd)
}
```

**Step 3: Update Long description**

Remove the category flag from the examples section (line 32):

```
  dotcor add ~/.zshrc --force            # Skip validation warnings
```

**Step 4: Remove category variable from runAdd**

In `runAdd()` function (line 48), remove category flag parsing:

```go
func runAdd(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	isTemplate, _ := cmd.Flags().GetBool("template")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
```

**Step 5: Remove category logic from processAddFile**

In `processAddFile()` (around line 228), remove the customRepoPath logic (lines 279-287):

Remove this block:
```go
	customRepoPath := ""
	if category != "" {
		// Category should be combined with the filename, not replace the entire path
		// e.g., --category shell for ~/.zshrc should produce "shell/zshrc"
		filename := filepath.Base(expanded)
		// Strip leading dot from filename for repo path
		repoFilename := strings.TrimPrefix(filename, ".")
		customRepoPath = filepath.Join(category, repoFilename)
	}
	repoPath, err := config.GenerateRepoPath(sourcePath, customRepoPath, cfg)
```

Replace with:
```go
	repoPath, err := config.GenerateRepoPath(sourcePath, cfg)
```

Also update function signature to remove category parameter (line 228):

```go
func processAddFile(cfg *config.Config, sourcePath string, force bool, isTemplate bool, dryRun bool) (addResult, string, error) {
```

And update the call in runAdd (line 175):

```go
	result, _, err := processAddFile(cfg, file, force, isTemplate, dryRun)
```

**Step 6: Build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

**Step 7: Commit**

```bash
git add cmd/dotcor/add.go
git commit -m "refactor: remove --category flag from add command"
```

---

### Task 4: Remove category-related tests

**Files:**
- Modify: `cmd/dotcor/add_test.go`

**Step 1: Find category tests**

Search for tests that use the category parameter or flag.

**Step 2: Remove or update category tests**

Remove any tests that specifically test category functionality. If tests test other things too (like basic add), update them to remove category parameter from `processAddFile` calls.

**Step 3: Run tests**

Run: `go test ./cmd/dotcor/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add cmd/dotcor/add_test.go
git commit -m "test: remove category-related test cases"
```

---

### Task 5: Add path collision test

**Files:**
- Create: `cmd/dotcor/add_test.go` (add new test)

**Step 1: Write test for adding same file twice**

```go
func TestAdd_PathCollision(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	dotcorDir := filepath.Join(tempDir, "dotcor")

	if err := os.Mkdir(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}

	testFile := filepath.Join(homeDir, ".zshrc")
	if err := os.WriteFile(testFile, []byte("zsh config"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	setupTestConfig(t, homeDir, dotcorDir)

	cfg := loadTestConfig(dotcorDir)

	// Act - Add file first time
	err := processAddFile(cfg, testFile, false, false, false)

	// Assert - Should succeed
	assert.NoError(t, err, "first add should succeed")

	// Act - Try to add same file again
	_, _, err = processAddFile(cfg, testFile, false, false, false)

	// Assert - Should skip or fail
	assert.Error(t, err, "second add should fail because file is already managed")
}
```

**Step 2: Run test to verify it passes**

Run: `go test ./cmd/dotcor/... -run TestAdd_PathCollision -v`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/dotcor/add_test.go
git commit -m "test: add path collision test case"
```

---

### Task 6: Update README.md examples

**Files:**
- Modify: `README.md`

**Step 1: Find tree diagram examples**

Search for diagrams showing file structure with categories.

**Step 2: Update all examples to show flat structure**

Change examples like:
```
.dotcor/
├── files/
│   ├── shell/
│   │   ├── zshrc
│   │   └── bashrc
│   └── git/
│       └── gitconfig
```

To:
```
.dotcor/
├── files/
│   ├── .zshrc
│   ├── .bashrc
│   └── .gitconfig
```

**Step 3: Update command examples**

Remove any `--category` flag usage from examples.

**Step 4: Commit**

```bash
git add README.md
git commit -m "docs: update README to reflect flat file organization"
```

---

### Task 7: Verify symlink creation works with flat paths

**Files:**
- Create: `internal/fs/symlink_test.go` (or add to existing tests)

**Step 1: Find existing symlink tests**

Check if there are tests for `ComputeRelativeSymlink` or symlink creation.

**Step 2: Add integration test for flat paths**

```go
func TestSymlinkWithFlatPaths(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	dotcorDir := filepath.Join(tempDir, "dotcor")
	filesDir := filepath.Join(dotcorDir, "files")

	if err := os.MkdirAll(filepath.Join(filesDir, ".config/nvim"), 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}

	homeDirAbs, err := filepath.Abs(homeDir)
	if err != nil {
		t.Fatalf("failed to get abs path: %v", err)
	}

	// Source file in files directory
	repoFile := filepath.Join(filesDir, ".config/nvim/init.vim")
	if err := os.WriteFile(repoFile, []byte("nvim config"), 0644); err != nil {
		t.Fatalf("failed to create repo file: %v", err)
	}

	// Where symlink should be
	symlinkPath := filepath.Join(homeDirAbs, ".config/nvim/init.vim")

	// Act - Create symlink
	cfg := testConfig()
	err = CreateSymlink(symlinkPath, repoFile, cfg)

	// Assert
	assert.NoError(t, err, "CreateSymlink should succeed")

	// Verify symlink points to correct target
	linkTarget, err := os.Readlink(symlinkPath)
	assert.NoError(t, err, "should be able to read symlink")
	assert.Contains(t, linkTarget, ".dotcor/files/.config/nvim/init.vim")
}
```

**Step 3: Run test**

Run: `go test ./internal/fs/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/fs/symlink_test.go
git commit -m "test: verify symlink creation with flat paths"
```

---

### Task 8: Full test suite and cleanup

**Files:**
- All files

**Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 2: Run linter if available**

Run: `gofmt -s -w .` or `go vet ./...`
Expected: No issues

**Step 3: Build project**

Run: `go build ./...`
Expected: SUCCESS

**Step 4: Manual smoke test**

```bash
# In a temp directory
mkdir -p test_dotcor
cd test_dotcor
./dotcor init
echo "test" > .zshrc
./dotcor add .zshrc
ls -la ~/.dotcor/files/
# Should show: .zshrc (not shell/zshrc)
```

**Step 5: Final commit if needed**

If any cleanup or fixes needed:
```bash
git add .
git commit -m "chore: final cleanup for flat file organization"
```

---

## Summary

This plan:
1. Simplifies `GenerateRepoPath()` to strip only `~/` prefix
2. Removes all categorization logic (map, prefix matching)
3. Removes `--category` flag from CLI
4. Updates tests to reflect new flat structure
5. Updates documentation
6. Adds test coverage for path collisions
7. Verifies symlink creation works correctly

Total estimated time: 30-45 minutes
Total commits: 8-9 atomic commits
