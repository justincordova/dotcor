# DotCor

![Version](https://img.shields.io/github/release/justincordova/dotcor.svg)

A symlink-based dotfile manager with automatic Git commits.

Edit your dotfiles directly—changes instantly appear in your repository. DotCor handles version control automatically, so you can focus on your configurations.

---

## Installation

**Homebrew:**
```bash
brew tap justincordova/dotcor
brew install dotcor
```

---

## Quick Start

```bash
# Initialize
dotcor init

# Add dotfiles (moves to repo, creates symlink)
dotcor add ~/.zshrc
dotcor add ~/.gitconfig
dotcor add ~/.config/nvim/*.lua

# List managed files
dotcor list

# Check status
dotcor status

# Commit and push changes
dotcor sync
```

---

## How It Works

```
~/.zshrc (symlink) ──> ~/.dotcor/files/.zshrc (actual file)
                                    │
                              Git repository
```

When you `add` a dotfile:
1. Moves file to `~/.dotcor/files/`
2. Creates symlink at original location
3. Commits to Git automatically

Edit dotfiles normally—changes are instantly in your repo. Run `sync` when ready to push.

---

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize DotCor repository |
| `add <file>` | Add dotfiles (supports globs) |
| `remove <file>` | Stop managing dotfiles |
| `list` | List managed dotfiles |
| `status` | Check symlink health & Git status |
| `sync` | Commit and push changes |
| `history <file>` | Show Git history |
| `restore <file>` | Restore from Git history or backup |
| `diff` | Show uncommitted changes |
| `backup-diff <file>` | Compare to last backup |
| `list-backups` | List available backups |
| `doctor` | Diagnose and fix issues |
| `clone <url>` | Clone dotfiles to new machine |
| `adopt` | Adopt existing symlinks |

---

## Common Workflows

### New Machine Setup

```bash
# Clone your dotfiles
dotcor clone git@github.com:you/dotfiles.git

# Or manually:
git clone git@github.com:you/dotfiles.git ~/.dotcor/files
dotcor init --apply
```

### Daily Editing

```bash
vim ~/.zshrc              # Edit normally
dotcor status            # See what changed
dotcor sync              # Commit and push
```

### Undo Changes

```bash
dotcor history ~/.zshrc
dotcor restore ~/.zshrc --to=HEAD~3
```

---

## Configuration

Config stored in `~/.dotcor/config.yaml`:

```yaml
version: "1.0"
repo_path: ~/.dotcor/files
git_enabled: true
ignore_patterns:
  - "*.log"
  - "*.swp"
  - ".env"
```

---

## Directory Structure

```
~/.dotcor/
├── config.yaml     # Metadata
├── backups/        # Automatic backups
└── files/          # Git repository
    ├── .git/
    ├── .zshrc
    ├── .bashrc
    └── .config/nvim/init.vim
```

---

## Requirements

- macOS 10.14+ (Mojave or later)
- Git

---

## Why DotCor?

| Tool | Approach |
|------|----------|
| **DotCor** | Symlinks + auto Git commits |
| GNU Stow | Symlinks only (manual Git) |
| Chezmoi | Templates + copy-based |
| yadm | Entire home as Git repo |

DotCor is for users who want to edit dotfiles directly without sync commands, with Git automation built-in.

---

## License

MIT

---

## Support

- **Issues:** https://github.com/justincordova/dotcor/issues
- **Discussions:** https://github.com/justincordova/dotcor/discussions
