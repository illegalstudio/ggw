# Configuration

ggw reads an optional config file at:

```
~/.config/ggw/config.yaml
```

## Keys

| Key        | Type   | Description |
|------------|--------|-------------|
| `base_dir` | string | Directory under which all workspaces live, nested as `<base_dir>/<org>/<repo>/<branch-slug>`. A leading `~` is expanded to `$HOME`. |
| `mode`     | string | What `ggw create` and `ggw pr` make by default: `worktree` (the default) or `cow`. |

## Precedence

The base directory is resolved in this order:

1. `base_dir` from `~/.config/ggw/config.yaml` (if set)
2. `$XDG_DATA_HOME/worktrees`
3. `~/.local/share/worktrees`

A configured `base_dir` is used directly — unlike the default, no `worktrees`
subdirectory is appended.

The workspace mode is resolved in this order:

1. `--cow` or `--wt` on the command line
2. the `GGW_MODE` environment variable (`worktree` or `cow`)
3. `mode` from `~/.config/ggw/config.yaml`
4. `worktree`

An unknown value is an error wherever it comes from — ggw never guesses.

## Copy-on-write workspaces

A `cow` workspace is a whole-directory snapshot of the repository's main
worktree, taken with the filesystem's block-sharing facility (reflinks). It
costs no disk space until one side is written to, and it carries every file git
does not track — `node_modules`, `vendor`, `.env`, build output.

### Requirements

| Platform | Mechanism | Filesystems |
|----------|-----------|-------------|
| Linux    | `cp --reflink=always` | btrfs, XFS with `reflink=1`, bcachefs |
| macOS    | `clonefile(2)` | APFS |

Two conditions must both hold:

- the filesystem must support block sharing — **ext4 does not**;
- `base_dir` must be on the **same filesystem** as the repository, because
  reflinks cannot cross mount points.

If either fails, `--cow` reports which one and stops. It never falls back to a
full copy: silently duplicating a multi-gigabyte working tree is a worse outcome
than an error.

### What a snapshot is, and is not

A snapshot is an independent repository. That has consequences worth knowing:

- **Its branch lives only inside it.** `ggw create --cow feature/x` creates
  `feature/x` in the snapshot; the original repository never hears about it
  until you push, or fetch from the snapshot.
- **Deleting it deletes the branch.** `ggw delete` therefore refuses to remove a
  snapshot holding commits that exist in no remote and in no branch of the
  source repository, and prints the `git fetch` that rescues the branch first.
  Uncommitted changes only add a line to the confirmation prompt, since a
  snapshot usually inherits some; under `--json`, where nothing can be
  confirmed, they are refused too. `--force` overrides either.
- **Two snapshots can share a branch.** git refuses to check out one branch in
  two worktrees; two independent repositories have no such rule. Use `--as` to
  give the second one its own directory.
- **It carries the source's uncommitted state.** A snapshot is taken from the
  main worktree as it is, dirty files included.
- **Absolute paths inside untracked files do not move.** A Python `.venv` or
  anything else that hard-codes its own location will point at the original.

### Getting work back to the repository

Every snapshot gets a remote named `main` pointing at the repository it came
from, so from inside the snapshot:

```bash
git fetch main               # pick up the original's new commits
git push main HEAD           # send this branch back
```

and from the original repository:

```bash
git fetch ~/.local/share/worktrees/acme/api/feature-x feature/x:feature/x
```

An existing remote called `main` is never overwritten.

## `ggw init`

`ggw init` creates the config file, seeded with the path ggw would use on this
system right now, so the file is behavior-preserving until you edit it. It
refuses to overwrite an existing config.

```bash
ggw init
# ✓ Config file created at ~/.config/ggw/config.yaml
```

## Project Provisioning (`.ggw.yaml`)

Each repository can include a `.ggw.yaml` file at its root (the main worktree)
to declare how new worktrees are provisioned. A missing file is a no-op — no
error is raised.

Run `ggw project-init` to create a starter file:

```bash
ggw project-init          # fails if .ggw.yaml already exists
ggw project-init --force  # overwrite an existing file
```

### Schema

All paths in `.ggw.yaml` are relative to the repository root.

```yaml
copy:
  - .env
  - config/local.php
symlink:
  - node_modules
  - vendor
post_create:
  - composer install --no-interaction
  - php artisan key:generate
```

| Key | Type | Description |
|-----|------|-------------|
| `copy` | list of strings | Paths copied recursively from the main worktree into the new worktree at the same relative path. |
| `symlink` | list of strings | Paths symlinked into the new worktree, pointing at the absolute path of the source in the main worktree. |
| `post_create` | list of strings | Shell commands (`sh -c`) run inside the new worktree after copy and symlink, in order. Stops at the first non-zero exit code. |

### Provisioning order

Steps always run in the fixed order: **copy → symlink → post_create**.

### Provisioning a copy-on-write workspace

A snapshot already contains everything `copy` and `symlink` exist to reproduce,
and those steps refuse to write over an existing destination. Both are therefore
**skipped** for `cow` workspaces, and only `post_create` runs. The same
`.ggw.yaml` works unchanged in either mode.

### Transactional rollback

If any step fails, `ggw` removes the new workspace and exits with an error. For a
worktree that is `git worktree remove --force`, and **the branch is always
kept**, so you can fix the issue and re-run `ggw create` or `ggw pr` without
losing it. For a copy-on-write workspace the directory is removed outright —
its branch was created moments earlier and held nothing.

Failure causes include:

- A path in `copy` or `symlink` is absolute or escapes the repo root via `..`.
- The source file or directory does not exist in the main worktree.
- The destination path already exists in the new worktree.
- A `post_create` command exits with a non-zero code.

### Skipping provisioning

Pass `--bare` to `ggw create` or `ggw pr` to skip all provisioning for a single run, even when `.ggw.yaml` is present.
