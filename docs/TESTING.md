# DotCor Testing Guide

Testing practices and conventions for DotCor v2.0.

## Overview

1. **Testify framework** — `github.com/stretchr/testify` for all assertions
2. **AAA Pattern** — Arrange-Act-Assert with section comments
3. **Pre-commit validation** — `go build ./... && go test ./...`
4. **Tests alongside code** — `*_test.go` next to the source file

## Coverage Targets

| Package | Target |
|---------|--------|
| config  | 85%    |
| core    | 90%    |
| stow    | 85%    |
| fs      | 85%    |
| git     | 80%    |

## Project Structure

```
dotcor/
├── cmd/dotcor/
│   └── main.go                  # thin entry point
│
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   ├── config_test.go
│   │   ├── paths.go
│   │   └── paths_test.go
│   ├── core/
│   │   ├── backup.go            → backup_test.go
│   │   ├── lock.go              → lock_test.go
│   │   ├── transaction.go       → transaction_test.go
│   │   ├── validator.go         → validator_test.go
│   │   ├── ignore.go            → ignore_test.go
│   │   ├── hooks.go             → hooks_test.go
│   │   └── templates.go         → templates_test.go
│   ├── fs/
│   │   ├── fs.go                → fs_test.go
│   │   └── symlink.go           → symlink_test.go
│   ├── git/
│   │   └── git.go               → git_test.go
│   ├── logger/
│   │   └── logger.go            → logger_test.go
│   └── stow/
│       ├── package.go           → package_test.go
│       ├── link.go              → link_test.go
│       ├── unlink.go            → unlink_test.go
│       └── migrate.go           → migrate_test.go
│
├── tui/
│   ├── app.go                   # root model
│   ├── dashboard.go
│   └── ...                      # views and styles
│
└── tests/
    └── integration_test.go      # stow workflow tests
```

## AAA Pattern

Every test has three clearly marked sections:

```go
func TestLink_SingleFile_CreatesSymlink(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    homeDir := filepath.Join(tempDir, "home")
    pkgDir := filepath.Join(tempDir, "repo", "zsh")
    require.NoError(t, os.MkdirAll(pkgDir, 0755))
    require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("zsh"), 0644))

    // Act
    result, err := stow.Link(pkgDir, homeDir)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, 1, result.Linked)
}
```

## Testify Usage

Use `require` for setup that must succeed (stops test on failure). Use `assert` for checks that should all report:

```go
require.NoError(t, err, "setup must succeed")
assert.Equal(t, expected, got, "result should match")
```

## TUI Model Tests

Test Bubble Tea models by calling `Update()` with messages and asserting state changes:

```go
func TestDashboard_StowKey_UpdatesPackageStatus(t *testing.T) {
    // Arrange
    model := tui.NewModel(cfg)

    // Act
    newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

    // Assert
    m := newModel.(tui.Model)
    assert.Equal(t, "linked", m.SelectedPackage().Status)
}
```

## Table-Driven Tests

Use for multiple similar scenarios:

```go
func TestValidateRepoPath(t *testing.T) {
    tests := []struct {
        name    string
        path    string
        wantErr bool
    }{
        {"valid relative path", "shell/zshrc", false},
        {"absolute path", "/shell/zshrc", true},
        {"path traversal", "../shell/zshrc", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateRepoPath(tt.path)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Test Naming

```
Test<Function>_<Scenario>_<ExpectedResult>
```

Examples:
- `TestLink_SingleFile_CreatesSymlink`
- `TestUnlink_NoSymlink_SkipsFile`
- `TestTransaction_Failure_RollsBack`

## Test Isolation

- Use `t.TempDir()` for all filesystem tests (auto-cleanup)
- Mark helpers with `t.Helper()`
- Each test is independent — no shared state, no execution order dependencies

## Running Tests

```bash
go test ./...                        # all tests
go test ./internal/stow/... -v       # specific package
go test ./... -coverprofile=cover.out && go tool cover -func=cover.out  # coverage
go test -race ./...                  # race detection
```

## Integration Tests

`tests/integration_test.go` covers full stow workflows:

- Discover packages, link, verify symlinks
- Unlink, verify cleanup
- Stow after edit, verify Git commit
- V1 layout migration

## Pre-Commit Workflow

Every commit must pass:

```bash
go build ./... && go test ./...
```

Fix any failures before committing.

## Checklist

- [ ] Tests use AAA pattern with section comments
- [ ] Tests use testify assertions
- [ ] Happy path, error path, and edge cases covered
- [ ] Tests use `t.TempDir()` for isolation
- [ ] Helpers marked with `t.Helper()`
- [ ] Coverage meets package target
- [ ] `go build ./... && go test ./...` passes
