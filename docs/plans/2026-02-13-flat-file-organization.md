# Flat File Organization Design

## Problem Statement

Currently DotCor auto-organizes files into categorized subdirectories (e.g., `files/shell/zshrc`, `files/vim/.vimrc`). Users want simpler organization where files are placed directly in `/files/` with full paths preserved.

## Design

### Overview

Remove all automatic categorization logic. Files are stored in `~/.dotcor/files/` using their full path relative to home directory.

### Examples

| Source Path | Repo Path |
|-------------|-----------|
| `~/.zshrc` | `files/.zshrc` |
| `~/.config/nvim/init.vim` | `files/.config/nvim/init.vim` |
| `~/.ssh/config` | `files/ssh/config` |
| `/etc/hosts` | `files/etc/hosts` |

### Implementation

**Changes to `internal/config/paths.go`:**

1. Remove `categoryMap` (lines 11-42)
2. Remove `getCategoryByPrefix()` function
3. Simplify `GenerateRepoPath()`:
   ```go
   func GenerateRepoPath(sourcePath string, cfg *Config) (string, error) {
       // Expand source path to absolute
       expanded, err := ExpandPath(sourcePath, cfg)
       if err != nil {
           return "", err
       }

       // Strip home directory prefix
       home, err := os.UserHomeDir()
       if err != nil {
           return "", fmt.Errorf("getting home directory: %w", err)
       }

       relPath := strings.TrimPrefix(expanded, home)
       relPath = strings.TrimPrefix(relPath, string(filepath.Separator))

       return relPath, nil
   }
   ```

**Changes to `cmd/dotcor/add.go`:**

1. Remove `--category` flag from `init()` function
2. Remove category flag parsing from `runAdd()`
3. Remove custom repo path logic from `processAddFile()` (lines 279-287)
4. Simplify `GenerateRepoPath()` call to remove second argument

### What Stays the Same

- **Config tracking**: `SourcePath` still stores full normalized path (`~/.zshrc`)
- **Symlinks**: Created correctly relative to file location
- **Template files**: `--template` flag appends `.template` extension

### What's Removed

- **`--category` flag**: No longer needed for manual categorization
- **`categoryMap`**: Automatic categorization logic removed
- **`getCategoryByPrefix()`**: Prefix-based categorization removed

### Migration

**No migration required.**

- Existing repos with categorized files continue to work
- Config references to old paths remain valid
- New files added will use flat structure
- Mixed repositories (some categorized, some flat) are supported

### Testing

**New test cases for `TestGenerateRepoPath`:**
- `~/.zshrc` → `.zshrc`
- `~/.config/nvim/init.vim` → `.config/nvim/init.vim`
- `~/.ssh/config` → `ssh/config`
- `/etc/hosts` → `/etc/hosts` (non-home paths)

**Remove test cases for:**
- Category-based organization (all category tests)
- Prefix matching
- Custom category override behavior

**Add test case for path collision:**
- Attempting to add the same file twice should fail gracefully
- Error message should indicate file is already managed

**Add test for symlink creation with flat paths:**
- Verify symlink points to correct repo path
- Test with both flat files (`files/.zshrc`) and nested (`files/.config/nvim/init.vim`)

**Update `README.md` examples:**
- Change all examples to show flat structure
- Update tree diagrams
- Remove `--category` flag examples

## Benefits

1. **Simpler mental model**: Files go where you expect
2. **No categorization complexity**: No mapping or prefix logic
3. **Full path preservation**: Easy to find files by original location
4. **Less code**: ~50 lines removed from paths.go and add.go
5. **Cleaner CLI**: Removes `--category` flag

## Trade-offs

- **Path collisions**: If two different source files resolve to same repo path (unlikely with full paths), second will fail. This is acceptable and already handled by existing validation.
- **No grouping**: Related files (like all shell configs) won't be grouped automatically.

## Alternatives Considered

1. **Strip leading dots**: Cleaner repo (`zshrc` vs `.zshrc`), but less obvious what the original file was. Rejected - user prefers full paths.

2. **Flatten all subdirectories**: All files in `/files/` root. Rejected - causes naming conflicts for files like `init.vim` in different source directories.

3. **Keep categorization as default, add `--flat` flag**: Adds complexity for a one-time decision. Rejected - simpler to just change default behavior.

4. **Keep `--category` flag for manual overrides**: Provides escape hatch for organization. Rejected - user wants full removal, no manual categorization needed.

## Version

Target release: **v0.7.4**
