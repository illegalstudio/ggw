# Project Provisioning (`.ggw.yaml`) — Design

Date: 2026-06-19

## Summary

Add per-project worktree provisioning. A new command `ggw project-init` writes a
`.ggw.yaml` file at the repository root. That file declares:

- **copy** — files/directories to copy into each new worktree,
- **symlink** — files/directories to symlink into each new worktree,
- **post_create** — shell commands to run inside each new worktree after it is set up.

Provisioning runs automatically after `ggw create` and `ggw pr` create a
worktree. It is **transactional**: if any step fails, ggw removes the freshly
created worktree (keeping the branch) so the user can fix the issue and re-run.

## Motivation

A fresh worktree is a clean checkout: it lacks the untracked, machine-local
files a repo needs to be runnable (`.env`, build caches, `node_modules`,
`vendor`, …) and has not had its dependencies installed. Today the user must
recreate this state by hand for every worktree. `.ggw.yaml` lets a project
declare, once and shared with the team, how to make every new worktree
ready to use.

## Command: `ggw project-init`

Creates `.ggw.yaml` at the **main worktree root** with a commented template.

- Group: *Configuration* (alongside `ggw init`).
- `Args`: none.
- Behavior: errors if `.ggw.yaml` already exists, unless `--force` is given,
  in which case it is regenerated.
- The file is written at the main worktree root (resolved via
  `worktree.List(root)[0].Path`), so it is the canonical project config even
  when the command is run from inside a generated worktree.
- Honors `--json`, emitting `{"created": true, "path": "<abs path>"}`.

Template content (sections present but empty/example, fully commented):

```yaml
# .ggw.yaml — per-project worktree provisioning.
#
# Paths are relative to the repository root. `copy` and `symlink` sources are
# taken from the main worktree; the destination is the same relative path in
# the newly created worktree.

# Files/directories copied (recursively) into each new worktree.
copy: []
  # - .env

# Files/directories symlinked into each new worktree. The symlink points at the
# absolute path in the main worktree.
symlink: []
  # - node_modules
  # - vendor

# Shell commands run (via `sh -c`) inside each new worktree, in order, after
# copy and symlink. Execution stops at the first non-zero exit code.
post_create: []
  # - composer install
  # - npm ci
```

## Config schema

```yaml
copy:        # []string — relative paths
symlink:     # []string — relative paths
post_create: # []string — shell command strings
```

Parsed with viper/yaml into:

```go
type Config struct {
    Copy       []string `mapstructure:"copy"`
    Symlink    []string `mapstructure:"symlink"`
    PostCreate []string `mapstructure:"post_create"`
}
```

A missing `.ggw.yaml` means "no provisioning configured" — not an error.

## Provisioning flow

Triggered by `ggw create` and `ggw pr`, immediately after the worktree is
created, unless `--bare` is passed.

Resolution:

- **Main worktree path**: `worktree.List(root)[0].Path` — git always lists the
  main worktree first. `.ggw.yaml` is read from there, and `copy`/`symlink`
  sources are resolved relative to it.
- **Destination**: the same relative path inside the new worktree.

Order of operations:

1. **copy** — for each entry, copy file or directory (recursively) from
   `<main>/<entry>` to `<dest>/<entry>`. Parent directories are created.
2. **symlink** — for each entry, create a symlink at `<dest>/<entry>` pointing
   at the **absolute** path `<main>/<entry>`. Parent directories are created.
3. **post_create** — run each command string via `sh -c`, in order, with
   working directory `<dest>`, inheriting the environment, streaming stdout and
   stderr to the user. Stop at the first non-zero exit code.

## Transactional rollback

The "transaction" covers the entire provisioning phase (steps 1–3 above), which
begins only after `worktree.Create`/PR checkout has succeeded. On **any**
failure:

1. Report which step/entry/command failed and the underlying error.
2. Remove the worktree with `git worktree remove --force <dest>` (removes the
   git registration and the directory).
3. **Keep the branch.** ggw never deletes the branch during rollback, so work
   on a pre-existing branch is never lost. (Re-running `ggw create <branch>`
   then takes the "branch exists locally → checkout" path.)
4. Exit non-zero.

If `git worktree remove --force` itself fails, both errors are reported and the
worktree is left in place for manual cleanup.

## Path validation

- Only relative paths are accepted for `copy` and `symlink` entries. Absolute
  paths, and relative paths that escape the repository via `..`, are rejected
  (treated as a failure → rollback).
- A `copy`/`symlink` **source that does not exist** in the main worktree is a
  failure → rollback. (Chosen over silent skipping for predictability under the
  transactional model.)
- If a **destination already exists**, it is a failure → rollback. A
  pre-existing destination signals a misconfiguration (e.g. copying a
  git-tracked file that is already present in the checkout).

## `--bare` flag

Added to `ggw create` and `ggw pr`. When set, the worktree is created with no
provisioning (no copy, symlink, or commands), regardless of `.ggw.yaml`.

## Code structure

- `internal/project/`
  - `project.go`: `Config`, `ConfigFileName = ".ggw.yaml"`,
    `Load(mainWorktreePath) (*Config, bool, error)` (bool = file exists),
    `WriteTemplate(path string, force bool) error`.
  - `provision.go`: `Provision(opts ProvisionOptions) error` performing
    copy/symlink/commands; path-validation helpers. `ProvisionOptions{ MainPath,
    DestPath string, Config *Config, Out io.Writer }`.
- `internal/cli/project_init.go`: the `project-init` command.
- `internal/cli/create.go` and `internal/cli/pr.go`: add `--bare`; call a shared
  helper (e.g. `provisionWorktree(root, dest, bare) error`) that resolves the
  main worktree, loads `.ggw.yaml`, runs `project.Provision`, and on error
  performs the rollback described above.

`worktree.Create` and the PR checkout logic are unchanged. Rollback uses the
existing `worktree.Remove(root, dest, force=true)`.

## Testing

Unit tests (`internal/project`):

- config parsing (all sections, missing file → not-exists, empty file).
- path validation: reject absolute paths and `..` escapes.
- copy of a file and of a directory (recursive) into a temp destination.
- symlink creation pointing at the absolute main path.
- `post_create` execution order and stop-on-first-non-zero.
- missing source → error.

E2E tests (`tests/`):

- `ggw project-init` creates `.ggw.yaml` (and `--force` regenerates).
- `ggw create` with a `.ggw.yaml` copies a file, symlinks a directory, and runs
  a command that produces an observable effect in the new worktree.
- a failing `post_create` command removes the worktree but leaves the branch.
- `--bare` skips provisioning.

## Documentation

Update `docs/commands.md` (new `ggw project-init`, `--bare` on create/pr) and
add a `.ggw.yaml` section to `docs/configuration.md`. Mention in `README.md`.
