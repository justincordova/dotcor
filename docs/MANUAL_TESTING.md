# Manual Testing Guide for DotCor

## Quick Start

### Build the Binary
```bash
# Using Makefile (recommended)
make binary

# Or directly
go build -o bin/dotcor ./cmd/dotcor

# Then run the TUI
./bin/dotcor
```

### Run with Test Environment
```bash
# Sandbox both config dir and home dir — won't touch your real dotfiles
DOTCOR_DIR=/tmp/dotcor-test DOTCOR_HOME=/tmp/dotcor-home ./bin/dotcor
```

---

## TUI Testing

### Interactive Mode
The TUI launches automatically when running `./bin/dotcor`. Use keyboard navigation:

- `j/k` or arrow keys — navigate up/down
- `Enter` — select/open
- `?` — toggle help
- `q` — quit

### Test with Isolated Environment

```bash
# Create sandbox home (symlinks will target this, not your real ~)
mkdir -p /tmp/dotcor-home

# Create test directory with packages
mkdir -p /tmp/dotcor-test/git /tmp/dotcor-test/nvim /tmp/dotcor-test/starship /tmp/dotcor-test/tmux /tmp/dotcor-test/zsh

# Add test files to each package
echo "git config" > /tmp/dotcor-test/git/config
echo "nvim config" > /tmp/dotcor-test/nvim/init.lua
echo "starship config" > /tmp/dotcor-test/starship/config.toml
echo "tmux config" > /tmp/dotcor-test/tmux/config
echo "zsh config" > /tmp/dotcor-test/zshrc

# Run with sandboxed environment
DOTCOR_DIR=/tmp/dotcor-test DOTCOR_HOME=/tmp/dotcor-home ./bin/dotcor
```

The TUI should show 5 packages in the left sidebar.

---

## Debug Mode

### Enable Debug Output
```bash
# Check startup messages in footer
DOTCOR_DIR=/tmp/dotcor-test ./bin/dotcor
```

The footer shows: `repo: <repoDir>, home: <homeDir>`

### Check Config Resolution
```go
// In internal/config/config.go - GetConfigDir() checks:
// 1. DOTCOR_DIR environment variable (for testing)
// 2. ~/.dotcor/ directory
```

---

## Common Manual Test Scenarios

### 1. Test Package Discovery
```bash
# Ensure test packages exist
ls /tmp/dotcor-test/

# Run and check if packages appear in TUI
DOTCOR_DIR=/tmp/dotcor-test ./bin/dotcor
```

### 2. Test Link Operation
```bash
# Select a package in TUI and press 'l' to link
# Should create symlinks in home directory
```

### 3. Test Unlink Operation
```bash
# Select a linked package and press 'u' to unlink
# Should remove symlinks
```

---

## Full Manual Test Workflow

### Complete E2E Test
```bash
# 1. Create test packages
mkdir -p /tmp/dotcor-test/git /tmp/dotcor-test/nvim /tmp/dotcor-test/zsh

# 2. Add test files
echo "[user]" > /tmp/dotcor-test/git/config
echo "name = Test" >> /tmp/dotcor-test/git/config
echo "set nocompatible" > /tmp/dotcor-test/nvim/init.lua
echo "export ZSH=\"$HOME/.oh-my-zsh\"" > /tmp/dotcor-test/zshrc

# 3. Run TUI
DOTCOR_DIR=/tmp/dotcor-test ./bin/dotcor

# 4. Navigate to package, press 'l' to link
# 5. Verify symlinks created in home
ls -la ~/
```

---

## Debugging Manual Tests

### Check Package Discovery
```bash
# Add debug logging in internal/stow/package.go
# DiscoverPackages() function
```

### Check Config
```go
// Verify config.GetConfigDir() returns correct path
// Priority: DOTCOR_DIR env > ~/.dotcor/
```

### Check Exclusions
```go
// isExcluded() in internal/stow/package.go
// Excludes: .git, .cache, node_modules, .dotcorrc, and any path starting with .
```

---

## Integration with Development Workflow

```bash
# Edit code
vim internal/stow/package.go

# Rebuild
make binary

# Test immediately
DOTCOR_DIR=/tmp/dotcor-test ./bin/dotcor

# Quick test cycle
make binary && DOTCOR_DIR=/tmp/dotcor-test ./bin/dotcor
```

---

## Tips for Manual Testing

1. **Use DOTCOR_DIR** — isolates test environment from real ~/.dotcor/
2. **Check footer** — shows repoDir and homeDir for debugging
3. **Test with empty packages** — verify empty state handling
4. **Test with excluded dirs** — verify .git, .cache are not shown as packages
5. **Test link/unlink** — verify symlinks created and removed correctly