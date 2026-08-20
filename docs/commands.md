# Commands Reference

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Emit machine-readable JSON for supported commands. Interactive prompts are refused or auto-confirmed where needed. |

JSON output follows these conventions:

- `list` emits `{ "worktrees": [...] }`.
- `create`, `pr`, `cd`, `delete`, and `shell-init` emit a small object describing the action.
- `exec` does not support `--json` because it streams another process through stdin, stdout, and stderr.
- `skills install` emits `{ "name": ..., "installations": [...] }`, never prompts, and reports per-destination failures in each item's `error` field without changing the exit code.

## `ggw list`

List worktrees registered for the current repository.

```bash
ggw list
ggw list --full-path
ggw --json list
```

| Flag | Description |
|------|-------------|
| `--full-path` | Show the full tildified path instead of the compact `[...]` form. |

The human output includes the branch name, worktree path, dirty marker, and upstream ahead/behind counters when an upstream is configured.

## `ggw create`

Create a worktree for a branch.

```bash
ggw create feature/login
ggw create                            # random name, e.g. intelligent-elephant
ggw create fix/api --from main
ggw create feature/login --bare   # skip provisioning for this run
```

| Flag | Description |
|------|-------------|
| `--from` | Base ref used when creating a new local branch. Defaults to `HEAD`. |
| `--bare` | Skip `.ggw.yaml` provisioning for this run. |

Behavior:

- If no branch argument is given, `ggw` generates a random Docker-style name (`adjective-noun`, e.g. `intelligent-elephant`), skipping names that already exist as a local branch, an `origin/*` tracking branch, or a worktree path.
- If the branch exists locally, `ggw` checks it out into a new worktree.
- If `origin/<branch>` exists locally, `ggw` creates a tracking branch.
- Otherwise, `ggw` creates a new branch from `--from` or `HEAD`.

The branch name is passed to git unchanged. Only the directory name is slugified, so `feature/login` is stored as `feature-login`.

Tab completion suggests existing local and `origin/*` branches that do **not** already have a worktree, so you can quickly spin up a worktree on an existing branch without creating a new one. Branches already checked out in a worktree are omitted.

