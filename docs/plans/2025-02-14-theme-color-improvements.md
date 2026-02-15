# Theme Color Improvements and Bug Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Apply light pink theme color to important command output sections and fix command issues

**Architecture:** Use existing color constants in main.go, add Cobra help templates for colored help output, improve error messages for invalid paths in remove command

**Tech Stack:** Cobra CLI framework, Go 1.21+, ANSI color codes

---

## Task 1: Color `dotcor init` success messages

**Files:**
- Modify: `cmd/dotcor/init.go`

**Step 1: Find init success messages in init.go**

Search for "DotCor initialized" or success messages near line 143 (after config.yaml creation)

**Step 2: Color "DotCor initialized successfully!" message**

Change from:
```go
fmt.Printf("DotCor initialized successfully!\n")
```

To:
```go
fmt.Printf("%sDotCor initialized successfully!%s\n", colorLightPink, colorReset)
```

**Step 3: Add colored initialization message near start**

After line 55 (after loading config), add:
```go
fmt.Printf("%sInitializing DotCor...%s\n", colorLightPink, colorReset)
```

**Step 4: Test colored init output**

Run:
```bash
rm -rf ~/.dotcor
./bin/dotcor init
```

Expected: See "Initializing DotCor..." and "DotCor initialized successfully!" in light pink

**Step 5: Commit**

```bash
git add cmd/dotcor/init.go
git commit -m "feat: color init success messages with theme pink"
```

---

## Task 2: Add `?` alias for help command

**Files:**
- Modify: `cmd/dotcor/main.go` (rootCmd setup, around line 87)

**Step 1: Find rootCmd definition and add alias**

After `Aliases: []string{...}` line (currently empty), add:

```go
var rootCmd = &cobra.Command{
    Use:     "dotcor",
    Short:   "A simple, fast dotfile manager with symlinks and Git automation",
    Aliases: []string{"?"},
    Long: `DotCor combines simplicity of GNU Stow with automatic Git commits.
...
```

**Step 2: Test ? alias**

Run:
```bash
./bin/dotcor ? | head -20
```

Expected: Help output displays (same as `dotcor --help`)

**Step 3: Commit**

```bash
git add cmd/dotcor/main.go
git commit -m "feat: add ? as alias for help command"
```

---

## Task 3: Color help description in banner

**Files:**
- Modify: `cmd/dotcor/main.go` (rootCmd.Long, around line 90)

**Step 1: Find Long description in rootCmd**

Locate rootCmd.Long around line 90 and color key phrase:

From:
```go
Long: `DotCor combines simplicity of GNU Stow with automatic Git commits.
...
```

To:
```go
Long: fmt.Sprintf(`DotCor combines %s%ssthe simplicity of GNU Stow%s%s with automatic Git commits.

Manage your dotfiles with symlinks - edit files directly, changes instantly
appear in your repository. Built-in Git automation handles commits and sync.`,
    colorRed, colorBold, colorReset, colorRed),
```

**Step 2: Test colored help description**

Run:
```bash
./bin/dotcor --help | head -10
```

Expected: "the simplicity of GNU Stow" appears in red

**Step 3: Commit**

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

**Step 3: Test colored help sections**

Run:
```bash
./bin/dotcor --help
./bin/dotcor add --help
```

Expected: "Available Commands:", "Flags:", "Global Flags:" appear in light pink

**Step 4: Commit**

```bash
git add cmd/dotcor/help_templates.go cmd/dotcor/main.go
git commit -m "feat: add colored help templates with theme pink"
```

---

## Task 5: Remove `dotcor list` command

**Files:**
- Delete: `cmd/dotcor/list.go`
- Modify: `scripts/test-manual.sh` (remove list references)

**Step 1: Delete list.go file**

Run:
```bash
rm cmd/dotcor/list.go
```

**Step 2: Remove list from test-manual.sh**

Search for "list" in test-manual.sh and remove references (lines suggesting `dotcor list` usage)

**Step 3: Verify build**

Run:
```bash
make binary
```

Expected: Binary builds successfully (no errors about missing list)

**Step 4: Test that list command is gone**

Run:
```bash
./bin/dotcor list 2>&1
```

Expected: "unknown command" error

**Step 5: Commit**

```bash
git add cmd/dotcor/list.go scripts/test-manual.sh
git commit -m "refactor: remove dotcor list command (use status instead)"
```

---

## Task 6: Improve `dotcor remove` error messages for invalid paths

**Files:**
- Modify: `cmd/dotcor/remove.go` (runRemove function, around line 86)

