# DotCor v2.0 Specification

A symlink-based dotfile manager with a Bubble Tea TUI and GNU Stow-style package layout, backed by automatic Git commits.

---

## Architecture

### Technology Stack

- **Language:** Go 1.26
- **TUI Framework:** Bubble Tea (Charm)
- **Styling:** Lip Gloss (Rosé Pine theme)
- **Components:** Bubbles, BubbleZone, Harmonica
- **Config:** YAML (`gopkg.in/yaml.v3`) — no Viper
- **Version Control:** Git (via `os/exec`)

### Design Principles

- **Stow-style packages:** Each dotfile group is a directory in `~/.dotcor/` that mirrors `$HOME`
- **Filesystem-only state:** No managed_files list — packages and symlinks are discovered from disk
- **Individual file symlinks:** Never symlink directories, always individual files with relative paths
- **Safety-first:** Backups before destructive operations, transaction rollback, file-based locking
- **Git automation:** Auto-commit after every stow/unstow/sync operation
- **macOS native:** Built for macOS with full symlink support

---

## Repository Layout

```
~/.dotcor/
├── .git/
├── .dotcorrc                  # minimal config
├── logs/
│   └── dotcor.log             # rotated, 5MB max
├── backups/
│   └── 2026-01-15_10-30-00/
│       └── .zshrc
├── zsh/                       # package: mirrors $HOME
│   └── .zshrc
├── nvim/
│   └── .config/nvim/
│       ├── init.lua
│       └── lua/
├── git/
│   └── .gitconfig
├── tmux/
│   ├── .tmux.conf
│   └── .tmux.theme.conf
└── starship/
    └── .config/starship.toml
```

### Packages

Top-level directories in `~/.dotcor/` are packages. Excluded: `.git`, `logs`, `backups`, `.stow-local-ignore`, dotfiles like `.dotcorrc`. Each package mirrors the target path from `$HOME`.

### Symlinks

Relative symlinks, individual files only (never directory symlinks).

- `~/.zshrc` → `../.dotcor/zsh/.zshrc`
- `~/.config/nvim/init.lua` → `../../../.dotcor/nvim/.config/nvim/init.lua`

### Config (`.dotcorrc`)

```yaml
git_enabled: true
git_remote: ""
ignore_patterns:
  - "*.key"
  - "*.pem"
  - ".env"
  - ".env.*"
  - "id_rsa"
  - "id_ed25519"
  - "*_history"
  - "*.log"
  - "*.swp"
  - "*.swo"
  - ".DS_Store"
```

No `managed_files` list. No `version` field. State is discovered from the filesystem.

### Discovery Algorithm

