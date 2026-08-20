---
name: ggw
description: Operate the ggw CLI to create, list, enter, and delete git worktrees stored in one predictable location per repository. Use when an AI agent is asked to install, configure, inspect, automate, troubleshoot, or run ggw, including creating worktrees for branches or GitHub pull requests, running commands inside a worktree, or setting up .ggw.yaml provisioning.
---

# Operate GGW

Use the installed `ggw` executable as the source of truth for the available commands and flags.

## Preflight

1. Run `command -v ggw` and `ggw --version` before operating it.
2. Run `ggw <command> --help` before using flags whose behavior is unclear.
3. Every command except `shell-init` must run **inside a git repository**; ggw resolves the repository from the current directory. `create` additionally needs an `origin` remote, because the worktree path is derived from it.
4. Worktrees are stored under a single derived path, not next to the repository. See `docs/storage.md` in the ggw repository for the layout and slug rules, and `docs/configuration.md` for the optional `~/.config/ggw/config.yaml`.
5. Refresh this skill after upgrading GGW. Bare `ggw skills install` opens an interactive menu and fails without a terminal, so name the destination explicitly: `ggw skills install --target claude`. `--target` is repeatable, not comma-separated. Install only to destinations the user already has. If GGW reports locally modified skill files, do not add `--force` without the user's approval.

## Inspect before mutating

These commands only read: `list`, `cd`, `shell-init`.

Run `ggw list --json` before anything that writes, and use its output to state the exact worktree you resolved — path and branch — before acting on it. Never pass a user's fuzzy name straight to a destructive command.

`ggw list --json` reports `path`, `branch`, `dirty`, `ahead`, `behind`, and `external` per worktree. A worktree with `"dirty": true` holds uncommitted work.

## Deleting a worktree destroys work

`ggw delete [name]` removes a worktree **and deletes its branch by default**. Only `--without-branch` keeps the branch, and the repository's default branch is protected automatically.

- Resolve the target with `ggw list --json` first, show the user the resolved path and branch, and confirm before running.
- `--force` removes a **dirty** worktree, discarding uncommitted changes, and skips the confirmation prompt. Never add it on your own initiative.
- Under `--json` the confirmation prompt is auto-accepted. `ggw --json delete <name>` deletes immediately with no interaction. Treat `--json` here as "already confirmed", never as a way to avoid asking the user.

## `ggw exec` runs arbitrary commands

`ggw exec [name] -- <cmd>...` runs everything after `--` verbatim inside the worktree's directory, with stdin/stdout/stderr piped through and the child's exit code propagated.

- This is the widest-reaching command in the CLI. Show the resolved worktree and the exact command line before running it.
- It does not support `--json` and exits with an error if you pass it.
- Omitting `[name]` opens an interactive selector, which is useless in a non-interactive context. Always pass an explicit name.

## Creating worktrees runs project provisioning

`ggw create [branch]` creates the branch if it does not exist and checks it out into a new worktree. With no argument it invents a random `adjective-noun` name. `ggw pr <id>` does the same for a GitHub pull request and requires an authenticated `gh` CLI.

Both then apply the repository's `.ggw.yaml`, if present: it copies files, creates symlinks, and runs `post_create` shell commands inside the new worktree.

- Read `.ggw.yaml` and tell the user which `post_create` commands will run before creating a worktree in an unfamiliar repository. They are arbitrary shell commands from the repo.
- Pass `--bare` to skip provisioning entirely for one run.
- If provisioning fails, ggw removes the new worktree but **keeps the branch**, so a retry is safe.
- `ggw project-init` writes a starter `.ggw.yaml` at the repository root and `ggw init` writes the global config. Both refuse to overwrite an existing file unless forced; `ggw project-init --force` overwrites the user's provisioning file, so confirm it first.

## `ggw cd` prints a path, it does not change directory

A binary cannot change its parent shell's directory. `ggw cd <name>` writes the resolved path to stdout.

- In a script, use `cd "$(ggw cd <name>)"`.
- For an interactive shell, `ggw shell-init <bash|zsh|fish>` prints a `ggw()` shell function that wraps the binary and turns `ggw cd` into a real chdir, plus completions. It is evaluated from the user's shell config; it is not a `gcd` alias.

## Produce automation-friendly output

Use `--json` whenever the output is parsed. Parse standard output only; never parse the styled tables, spinners, or prompts.

- `--json` suppresses prompts. Where a prompt would have selected between several matches — `cd`, `delete`, and `exec` with an ambiguous or missing name — the command fails instead of guessing. Pass a name specific enough to resolve to exactly one worktree.
- `exec` does not support `--json` at all.
- On failure, ggw emits a single `{"error": "..."}` object and exits non-zero.
- `ggw skills install` reports each destination in an `installations` array; a per-destination failure appears in that item's `error` field and does **not** change the exit code.
