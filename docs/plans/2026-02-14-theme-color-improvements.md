# Theme Color Improvements and Bug Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Apply light pink theme color to all important command output sections and fix command issues

**Architecture:** Use existing color constants in main.go, add Cobra help templates for colored help output, improve error messages for invalid paths in remove command

**Tech Stack:** Cobra CLI framework, Go 1.21+, ANSI color codes

**Pre-commit verification for ALL tasks:**
```bash
go build ./...
go test ./...
```

---

## Task 1: Color `dotcor init` important messages

**Files:**
- Modify: `cmd/dotcor/init.go`

**Step 1: Add "Initializing DotCor..." message with color**

After line 58 (configureLogger call), add:
```go
fmt.Printf("%sInitializing DotCor...%s\n", colorLightPink, colorReset)
```

**Step 2: Color "DotCor initialized successfully!" message**

Change line 157 from:
```go
fmt.Println("DotCor initialized successfully!")
```

To:
```go
fmt.Printf("%sDotCor initialized successfully!%s\n", colorLightPink, colorReset)
```

**Step 3: Color "Next steps:" header**

Change line 159 from:
```go
fmt.Println("Next steps:")
```

To:
```go
fmt.Printf("Next steps:\n")
```

Then color the whole section (lines 159-163) by adding pink to the header:
```go
fmt.Printf("%sNext steps:%s\n", colorLightPink, colorReset)
```

**Step 4: Color "Creating symlinks..." message in applySymlinks**

Find line 175 in applySymlinks function and change from:
```go
fmt.Printf("\nCreating symlinks for %d files...\n", len(files))
```

To:
```go
fmt.Printf("\n%sCreating symlinks for %d files...%s\n", colorLightPink, len(files), colorReset)
```

**Step 5: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 6: Commit**

```bash
git add cmd/dotcor/init.go
git commit -m "feat: color init important messages with theme pink"
```

---

## Task 2: Add `?` alias for help command

**Files:**
- Modify: `cmd/dotcor/main.go` (rootCmd setup, around line 87)

**Step 1: Find rootCmd definition and add alias**

Add `Aliases: []string{"?"}` to rootCmd:

```go
var rootCmd = &cobra.Command{
    Use:     "dotcor",
    Short:   "A simple, fast dotfile manager with symlinks and Git automation",
    Aliases: []string{"?"},
    Long: `DotCor combines the simplicity of GNU Stow with automatic Git commits.
...
```

**Step 2: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 3: Test ? alias**

```bash
./bin/dotcor ? | head -20
```

Expected: Help output displays (same as `dotcor --help`)

**Step 4: Commit**

```bash
git add cmd/dotcor/main.go
git commit -m "feat: add ? as alias for help command"
```

---

## Task 3: Color help description in banner

**Files:**
- Modify: `cmd/dotcor/main.go` (rootCmd.Long, around line 90)

**Step 1: Find Long description in rootCmd**

Locate rootCmd.Long around line 90 and color "the simplicity of GNU Stow" in red:

From:
```go
Long: `DotCor combines the simplicity of GNU Stow with automatic Git commits.
...
```

To:
```go
Long: fmt.Sprintf(`DotCor combines %s%ssthe simplicity of GNU Stow%s%s with automatic Git commits.

Manage your dotfiles with symlinks - edit files directly, changes instantly
appear in your repository. Built-in Git automation handles commits and sync.`,
    colorRed, colorBold, colorReset, colorRed),
```

**Step 2: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 3: Test colored help description**

```bash
./bin/dotcor --help | head -10
```

Expected: "the simplicity of GNU Stow" appears in red

**Step 4: Commit**

```bash
git add cmd/dotcor/main.go
git commit -m "feat: color help description with red"
```

---

## Task 4: Create custom Cobra help templates for colored sections

**Files:**
- Create: `cmd/dotcor/help_templates.go`
- Modify: `cmd/dotcor/main.go` (init function)

**Step 1: Create help templates file**

Create `cmd/dotcor/help_templates.go` with:

```go
package main

// Cobra help templates with colors
var helpTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

%sAvailable Commands:%s{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

%sFlags:%s{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

%sGlobal Flags:%s{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

var usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.UseLine}} [command]{{end}}{{if .HasAvailableSubCommands}}

{{if not (eq .Name "help")}}%sAvailable Commands:%s{{end}}{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

%sFlags:%s{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

%sGlobal Flags:%s{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

var commandTemplate = `Usage:

  {{.UseLine}}

