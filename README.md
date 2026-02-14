# DotCor
![Version](https://img.shields.io/github/release/justincordova/dotcor.svg)

A simple, fast dotfile manager built in Go with symlinks and Git automation.

DotCor combines the simplicity of GNU Stow with the convenience of automatic Git commits, making it easy to manage your dotfiles across machines.

---

## Features

- **Symlink-based** - Edit files directly, changes instantly appear in your repository
- **Zero-config** - Automatic path organization with sensible defaults
- **Git automation** - Auto-commits after every operation, manual Git usage optional
- **macOS native** - Built for macOS with full symlink support out of box
- **Simple CLI** - Easy-to-use commands for everyday dotfile management
- **Git history** - Built-in restore and history commands leveraging Git

---

## Quick Start

### Installation

**Homebrew:**
```bash
brew tap justincordova/dotcor
brew install dotcor
```

**From source:**
```bash
git clone https://github.com/justincordova/dotcor.git
cd dotcor
go build -o dotcor cmd/dotcor/main.go
sudo mv dotcor /usr/local/bin/
```

### Basic Usage

```bash
# Initialize DotCor
dotcor init

# Add your dotfiles (moves file to repo, creates symlink)
dotcor add ~/.zshrc
dotcor add ~/.gitconfig
dotcor add ~/.config/nvim/*.lua  # Add with glob pattern

# List managed files
dotcor list

# Check status
dotcor status
dotcor status --prompt  # Quick summary for shell prompts

# Edit your dotfiles as usual (changes are immediately in the repo)
vim ~/.zshrc

# Commit and push all changes
dotcor sync

# View history
dotcor history ~/.zshrc

# Restore from history (now supports multiple files)
dotcor restore ~/.zshrc ~/.bashrc --to=HEAD~5

# View backups
dotcor list-backups ~/.zshrc

# Compare to last backup
dotcor backup-diff ~/.zshrc
```

---

## How It Works

### Storage Model

DotCor uses **symlinks** to keep your actual dotfiles in a Git repository while making them accessible in their original locations:

1. **Add** - Moves your dotfile to `~/.dotcor/files/` and creates a symlink
2. **Edit** - You edit the file normally, changes are instantly in the repository
3. **Sync** - Commits all changes and pushes to remote

```
~/.zshrc (symlink) ──points to──> ~/.dotcor/files/.zshrc (actual file)
                                           ↓
                                       Git repository
```

### Directory Structure

```
~/.dotcor/
├── config.yaml          # Metadata: which files are managed
└── files/               # Git repository with your actual dotfiles
    ├── .git/
    ├── .zshrc           ← actual file
    ├── .bashrc          ← actual file
    └── .config/
        └── nvim/
            └── init.vim ← actual file

# Your home directory (symlinks):
~/.zshrc                 → symlink to ~/.dotcor/files/.zshrc
~/.bashrc                → symlink to ~/.dotcor/files/.bashrc
~/.config/nvim/init.vim  → symlink to ~/.dotcor/files/.config/nvim/init.vim
```

### Workflow Benefits

**Traditional copy-based tools:**
```bash
vim ~/.zshrc              # Edit file
dotcor track ~/.zshrc    # Remember to update repo (often forgotten!)
dotcor push              # Push changes
```

**DotCor with symlinks:**
```bash
vim ~/.zshrc              # Edit file (changes instantly in repo)
dotcor sync              # Commit and push (when ready)
```

---

## Commands

### `dotcor init`

Initialize DotCor repository.

```bash
dotcor init
```

Creates:
- `~/.dotcor/` directory structure
- Git repository in `~/.dotcor/files/`
- Default configuration file

**For new machine setup:**
```bash
# Clone your dotfiles first
git clone https://github.com/you/dotfiles ~/.dotcor/files

# Then initialize with --apply to create all symlinks
dotcor init --apply
```

---

### `dotcor add <file>`

Add a dotfile or use glob patterns to add multiple files to DotCor.

```bash
dotcor add ~/.zshrc
dotcor add ~/.config/nvim/*.lua  # Glob pattern for multiple files
dotcor add ~/.gitconfig ~/.bashrc  # Multiple files at once
```

What it does:
1. Moves file to `~/.dotcor/files/`
2. Creates symlink at original location
3. Records in `config.yaml`
4. Git commits automatically

**Note:** Use glob patterns (`*`, `**`) to add multiple files from directories. Directories cannot be added directly.

---

### `dotcor list`

List all managed dotfiles.

```bash
dotcor list
```

Output:
```
Managed dotfiles (1):

SOURCE PATH          REPO PATH         ADDED AT
~/.zshrc             .zshrc            Jan 04 10:30
```

---

### `dotcor status`

Show status of managed dotfiles and repository.

```bash
dotcor status
```

Shows:
- Symlink health (working, broken, conflicts)
- Git repository status (uncommitted changes, ahead/behind)

Example output:
```
Symlinks:
✓ ~/.zshrc                 → .zshrc
✓ ~/.bashrc                → .bashrc
✗ ~/.vimrc                 → .vimrc (broken: target missing)

Repository:
● 2 uncommitted changes
↑ 1 commit ahead of origin/main

Run 'dotcor sync' to commit and push changes
```

---

### `dotcor sync`

Commit all changes and push to remote.

```bash
dotcor sync
```

What it does:
1. Detects changed files
2. Prompts to remove deleted files from config
3. Commits with message: "Sync dotfiles - {date}"
4. Pushes to remote (if configured)

**Flags:**
- `--no-push` - Commit but don't push to remote

---

### `dotcor remove <file>`

Stop managing a dotfile.

```bash
dotcor remove ~/.zshrc
```

Interactive prompts:
1. Remove symlink? (y/N)
2. Delete from repository? (y/N)
3. Automatically copies file back if keeping it

**Flags:**
- `--keep-file` - Keep file at source location after removing symlink

---

### `dotcor restore <file>`

Restore a dotfile from Git history.

```bash
# Restore from latest commit (undo local edits)
dotcor restore ~/.zshrc

# Restore from specific commit
dotcor restore ~/.zshrc --to=HEAD~5
dotcor restore ~/.zshrc --to=abc123f
```

---

### `dotcor history <file>`

Show Git history for a dotfile.

```bash
dotcor history ~/.zshrc
```

Output:
```
History for ~/.zshrc (.zshrc):

abc123f - 2025-01-04 15:30 - Update zsh aliases
def456a - 2025-01-03 09:15 - Add new PATH entries
789beef - 2025-01-02 14:22 - Sync dotfiles

Use 'dotcor restore ~/.zshrc --to=<commit>' to restore
```

**Flags:**
- `-n <number>` - Number of commits to show (default: 10)

---

### `dotcor backup-diff <file>`

Show changes between current files and latest backups.

```bash
# Show diff for specific file
dotcor backup-diff ~/.zshrc

# Show diff summary with stats
dotcor backup-diff ~/.zshrc --stat
```

Useful for reviewing what would be restored from backup before actually restoring.

---

### `dotcor list-backups [file]`

List all available backups for managed dotfiles.

```bash
# List all backups
dotcor list-backups

# List backups for specific file
dotcor list-backups ~/.zshrc
```

Shows:
- Backup timestamp and age
- File size
- Full backup path

---

## Use Cases

### New Machine Setup

**On your main machine:**
```bash
dotcor init
dotcor add ~/.zshrc ~/.gitconfig ~/.config/nvim
cd ~/.dotcor/files
git remote add origin git@github.com:you/dotfiles.git
dotcor sync
```

**On a new machine:**
```bash
# Install DotCor, then:
git clone git@github.com:you/dotfiles.git ~/.dotcor/files
dotcor init --apply
# All your dotfiles are now symlinked and ready!
```

---

### Daily Workflow

Edit your dotfiles as usual:

```bash
vim ~/.zshrc              # Add new aliases
vim ~/.gitconfig          # Update Git settings
vim ~/.config/nvim/init.vim
```

When you want to save your changes:

```bash
dotcor status            # See what changed
dotcor sync              # Commit and push
```

On another machine:

```bash
cd ~/.dotcor/files
git pull                  # Changes are immediately active via symlinks!
```

---

### Undo Mistakes

Made a bad change? Easy to undo:

```bash
# View history
dotcor history ~/.zshrc

# Restore previous version
dotcor restore ~/.zshrc --to=HEAD~1

# Or use Git directly
cd ~/.dotcor/files
git log .zshrc
git checkout HEAD~5 -- .zshrc
```

---

## Configuration

Configuration is stored in `~/.dotcor/config.yaml`:

```yaml
version: "1.0"
repo_path: ~/.dotcor/files
git_enabled: true
git_remote: ""
ignore_patterns:
  - "*.log"
  - "*.swp"
  - ".env"
  - ".env.*"
  - "*.key"
  - "*.pem"

managed_files:
  - source_path: ~/.zshrc
    repo_path: .zshrc
    added_at: 2025-01-04T10:30:00Z
    has_uncommitted: false

  - source_path: ~/Library/Preferences/foo.plist
    repo_path: Library/Preferences/foo.plist
    added_at: 2025-01-04T10:31:00Z
    has_uncommitted: false
  ```

---

## Advanced Usage

### Manual Git Operations

DotCor auto-commits for convenience, but you can always use Git directly:

```bash
cd ~/.dotcor/files

# View detailed history
git log --oneline --graph

# Create branches
git checkout -b experimental

# Cherry-pick changes
git cherry-pick abc123f

# Advanced operations
git rebase -i HEAD~10
```

### Setting Up Remote

```bash
cd ~/.dotcor/files
git remote add origin git@github.com:you/dotfiles.git
git branch -M main
git push -u origin main
```

Now `dotcor sync` will automatically push to your remote.

---

## macOS Support

DotCor is built exclusively for macOS, taking advantage of native symlink support.

### Requirements

- macOS 10.14 (Mojave) or later
- Git (for version control)

---

## Why DotCor?

### vs. Manual Git Repository

- **DotCor:** Symlinks make edits instant, auto-commits, path organization
- **Manual:** You handle everything yourself, easy to forget to commit

### vs. GNU Stow

- **DotCor:** Git auto-commits, cross-platform, convenience commands
- **Stow:** Minimal, Unix-only, requires manual Git management

### vs. Chezmoi

- **DotCor:** Simple, minimal learning curve, standard Git workflow
- **Chezmoi:** Feature-rich with templates, secrets, more complexity

### vs. yadm

- **DotCor:** Explicit file management, organized repo structure
- **yadm:** Entire home directory as Git repo, less organized

**DotCor is for users who want:**
- GNU Stow's simplicity with Git automation built-in
- To edit dotfiles directly without manual sync commands
- A tool that stays out of your way
- Simple cross-platform dotfile management

---

## Development

### Project Structure

```
dotcor/
├── cmd/dotcor/          # CLI commands (Cobra)
│   ├── main.go
│   ├── init.go
│   ├── add.go
│   ├── remove.go
│   ├── list.go
│   ├── status.go
│   ├── sync.go
│   ├── restore.go
│   ├── history.go
│   ├── diff.go
│   ├── adopt.go
│   ├── doctor.go
│   ├── rebuild.go
│   ├── rebuild-links.go
│   ├── clone.go
│   ├── cleanup.go
│   └── test_helpers.go
├── internal/
│   ├── config/          # Configuration management
│   │   ├── config.go
│   │   ├── paths.go
│   │   └── migrate.go
│   ├── core/            # Business logic
│   │   ├── backup.go
│   │   ├── transaction.go
│   │   ├── hooks.go
│   │   ├── templates.go
│   │   ├── validator.go
│   │   ├── lock.go
│   │   └── ignore.go
│   ├── fs/              # File operations
│   │   ├── fs.go
│   │   └── symlink.go
│   ├── git/             # Git integration
│   │   └── git.go
│   └── logger/          # Structured logging
│       └── logger.go
├── tests/
│   └── integration_test.go
├── docs/
│   ├── LOGGING.md
│   ├── TESTING.md
│   ├── RELEASING.md
│   └── PLAN.md
├── README.md
└── CLAUDE.md
```

### Building

```bash
go build -o dotcor cmd/dotcor/main.go
```

### Running

```bash
go run cmd/dotcor/main.go [command]
```

### Releases

Releases are automated via GoReleaser and GitHub Actions. See `RELEASING.md` for:
- Creating new releases
- Testing releases locally
- Troubleshooting release issues

### Contributing

See `PLAN.md` for implementation details and development roadmap.

---

## License

MIT License - see LICENSE file for details

---

## Author

Built by Justin Cordova

---

## Support

- **Issues:** https://github.com/justincordova/dotcor/issues
- **Discussions:** https://github.com/justincordova/dotcor/discussions