If a `.ggw.yaml` exists at the repository root, `ggw create` provisions the new worktree automatically (copy → symlink → post_create). If provisioning fails, the worktree is removed but the branch is kept. See [`ggw project-init`](#ggw-project-init) and [Project Provisioning](configuration.md#project-provisioning-ggwyaml).

## `ggw pr`

Create a worktree for a GitHub pull request.

```bash
ggw pr 123
ggw --json pr 123
ggw pr 123 --bare   # skip provisioning for this run
```

| Flag | Description |
|------|-------------|
| `--bare` | Skip `.ggw.yaml` provisioning for this run. |

`ggw pr` requires [GitHub CLI](https://cli.github.com/) to be installed and authenticated. If `gh` is not available, the command exits with installation guidance.

Behavior:

- Creates a detached worktree at `.../<org>/<repo>/pr-<id>/`.
- Runs `gh pr checkout <id>` inside that worktree.
- Leaves the checkout on the branch selected by `gh`, preserving tracking metadata so `git push` works when GitHub permits pushing to the PR branch.

For PRs from external forks, pushing still depends on GitHub permissions such as maintainer edit access.

If a `.ggw.yaml` exists at the repository root, `ggw pr` provisions the new worktree automatically (copy → symlink → post_create). If provisioning fails, the worktree is removed but the branch is kept. See [`ggw project-init`](#ggw-project-init) and [Project Provisioning](configuration.md#project-provisioning-ggwyaml).

## `ggw cd`

Print the absolute path of a matching worktree.

```bash
ggw cd feature/login
ggw cd feature-login
ggw --json cd feature/login
```

Matching order:

1. Exact branch name or exact path.
2. Exact worktree directory basename.
3. Case-insensitive substring match against branch or path.

If no argument is provided, or if multiple substring matches are found, `ggw` opens an interactive selector. In `--json` mode, interactive disambiguation is refused.

Use shell integration if you want `ggw cd` to change the current shell directory. See [Shell Integration](shell-integration.md).

## `ggw exec`

Run a command inside a worktree.

```bash
ggw exec feature/login -- npm install
ggw exec feature/login -- git status --short
ggw exec -- pwd
```

Everything after `--` is passed to the child command unchanged. The child process inherits stdin, stdout, and stderr, and its exit code is propagated.

`ggw exec` requires a command after `--` and does not support `--json`.

## `ggw delete`

Delete a worktree.

```bash
ggw delete feature/login
ggw delete feature/login --force
ggw delete feature/login --without-branch
ggw --json delete feature/login --force
```

| Flag | Description |
|------|-------------|
| `--force` | Remove dirty worktrees too and skip confirmation. |
| `--without-branch` | Keep the local branch after removing the worktree. |

By default, `ggw delete` removes both the selected worktree and its local branch. The current worktree and the main worktree are protected from deletion.

## `ggw shell-init`

Print shell integration for `bash`, `zsh`, or `fish`.

```bash
eval "$(ggw shell-init bash)"
eval "$(ggw shell-init zsh)"
ggw shell-init fish | source
```

The generated script makes `ggw cd` perform a real shell `cd` and installs Cobra-powered tab completion for commands and worktree names.

## `ggw init`

Create the global config file (`~/.config/ggw/config.yaml`), seeded with this
system's current default worktrees directory.

```bash
ggw init
ggw --json init
```

Behavior:

- Fails if the config file already exists.
- The seeded `base_dir` is the path ggw would use right now, so the file is
  behavior-preserving until edited.

See [Configuration](configuration.md) for the file format and precedence.

## `ggw project-init`

Create a `.ggw.yaml` provisioning file at the repository root.

```bash
ggw project-init
ggw project-init --force   # overwrite an existing file
```

| Flag | Description |
|------|-------------|
| `--force` | Overwrite an existing `.ggw.yaml`. |

`.ggw.yaml` declares how each new worktree is set up after `ggw create` or `ggw pr`:

```yaml
copy:            # files/dirs copied from the main worktree
  - .env
symlink:         # files/dirs symlinked to the main worktree
  - node_modules
  - vendor
post_create:     # shell commands run in the new worktree, in order
  - composer install
```

Provisioning runs in the fixed order copy → symlink → post_create. If any step fails, the new worktree is removed (the branch is kept) so you can fix the issue and re-run. Pass `--bare` to `create` or `pr` to skip provisioning for a single run.

See [Project Provisioning](configuration.md#project-provisioning-ggwyaml) for the full schema reference.

## `ggw skills install`

Install the AI agent skill bundled with the `ggw` binary, so coding agents know
how to drive the CLI safely.

```bash
# Interactive: multi-select menu, both destinations preselected
ggw skills install

# Non-interactive: one destination
ggw skills install --target claude

# Non-interactive: every destination, machine-readable result
ggw --json skills install
```

| Flag | Description |
|------|-------------|
| `--target` | Install only to this destination (`agents`, `claude`). Repeatable. Skips the menu. |
| `--force` | Replace an existing skill that differs from the bundled version. |

Destinations:

- `~/.agents/skills/ggw` (`agents`) — Codex and other Agent Skills hosts
- `~/.claude/skills/ggw` (`claude`) — Claude Code

The two destinations are installed independently: a conflict in one is reported
on that destination and does not stop the other.

### Behavior

Reinstalling is safe. ggw records a SHA-256 digest of what it installed in a
`.ggw-managed.json` marker inside the destination, and compares it against both
the bundled skill and the files on disk:

| Situation | Status | Needs `--force` |
|---|---|---|
| Destination does not exist | `installed` | no |
| Files match the bundled skill | `up-to-date` | no |
| ggw installed it and you have not edited it | `updated` | no |
| You edited the files, or the directory was not created by ggw | `replaced` | **yes** |

Installation is atomic: contents are staged in a sibling temporary directory and
moved into place, and the previous copy is kept until the move succeeds.

The bundled skill is not upgraded automatically. After `brew upgrade ggw` (or any
other upgrade), re-run `ggw skills install` to refresh it; the command is
idempotent and reports `updated`.

### JSON output

`ggw --json skills install` never prompts. Without `--target` it installs every
destination. It emits:

```json
{
  "name": "ggw",
  "installations": [
    { "target": "agents", "path": "/Users/me/.agents/skills/ggw", "status": "installed" },
    { "target": "claude", "path": "/Users/me/.claude/skills/ggw", "error": "skill already exists at ... rerun with --force to replace it" }
  ]
}
```

Per-destination failures appear in `error` and do not change the exit code. An
unknown `--target` is a command-level error and does exit non-zero.