**Step 1: Add path validation and helpful error messages**

Before line 86 (the loop through args), add validation:

```go
// Validate paths and provide helpful error messages
for _, arg := range args {
    // Check if it's a tilde path
    if !strings.HasPrefix(arg, "~") && !strings.HasPrefix(arg, "/") && !strings.HasPrefix(arg, ".") {
        fmt.Fprintf(os.Stderr, "  %s[!]%s %s: use ~ for home directory (e.g., ~/.zshrc)\n", colorYellow, colorReset, arg)
        fmt.Fprintf(os.Stderr, "      Use 'dotcor status' to see managed files\n")
    }
}
```

**Step 2: Test improved error message**

Run:
```bash
./bin/dotcor remove .zsrhc 2>&1
```

Expected: Helpful message suggesting to use ~ for home directory paths

**Step 3: Commit**

```bash
git add cmd/dotcor/remove.go
git commit -m "feat: improve remove error messages with path hints"
```

---

## Task 7: Color section headers in `dotcor status`

**Files:**
- Modify: `cmd/dotcor/status.go` (various output functions)

**Step 1: Find section headers in status output**

Search for section titles like "Status:", "Files:", "Git:", "Repository:", etc.

**Step 2: Color all section headers**

Examples to color (around lines 155-170):

From:
```go
fmt.Printf("  %sStatus%s\n", colorBold, colorReset)
fmt.Printf("  %s──────%s\n", colorDim, colorReset)
```

To:
```go
fmt.Printf("  %s%sStatus%s\n", colorBold, colorLightPink, colorReset)
fmt.Printf("  %s──────%s\n", colorDim, colorReset)
```

Find all similar patterns and add `colorLightPink` to section headers.

**Step 3: Test colored status sections**

Run:
```bash
./bin/dotcor status
```

Expected: All section headers (Status, Files, Git, Repository, etc.) in light pink

**Step 4: Commit**

```bash
git add cmd/dotcor/status.go
git commit -m "feat: color status section headers with theme pink"
```

---

## Task 8: Update tests for color output (if needed)

**Files:**
- Modify: `cmd/dotcor/test_helpers.go` (create helper if needed)
- Modify: `cmd/dotcor/*_test.go` (any tests checking output)

**Step 1: Find tests that check plain output**

Search for test files that check exact string output

**Step 2: Update tests to ignore color codes or strip them**

Example helper function to add to test_helpers.go:

```go
// StripANSIColors removes ANSI escape codes from strings for testing
func StripANSIColors(s string) string {
    re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
    return re.ReplaceAllString(s, "")
}
```

**Step 3: Run tests**

Run:
```bash
go test ./cmd/dotcor/...
```

Expected: All tests pass (color codes don't break tests)

**Step 4: Commit**

```bash
git add cmd/dotcor/test_helpers.go cmd/dotcor/*_test.go
git commit -m "test: handle ANSI color codes in output tests"
```

---

## Task 9: Final verification and cleanup

**Files:**
- All modified files

**Step 1: Run full test suite**

```bash
go test ./...
```

Expected: All tests pass

**Step 2: Build and verify binary**

```bash
make binary
./bin/dotcor --help
./bin/dotcor init
./bin/dotcor status
./bin/dotcor remove ~/.zshrc --dry-run
```

Expected: All commands show colored output appropriately

**Step 3: Test interactive mode**

```bash
make test-clean
make test-interactive
```

In the interactive shell, test:
```bash
dotcor ?
dotcor add ~/.zshrc
dotcor status
dotcor remove .zsrhc
```

Expected: ? works, remove shows helpful error, status sections are colored

**Step 4: Final commit**

```bash
git add .
git commit -m "chore: final verification of color theme improvements"
```

---

## Summary of Changes

1. **Colored init messages** - Light pink for "Initializing DotCor..." and success message
2. **Help alias** - `?` now works as alias for help
3. **Colored help sections** - Usage:, Available Commands:, Flags: in light pink
4. **Removed list command** - Use `dotcor status` instead
5. **Improved remove errors** - Helpful hints for invalid paths
6. **Colored status sections** - All section headers in light pink
7. **Red help description** - "the simplicity of GNU Stow" in banner help

**Testing checklist:**
- [ ] All commands show colored sections
- [ ] `?` alias works
- [ ] `dotcor list` removed and shows error
- [ ] `dotcor remove` shows helpful error for `.zsrhc`
- [ ] All tests pass
- [ ] Build succeeds
