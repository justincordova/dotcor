# DotCor v2.0 — Known Issues

Pre-release review findings. Verified against code on 2026-04-22, organized by theme.
Severity legend: **C** = Critical · **H** = High · **M** = Medium · **L** = Low.
Flags: `[sec]` security · `[data]` data-loss risk · `[ux]` user-facing only.

---

## Cross-cutting root causes

Two architectural gaps account for the majority of correctness issues below.
Fixing these first removes whole classes of bugs:

- **Safety primitives exist but are unused.** `internal/core/transaction.go` (rollback)
  and `internal/core/lock.go` (cross-process lock) are fully implemented and tested but
  never called from `internal/stow/` or `tui/`. Most "silent partial failure" bugs
  (#1, #3, #11, #12, #21, #27, #45) are direct consequences. See themes A and B.
- **Refresh dispatch is inconsistent.** Only `stowResultMsg` and `classifyResultMsg`
  trigger `refreshAll()`. `statusMsg`, `errMsg`, `settingsMsg`, and `autoCommitCmd`
  do not — producing the entire "stale dashboard" cluster (#5, #6, #13, #16).

The two highest-leverage commits in this list are: (a) wiring `core.Transaction`
into the three execute paths in `classify.go`, and (b) routing every terminal
message through a single refresh path.

---

## Severity table

Legend: ✅ = fixed and committed.

| #  | Title                                                | Sev | Flags        | Effort |
|----|------------------------------------------------------|-----|--------------|--------|
| 3  | ✅ `deletePackage` silent backup failure             | C   | data         | S      |
| 27 | ✅ Implicit adoption of foreign symlinks during stow | C   | data         | M      |
| 39 | ✅ Git commands have no timeout — TUI can wedge      | C   | ux           | S      |
| 40 | ✅ Ignore patterns defined but never enforced        | C   | sec, data    | M      |
| 1  | ✅ `removeFileFromPackage` data-loss window          | H   | data         | S      |
| 10 | ✅ `core.Transaction` never used in stow operations  | H   | data         | L      |
| 11 | ✅ `executeAdopt` silent `nil` on partial failure    | H   | data         | S      |
| 12 | ✅ `copyDir` flattens symlinks in package backups    | H   | data         | S      |
| 21 | ✅ Migration has no rollback                         | H   | data         | M      |
| 6  | ✅ History "D" key does nothing                      | H   | ux           | S      |
| 14 | ✅ Adopt `o` keybinding undiscoverable               | H   | ux           | S      |
| 2  | ✅ `isExcluded` blanket dot-prefix hides packages    | M   | ux           | S      |
| 4  | ✅ Settings remote URL not applied to git            | M   |              | S      |
| 5  | ✅ `statusMsg`/`errMsg` don't refresh dashboard      | M   | ux           | S      |
| 7  | ✅ `restoreFromCommit` TOCTOU on `filePath`          | M   | data (rare)  | S      |
| 8  | ✅ `Unlink` missing `EvalSymlinks` on macOS          | M   |              | S      |
| 9  | ✅ `core.Lock` never used in TUI path                | M   | data (race)  | M      |
| 13 | ✅ Diff view commit feedback invisible               | M   | ux           | S      |
| 15 | ✅ Negative panel width on narrow terminals          | M   | ux           | S      |
| 16 | ✅ Auto-commit races with `refreshAll`               | M   | ux           | S      |
| 17 | ✅ Detached HEAD shows "no git repository"           | M   | ux           | S      |
| 22 | ✅ `LinkWithBackup` count conflates linked+resolved  | M   | ux           | S      |
| 23 | ✅ Add view zero feedback when all already managed   | M   | ux           | S      |
| 36 | ✅ Confirm modal has no height bound                 | M   | ux           | S      |
| 38 | ✅ `buildHomeSymlinkIndex` silent fallback           | M   |              | S      |
| 41 | ✅ Push/clone progress freezes on auth prompts       | M   | ux           | S      |
| 42 | ✅ `SetRemote` accepts arbitrary URL schemes         | M   | sec          | S      |
| 43 | ✅ `DiscoverPackages` aborts on one unreadable file  | M   | ux           | S      |
| 44 | ✅ `cfg.Logger` may be nil after `LoadConfig`        | M   |              | S      |
| 45 | ✅ `LinkWithBackup` ignores restore-write error      | M   | data         | S      |
| 46 | ✅ `expandBrowserPath` allows `..` traversal         | M   | sec          | S      |
| 47 | ✅ Adopt of large files via `os.ReadFile` (no cap)   | M   | data         | S      |
| 18 | ✅ `stow.Link` re-discovers packages mid-operation  | L   | perf         | S      |
| 19 | `buildHomeSymlinkIndex` walks `$HOME` per classify   | L   | perf         | M      |
| 20 | ✅ `repoSizeMB` recalculates on every refresh        | L   | perf         | S      |
| 24 | ✅ `git diff HEAD` raw error when no commits exist   | L   | ux           | S      |
| 28 | ✅ `expandBrowserPath` mishandles bare `~`           | L   | ux           | S      |
| 31 | ✅ Browser jump cursor not reset                     | L   | ux           | S      |
| 32 | ✅ `fuzzyMatch` breaks for non-ASCII queries         | L   |              | S      |
| 34 | ✅ `settingsIgnoreIdx` not clamped after deletion    | L   | ux           | S      |
| 35 | ✅ `renderPackageCard` doesn't truncate long names   | L   | ux           | S      |
| 37 | ✅ Help dialog height not bounded                    | L   | ux           | S      |
| 48 | ✅ `cleanDirChain` walk-up missing `EvalSymlinks`    | L   |              | S      |
| —  | ✅ Polish: #29 empty parent dirs, #30 future ts      | L   |              | —      |
| 25 | ✅ Logger file handle never closed                   | L   |              | S      |
| 26 | ✅ `countFilesRecursive` skips symlinks              | L   |              | S      |
| 33 | ✅ `loadLogs` reads entire log file                  | L   |              | S      |

---

## Theme A — Safety infrastructure unused

These are the upstream causes of most data-integrity bugs. Address before downstream patches.

### 9. `core.Lock` never used in TUI path · M

**File:** `internal/core/lock.go` — zero non-test callers.

`AcquireLock`/`ReleaseLock` are implemented and tested but no production code
invokes them. Two `dotcor` instances against the same `~/.dotcor` can race on
`os.Symlink`/`os.Rename` of `.dotcor-tmp`, and on the `pre-delete-<ts>` backup
path (which is timestamped to second-resolution and so collides under concurrency).
SPEC.md and CLAUDE.md both list file-based locking as a design principle.

**Fix:** Acquire a session-wide lock in `cmd/dotcor/main.go` before `p.Run()`,
release on exit. On `IsStale`, auto-clean. Surface acquisition failure as a
startup error showing PID/host from the lock file.

### 10. `core.Transaction` never used in stow operations · H · [data]

**File:** `internal/core/transaction.go` — zero callers in `internal/stow/`.

`Link`, `Unlink`, `Adopt`, `executeAdd`, `executeAdopt`, and `executeTrack` all do
ad-hoc `os.WriteFile`/`os.Symlink`/`os.Rename` with best-effort `_ = os.Remove(...)`
compensations (often with discarded errors). This is the upstream cause of #1,
#3, #11, #12, #21, #45.

**Fix:** Wire `core.Transaction` into the three execute paths in `classify.go`
and into `LinkWithBackup`. The current op set lacks a
`ReplaceFileWithSymlinkOp` — add it and capture the original bytes so adopt
rollback is possible. Rollback for `executeAdopt` must also stash the
pre-adopt `$HOME` symlink target (currently not captured).

---

## Theme B — Destructive operations without safety nets

### 3. `deletePackage` silent backup failure · C · [data] · ✅ fixed in d285f5a

**File:** `tui/app.go:1132-1144`

```go
_ = os.MkdirAll(filepath.Dir(backupPath), 0755)
_ = copyDir(pkgDir, backupPath)
if err := os.RemoveAll(pkgDir); err != nil { ... }
```

Both backup steps discard errors with `_ =`. `os.RemoveAll` runs regardless. The
success message at line 1144 unconditionally claims `"backup saved"`. A partial
backup is worse than no backup: the user trusts a recovery path that silently
skips files.

**Fix:** Check both errors. On backup failure, `os.RemoveAll(backupPath)` to
clean partial state, then return the error. Include the concrete backup path in
the success message so the user can verify. Long term, use `core.Transaction`.

### 1. `removeFileFromPackage` data-loss window · H · [data]

**File:** `tui/app.go:1185-1201`

Sequence: read repo bytes → `os.Remove($HOME symlink)` → `os.WriteFile($HOME, data, perm)` → `os.Remove(repo file)`. If `os.WriteFile` fails after the symlink is gone, the user sees a missing `$HOME` file. The repo copy is preserved (function returns before line 1203), so data is recoverable but the user may not know that. `os.WriteFile` is also not atomic — a partial write on disk-full leaves a truncated file, which is worse than the reviewer suggested.

**Fix:** Write to `target+".tmp"`, then `os.Rename` (atomic on same FS). On rename failure, recreate the symlink. Prefer `core.Transaction` for project-wide consistency.

### 11. `executeAdopt` silently returns `nil` on partial failure · H · [data]

**File:** `internal/stow/classify.go:691-711`

After the `$HOME` symlink is repointed to the repo, three failure paths
(`filepath.Rel`, `os.Symlink` to tmp, `os.Rename`) return `nil`. The original
source file is then orphaned: it sits on disk as a regular file while `$HOME`
points elsewhere. A user editing the orphan loses changes silently.

**Fix:** Emit a `ClassificationFailure` with reason "home repointed but source
not relinked: <path>". Do **not** roll back the home repoint — that's the
intended end state. The bare `_ = os.WriteFile(srcPath, srcData, srcPerm)` at
line 706 must also check its error, else `srcPath` may end up missing entirely.

### 12. `copyDir` flattens symlinks in package backups · H · [data]

**File:** `tui/app.go:1147-1166`

`filepath.Walk` provides `Lstat`-based info (so symlinks are detectable), but
`copyDir` only branches on `info.IsDir()`. Symlinks fall through to
`os.ReadFile` (follows link) + `os.WriteFile` (regular file). Any package
containing a symlink (e.g. `nvim/after/ftplugin -> ../ftplugin`) is backed up
as a flattened tree that cannot be restored. Broken symlinks abort the entire
backup walk.

**Fix:** Branch on `info.Mode()&os.ModeSymlink != 0` first; use
`os.Readlink` + `os.Symlink`. Continue on broken-symlink errors.

### 21. Migration has no rollback · H · [data]

**File:** `internal/stow/migrate.go:77-100`

`ExecuteMigration` does `os.Rename` in a loop, then `cleanEmptyParents`
progressively destroys the source tree. If step N fails, steps 1..N-1 are
already done with no recovery path; the user is stranded between v1 and v2.

**Fix:** Build a reverse plan alongside the forward plan; on failure, walk the
reverse plan. Or: use `core.Transaction` (its `MoveFileOp` has the right shape).

### 27. Implicit adoption of foreign symlinks during stow · C · [data] · ✅ fixed in 2e79886

**File:** `internal/stow/link.go:74-153`

`linkAutoDetectedFile` runs unconditionally during `Link`. Foreign symlinks
(those pointing outside the repo) are silently copied into the repo, the
original is removed, and a tmp-renamed symlink is put in its place. The user
pressed `s` expecting only new symlinks, but mutations happen to files they
never agreed to adopt. This contradicts the explicit `o`-key adopt confirmation
flow in `tui/app.go:524-550`.

**Fix:** Skip foreign symlinks during `Link`. Surface them in the stow result
(`Foreign` count) and require the explicit Add/Adopt flow. Or gate behind a
config flag with safe default.

### 45. `LinkWithBackup` ignores restore-write error · M · [data]

**File:** `internal/stow/link.go:248-266`

Conflict-resolution path: write backup → remove target → create symlink. If the
final `os.Symlink` fails, the code attempts `os.WriteFile` to restore, but the
error is discarded with `_ =`. If the parent dir was already removed by a
sibling cleanup, the file vanishes from `$HOME` with only the backup directory
as recovery, and the user is told "N conflicts remaining" with no hint that
restoration failed.

**Fix:** Check `os.WriteFile` error; emit a distinct failure entry: "restore
failed, backup preserved at <path>".

### 47. Adopt of large files via `os.ReadFile` (no cap) · M · [data]

**Files:** `internal/stow/link.go:105-123, 225-246`; `internal/stow/adopt.go:57-88`

`safeReadFile` caps reads at 100 MiB in `classify.go:104` but the `link.go` and
`adopt.go` adopt paths use bare `os.ReadFile` with no cap. A user adopting a
multi-GB media file or font cache OOMs the TUI. Additionally, the read+write
pattern silently drops xattrs, ACLs, and `setcap` capability bits — adopting a
binary with `cap_net_bind_service` silently disables it.

**Fix:** Size-gate via `os.Lstat` before the read. On same-filesystem moves,
prefer `os.Rename` (preserves attrs). Document attribute behavior in SPEC.

---

## Theme C — Security & data exfiltration

### 40. Ignore patterns defined but never enforced · C · [sec, data] · ✅ fixed in 3053a28

**Files:** `internal/core/ignore.go`, `internal/core/validator.go:240`,
`internal/stow/classify.go` (consumer absent)

`ShouldIgnore`, `FilterByPatterns`, `DetectSecrets`, and `IsSecretFile` are all
implemented and tested. `cfg.IgnorePatterns` defaults to `*.key`, `*.pem`,
`.env`, `id_rsa*`, `id_ed25519*`. The Settings UI lets users manage the list.
**But neither `ClassifyFiles` nor `walkAndClassify` nor `DiscoverPackages` ever
calls any of these functions.** A user adding `~/.ssh/` will get private keys
copied into the repo and pushed to the remote on the next sync.

**Fix:** Call `ShouldIgnore(relPath, cfg.IgnorePatterns)` in
`classify.walkAndClassify` and skip matched files. Surface a "filtered N files
matching ignore patterns" line in the preview. Optionally run `DetectSecrets`
and require explicit confirmation for matches.

### 42. `SetRemote` accepts arbitrary URL schemes · M · [sec]

**File:** `internal/git/git.go:287-307`; consumers `tui/app.go:737`,
`tui/settings_view.go:239` (planned per #4)

`remoteURL` is forwarded directly as an argv element to `git remote set-url`.
Git itself accepts the `ext::sh -c '...'` transport, which executes arbitrary
commands on next fetch/push. The remote URL persists in `.dotcorrc` (a
synced file in many setups), so a poisoned config from another machine becomes
RCE the next time the user syncs.

**Fix:** Allowlist schemes (`https://`, `ssh://`, `git://`, `user@host:path`).
Reject `ext::`, `file://`, anything starting with `-`. Validate at both the
init flow and the settings flow. Pass `--` separator to git for defense in
depth.

### 46. `expandBrowserPath` allows `..` traversal outside `$HOME` · M · [sec]

**File:** `tui/add_view.go:974-982`

`expandBrowserPath("../../etc", homeDir)` returns `/etc`. `filepath.Clean` is
not applied to the jump input, and there is no post-resolution check that the
result is under `homeDir`. The browser then loads the directory and allows
selection. Files outside `$HOME` are classified as `Track`, but the user can
silently ship `/etc/sudoers` into a public dotfiles repo.

**Fix:** `filepath.Clean` then verify
`strings.HasPrefix(resolved, homeDir+string(filepath.Separator))`. Reject
otherwise with an explicit "outside home" status.

---

## Theme D — Git integration

### 39. Git commands have no timeout — TUI can wedge · C · [ux] · ✅ fixed in 7189c7d

**File:** `internal/git/git.go` — `runGitCommand` (line 18) wraps a 30s
context, but is **only** used by `InitRepo` (line 53). Every other git call
— `AutoCommit`, `AutoCommitFiles`, `Sync`, `SyncDetailed`, `PushWithProgress`,
`Pull`, `Clone`, `CloneWithProgress`, `GetDiff`, `RestoreFile`,
`GetFileHistory`, `GetStatus`, `HasChanges`, `GetRemoteURL` — uses bare
`exec.Command` with no context.

A hung remote (dead network, SSH passphrase prompt, slow push) blocks the Tea
command goroutine forever. Since `autoCommitCmd` chains after nearly every
action, one bad operation freezes the whole app with no in-TUI cancel.

**Fix:** Route all git calls through `runGitCommand`. Use a longer timeout
(2-5min) for push/pull/clone. Surface timeouts as actionable errors.

### 41. Push/clone progress freezes on auth prompts · M · [ux]

**File:** `internal/git/git.go:271-273, 499-501`

`PushWithProgress`/`CloneWithProgress` set `Stdout = nil` and `Stderr = nil`,
inheriting from the parent TTY. Under the alt-screen TUI, git's prompt for
SSH passphrase or HTTP credentials writes to the TTY underneath the redraw and
waits on stdin. The user sees a frozen app with no indication of why.

**Fix:** Set `cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0",
"GIT_ASKPASS=echo")` to fail fast on auth requirements. Pipe stdout/stderr
into a progress modal or the log view.

### 4. Settings remote URL not applied to git · M

**File:** `tui/settings_view.go:238-247`

Enter handler assigns `m.cfg.GitRemote` and saves `.dotcorrc` but never calls
`git.SetRemote`. The init flow at `tui/app.go:737` does it correctly.
Push/pull continue to use the old `.git/config` URL. Settings UI shows the
new value, creating silent divergence.

**Fix:** Call `git.SetRemote(m.repoDir, "origin", m.settingsInput.Value())`
**before** `SaveConfig` (so a git failure doesn't poison `.dotcorrc`). Handle
empty URL with `git remote remove origin`. After fixing, also subject the URL
to #42's allowlist.

### 17. Detached HEAD shows "no git repository" · M · [ux]

**Files:** `internal/git/git.go:325-330`; `tui/dashboard.go:559-580`

`git branch --show-current` returns empty on detached HEAD, on rebase/bisect,
and on a fresh repo with no commits. The dashboard's git bar then renders no
pills, and the fallback at line 579 shows "no git repository" — inside a real
repo.

**Fix:** In `GetStatus`, when `--show-current` is empty, fall back to
`git rev-parse --short HEAD` and add `Detached bool` to `StatusInfo`. The
dashboard renders `(detached: abc1234)` with a yellow pill. The "no git
repository" branch should additionally check `!git.IsRepo(...)`.

### 24. `git diff HEAD` raw error when no commits exist · L · [ux]

**File:** `tui/diff_view.go:76-84`

Fresh `git init` + no commits → `git diff HEAD` returns `fatal: bad revision
'HEAD'`. The TUI surfaces this raw text. Friendly fallback would be "No
commits yet — press S to create the first commit."

---

## Theme E — UI state staleness (refresh dispatch)

These all reduce to the cross-cutting "refresh dispatch is inconsistent" root cause.

### 5. `statusMsg`/`errMsg` don't refresh dashboard · M · [ux]

**File:** `tui/app.go:261-269`

`stowResultMsg` and `classifyResultMsg` call `m.refreshAll()`. `statusMsg`,
`errMsg`, and `settingsMsg` do not. Operations that emit these — restore from
history, commit from diff view, clean backups — leave the dashboard stale
(package list, git status, recent commits, backups list).

**Fix:** Append `m.refreshAll()` to all three handlers.

### 6. History "D" key does nothing · H · [ux] · ✅ fixed in b426104

**File:** `tui/history_view.go:144-148`

`D` calls `diffFromCommit` which emits `diffMsg`. The handler at
`tui/diff_view.go:42-49` writes to `m.viewport` but never sets
`m.activeView = DiffView`. The user stays on history, sees no change, and
stale diff content surfaces if they later open the diff view.

**Fix:** Set `m.activeView = DiffView` in the `D` key handler (not in
`diffMsg`, which would also fire on unrelated diff fetches).

### 13. Diff view commit feedback invisible · M · [ux]

**File:** `tui/diff_view.go:19-37, 98-107`

`commitDiff` returns `statusMsg`/`errMsg`. The global handler stores them, but
`viewDiff()` renders only `header`, `viewport.View()`, and `footer` — never
`m.statusMsg` or `m.err`. Only the dashboard's git bar surfaces those, and the
dashboard isn't visible while in diff view. The 3-second status clear fires
silently.

**Fix:** Add a status row above the footer in `viewDiff` that reads
`m.statusMsg`/`m.err`.

### 16. Auto-commit races with `refreshAll` · M · [ux]

**Files:** `tui/app.go:286-293, 941-956, 977-983`

`autoCommitCmd` and `refreshAll` are dispatched concurrently via `tea.Batch`.
The status check can race the commit (seeing pre-commit dirty state) or the
working-tree walk can include transient `.dotcor-tmp` artifacts. `autoCommitCmd`
returns `nil` (no follow-up message), so the post-commit state never refreshes
until the next user action.

**Fix:** Chain — return a `autoCommittedMsg` from `autoCommitCmd` and trigger
`refreshAll` from its handler. Add `*.dotcor-tmp` to a `.gitignore` written
at init.

---

## Theme F — TOCTOU & concurrency

### 7. `restoreFromCommit` TOCTOU on `filePath` · M · [data — rare]

**Files:** `tui/history_view.go:130-142, 195-214`; `tui/app.go:843-852`

The dialog captures only `confirmRestoreRef`. `filePath` is recomputed at
confirm time from `m.selectedPkg`/`m.selectedFile`/`m.expanded`. A
`packagesMsg` arriving between dialog-open and Enter (e.g. from `Init()`'s
initial discovery) can shift indices, causing `git.RestoreFile` to operate on
the wrong file.

`m.expanded` is keyed by **index** (`map[int]bool`), not name, so package
reordering scrambles the expansion map and compounds the risk.

`diffFromCommit` (`history_view.go:216-236`) has the **identical** TOCTOU
shape — fix it at the same time.

**Fix:** Add `confirmFilePath string` to `Model`. Capture in the dialog-open
handler. Read it in `restoreFromCommit(ref, filePath)`. Reset it in
`clearConfirm()`.

### 8. `Unlink` missing `EvalSymlinks` on macOS · M

**File:** `internal/stow/unlink.go:55-69`

The comparison `resolved != filepath.Clean(path)` does not run either side
through `filepath.EvalSymlinks`. `internal/stow/classify.go:241` already does
this for the same comparison and explicitly cites macOS `/var → /private/var`.
Triggers when `repoDir`, `homeDir`, or any link path contains a symlink alias
segment. Result: `unstow` reports success but leaves repo-owned symlinks in
place; `result.Unlinked` under-counts.

**Fix:** Pre-resolve `pkgDir` / `repoDir` once at function entry (mirror the
`classify.go:127-131` pattern) and `EvalSymlinks` both sides of the compare,
falling back to the cleaned form on error.

### 48. `cleanDirChain` walk-up missing `EvalSymlinks` · L

**File:** `internal/stow/unlink.go:94-118`

The stop condition compares `dir` against `filepath.Dir(homeDir)` via string
prefix. If `homeDir` is a symlink (some SSO setups) or contains a `/var`-style
alias, the walk-up may ascend past the intended root before the prefix check
fires. Same family as #8.

**Fix:** Resolve `homeDir` once at entry; compare resolved forms.

---

## Theme G — Discoverability & UX gaps

### 14. Adopt `o` keybinding undiscoverable · H · [ux] · ✅ fixed in 6e5ebe2

**Files:** `tui/keys.go:55`, `tui/help_view.go:14-54`,
`tui/dashboard.go:601-613`

The binding exists in `keys.go` and is in the bubble keymap's
`ShortHelp`/`FullHelp`. But the TUI uses **custom** help and footer renders
that ignore the keymap. The custom Packages category in `help_view.go` and
`allHints` in the footer both omit `o`. Adopt is a non-obvious, somewhat
destructive operation; users without SPEC knowledge cannot find it.

**Fix:** Add `formatBinding("o", "adopt")` to `help_view.go` Packages
category. Add `kbd("o", "adopt")` to `allHints` in `renderFooter`.

### 2. `isExcluded` blanket dot-prefix hides packages · M · [ux]

**File:** `internal/stow/package.go:55-63`

```go
if strings.HasPrefix(name, ".") {
    return true
}
```

Beyond the explicit `excludedDirs` map, any dot-prefixed directory is
silently excluded. Packages named `.ssh`, `.config`, `.gnupg`, `.aws`,
`.kube`, `.docker` (common with GNU Stow conventions) are invisible to
`DiscoverPackages` and to `classify.go:181, 266`. SPEC.md is ambiguous on
this; the explicit `excludedDirs` map suggests it shouldn't be a blanket rule.

**Fix:** Remove the blanket check. **Also update**
`TestDiscoverPackages_ExcludesDotfileDirs` (`package_test.go:135-152`) which
currently asserts the buggy behavior. Reconcile SPEC.md line 57 wording.

### 23. Add view zero feedback when all already managed · M · [ux]

**File:** `tui/app.go:322-343`

When all selected files have `ClassManaged`, `total = 0` and `parts` is empty.
No status message is shown. The user completes the entire add wizard
(select → preview → confirm) and is dropped on the dashboard with no signal
that nothing happened.

**Fix:** Else-branch setting `m.statusMsg = fmt.Sprintf("%d already managed",
managedCount)`.

### 22. `LinkWithBackup` count conflates linked + resolved · M · [ux]

**File:** `internal/stow/link.go:208-270`

`LinkWithBackup` mutates the original `LinkResult` (`Linked++`, `Skipped--`)
for each resolved conflict. `tui/app.go:1033` displays `result.Linked` as
"Resolved N conflicts in PkgName", implying all `Linked` were conflicts.

**Fix:** Track `Resolved int` separately on `LinkResult`. Update the message:
"Linked N (resolved M conflicts)".

---

## Theme H — Layout & rendering

### 15. Negative panel width on narrow terminals · M · [ux]

**Files:** `tui/dashboard.go:186-197`; `tui/app.go:225-231`

`leftWidth` clamps to min 36, but `rightWidth = m.width - leftWidth` goes
negative when `m.width < 36`. The `WindowSizeMsg` handler also computes
`m.viewport.Width = msg.Width - 4` with no clamp.

**Fix:** Clamp `m.width`, `m.height`, `m.viewport.Width`, `m.viewport.Height`
to ≥ 0 in `WindowSizeMsg`. After computing `rightWidth`, guard
`if rightWidth < 10 { rightWidth = 10; leftWidth = max(0, m.width-rightWidth) }`.
Or render a "terminal too narrow" placeholder under a threshold.

### 35. `renderPackageCard` doesn't truncate long names · L · [ux]

**File:** `tui/dashboard.go:328-383`

`pkg.Name` is rendered without truncation; the `gap` calculation clamps to 1
but doesn't shrink the name. A long name + tag overflows `contentWidth`.

**Fix:** Truncate name to `contentWidth - rightW - 3` before styling.

### 36. Confirm modal has no height bound · M · [ux]

**File:** `tui/modal.go:9-35`

`MaxWidth` is set; no `MaxHeight`. Conflict lists (one line per conflict at
`tui/app.go:279-283`) and adoptable file lists (`tui/app.go:542-546`) can
overflow on tall lists or short terminals — confirm/cancel may go off-screen.

**Fix:** Truncate body to `m.height - 8` lines with a `... N more` suffix.
Apply `MaxHeight(m.height - 4)` to the modal style.

### 37. Help dialog height not bounded · L · [ux]

**File:** `tui/help_view.go:95-103`

No `MaxHeight`; height is content-driven. Today's content is short, so not
triggerable. Bound for forward safety.

---

## Theme I — Resilience & misc correctness

### 38. `buildHomeSymlinkIndex` silent fallback · M

**File:** `internal/stow/classify.go:136-140`

On error, falls back to an empty index without logging. Files that should
classify as `ClassAdopt` (existing `$HOME → external` symlinks the user wants
to import) silently become `ClassAdd` — which moves the source and creates a
new symlink, breaking any external tool that owned the original target.

**Fix:** Surface as a `Warnings []string` field on `ClassificationPlan`. Log
via `cfg.Logger` at minimum. Display in the preview.

### 43. `DiscoverPackages` aborts on one unreadable file · M · [ux]

**File:** `internal/stow/package.go:101-157`

The walk callback returns `err` on line 106. A single `chmod 000` file
anywhere in the repo aborts the entire package walk, then `DiscoverPackages`
propagates the error (line 83). The TUI shows "err" with no package state for
the whole session. `walkAndClassify` (`classify.go:332`) handles the same
case correctly by logging and returning `nil`.

**Fix:** In the per-file callback, log via `cfg.Logger` and return `nil` to
continue. Surface skipped files in a result-summary field.

### 44. `cfg.Logger` may be nil after `LoadConfig` · M

**File:** `internal/config/config.go:66-` (LoadConfig); consumers:
`internal/core/hooks.go`, `backup.go`, `templates.go`

`LoadConfig` does not initialize `cfg.Logger`. `NewDefaultConfig` does;
`main.go` overwrites afterward. If `LoadConfig` succeeds but logger setup
fails, `cfg.Logger` stays nil and any unguarded call panics. Several core
packages call `cfg.Logger.X` without a nil check.

**Fix:** In `LoadConfig`, initialize `cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))`
before returning. Audit core packages for unguarded `cfg.Logger.*` calls.

---

## Theme J — Performance

These are real but only matter for large repos / deep `$HOME` trees. Profile
before fixing.

### 18. `stow.Link` re-discovers packages mid-operation · L · [perf]

**File:** `internal/stow/link.go:53-69`

After walking and linking in-repo files, `Link` calls `DiscoverPackages()`
again to find auto-detected files — a full FS walk including `$HOME`.

**Fix:** Restructure `Link` to take an already-discovered `*Package` and an
opt-in callback for foreign-symlink handling (which is gated by #27 anyway).

### 19. `buildHomeSymlinkIndex` walks `$HOME` per classify · L · [perf]

**File:** `internal/stow/classify.go:216-255`

Walks `homeDir` (with `classifySkipDirs`) every time the user selects files in
the Add view. Bounded by skip-list but still per-action.

**Fix:** Cache once per TUI session in `Model`. Invalidate on
stow/unstow/adopt/sync.

### 20. `repoSizeMB` recalculates on every refresh · L · [perf]

**File:** `tui/app.go:249`

`m.repoSizeCached = repoSizeMB(m.repoDir)` runs synchronously inside the
`packagesMsg` handler, which fires after every operation. Misleadingly named
"cached".

**Fix:** Compute on startup and after sync only; or make it an async
`tea.Cmd` with its own message.

---

## Theme K — Minor polish (low signal, group-fix)

These are real but trivial. Land in a single "polish" commit if convenient,
or leave for opportunistic cleanup.

- **25.** Logger file handle never closed (`internal/logger/logger.go:53`) —
  OS reclaims on exit. Real consequence is no in-session rotation; better
  addressed by switching to time-based rotation.
- **26.** `countFilesRecursive` skips symlinks (`tui/add_view.go:1080-1088`).
- **28.** `expandBrowserPath` mishandles bare `~` (`tui/add_view.go:974-982`)
  — one-line fix: `if raw == "~" { return homeDir, nil }`.
- **29.** Empty parent dirs left in repo after `removeFileFromPackage`
  (`tui/app.go:1168-1212`).
- **30.** `formatRelativeTime` doesn't handle future timestamps
  (`tui/diff_view.go:148-161`) — `if d < 0 { return "just now" }`.
- **31.** Browser jump doesn't reset cursor to target
  (`tui/add_view.go:934-971`).
- **32.** `fuzzyMatch` breaks for non-ASCII queries
  (`tui/dashboard.go:699-712`) — iterate `q` as runes.
- **33.** `loadLogs` reads entire log file (`tui/app.go:1274-1301`) — capped
  at ~10 MB by rotation; `bufio.Scanner` from tail would be cleaner.
- **34.** `settingsIgnoreIdx` not clamped after deleting last pattern
  (`tui/settings_view.go:204-210`).

---

## Top priorities (ranked shortlist)

The first six commits give the largest correctness/UX win for the smallest diff:

1. ✅ **#3** `deletePackage` silent backup failure — destructive + actively misleading
2. ✅ **#40** Ignore patterns unenforced — secrets ship to remote
3. ✅ **#39** Git commands have no timeout — root cause of "TUI froze" reports
4. ✅ **#6** History "D" does nothing — advertised feature broken
5. ✅ **#27** Implicit adoption during stow — silent unauthorized mutation
6. ✅ **#14** Adopt `o` undiscoverable — hides safety-critical feature

### Next batch (in order of leverage)

1. ✅ **#21** Migration has no rollback — H, data
2. ✅ **#5** `statusMsg`/`errMsg` don't refresh dashboard — M, ux
3. ✅ **#13** Diff view commit feedback invisible — M, ux
4. ✅ **#43** `DiscoverPackages` aborts on one unreadable file — M, ux
5. ✅ **#2** `isExcluded` blanket dot-prefix hides packages — M, ux
6. ✅ **#17** Detached HEAD shows "no git repository" — M, ux
7. ✅ **#10** `core.Transaction` never used in stow operations — H, data
   (folded in fixes for #11 silent nil and #45 restore-write swallowed;
   #22 Resolved counter added)

### Remaining

- **#19** `buildHomeSymlinkIndex` walks `$HOME` per classify — perf only;
  bounded by `classifySkipDirs`. Requires a session-scoped cache with
  invalidation on stow/unstow/adopt/sync. Non-blocking; deferred until
  there's a concrete complaint from a user with a large `$HOME`.

Every other issue from the original review is resolved. The repo is
0-lint, 434 tests passing.
