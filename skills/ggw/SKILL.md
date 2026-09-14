---
name: ggw
description: Operate the ggw CLI to create, list, enter, and delete workspaces — git worktrees or copy-on-write snapshots — stored in one predictable location per repository. Use when an AI agent is asked to install, configure, inspect, automate, troubleshoot, or run ggw, including creating workspaces for branches or GitHub pull requests, running commands inside one, or setting up .ggw.yaml provisioning.
---

# Operate GGW

Use the installed `ggw` executable as the source of truth for the available commands and flags.

## Preflight

1. Run `command -v ggw` and `ggw --version` before operating it.
2. Run `ggw <command> --help` before using flags whose behavior is unclear.
3. Every command except `shell-init` must run **inside a git repository**; ggw resolves the repository from the current directory. `create` additionally needs an `origin` remote, because the workspace path is derived from it.
4. Workspaces are stored under a single derived path, not next to the repository. See `docs/storage.md` in the ggw repository for the layout and slug rules, and `docs/configuration.md` for the optional `~/.config/ggw/config.yaml`.
5. Refresh this skill after upgrading GGW. Bare `ggw skills install` opens an interactive menu and fails without a terminal, so name the destination explicitly: `ggw skills install --target claude`. `--target` is repeatable, not comma-separated. Install only to destinations the user already has. If GGW reports locally modified skill files, do not add `--force` without the user's approval.

## Two kinds of workspace, one namespace

`create` and `pr` make either a **worktree** (a git worktree) or a **cow** workspace (a copy-on-write snapshot of the repository that also carries every untracked file). Both live at `<base>/<org>/<repo>/<slug>`, and `list`, `cd`, `exec` and `delete` treat them alike.

Which one you get, in decreasing priority: the `--cow` / `--wt` flags (mutually exclusive), `$GGW_MODE`, `mode` in the config file, then `worktree`.

What changes with `cow`, and matters to you:

- It is an **independent repository**. Its branch exists only inside it until someone pushes or fetches from it. `git worktree list` in the source repository will not show it.
- **Deleting it deletes that branch** — see below.
- It needs a filesystem that can share blocks (btrfs, XFS with `reflink=1`, bcachefs, APFS) with the repository and the base directory on the *same* filesystem. When that does not hold, `--cow` fails with a diagnosis and never silently makes a full copy. Do not retry it; use `--wt`, or report the message to the user.
- Every snapshot gets a remote named `main` pointing at the repository it came from.

## Inspect before mutating

These commands only read: `list`, `cd`, `shell-init`.

Run `ggw list --json` before anything that writes, and use its output to state the exact worktree you resolved — path and branch — before acting on it. Never pass a user's fuzzy name straight to a destructive command.

`ggw list --json` reports `kind`, `path`, `branch`, `dirty`, `ahead`, `behind`, `external`, and `main` per workspace. The human-readable `ggw list` prints, for each workspace, exactly the name `cd`, `exec` and `delete` accept. `kind` is `"worktree"` or `"cow"`; `"dirty": true` means uncommitted work; `"main": true` marks the repository's main worktree, which cannot be deleted.

Do not assume a branch identifies one workspace. Two `cow` workspaces created with `--as` can share a branch, and then only their paths tell them apart.

## Deleting a workspace destroys work

`ggw delete [name]` removes a workspace **and deletes its branch by default**. Only `--without-branch` keeps the branch, and only for a worktree; the repository's default branch is protected automatically.

- Resolve the target with `ggw list --json` first, show the user the resolved path, kind and branch, and confirm before running.
- `--force` removes a workspace holding unsaved work, discarding it, and skips the confirmation prompt. Never add it on your own initiative.
- Under `--json` the confirmation prompt is auto-accepted. `ggw --json delete <name>` deletes immediately with no interaction. Treat `--json` here as "already confirmed", never as a way to avoid asking the user.

Deleting a **cow** workspace is irreversible in a way deleting a worktree is not: its branch exists nowhere else, so the commits go with the directory. ggw refuses such a delete outright — not a prompt, an error — when the workspace holds commits reachable from no remote and from no branch of the source repository, and the error carries the `git fetch` that saves the branch into the source repository. Run that command, or relay it to the user. Reaching for `--force` instead destroys the work the refusal was protecting.

Uncommitted changes are a separate, weaker signal: a snapshot inherits whatever the source had in progress, so it is often dirty with nothing of its own at stake. Interactively they only add a line to the confirmation. Under `--json` there is nothing to confirm, so ggw refuses and asks for `--force` — expect to hit this routinely, and check with the user rather than adding `--force` reflexively.

## `ggw exec` runs arbitrary commands

`ggw exec [name] -- <cmd>...` runs everything after `--` verbatim inside the workspace's directory, with stdin/stdout/stderr piped through and the child's exit code propagated.

- This is the widest-reaching command in the CLI. Show the resolved workspace and the exact command line before running it.
- It does not support `--json` and exits with an error if you pass it.
- Omitting `[name]` opens an interactive selector, which is useless in a non-interactive context. Always pass an explicit name.

## Creating workspaces runs project provisioning

`ggw create [branch]` creates the branch if it does not exist and checks it out into a new workspace. With no argument it invents a random `adjective-noun` name. `--as <name>` sets the directory name independently of the branch, which is what allows two `cow` workspaces on one branch. `ggw pr <id>` does the same for a GitHub pull request and requires an authenticated `gh` CLI.

Both then apply the repository's `.ggw.yaml`, if present: it copies files, creates symlinks, and runs `post_create` shell commands inside the new workspace. In `cow` mode the copy and symlink steps are skipped — the snapshot already contains them — and only `post_create` runs.

- Read `.ggw.yaml` and tell the user which `post_create` commands will run before creating a workspace in an unfamiliar repository. They are arbitrary shell commands from the repo.
- Pass `--bare` to skip provisioning entirely for one run.
- If provisioning fails, ggw removes the new workspace but **keeps any pre-existing branch**, so a retry is safe.
- `ggw project-init` writes a starter `.ggw.yaml` at the repository root and `ggw init` writes the global config. Both refuse to overwrite an existing file unless forced; `ggw project-init --force` overwrites the user's provisioning file, so confirm it first.

## `ggw cd` prints a path, it does not change directory

A binary cannot change its parent shell's directory. `ggw cd <name>` writes the resolved path to stdout.

- In a script, use `cd "$(ggw cd <name>)"`.
- For an interactive shell, `ggw shell-init <bash|zsh|fish>` prints a `ggw()` shell function that wraps the binary and turns `ggw cd` into a real chdir, plus completions. It is evaluated from the user's shell config; it is not a `gcd` alias.

## Produce automation-friendly output

Use `--json` whenever the output is parsed. Parse standard output only; never parse the styled tables, spinners, or prompts.

- `--json` suppresses prompts. Where a prompt would have selected between several matches — `cd`, `delete`, and `exec` with an ambiguous or missing name — the command fails instead of guessing. Pass a name specific enough to resolve to exactly one workspace; when a branch names two, use the directory name.
- `exec` does not support `--json` at all.
- On failure, ggw emits a single `{"error": "..."}` object and exits non-zero.
- `ggw skills install` reports each destination in an `installations` array; a per-destination failure appears in that item's `error` field and does **not** change the exit code.