1. Scan `~/.dotcor/` for package directories (exclude `.git`, `backups`, `logs`, `.stow-local-ignore`)
2. For each package, walk its tree to find all files
3. For each file, compute the expected symlink target in `$HOME`
4. Check if symlink exists and points to the correct target
5. Report status: linked / unlinked / conflict (file exists but isn't a symlink)

---

## TUI Interface

### Libraries

| Library | Purpose |
|---------|---------|
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/lipgloss` | Styling and layout |
| `github.com/charmbracelet/bubbles` | List, viewport, textinput, help, spinner, progress, table |
| `github.com/erikgeiser/bubblezone` | Mouse click and scroll zones |
| `github.com/charmbracelet/harmonica` | Smooth animations (spinners, progress) |
| `github.com/aymanbagabas/go-osc52/v2` | Clipboard support (copy paths, diff content) |
| `github.com/charmbracelet/log` | Charm-native structured logger (replaces custom slog setup) |

### Color Scheme: Rosé Pine

| Role | Color | Hex |
|------|-------|-----|
| Accent (headers, borders) | Rose | `#ebbcba` |
| Success (linked, clean) | Pine | `#31748f` |
| Warning (uncommitted) | Gold | `#f6c177` |
| Error (broken, conflicts) | Love | `#eb6f92` |
| Highlight (selected) | Iris | `#c4a7e7` |
| Dim (inactive) | Muted | `#6e6a86` |
| Text | Foam | `#9ccfd8` |
| Subtle (backgrounds) | Overlay | `#1f1d2e` |

### Main Dashboard

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  DotCor v2.0.0                                                     1 package │
├──────────────────────────────────────┬──────────────────────────────────────┤
│  Packages (5)                   [1]  │  zsh                                    │
│ ┌──────────────────────────────────┐ │ ┌────────────────────────────────────┐ │
│ │▶ zsh                        ✓   │ │ │ .zshrc  → ~/.zshrc          linked │ │
│ │  nvim                       ✓   │ │ │                                    │ │
│ │  git                        ⚠   │ │ │                                    │ │
│ │  tmux                       ✓   │ │ │                                    │ │
│ │  starship                   ✗   │ │ │                                    │ │
│ └──────────────────────────────────┘ │ └────────────────────────────────────┘ │
├──────────────────────────────────────┴──────────────────────────────────────┤
│  ● 1 uncommitted change  ↑2 ahead  git/main                                │
├─────────────────────────────────────────────────────────────────────────────┤
│  ?_help  a_dd  d_remove  s_tow  u_nstow  S_sync  /_search  q_quit         │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Keybindings

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate |
| `Enter` | Expand / select |
| `a` | Add file (opens add flow) |
| `d` | Remove / unlink selected file |
| `s` | Stow (link) selected package |
| `u` | Unstow (unlink) selected package |
| `S` | Sync (git commit + push) |
| `D` | View diff for selected |
| `H` | View history for selected |
| `L` | Toggle log viewer |
| `/` | Search packages and files |
| `?` | Full keybinding help |
| `q` | Quit |
| `p` | Push |
| `P` | Pull |
| `r` | Restore from git |

### Screens

- **Add Flow** (`a`): Path input, auto-detected package, preview tree, secret scan
- **Diff View** (`D`): Scrollable diff with syntax highlighting, commit option
- **History View** (`H`): Git log for selected file, restore from any commit
- **Logs View** (`L`): File log viewer with level filtering (debug/info/warn/error)
- **Help** (`?`): Full keybinding reference
- **Settings**: Git remote, ignore patterns, backup management

### Conflict Resolution

When stowing and a file exists at target (not a symlink):

```
┌──────────────────────────────────────────────────────────────────┐
│  Conflict: ~/.config/nvim/init.lua                               │
│                                                                  │
│  File exists and is not a symlink.                               │
│                                                                  │
│  [b]ackup + replace   [s]kip   [a]dopt   [Esc]cancel           │
└──────────────────────────────────────────────────────────────────┘
```

---

## State Management

```go
type AppState struct {
    Config       *config.Config
    Packages     []stow.Package
    SelectedPkg  int
    SelectedFile int
    ActiveView   View
    GitStatus    git.StatusInfo
    Width, Height int
    StatusMsg    string
    Err          error
}

type Package struct {
    Name   string
    Path   string
    Files  []FileEntry
    Status PackageStatus  // linked, partial, unlinked
}

type FileEntry struct {
    RelPath    string  // .zshrc (relative within package)
    TargetPath string  // ~/.zshrc (computed symlink target)
    IsLinked   bool
    Exists     bool
    IsSymlink  bool
}
```

### Background Commands (Bubble Tea Cmd pattern)

| Trigger | Cmd | Returns |
|---------|-----|---------|
| Startup | `discoverPackages()` | `[]Package` |
| Startup | `fetchGitStatus()` | `git.StatusInfo` |
| After stow/unstow | `discoverPackages()` | refreshed `[]Package` |
| After sync | `fetchGitStatus()` | refreshed `git.StatusInfo` |
| Select file | `getFileDiff(path)` | diff string |
| Open history | `getFileHistory(path)` | `[]CommitInfo` |

---

## Data Flow

### Stow a Package

```
User presses 's' on "nvim"
  → tui.Update(KeyMsg{s})
  → stow.Link(cfg, "nvim")
      → Walk package tree
      → For each file:
          → Backup original if exists
          → Create parent dirs in $HOME
          → Create relative symlink (individual files only)
      → Return result
  → git.AutoCommit("stow nvim")
  → discoverPackages() (refresh)
  → fetchGitStatus() (refresh)
  → StatusMsg: "Stowed nvim (5 files linked)"
```

### Stow Folding Rules

- Always symlink individual files, never directories
- Create parent directories in `$HOME` as needed
- After unstow, remove empty directories in `$HOME`

---

## First-Run Experience

```
$ dotcor
Not initialized. Create ~/.dotcor? (y/N): y
✓ Created ~/.dotcor/
✓ Initialized git repository
```

Simple `y/N` prompt before TUI launches, same pattern as lazygit.

## V1 Migration

On startup, detect old layout (`~/.dotcor/files/` exists). Offer one-time migration:

```
$ dotcor
Found v1.x layout in ~/.dotcor/files/. Migrate to v2.0? (y/N): y
  shell/zshrc     → zsh/.zshrc
  git/gitconfig   → git/.gitconfig
✓ Migration complete
```

## CLI Flags

Minimal — processed before TUI launch. No Cobra.

| Flag | Action |
|------|--------|
| `--version` | Print version, exit |
| `--debug` | Set log level to debug |
| `--log-level` | Set log level (debug/info/warn/error) |

---

## Logging

- Default: `~/.dotcor/logs/dotcor.log`
- Auto-rotated, 5MB max
- TUI toggleable log viewer (`L`) with level filtering
- No stderr when TUI running

---

## Project Structure

```
dotcor/
├── cmd/dotcor/main.go          # thin entry point: flags, init prompt, launch TUI
├── internal/
│   ├── config/
│   │   ├── config.go           # simplified .dotcorrc load/save
│   │   └── paths.go            # path utilities
│   ├── core/
│   │   ├── backup.go           # backup/restore
│   │   ├── lock.go             # file locking
│   │   ├── transaction.go      # transactions
│   │   ├── ignore.go           # ignore patterns
│   │   ├── hooks.go            # hook system
│   │   └── templates.go        # template substitution
│   ├── fs/
│   │   ├── fs.go               # file operations
│   │   └── symlink.go          # symlink operations
│   ├── git/
│   │   └── git.go              # git wrapper
│   ├── logger/
│   │   └── logger.go           # file logging + rotation
│   └── stow/
│       ├── package.go          # package discovery, validation
│       ├── link.go             # symlink creation
│       ├── unlink.go           # symlink removal + empty dir cleanup
│       └── migrate.go          # v1 → v2 migration
├── tui/
│   ├── app.go                  # root bubble tea model
│   ├── dashboard.go            # main dashboard view
│   ├── add_view.go             # add file flow
│   ├── diff_view.go            # diff viewer
│   ├── history_view.go         # history browser
│   ├── help_view.go            # keybinding help
│   ├── logs_view.go            # log viewer
│   ├── settings_view.go        # settings editor
│   ├── styles.go               # Rosé Pine lip gloss definitions
│   └── keys.go                 # keybinding definitions
├── go.mod
├── go.sum
├── README.md
├── CLAUDE.md
└── docs/
    ├── SPEC.md                 # this file
    ├── TESTING.md
    ├── LOGGING.md
    └── RELEASING.md
```

---

## What Stays vs What Goes

### Keep (refactor as needed)

| Package | Changes |
|---------|---------|
| `internal/config` | Simplify: no `managed_files`, no Viper. Just `.dotcorrc` parsing. |
| `internal/core/*` | Keep as-is (backup, lock, transaction, ignore, hooks, templates) |
| `internal/fs/*` | Keep as-is |
| `internal/git/*` | Keep as-is |
| `internal/logger/*` | Refactor: remove Cobra dep, default to file logging |

### Remove entirely

- `cmd/dotcor/*.go` (all CLI commands) — replaced by `tui/`
- `github.com/spf13/cobra`
- `github.com/spf13/viper`
- `github.com/spf13/pflag`
- `github.com/joho/godotenv`

### New packages

| Package | Purpose |
|---------|---------|
| `internal/stow/` | Package discovery, link/unlink, v1 migration |
| `tui/` | All Bubble Tea models and views |

---

## Error Handling

Errors appear as status messages in the TUI footer bar. Critical errors show centered modals with actionable steps. Git commit failures don't fail operations — they surface as warnings.

---

## Testing Strategy

| Layer | Approach |
|-------|----------|
| `internal/stow/` | Unit tests with temp directories |
| `tui/` | Bubble Tea model tests: `Update()` with messages, assert state |
| Integration | Full flow: init → stow → edit → sync → unstow |

---

## Release

After implementation and manual testing, tag as `v2.0.0`:

```bash
git tag -a v2.0.0 -m "v2.0.0: TUI rewrite with Stow-style layout"
git push origin v2.0.0
```
