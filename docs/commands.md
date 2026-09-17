# Commands Reference

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Emit machine-readable JSON for supported commands. Interactive prompts are refused or auto-confirmed where needed. |

## Workspace Kinds

`create` and `pr` make one of two kinds of workspace, both stored at
`<base>/<org>/<repo>/<slug>` and treated alike by every other command:

| Flag | Kind | What it is |
|------|------|-----------|
| `--wt` | `worktree` | a git worktree |
| `--cow` | `cow` | a copy-on-write snapshot of the repository, untracked files included |

The two flags are mutually exclusive. Without either, the kind comes from
`GGW_MODE`, then `mode` in the config file, then `worktree`. See
[Configuration](configuration.md#copy-on-write-workspaces) for requirements and
trade-offs.

JSON output follows these conventions:

- `list` emits `{ "worktrees": [...] }`, each entry carrying a `kind` of `"worktree"` or `"cow"`, and `main: true` on the repository's main worktree.
- `create`, `pr`, `cd`, `delete`, and `shell-init` emit a small object describing the action.
- `exec` does not support `--json` because it streams another process through stdin, stdout, and stderr.
- `skills install` emits `{ "name": ..., "installations": [...] }`, never prompts, and reports per-destination failures in each item's `error` field without changing the exit code.
- `skills verify` emits `{ "name": ..., "verifications": [...] }` and exits non-zero when an installed skill is stale; the JSON payload is still emitted in that case.

## `ggw list`

List every workspace of the current repository: its git worktrees and its
copy-on-write snapshots.

```bash
ggw list
ggw list --full-path
ggw --json list
```

| Flag | Description |
|------|-------------|
| `--full-path` | Show the full tildified path instead of the compact `[...]` form. |

The human output includes the branch name, workspace path, dirty marker, and upstream ahead/behind counters when an upstream is configured. Copy-on-write workspaces are tagged `[cow]`.

## `ggw create`

Create a workspace for a branch.

```bash
ggw create feature/login
ggw create                            # random name, e.g. intelligent-elephant
ggw create fix/api --from main
ggw create feature/login --cow        # copy-on-write snapshot
ggw create feature/login --as review  # choose the directory name
ggw create feature/login --bare       # skip provisioning for this run
```

| Flag | Description |
|------|-------------|
| `--from` | Base ref used when creating a new local branch. Defaults to `HEAD`. |
| `--as` | Directory name for the workspace. Defaults to the slugified branch. |
| `--cow` | Make a copy-on-write snapshot instead of a worktree. |
| `--wt` | Make a git worktree. |
| `--bare` | Skip `.ggw.yaml` provisioning for this run. |

Behavior:

- If no branch argument is given, `ggw` generates a random Docker-style name (`adjective-noun`, e.g. `intelligent-elephant`), skipping names that already exist as a local branch, an `origin/*` tracking branch, or a workspace path.
- If the branch exists locally, `ggw` checks it out into a new workspace.
- If `origin/<branch>` exists locally, `ggw` creates a tracking branch.
- Otherwise, `ggw` creates a new branch from `--from` or `HEAD`.

The branch name is passed to git unchanged. Only the directory name is slugified, so `feature/login` is stored as `feature-login`.

`--as` decouples the directory from the branch. It is what allows two
copy-on-write workspaces on the same branch — git refuses to check out one
branch in two worktrees, but two independent repositories have no such rule.
When a branch names more than one workspace, `cd`, `exec` and `delete` stop
resolving it and use the directory names instead.

Tab completion suggests existing local and `origin/*` branches that do **not** already have a workspace, so you can quickly spin up one on an existing branch without creating a new one. Branches already checked out are omitted.

If a `.ggw.yaml` exists at the repository root, `ggw create` provisions the new workspace automatically (copy → symlink → post_create). For a `cow` workspace the copy and symlink steps are skipped — the snapshot already contains them — and only `post_create` runs. If provisioning fails, the workspace is removed but any pre-existing branch is kept. See [`ggw project-init`](#ggw-project-init) and [Project Provisioning](configuration.md#project-provisioning-ggwyaml).

## `ggw pr`

Create a workspace for a GitHub pull request.

```bash
ggw pr 123
ggw --json pr 123
ggw pr 123 --cow    # copy-on-write snapshot
ggw pr 123 --bare   # skip provisioning for this run
```

| Flag | Description |
|------|-------------|
| `--as` | Directory name for the workspace. Defaults to `pr-<id>`. |
| `--cow` | Make a copy-on-write snapshot instead of a worktree. |
| `--wt` | Make a git worktree. |
| `--bare` | Skip `.ggw.yaml` provisioning for this run. |

`ggw pr` requires [GitHub CLI](https://cli.github.com/) to be installed and authenticated. If `gh` is not available, the command exits with installation guidance.

Behavior:

- Creates a workspace at `.../<org>/<repo>/pr-<id>/` with no branch of its own: a detached worktree, or a snapshot left where the repository's HEAD was.
- Runs `gh pr checkout <id>` inside it.
- Leaves the checkout on the branch selected by `gh`, preserving tracking metadata so `git push` works when GitHub permits pushing to the PR branch.

For PRs from external forks, pushing still depends on GitHub permissions such as maintainer edit access.

If a `.ggw.yaml` exists at the repository root, `ggw pr` provisions the new workspace automatically (copy → symlink → post_create). If provisioning fails, the workspace is removed but the branch is kept. See [`ggw project-init`](#ggw-project-init) and [Project Provisioning](configuration.md#project-provisioning-ggwyaml).

## `ggw cd`

Print the absolute path of a matching workspace.

```bash
ggw cd feature/login
ggw cd feature-login
ggw --json cd feature/login
```

Matching order:

1. Exact path.
2. Exact branch name.
3. Handle — the branch name, or trailing path segments for a workspace a branch cannot name on its own.
4. Exact workspace directory basename.
5. Case-insensitive substring match against branch or path.

If no argument is provided, or if a step matches more than one workspace, `ggw` opens an interactive selector. In `--json` mode, interactive disambiguation is refused.

A branch shared by two workspaces names neither, so it goes to the selector rather than silently picking one. Directory basenames are the exception: they resolve to the first match, which is the main worktree when it shares a basename with a detached workspace.

Use shell integration if you want `ggw cd` to change the current shell directory. See [Shell Integration](shell-integration.md).

## `ggw exec`

Run a command inside a workspace.

```bash
ggw exec feature/login -- npm install
ggw exec feature/login -- git status --short
ggw exec -- pwd
```

Everything after `--` is passed to the child command unchanged. The child process inherits stdin, stdout, and stderr, and its exit code is propagated.

`ggw exec` requires a command after `--` and does not support `--json`.

## `ggw delete`

Delete a workspace.

```bash
ggw delete feature/login
ggw delete feature/login --force
ggw delete feature/login --without-branch
ggw --json delete feature/login --force
```

| Flag | Description |
|------|-------------|
| `--force` | Remove a workspace holding unsaved work, and skip confirmation. |
| `--without-branch` | Keep the local branch after removing a worktree. Has no meaning for a `cow` workspace. |

By default, `ggw delete` removes both the selected worktree and its local branch. The current workspace and the main worktree are protected from deletion.

Deleting a **copy-on-write workspace** removes its branch with it, because the
branch exists nowhere else.

`ggw delete` therefore refuses outright — no confirmation prompt — when such a
workspace holds **commits that exist nowhere else**, and prints the `git fetch`
that saves the branch into the source repository first. A commit counts as safe
when it is reachable from a remote-tracking ref, or from a branch of the
repository the snapshot came from. History the snapshot merely inherited is
never counted, and neither is a second local branch inside the snapshot — that
ref is in the very directory about to be removed.

**Uncommitted changes** are treated more lightly, because a snapshot inherits
whatever the source had in progress and is therefore often dirty from birth with
nothing of its own at stake. They are mentioned in the confirmation prompt rather
than blocking the command. Under `--json` there is no prompt to answer, so a
dirty workspace is refused and `--force` is required — the same protection git
gives a dirty worktree.

## `ggw shell-init`

Print shell integration for `bash`, `zsh`, or `fish`.

```bash
eval "$(ggw shell-init bash)"
eval "$(ggw shell-init zsh)"
ggw shell-init fish | source
```

The generated script makes `ggw cd` perform a real shell `cd` and installs Cobra-powered tab completion for commands and workspace names.

## `ggw init`

Create the global config file (`~/.config/ggw/config.yaml`), seeded with this
system's current default base directory and `mode: worktree`.

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

`.ggw.yaml` declares how each new workspace is set up after `ggw create` or `ggw pr`:

```yaml
copy:            # files/dirs copied from the main worktree
  - .env
symlink:         # files/dirs symlinked to the main worktree
  - node_modules
  - vendor
post_create:     # shell commands run in the new worktree, in order
  - composer install
```

Provisioning runs in the fixed order copy → symlink → post_create. In `cow` mode copy and symlink are skipped, since the snapshot already contains them. If any step fails, the new workspace is removed (a pre-existing branch is kept) so you can fix the issue and re-run. Pass `--bare` to `create` or `pr` to skip provisioning for a single run.

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

### Stale-skill notice

When an installed skill no longer matches the one bundled with the running
binary, interactive commands end with a short stderr notice pointing at the
destination and at the remedies. The notice never appears under `--json`, never
on `skills`, `shell-init`, `completion`, `cd`, `exec`, any `--help` or help
output, shell tab-completion internals, or version output,
and never when no skill is installed. Check on demand with
[`ggw skills verify`](#ggw-skills-verify); silence the notice permanently by
setting `suppress_skills_notice: true` in the
[config file](configuration.md).

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

## `ggw skills verify`

Check whether the installed copies of the bundled AI agent skill match the
skill carried by the running `ggw` binary, using the SHA-256 digests recorded
at install time. Read-only: nothing is written, nothing is prompted for.

```bash
# Every known destination
ggw skills verify

# One destination only
ggw skills verify --target claude

# Machine-readable, for scripts and CI
ggw --json skills verify
```

| Flag | Description |
|------|-------------|
| `--target` | Verify only this destination (`agents`, `claude`). Repeatable. |

Each destination reports one status:

| Status | Meaning |
|---|---|
| `up-to-date` | matches the bundled skill |
| `outdated` | installed by ggw, never edited, but from another ggw version — `ggw skills install` refreshes it |
| `modified` | differs from the bundled skill and was edited locally or not written by ggw — refreshing it needs `ggw skills install --force` |
| `not-installed` | no skill at this destination |

The exit code is `1` when any installed destination is `outdated` or
`modified`, so the check can gate scripts. Destinations that are not installed
do not fail the command.

### JSON output

```json
{
  "name": "ggw",
  "verifications": [
    { "target": "agents", "path": "/Users/me/.agents/skills/ggw", "status": "outdated" },
    { "target": "claude", "path": "/Users/me/.claude/skills/ggw", "status": "not-installed" }
  ]
}
```

When the exit code is `1` because a skill is stale, the full payload above is
still emitted. Per-destination inspection failures appear in the item's `error`
field and do not change the exit code.
