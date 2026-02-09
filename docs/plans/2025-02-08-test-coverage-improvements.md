# Test Coverage Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Increase overall test coverage from 21.8% to 85%+ by writing comprehensive tests for all missing commands and error paths.

**Architecture:** Follow TESTING.md conventions: AAA pattern, testify framework, `t.TempDir()` for isolation, subtests for scenarios.

**Tech Stack:** Go 1.25.5, testify v1.11.1, go testing package

---

## Current State

| Package | Target | Actual | Gap |
|---------|--------|--------|-----|
| cmd/dotcor | 75% | 3.1% | **71.9%** |
| config | 85% | 46.4% | 38.6% |
| core | 90% | 48.8% | 41.2% |
| fs | 85% | 50.5% | 34.5% |
| git | 80% | 49.5% | 30.5% |
| **Overall** | **85%** | **21.8%** | **63.2%** |

**Missing Command Tests (11 of 14):**
- list, status, sync, restore, history, diff, adopt, doctor, rebuild-config, rebuild-links, clone, cleanup

---

## Task Breakdown

### Task 1: Add list command tests

**Files:**
- Create: `cmd/dotcor/list_test.go`
- Test: `cmd/dotcor/list_test.go`

**Step 1: Write failing test - basic listing**

```go
func TestList_NoFiles_PrintsEmptyMessage(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfigWithDir(t, tempDir)

    // Act
    // Call list command logic

    // Assert
    // Verify empty message is shown
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestList_NoFiles_PrintsEmptyMessage`
Expected: FAIL (function doesn't exist yet)

**Step 3: Write minimal implementation**

Create `cmd/dotcor/list_test.go` with basic test structure using test_helpers.go

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestList_NoFiles_PrintsEmptyMessage`
Expected: PASS

**Step 5: Add remaining list tests**

Add subtests for:
- `TestList_SingleFile_DisplaysCorrectly`
- `TestList_MultipleFiles_DisplaysInTable`
- `TestList_UncommittedFile_ShowsWarning`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/list_test.go
git commit -m "test: add comprehensive tests for list command"
```

---

### Task 2: Add status command tests

**Files:**
- Create: `cmd/dotcor/status_test.go`

**Step 1: Write failing test - basic status with no files**

```go
func TestStatus_NotInitialized_ReturnsError(t *testing.T) {
    // Arrange
    // Create temp directory without config

    // Act
    // Call status command

    // Assert
    // Verify error about not initialized
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestStatus_NotInitialized_ReturnsError`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/status_test.go` with test helpers from `test_helpers.go`

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestStatus_NotInitialized_ReturnsError`
Expected: PASS

**Step 5: Add remaining status tests**

Add subtests for:
- `TestStatus_ValidSymlink_ShowsOK`
- `TestStatus_BrokenSymlink_ShowsError`
- `TestStatus_UncommittedChanges_ShowsWarning`
- `TestStatus_GitAheadBehind_ShowsCounts`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/status_test.go
git commit -m "test: add comprehensive tests for status command"
```

---

### Task 3: Add sync command tests

**Files:**
- Create: `cmd/dotcor/sync_test.go`

**Step 1: Write failing test - sync with no changes**

```go
func TestSync_NoChanges_ReturnsEarly(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfigWithGit(t, tempDir)

    // Act
    // Run sync command

    // Assert
    // Verify returns early with appropriate message
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestSync_NoChanges_ReturnsEarly`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/sync_test.go` with mockable git operations or temp git repos

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestSync_NoChanges_ReturnsEarly`
Expected: PASS

**Step 5: Add remaining sync tests**

Add subtests for:
- `TestSync_WithChanges_CommitsAndPushes`
- `TestSync_GitCommitFails_MarksUncommitted`
- `TestSync_PushFails_ContinuesWithoutError`
- `TestSync_NopushFlag_SkipsPush`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/sync_test.go
git commit -m "test: add comprehensive tests for sync command"
```

---

### Task 4: Add restore command tests

**Files:**
- Create: `cmd/dotcor/restore_test.go`

**Step 1: Write failing test - restore to HEAD**

```go
func TestRestore_Head_RestoresLatest(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfigWithFileHistory(t, tempDir)

    // Act
    // Run restore command

    // Assert
    // Verify file is restored to HEAD version
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestRestore_Head_RestoresLatest`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/restore_test.go` with backup and git history mocks

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestRestore_Head_RestoresLatest`
Expected: PASS

**Step 5: Add remaining restore tests**

Add subtests for:
- `TestRestore_SpecificCommit_RestoresCorrectVersion`
- `TestRestore_PreviewFlag_ShowsDiff`
- `TestRestore_NonexistentFile_ReturnsError`
- `TestRestore_BackupCreated_RestorePointAvailable`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/restore_test.go
git commit -m "test: add comprehensive tests for restore command"
```

---

### Task 5: Add history command tests

**Files:**
- Create: `cmd/dotcor/history_test.go`

**Step 1: Write failing test - show history for file**

```go
func TestHistory_SingleFile_ShowsCommits(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfigWithHistory(t, tempDir)

    // Act
    // Run history command

    // Assert
    // Verify commit list is displayed
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestHistory_SingleFile_ShowsCommits`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/history_test.go` with git log mocks

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestHistory_SingleFile_ShowsCommits`
Expected: PASS

**Step 5: Add remaining history tests**

Add subtests for:
- `TestHistory_LimitFlag_ShowsSpecifiedCount`
- `TestHistory_UnmanagedFile_ReturnsError`
- `TestHistory_NoHistory_EmptyOutput`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/history_test.go
git commit -m "test: add comprehensive tests for history command"
```

---

### Task 6: Add diff command tests

**Files:**
- Create: `cmd/dotcor/diff_test.go`

**Step 1: Write failing test - show diff for all changes**

```go
func TestDiff_NoArguments_ShowsAllChanges(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfigWithChanges(t, tempDir)

    // Act
    // Run diff command

    // Assert
    // Verify diff output is shown
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestDiff_NoArguments_ShowsAllChanges`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/diff_test.go` with git diff mocks

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestDiff_NoArguments_ShowsAllChanges`
Expected: PASS

**Step 5: Add remaining diff tests**

Add subtests for:
- `TestDiff_WithFileArgument_ShowsFileDiff`
- `TestDiff_StatFlag_ShowsSummary`
- `TestDiff_NoChanges_ShowsNothing`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/diff_test.go
git commit -m "test: add comprehensive tests for diff command"
```

---

### Task 7: Add adopt command tests

**Files:**
- Create: `cmd/dotcor/adopt_test.go`

**Step 1: Write failing test - adopt existing symlink**

```go
func TestAdopt_ValidSymlink_AdoptsFile(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfigWithSymlink(t, tempDir)

    // Act
    // Run adopt command

    // Assert
    // Verify symlink is copied to repo and updated
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestAdopt_ValidSymlink_AdoptsFile`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/adopt_test.go` with symlink setup helpers

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestAdopt_ValidSymlink_AdoptsFile`
Expected: PASS

**Step 5: Add remaining adopt tests**

Add subtests for:
- `TestAdopt_Nonsymlink_ReturnsError`
- `TestAdopt_AlreadyManaged_ReturnsError`
- `TestAdopt_PointingToRepo_ReturnsError`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/adopt_test.go
git commit -m "test: add comprehensive tests for adopt command"
```

---

### Task 8: Add doctor command tests

**Files:**
- Create: `cmd/dotcor/doctor_test.go`

**Step 1: Write failing test - doctor healthy system**

```go
func TestDoctor_HealthySystem_AllChecksPass(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateHealthyTestConfig(t, tempDir)

    // Act
    // Run doctor command

    // Assert
    // Verify all checks pass with ✓ symbols
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestDoctor_HealthySystem_AllChecksPass`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/doctor_test.go` with various diagnostic scenarios

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestDoctor_HealthySystem_AllChecksPass`
Expected: PASS

**Step 5: Add remaining doctor tests**

Add subtests for:
- `TestDoctor_BrokenSymlink_ShowsError`
- `TestDoctor_StaleLock_DetectsAndClears`
- `TestDoctor_MissingGit_ReturnsWarning`
- `TestDoctor_FixFlag_AutoRepairs`
- `TestDoctor_PermissionError_DetectsIssue`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/doctor_test.go
git commit -m "test: add comprehensive tests for doctor command"
```

---

### Task 9: Add rebuild-config command tests

**Files:**
- Create: `cmd/dotcor/rebuild_test.go`

**Step 1: Write failing test - rebuild from existing repo**

```go
func TestRebuildConfig_ValidRepo_ReconstructsConfig(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfigWithManagedFiles(t, tempDir)

    // Act
    // Run rebuild-config command

    // Assert
    // Verify config is reconstructed correctly
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestRebuildConfig_ValidRepo_ReconstructsConfig`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/rebuild_test.go` with repo scanning tests

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestRebuildConfig_ValidRepo_ReconstructsConfig`
Expected: PASS

**Step 5: Add remaining rebuild tests**

Add subtests for:
- `TestRebuildConfig_DryRun_ShowsPreview`
- `TestRebuildConfig_CorruptConfig_Reconstructs`
- `TestRebuildConfig_OrphanedFiles_ShowsWarning`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/rebuild_test.go
git commit -m "test: add comprehensive tests for rebuild-config command"
```

---

### Task 10: Add rebuild-links command tests

**Files:**
- Create: `cmd/dotcor/rebuild-links_test.go`

**Step 1: Write failing test - rebuild all templates**

```go
func TestRebuildLinks_TemplateFiles_RendersCorrectly(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfigWithTemplates(t, tempDir)

    // Act
    // Run rebuild-links command

    // Assert
    // Verify templates are rendered with correct values
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestRebuildLinks_TemplateFiles_RendersCorrectly`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/rebuild-links_test.go` with template rendering tests

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestRebuildLinks_TemplateFiles_RendersCorrectly`
Expected: PASS

**Step 5: Add remaining rebuild-links tests**

Add subtests for:
- `TestRebuildLinks_NonTemplate_Skips`
- `TestRebuildLinks_HostnameVariable_Replaced`
- `TestRebuildLinks_OSVariable_Replaced`
- `TestRebuildLinks_UserVariable_Replaced`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/rebuild-links_test.go
git commit -m "test: add comprehensive tests for rebuild-links command"
```

---

### Task 11: Add clone command tests

**Files:**
- Create: `cmd/dotcor/clone_test.go`

**Step 1: Write failing test - clone repository**

```go
func TestClone_ValidURL_ClonesAndInitializes(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    testRepo := CreateTestGitRepo(t)

    // Act
    // Run clone command with test repo URL

    // Assert
    // Verify repo is cloned and config initialized
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestClone_ValidURL_ClonesAndInitializes`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/clone_test.go` with mock git clone operations

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestClone_ValidURL_ClonesAndInitializes`
Expected: PASS

**Step 5: Add remaining clone tests**

Add subtests for:
- `TestClone_InvalidURL_ReturnsError`
- `TestClone_DirectoryExists_ReturnsError`
- `TestClone_NoGitInstalled_ReturnsError`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/clone_test.go
git commit -m "test: add comprehensive tests for clone command"
```

---

### Task 12: Add cleanup-backups command tests

**Files:**
- Create: `cmd/dotcor/cleanup_test.go`

**Step 1: Write failing test - cleanup old backups**

```go
func TestCleanup_OldBackups_DeletesCorrectly(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfigWithOldBackups(t, tempDir)

    // Act
    // Run cleanup-backups command

    // Assert
    // Verify only old backups are deleted
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestCleanup_OldBackups_DeletesCorrectly`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `cmd/dotcor/cleanup_test.go` with backup cleanup tests

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestCleanup_OldBackups_DeletesCorrectly`
Expected: PASS

**Step 5: Add remaining cleanup tests**

Add subtests for:
- `TestCleanup_KeepLastFlag_PreservesRecent`
- `TestCleanup_NoBackups_ReturnsEmpty`
- `TestCleanup_ForceFlag_SkipsPrompt`
- `TestCleanup_Preview_ShowsWhatWouldBeDeleted`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/cleanup_test.go
git commit -m "test: add comprehensive tests for cleanup-backups command"
```

---

### Task 13: Increase config package coverage (46.4% → 85%)

**Files:**
- Modify: `internal/config/config_test.go`

**Step 1: Write failing test - GetManagedFilesForPlatform filtering**

```go
func TestGetManagedFilesForPlatform_PlatformMatch_ReturnsCorrect(t *testing.T) {
    // Arrange
    cfg := &Config{
        ManagedFiles: []ManagedFile{
            {Platforms: []string{"darwin"}, SourcePath: "~/.zshrc"},
            {Platforms: []string{"linux"}, SourcePath: "~/.bashrc"},
            {Platforms: []string{}, SourcePath: "~/.gitconfig"},
        },
    }

    // Act
    files := cfg.GetManagedFilesForPlatform()

    // Assert
    // Verify only matching platform files are returned
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -v -run TestGetManagedFilesForPlatform_PlatformMatch_ReturnsCorrect`
Expected: FAIL

**Step 3: Write minimal implementation**

Add test to `internal/config/config_test.go`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -v -run TestGetManagedFilesForPlatform_PlatformMatch_ReturnsCorrect`
Expected: PASS

**Step 5: Add remaining config tests**

Add tests for:
- `TestMarkAsUncommitted_UpdatesFileFlag`
- `TestClearUncommitted_ClearsFileFlag`
- `TestGetUncommittedFiles_ReturnsOnlyFlagged`
- `TestSaveConfig_AtomicWrite`
- `TestLoadConfig_CorruptFile_ReturnsError`
- `TestLoadConfig_VersionMismatch_Migrates`
- `TestRemoveManagedFile_BySourcePath_Deletes`

**Step 6: Commit**

```bash
go build ./... && go test ./internal/config -v
git add internal/config/config_test.go
git commit -m "test: increase config package coverage to 85%+"
```

---

### Task 14: Increase core package coverage (48.8% → 90%)

**Files:**
- Modify: `internal/core/transaction_test.go`
- Modify: `internal/core/backup_test.go`
- Create: `internal/core/hooks_test.go`
- Create: `internal/core/templates_test.go`

**Step 1: Write failing test - Transaction rollback on partial failure**

```go
func TestTransaction_Execute_Failure_RollsBackAll(t *testing.T) {
    // Arrange
    cfg := CreateTestConfig(t)
    tx := NewTransaction(cfg)

    op1 := &mockOperation{doErr: nil}
    op2 := &mockOperation{doErr: errors.New("fail")}
    op3 := &mockOperation{doErr: nil}

    // Act
    tx.Execute(op1)
    err := tx.Execute(op2)
    tx.Execute(op3)

    // Assert
    assert.Error(t, err)
    assert.Equal(t, 1, op1.undoCalls, "op1 should be rolled back")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core -v -run TestTransaction_Execute_Failure_RollsBackAll`
Expected: FAIL

**Step 3: Write minimal implementation**

Update `internal/core/transaction_test.go` with rollback scenarios

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core -v -run TestTransaction_Execute_Failure_RollsBackAll`
Expected: PASS

**Step 5: Add remaining core tests**

For transaction_test.go:
- `TestTransaction_Execute_Committed_FailsToExecute`
- `TestTransaction_Rollback_Empty_NoOp`
- `TestTransaction_Commit_ClearsExecuted`

For hooks_test.go (new file):
- `TestRunHook_HookExists_ExecutesHook`
- `TestRunHook_HookNotFound_SkipsGracefully`
- `TestRunHook_HookFails_LogsWarning`
- `TestRunHook_EnvironmentVariables_PassedToHook`

For templates_test.go (new file):
- `TestSubstituteTemplate_ReplacesAllVariables`
- `TestGetTemplateContext_ReturnsCurrentValues`
- `TestIsTemplateFile_DetectsExtension`
- `TestStripTemplateExtension_RemovesDotTemplate`

For backup_test.go (extend existing):
- `TestGetBackupsForFile_ReturnsOnlyMatching`
- `TestGetLatestBackup_ReturnsMostRecent`
- `TestPreviewCleanup_CalculatesCorrectCandidates`
- `TestCleanOldBackups_KeepLast_PreservesRecent`

**Step 6: Commit**

```bash
go build ./... && go test ./internal/core -v
git add internal/core/transaction_test.go internal/core/backup_test.go internal/core/hooks_test.go internal/core/templates_test.go
git commit -m "test: increase core package coverage to 90%+"
```

---

### Task 15: Increase fs package coverage (50.5% → 85%)

**Files:**
- Modify: `internal/fs/symlink_test.go`
- Modify: `internal/fs/fs_test.go`

**Step 1: Write failing test - symlink status detection**

```go
func TestGetSymlinkStatus_BrokenSymlink_ReturnsCorrectStatus(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    targetFile := filepath.Join(tempDir, "target.txt")
    symlinkFile := filepath.Join(tempDir, "link.txt")
    os.WriteFile(targetFile, []byte("content"), 0644)
    os.Symlink(targetFile, symlinkFile)
    os.Remove(targetFile) // Make it broken

    // Act
    status, err := fs.GetSymlinkStatus(symlinkFile, targetFile)

    // Assert
    assert.NoError(t, err)
    assert.False(t, status.TargetExists, "target should not exist")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/fs -v -run TestGetSymlinkStatus_BrokenSymlink_ReturnsCorrectStatus`
Expected: FAIL

**Step 3: Write minimal implementation**

Add tests to existing test files

**Step 4: Run test to verify it passes**

Run: `go test ./internal/fs -v -run TestGetSymlinkStatus_BrokenSymlink_ReturnsCorrectStatus`
Expected: PASS

**Step 5: Add remaining fs tests**

For symlink_test.go:
- `TestResolveSymlink_ReturnsAbsolutePath`
- `TestIsRelativeSymlink_DetectsRelativePaths`
- `TestSymlinkPointsToRepo_ChecksRepositoryMembership`
- `TestCreateSymlink_RelativePath_ComputedCorrectly`

For fs_test.go:
- `TestIsWritable_Directory_TrueWhenWritable`
- `TestIsWritable_File_TrueWhenWritable`
- `TestIsWritable_Nonexistent_ChecksParent`
- `TestGetFileMode_ReturnsPermissions`
- `TestRemoveAll_DeletesDirectoryTree`

**Step 6: Commit**

```bash
go build ./... && go test ./internal/fs -v
git add internal/fs/symlink_test.go internal/fs/fs_test.go
git commit -m "test: increase fs package coverage to 85%+"
```

---

### Task 16: Increase git package coverage (49.5% → 80%)

**Files:**
- Modify: `internal/git/git_test.go`

**Step 1: Write failing test - GetChangedFiles filters correctly**

```go
func TestGetChangedFiles_ReturnsOnlyChangedFiles(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    testRepo := CreateTestGitRepo(t)
    CreateFileInRepo(t, testRepo, "file1.txt")
    CommitRepo(t, testRepo)
    CreateFileInRepo(t, testRepo, "file2.txt")

    // Act
    files, err := GetChangedFiles(testRepo)

    // Assert
    assert.NoError(t, err)
    assert.Len(t, files, 1, "only file2 should be changed")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/git -v -run TestGetChangedFiles_ReturnsOnlyChangedFiles`
Expected: FAIL

**Step 3: Write minimal implementation**

Add tests to `internal/git/git_test.go`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/git -v -run TestGetChangedFiles_ReturnsOnlyChangedFiles`
Expected: PASS

**Step 5: Add remaining git tests**

Add tests for:
- `TestSetRemote_UpdatesExisting`
- `TestSetRemote_CreatesNew`
- `TestGetConfig_ReturnsValue`
- `TestSetConfig_SetsValue`
- `TestStageFile_StageCorrectFile`
- `TestUnstageFile_UnstagesFile`
- `TestPull_FetchesAndMerges`
- `TestGetCurrentCommit_ReturnsHash`

**Step 6: Commit**

```bash
go build ./... && go test ./internal/git -v
git add internal/git/git_test.go
git commit -m "test: increase git package coverage to 80%+"
```

---

### Task 17: Add error path tests to add command

**Files:**
- Modify: `cmd/dotcor/add_test.go`

**Step 1: Write failing test - add with lock contention**

```go
func TestAdd_LockHeld_ReturnsError(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfigWithLock(t, tempDir)

    // Act
    err := runAddCommand(cfg, []string{"~/.zshrc"})

    // Assert
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "lock is held")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestAdd_LockHeld_ReturnsError`
Expected: FAIL

**Step 3: Write minimal implementation**

Extend existing `add_test.go` with error path tests

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestAdd_LockHeld_ReturnsError`
Expected: PASS

**Step 5: Add remaining error path tests**

Add tests for:
- `TestAdd_FileDoesNotExist_ReturnsError`
- `TestAdd_PermissionDenied_ReturnsError`
- `TestAdd_HookFails_LogsWarning`
- `TestAdd_BackupFails_ReturnsError`
- `TestAdd_TransactionFails_RestoresBackup`
- `TestAdd_GitCommitFails_MarksUncommitted`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/add_test.go
git commit -m "test: add error path tests to add command"
```

---

### Task 18: Add error path tests to init command

**Files:**
- Modify: `cmd/dotcor/init_test.go`

**Step 1: Write failing test - init with existing dotcor directory**

```go
func TestInit_ExistingDotcorDir_ReturnsError(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    CreateDotcorDir(t, tempDir)

    // Act
    err := runInitCommand(tempDir)

    // Assert
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "already initialized")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor -v -run TestInit_ExistingDotcorDir_ReturnsError`
Expected: FAIL

**Step 3: Write minimal implementation**

Extend existing `init_test.go` with error scenarios

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor -v -run TestInit_ExistingDotcorDir_ReturnsError`
Expected: PASS

**Step 5: Add remaining error path tests**

Add tests for:
- `TestInit_SymlinkNotSupported_ReturnsError`
- `TestInit_GitNotInstalled_ShowsWarning`
- `TestInit_CreateDirectoryFails_ReturnsError`
- `TestInit_InteractiveMode_ScansDotfiles`
- `TestInit_ApplyFlag_CreatesSymlinks`

**Step 6: Commit**

```bash
go build ./... && go test ./cmd/dotcor -v
git add cmd/dotcor/init_test.go
git commit -m "test: add error path tests to init command"
```

---

### Task 19: Consolidate duplicate functions in fs package

**Files:**
- Modify: `internal/fs/fs.go`

**Step 1: Remove duplicate `fileExists` function**

```go
// fs.go:21-24
// DELETE this function (duplicate of Exists())
```

**Step 2: Update all callers to use `Exists`**

Search for `fileExists` usage and replace with `Exists`

**Step 3: Run tests to verify no breakage**

Run: `go test ./internal/fs -v`
Expected: All tests still pass

**Step 4: Commit**

```bash
go build ./... && go test ./internal/fs -v
git add internal/fs/fs.go
git commit -m "refactor: remove duplicate fileExists function"
```

---

### Task 20: Remove unused helper function in backup.go

**Files:**
- Modify: `internal/core/backup.go`

**Step 1: Remove unused `findPathSeparator` function**

```go
// backup.go:229-237
// DELETE this function (unused)
```

**Step 2: Run tests to verify no breakage**

Run: `go test ./internal/core -v`
Expected: All tests still pass

**Step 3: Commit**

```bash
go build ./... && go test ./internal/core -v
git add internal/core/backup.go
git commit -m "refactor: remove unused findPathSeparator helper"
```

---

### Task 21: Final coverage verification

**Files:**
- No files created, just verification

**Step 1: Run all tests with coverage**

Run: `go test ./... -coverprofile=coverage.out`
Expected: All tests pass

**Step 2: Generate coverage report**

Run: `go tool cover -func=coverage.out | grep total`
Expected: Coverage percentage displayed

**Step 3: Verify coverage meets targets**

Check each package:
- cmd/dotcor ≥ 75%
- config ≥ 85%
- core ≥ 90%
- fs ≥ 85%
- git ≥ 80%
- overall ≥ 85%

**Step 4: If coverage still below targets, identify remaining gaps**

Run: `go tool cover -func=coverage.out | sort -k3 -n | head -20`
Analyze lowest-coverage functions and add targeted tests

**Step 5: Update TESTING.md with coverage results**

Add section showing before/after coverage for each package

**Step 6: Commit**

```bash
git add docs/TESTING.md
git commit -m "docs: update TESTING.md with coverage improvements"
```

---

### Task 22: Tag release v0.5.5

**Files:**
- Modify: `cmd/dotcor/main.go` (update version)

**Step 1: Update version to 0.5.5**

```go
// cmd/dotcor/main.go:15
var (
    version = "0.5.5" // Test coverage improvements
)
```

**Step 2: Verify build succeeds**

Run: `go build ./...`
Expected: No errors

**Step 3: Verify all tests pass**

Run: `go test ./...`
Expected: All tests pass

**Step 4: Verify coverage meets targets**

Run: `go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out | grep total`
Expected: Overall coverage ≥ 85%

**Step 5: Create annotated tag**

```bash
git add cmd/dotcor/main.go
git commit -m "chore: bump version to 0.5.5"
git tag -a v0.5.5 -m "Release v0.5.5: Comprehensive test coverage improvements

- Added tests for all 11 missing commands
- Increased config package coverage to 85%+
- Increased core package coverage to 90%+
- Increased fs package coverage to 85%+
- Increased git package coverage to 80%+
- Overall test coverage: 21.8% → 85%+
- Added error path tests for critical commands
- Removed duplicate and unused code
- All tests passing"
```

**Step 6: Push tag**

```bash
git push origin main
git push origin v0.5.5
```

---

## Execution Options

Plan complete. Choose execution approach:

**1. Subagent-Driven (Recommended)**
   - I'll dispatch a fresh subagent per task
   - Review between tasks for quality
   - Fast iteration with commits between tasks
   - Stay in this session

**2. Parallel Session (Separate)**
   - Open new session with executing-plans
   - Batch execution with checkpoints
   - You run tasks in new session

**Which approach?**

---

## Remember

- Each task builds and tests before committing
- Follow TDD: test → implement → test → commit
