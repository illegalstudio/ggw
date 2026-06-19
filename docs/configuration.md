# Configuration

ggw reads an optional config file at:

```
~/.config/ggw/config.yaml
```

## Keys

| Key        | Type   | Description |
|------------|--------|-------------|
| `base_dir` | string | Directory under which all worktrees live, nested as `<base_dir>/<org>/<repo>/<branch-slug>`. A leading `~` is expanded to `$HOME`. |

## Precedence

The worktrees base directory is resolved in this order:

1. `base_dir` from `~/.config/ggw/config.yaml` (if set)
2. `$XDG_DATA_HOME/worktrees`
3. `~/.local/share/worktrees`

A configured `base_dir` is used directly — unlike the default, no `worktrees`
subdirectory is appended.

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

### Transactional rollback

If any step fails, `ggw` removes the new worktree (`git worktree remove --force`) and exits with an error. **The branch is always kept**, so you can fix the issue and re-run `ggw create` or `ggw pr` without losing the branch.

Failure causes include:

- A path in `copy` or `symlink` is absolute or escapes the repo root via `..`.
- The source file or directory does not exist in the main worktree.
- The destination path already exists in the new worktree.
- A `post_create` command exits with a non-zero code.

### Skipping provisioning

Pass `--bare` to `ggw create` or `ggw pr` to skip all provisioning for a single run, even when `.ggw.yaml` is present.