{{if .HasAvailableSubCommands}}%sAvailable Commands:%s{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

%sFlags:%s
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

%sGlobal Flags:%s
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
```

**Step 2: Apply templates in main.go init function**

After line 104 (persistent flags), add:

```go
// Set custom help templates with colors
rootCmd.SetHelpTemplate(fmt.Sprintf(helpTemplate, colorLightPink, colorReset, colorLightPink, colorReset, colorLightPink, colorReset))
rootCmd.SetUsageTemplate(fmt.Sprintf(usageTemplate, colorLightPink, colorReset, colorLightPink, colorReset, colorLightPink, colorReset))
rootCmd.SetCommandTemplate(fmt.Sprintf(commandTemplate, colorLightPink, colorReset, colorLightPink, colorReset, colorLightPink, colorReset))
```

**Step 3: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 4: Test colored help sections**

```bash
./bin/dotcor --help
./bin/dotcor add --help
./bin/dotcor status --help
```

Expected: "Available Commands:", "Flags:", "Global Flags:" appear in light pink

**Step 5: Commit**

```bash
git add cmd/dotcor/help_templates.go cmd/dotcor/main.go
git commit -m "feat: add colored help templates with theme pink"
```

---

## Task 5: Color section headers in `dotcor remove`

**Files:**
- Modify: `cmd/dotcor/remove.go`

**Step 1: Color "Summary:" header**

Change line 102 from:
```go
fmt.Println("Summary:")
```

To:
```go
fmt.Printf("%sSummary:%s\n", colorLightPink, colorReset)
```

**Step 2: Enhance "not managed" error message with path hint**

Find the "not managed" error around line 89 and enhance it:

From:
```go
fmt.Fprintf(os.Stderr, "  %s[X]%s %s: not managed\n", colorRed, colorReset, arg)
```

To:
```go
fmt.Fprintf(os.Stderr, "  %s[X]%s %s: not managed\n", colorRed, colorReset, arg)
// Suggest using ~ for relative paths
if !strings.HasPrefix(arg, "~") && !strings.HasPrefix(arg, "/") && !strings.HasPrefix(arg, ".") {
    fmt.Fprintf(os.Stderr, "      %s[!]%s Tip: Use ~ for home directory (e.g., ~/.zshrc)\n", colorYellow, colorReset)
}
```

**Step 3: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 4: Test improved messages**

```bash
./bin/dotcor remove .zsrhc 2>&1
```

Expected: Shows helpful tip about using ~ for home directory paths

**Step 5: Commit**

```bash
git add cmd/dotcor/remove.go
git commit -m "feat: color remove summary and improve error messages"
```

---

## Task 6: Color section headers in `dotcor history`

**Files:**
- Modify: `cmd/dotcor/history.go`

**Step 1: Color "History for [file]:" header**

Find line 110 and change from:
```go
fmt.Printf("History for %s:\n", filePath)
```

To:
```go
fmt.Printf("%sHistory for %s:%s\n", colorLightPink, filePath, colorReset)
```

**Step 2: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 3: Commit**

```bash
git add cmd/dotcor/history.go
git commit -m "feat: color history command header with theme pink"
```

---

## Task 7: Color section headers in `dotcor restore`

**Files:**
- Modify: `cmd/dotcor/restore.go`

**Step 1: Color "Current version:" header**

Find line 139 and change from:
```go
fmt.Printf("\nCurrent version:\n")
```

To:
```go
fmt.Printf("\n%sCurrent version:%s\n", colorLightPink, colorReset)
```

**Step 2: Color "Backup:" header**

Find line 223 and change from:
```go
fmt.Printf("Backup: %s (%s)\n", backup.BackupPath, backup.Timestamp.Format("2006-01-02 15:04:05"))
```

To:
```go
fmt.Printf("%sBackup:%s %s (%s)\n", colorLightPink, colorReset, backup.BackupPath, backup.Timestamp.Format("2006-01-02 15:04:05"))
```

**Step 3: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 4: Commit**

```bash
git add cmd/dotcor/restore.go
git commit -m "feat: color restore section headers with theme pink"
```

---

## Task 8: Color section headers in `dotcor cleanup`

**Files:**
- Modify: `cmd/dotcor/cleanup.go`

**Step 1: Color "Current backups:" header**

Find line 81 and change from:
```go
fmt.Printf("Current backups: %d files, %s\n", backupCount, formatSize(totalSize))
```

To:
```go
fmt.Printf("%sCurrent backups:%s %d files, %s\n", colorLightPink, colorReset, backupCount, formatSize(totalSize))
```

**Step 2: Color "Remaining:" header**

Find line 134 and change from:
```go
fmt.Printf("Remaining: %d files, %s\n", newCount, formatSize(newSize))
```

To:
```go
fmt.Printf("%sRemaining:%s %d files, %s\n", colorLightPink, colorReset, newCount, formatSize(newSize))
```

**Step 3: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 4: Commit**

```bash
git add cmd/dotcor/cleanup.go
git commit -m "feat: color cleanup section headers with theme pink"
```

---

## Task 9: Color section headers in `dotcor rebuild`

**Files:**
- Modify: `cmd/dotcor/rebuild.go`

**Step 1: Color "Missing from repository:" header**

Find line 126 and change from:
```go
fmt.Printf("Missing from repository (%d):\n", len(missing))
```

To:
```go
fmt.Printf("%sMissing from repository:%s (%d)\n", colorLightPink, colorReset, len(missing))
```

**Step 2: Color "Not in configuration:" header**

Find line 134 and change from:
```go
fmt.Printf("Not in configuration (%d):\n", len(orphaned))
```

To:
```go
fmt.Printf("%sNot in configuration:%s (%d)\n", colorLightPink, colorReset, len(orphaned))
```

**Step 3: Color "Found untracked files:" header**

Find line 175 and change from:
```go
fmt.Printf("Found %d untracked file(s):\n", len(untracked))
```

To:
```go
fmt.Printf("%sFound untracked files:%s %d\n", colorLightPink, colorReset, len(untracked))
```

**Step 4: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 5: Commit**

```bash
git add cmd/dotcor/rebuild.go
git commit -m "feat: color rebuild section headers with theme pink"
```

---

## Task 10: Color section headers in `dotcor list-backups`

**Files:**
- Modify: `cmd/dotcor/list-backups.go`

**Step 1: Color "Found X backup(s):" header**

Find line 99 and change from:
```go
fmt.Printf("Found %d backup(s):\n\n", len(backups))
```

To:
```go
fmt.Printf("%sFound %d backup(s):%s\n\n", colorLightPink, len(backups), colorReset)
```

**Step 2: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 3: Commit**

```bash
git add cmd/dotcor/list-backups.go
git commit -m "feat: color list-backups header with theme pink"
```

---

## Task 11: Color section headers in `dotcor backup-diff`

**Files:**
- Modify: `cmd/dotcor/backup-diff.go`

**Step 1: Color "Changes since backup for [file]:" header**

Find line 71 and change from:
```go
fmt.Printf("Changes since backup for %s:\n", mf.SourcePath)
```

To:
```go
fmt.Printf("%sChanges since backup for %s:%s\n", colorLightPink, mf.SourcePath, colorReset)
```

**Step 2: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 3: Commit**

```bash
git add cmd/dotcor/backup-diff.go
git commit -m "feat: color backup-diff header with theme pink"
```

---

## Task 12: Color section headers in main (quick status)

**Files:**
- Modify: `cmd/dotcor/main.go`

**Step 1: Color "Status:" header in showQuickStatus**

Find line 155 and change from:
```go
fmt.Printf("  %sStatus%s\n", colorBold, colorReset)
```

To:
```go
fmt.Printf("  %s%sStatus%s\n", colorBold, colorLightPink, colorReset)
```

**Step 2: Color "Get started:" header in runRoot**

Find line 127 and change from:
```go
fmt.Printf("  %sGet started:%s\n", colorDim, colorReset)
```

To:
```go
fmt.Printf("  %sGet started:%s\n", colorLightPink, colorReset)
```

**Step 3: Color "Commands:" header in showQuickStatus**

Find line 192 and change from:
```go
fmt.Printf("  %sCommands:%s  status · add · sync · --help\n", colorDim, colorReset)
```

To:
```go
fmt.Printf("  %sCommands:%s  status · add · sync · --help\n", colorLightPink, colorReset)
```

**Step 4: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 5: Commit**

```bash
git add cmd/dotcor/main.go
git commit -m "feat: color main command section headers with theme pink"
```

---

## Task 13: Remove `dotcor list` command

**Files:**
- Delete: `cmd/dotcor/list.go`
- Modify: `scripts/test-manual.sh` (remove list references)
- Modify: `cmd/dotcor/init.go` (update "Next steps" list)

**Step 1: Delete list.go file**

```bash
rm cmd/dotcor/list.go
```

**Step 2: Remove list from init.go "Next steps"**

In init.go around line 161, remove or update the line:
```go
fmt.Println("  dotcor list             # List managed files")
```

Replace with:
```go
fmt.Println("  dotcor status           # List managed files and status")
```

**Step 3: Remove list from test-manual.sh**

Search for "list" in test-manual.sh and remove references to `dotcor list` command

**Step 4: Verify build and tests**

```bash
go build ./...
go test ./...
```

Expected: Build succeeds, all tests pass

**Step 5: Test that list command is gone**

```bash
./bin/dotcor list 2>&1
```

Expected: "unknown command" error

**Step 6: Commit**

```bash
git add cmd/dotcor/list.go cmd/dotcor/init.go scripts/test-manual.sh
git commit -m "refactor: remove dotcor list command (use status instead)"
```

---

## Task 14: Update tests for color codes (if needed)

**Files:**
- Modify: `cmd/dotcor/*_test.go` (any tests checking output)

**Step 1: Find tests that check exact string output**

```bash
grep -r "fmt.Println" cmd/dotcor/*_test.go | grep -i "assert\|equal"
```

Or check for tests that test command output

**Step 2: If tests exist, create StripANSIColors helper**

In `cmd/dotcor/test_helpers.go`, add if not exists:

```go
import "regexp"

// StripANSIColors removes ANSI escape codes from strings for testing
func StripANSIColors(s string) string {
    re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
    return re.ReplaceAllString(s, "")
}
```

**Step 3: Update tests to use StripANSIColors**

If any tests check output strings, wrap expected/actual values in StripANSIColors()

**Step 4: Verify all tests pass**

```bash
go test ./...
```

Expected: All tests pass

**Step 5: Commit**

```bash
git add cmd/dotcor/test_helpers.go cmd/dotcor/*_test.go
git commit -m "test: handle ANSI color codes in output tests"
```

---

## Task 15: Final verification

**Files:**
- All modified files

**Step 1: Run full test suite**

```bash
go test ./...
```

Expected: All tests pass

**Step 2: Build binary**

```bash
make binary
```

Expected: Binary builds successfully

**Step 3: Verify all color changes**

Test each command manually:

```bash
# Test help colors and ? alias
./bin/dotcor ?
./bin/dotcor --help
./bin/dotcor add --help

# Test init colors
rm -rf ~/.dotcor
./bin/dotcor init

# Test status colors (already colored, verify still works)
./bin/dotcor status

# Test remove colors
./bin/dotcor remove .zsrhc 2>&1

# Verify list is removed
./bin/dotcor list 2>&1
```

Expected:
- All help sections (Available Commands:, Flags:, Global Flags:) in light pink
- ? alias works
- init shows "Initializing DotCor..." and "DotCor initialized successfully!" in light pink
- remove shows colored Summary and helpful error message
- list command shows "unknown command"
- All other commands show appropriate colored sections

**Step 4: Final commit**

```bash
git add .
git commit -m "chore: final verification of color theme improvements"
```

---

## Summary of Changes

1. **init command** - Color "Initializing DotCor...", "DotCor initialized successfully!", "Next steps:", "Creating symlinks..."
2. **Help alias** - `?` now works as alias for help
3. **Help description** - "the simplicity of GNU Stow" in red
4. **Help templates** - Usage:, Available Commands:, Flags: in light pink
5. **remove command** - Color "Summary:", enhance "not managed" error with path hints
6. **history command** - Color "History for [file]:"
7. **restore command** - Color "Current version:", "Backup:"
8. **cleanup command** - Color "Current backups:", "Remaining:"
9. **rebuild command** - Color "Missing from repository:", "Not in configuration:", "Found untracked files:"
10. **list-backups command** - Color "Found X backup(s):"
11. **backup-diff command** - Color "Changes since backup for [file]:"
12. **main command** - Color "Status:", "Get started:", "Commands:"
13. **Remove list command** - Delete and update references
14. **Test compatibility** - Handle ANSI color codes in tests

**Commands already using light pink (no changes needed):**
- add.go: "Summary:" ✓
- sync.go: "Sync Preview", "Uncommitted changes:" ✓
- doctor.go: "DotCor Doctor" ✓
- status.go: "DotCor Status", "Managed Files:", "Git Repository:", "Summary:" ✓
- internal/core/validation.go: "Pre-flight checks:" ✓

**Testing checklist:**
- [ ] All commands show colored section headers
- [ ] `?` alias works
- [ ] `dotcor list` removed and shows error
- [ ] `dotcor remove` shows helpful error for `.zsrhc`
- [ ] All tests pass
- [ ] Build succeeds
- [ ] Manual verification of each command
