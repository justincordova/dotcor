# dotcor v2.0 Specification

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

Top-level directories in `~/.dotcor/` are packages. Excluded: `.git`, `logs`, `backups`, `.stow-local-ignore`, `.dotcorrc`. Stow-style dot-prefixed package names (e.g. `.ssh`, `.config`, `.gnupg`, `.aws`) are valid packages and are discovered. Each package mirrors the target path from `$HOME`.

`repo/<pkg>/<rel>` always maps to `$HOME/<rel>`. Every component depends on
this: `Link` and `Unlink` compute the target that way, and `DiscoverPackages`
derives link status from it. Repo-relative paths for anything sourced from
`$HOME` are therefore measured from `$HOME`, never from the directory the user
happened to select — adding `~/.config/nvim/init.lua` yields
`nvim/.config/nvim/init.lua`, not a bare `init.lua`.

Two distinct source files can never claim one repo destination; a collision is
refused with a warning rather than letting the second overwrite the first.

**Importing an existing GNU Stow repository.** A dirs-only selection is treated
as a Stow parent (its children become packages whose contents are already
`$HOME`-relative) only when there is evidence it has actually been stowed: a
`$HOME` symlink resolving into that package. The evidence is required
per-package. A purely structural test would misfire on `~/.config` and
`~/.local`, which are dirs-only on almost every machine. An unlinked Stow repo
shows no evidence and is mirrored in place instead — a layout difference, never
data loss.

### Symlinks

Relative symlinks, individual files only (never directory symlinks).

- `~/.zshrc` → `../.dotcor/zsh/.zshrc`
- `~/.config/nvim/init.lua` → `../../../.dotcor/nvim/.config/nvim/init.lua`

The relative target is computed from the link's **physical** parent — both ends
resolved through `EvalSymlinks` — because that is the directory the kernel
walks. Computing it lexically produces a silently dangling link whenever an
ancestor is itself a symlink (`~/.config` → another volume) and the two paths
sit at different depths. The same resolved comparison decides whether a link is
already ours, so `Link`, `Unlink` and discovery cannot disagree about one
symlink.

Swaps are atomic (staging file + `rename`), so a crash leaves either the old
state or the new one, never neither.

### Config (`.dotcorrc`)

```yaml
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

Patterns use `.gitignore` semantics, not bare `filepath.Match`:

- A pattern with no separator matches any path **segment**, so `node_modules`
  excludes everything beneath it and `.env` matches at any depth.
- A pattern with separators matches any trailing run of segments, so `.ssh/*`
  matches `~/.ssh/id_rsa` without being anchored.
- `**` matches zero or more whole segments (`**/*.log`, `secrets/**`).

An absent `ignore_patterns` key falls back to the defaults above; only an
explicit empty list (`ignore_patterns: []`) disables filtering. The patterns
are enforced on every path that writes into the repo — classification, `Link`'s
auto-detect pass, and `Adopt`.

`git_remote` is stored **without credentials**. `.dotcorrc` lives inside the
repository and is picked up by `git add -A`, so a remote entered as
`https://user:token@host` (or the colon-less `https://<token>@host` form) would
otherwise be committed and pushed. The operative URL lives in `.git/config`,
which is never staged.

### Permissions

`~/.dotcor` and everything dotcor creates under it are `0700`. The repository
holds the user's dotfiles — routinely `~/.ssh`, `~/.gnupg` and `~/.aws`
material — and the backup tree mirrors `$HOME` paths, so a world-traversable
directory would leak filenames even where file modes are correct. An existing
`0755` repository is tightened on the next run.

Repo copies carry the source file's mode, applied explicitly after the write
(`open(2)` ignores the mode argument for an existing file).

### Locking

A single lock file at `~/.dotcor/.lock` serialises whole sessions.

- It is published atomically with its content (temp file + `os.Link`, falling
  back to `O_EXCL` on filesystems without hard links), so a concurrent acquirer
  never observes a zero-length lock and mistakes it for a stale one.
- Staleness is decided by **owner liveness**, not age: the timestamp is written
  once at acquisition and never refreshed, while the TUI holds the lock for the
  whole session. A process that exists but belongs to another user counts as
  alive.
- A lock written on another host is judged by age alone, since the local
  process table says nothing about it. The same applies to an unparsable lock.
- Reclaiming a stale lock is done by rename-then-verify, so it can never delete
  a lock another process has just published.

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
│  dotcor v2.0.0                                                     1 package │
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
| `s` | Stow (link) selected package |
| `u` | Unstow (unlink) selected package |
| `A` | Stow all packages |
| `a` | Add file (opens add flow) |
| `d` | Remove / unlink selected file |
| `x` | Delete package |
| `i` | Initialize git in repo |
| `S` | Sync (git commit + push) |
| `p` | Push |
| `P` | Pull |
| `D` | View diff for selected |
| `H` | View history for selected |
| `L` | Toggle log viewer |
| `,` | Settings |
| `/` | Search packages and files |
| `?` | Full keybinding help |
| `q` | Quit |

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
  → stow.Link(repoDir, homeDir, "nvim", ignorePatterns)
      → Walk package tree
      → For each file:
          → Create parent dirs in $HOME
          → Create relative symlink (individual files only)
      → Auto-detect pass: adopt untracked files under the managed
        $HOME root, skipping anything matching ignorePatterns
      → Return result (conflicts are resolved separately, with a
        backup, via LinkWithBackup)
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
