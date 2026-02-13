# Manual Testing Guide for DotCor

## Quick Start

### Build the Binary
```bash
# Using Makefile (recommended)
make binary

# Or directly
go build -o bin/dotcor ./cmd/dotcor

# Then use it
./bin/dotcor
```

### Install to PATH
```bash
make install
# Now you can run: dotcor from anywhere
```

---

## Testing Commands Safely

## Quick Start

### Build the Binary
```bash
# Using Makefile (recommended)
make binary

# Or directly
go build -o bin/dotcor ./cmd/dotcor

# Then use it
./bin/dotcor
```

### Install to PATH
```bash
make install
# Now you can run: dotcor from anywhere
```

---

## Testing Commands Safely

The project includes a dedicated manual testing environment in **`.manual-test/`** (in project root). This directory is gitignored so test files won't be committed.

### Option 1: Interactive Testing (Recommended!)
```bash
make test-interactive
# This opens a shell with:
#   - HOME set to .manual-test/
#   - PATH includes bin/ (so 'dotcor' uses v0.7.3)
#   - Test environment isolated from real dotfiles
```

Now you can run commands normally:
```bash
dotcor init          # Uses built binary (v0.7.3)
dotcor status
dotcor add ~/.testrc
```

**Important:** In interactive mode, `dotcor` uses the built binary from `bin/` (not system version).

---

## Common Manual Test Scenarios

### 1. Test Init Flow
```bash
# Create test directory
mkdir -p /tmp/test-dotcor
cd /tmp/test-dotcor

# Initialize
./bin/dotcor init

# Check what was created
ls -la ~/.dotcor/
```

### 2. Test Adding Files
```bash
# Create a test file
echo "export TEST_VAR='hello'" > ~/.testrc

# Add it
./bin/dotcor add ~/.testrc

# Verify symlink created
ls -la ~/.testrc

# Check list
./bin/dotcor list
```

### 3. Test Status
```bash
# Check status of all files
./bin/dotcor status

# Check quick status
./bin/dotcor status --quick

# Check specific file
./bin/dotcor status ~/.testrc
```

### 4. Test Editing Files (Symlink Magic)
```bash
# Edit the file normally
echo "export NEW_VAR='world'" >> ~/.testrc

# Changes are immediately in repo!
./bin/dotcor status

# Commit the changes
./bin/dotcor sync
```

### 5. Test Removing Files
```bash
# Stop managing a file
./bin/dotcor remove ~/.testrc

# Choose options interactively
# Keep file? y
# Delete from repo? n
```

### 6. Test History/Restore
```bash
# View history
./bin/dotcor history ~/.testrc

# Restore from specific commit
./bin/dotcor restore ~/.testrc --to=HEAD~1
```

### 7. Test Backup/Restore
```bash
# View backups
./bin/dotcor list-backups

# See diff from backup
./bin/dotcor backup-diff ~/.testrc
```

---

## Full Manual Test Workflow

### Complete E2E Test
```bash
# 1. Clean slate
rm -rf ~/.dotcor
rm -f ~/.testrc ~/.config/nvim/*

# 2. Initialize
./bin/dotcor init

# 3. Create test files
mkdir -p ~/.config/nvim
echo "vim config" > ~/.config/nvim/init.lua
echo "zsh config" > ~/.zshrc

# 4. Add files
./bin/dotcor add ~/.zshrc
./bin/dotcor add ~/.config/nvim/*.lua

# 5. Check status
./bin/dotcor status
./bin/dotcor list

# 6. Edit file (symlink magic test)
echo "new line" >> ~/.zshrc
./bin/dotcor status  # Should show uncommitted

# 7. Sync
./bin/dotcor sync

# 8. History
./bin/dotcor history ~/.zshrc

# 9. Restore
./bin/dotcor restore ~/.zshrc --to=HEAD~1

# 10. Remove
./bin/dotcor remove ~/.zshrc
```

---

## Testing Specific Features

### Test Glob Patterns
```bash
mkdir -p ~/.config/nvim
echo "1" > ~/.config/nvim/a.lua
echo "2" > ~/.config/nvim/b.lua
echo "3" > ~/.config/nvim/c.lua

# Add all at once
./bin/dotcor add ~/.config/nvim/*.lua
```

### Test Dry Run
```bash
# See what would happen without making changes
./bin/dotcor add ~/.testrc --dry-run
./bin/dotcor remove ~/.testrc --dry-run
```

### Test Categories
```bash
# List grouped by category
./bin/dotcor list --category

# Show count per category
./bin/dotcor list --categories
```

### Test Doctor
```bash
# Check health
./bin/dotcor doctor

# Auto-fix issues
./bin/dotcor doctor --fix
```

---

## Debugging Manual Tests

### Enable Debug Logging
```bash
./bin/dotcor status --debug
```

### Write Logs to File
```bash
./bin/dotcor add ~/.testrc --log-file=/tmp/dotcor-debug.log
cat /tmp/dotcor-debug.log
```

### Check Lock Issues
```bash
# View lock status
cat ~/.dotcor/.lock

# Force remove stale lock
rm ~/.dotcor/.lock
```

---

## Testing with Git Integration

### Setup Test Git Repo
```bash
# Initialize with git remote
cd ~/.dotcor/files
git init
git remote add origin git@github.com:you/test-repo.git

# Test sync
cd ~
./bin/dotcor init
./bin/dotcor add ~/.testrc
./bin/dotcor sync  # Should commit and push
```

---

## Performance Testing

### Add Many Files
```bash
# Create 100 test files
for i in {1..100}; do
  echo "content $i" > ~/.test-file-$i
  ./bin/dotcor add ~/.test-file-$i
done

# Check performance
time ./bin/dotcor status
```

---

## Cleanup After Testing

```bash
# Remove all test files and dotcor
rm -rf ~/.dotcor
rm -f ~/.testrc ~/.test-file-*
rm -rf ~/.config/nvim

# Verify clean
ls -la ~/.dotcor  # Should fail - directory doesn't exist
```

---

## Tips for Manual Testing

1. **Use tab completion**: `dotcor <TAB>` shows available commands
2. **Check --help**: Every command has `--help` for options
3. **Test with empty repo**: Verify empty state handling
4. **Test with corrupted data**: Try missing files, broken symlinks
5. **Test edge cases**: Empty files, large files, special characters
6. **Test with actual tools**: `vim ~/.zshrc` and verify sync works

---

## Integration with Development Workflow

```bash
# Edit code
vim cmd/dotcor/status.go

# Rebuild
make binary

# Test immediately
./bin/dotcor status

# Quick test cycle
make binary && ./bin/dotcor status
```

This rapid edit-build-test cycle is the main advantage of building a binary over `go run`.
