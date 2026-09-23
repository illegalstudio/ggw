<h1 align="center">GGW</h1>

<p align="center">
  <em>Git worktrees, ergonomic.</em>
</p>

<p align="center">
  <a href="https://github.com/illegalstudio/ggw/stargazers"><img src="https://img.shields.io/github/stars/illegalstudio/ggw?style=flat-square&logo=github&logoColor=white&label=stars&color=00ADD8" alt="Stars"></a>
  <a href="https://github.com/illegalstudio/ggw/releases"><img src="https://img.shields.io/github/downloads/illegalstudio/ggw/total?style=flat-square&logo=github&logoColor=white&label=downloads&color=00ADD8" alt="Downloads"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/illegalstudio/ggw?style=flat-square&color=00ADD8" alt="License: MIT"></a>
  <a href="https://x.com/nahime0"><img src="https://img.shields.io/badge/Follow-%40nahime0-00ADD8?style=flat-square&logo=x&logoColor=white" alt="Follow @nahime0 on X"></a>
</p>

<p align="center">
  <strong>Predictable paths &middot; Real shell <code>cd</code> &middot; Copy-on-write workspaces &middot; GitHub PR worktrees &middot; Single Go binary</strong>
</p>

<p align="center">
  GGW stores every workspace of every repo in one predictable location, derived from the repo's <code>origin</code> remote and the branch name — so you always know where your work lives and can jump between them with a real shell <code>cd</code>. A workspace is a git worktree, or a copy-on-write snapshot that brings your untracked files along for free.
</p>

<p align="center">
  <img src="demo/list-create-cd.gif" alt="ggw demo: list, create and cd" width="640">
</p>

---

Workspaces are stored under a single predictable path, derived from the repo's
`origin` remote and the branch name:

```
~/.local/share/worktrees/<org>/<repo>/<branch-slug>/
```

For example, in a repo whose `origin` is `git@github.com:acme/api.git`:

| Branch          | Workspace path                                          |
|-----------------|--------------------------------------------------------|
| `feature/login` | `~/.local/share/worktrees/acme/api/feature-login/`     |
| `hotfix-123`    | `~/.local/share/worktrees/acme/api/hotfix-123/`        |
| `BugFix/User A` | `~/.local/share/worktrees/acme/api/bugfix-user-a/`     |

`XDG_DATA_HOME` is respected: if set, workspaces live under
`$XDG_DATA_HOME/worktrees/...`.

The branch name is **not** slugified — it is passed unchanged to git.
Only the directory name is slugified.

## Two kinds of workspace

Both kinds live in the same place and every command treats them alike.

|                         | `worktree` (default)          | `cow`                                     |
|-------------------------|-------------------------------|-------------------------------------------|
| What it is              | a git worktree                | a copy-on-write snapshot of the repository |
| Untracked files         | not carried over              | **all of them**, at no disk cost           |
| `node_modules`, `.env`  | reproduced by `.ggw.yaml`     | already there                              |
| Branch                  | lives in the repository       | lives only in the snapshot                 |
| Same branch twice       | git refuses                   | allowed (see `--as`)                       |
| Requirements            | none                          | btrfs, XFS with `reflink=1`, bcachefs, APFS |

A copy-on-write workspace shares its data blocks with the original until one
side writes, so snapshotting a repository with two gigabytes of `node_modules`
costs milliseconds and no disk space. The trade-off is that it is an independent
repository: commits made in it reach the original only once you push, and
deleting it deletes its branch. ggw refuses to delete one that holds work
existing nowhere else.

```bash
ggw create --cow feature/login   # snapshot, untracked files included
ggw create --wt  feature/login   # git worktree
ggw create       feature/login   # whichever `mode` says (default: worktree)
```

If the filesystem cannot share blocks, `--cow` **fails** — it never falls back
to duplicating gigabytes behind your back.

## Install

### Homebrew

```bash
brew install illegalstudio/tap/ggw
```

### mise

```bash
mise use -g github:illegalstudio/ggw   # install globally
mise use github:illegalstudio/ggw      # or pin it in the project's mise.toml
```

ggw is not in the aqua registry, so the `github:` prefix is required. mise
picks the right release archive for your OS/arch automatically; pin a version
with `github:illegalstudio/ggw@0.3.3`.

### Go

```bash
go install github.com/illegalstudio/ggw/cmd/ggw@latest
```

### From source

```bash
git clone https://github.com/illegalstudio/ggw.git
cd ggw
make && make install
```

### AI agent skill

ggw bundles an Agent Skills-compatible skill that teaches AI agents how to drive
the CLI safely. Install it for the current user after installing or upgrading ggw:

```bash
ggw skills install
```

Both `~/.agents/skills/ggw` (Codex and compatible hosts) and
`~/.claude/skills/ggw` (Claude Code) are preselected. Reinstalling is safe: an
unmodified copy is updated in place, and local edits are protected until you pass
`--force`. When an installed skill falls out of sync with the binary, ggw says
so on stderr after interactive commands; `ggw skills verify` checks on demand,
and `suppress_skills_notice: true` in the config silences the reminder. See
[Commands](docs/commands.md#ggw-skills-install) for the full reference.

## Usage

```bash
# from inside a repo:
ggw list                              # show all workspaces of the current repo
ggw list --json                       # machine-readable output
ggw create feature/login              # create workspace at .../<org>/<repo>/feature-login/
                                      # creates the branch from HEAD if it does not exist
ggw create                            # random name (e.g. intelligent-elephant)
ggw create fix --from main            # create branch from a specific base
ggw create feature/login --cow        # copy-on-write snapshot instead of a worktree
ggw create feature/login --as review  # choose the directory name
ggw pr 123                            # create a tracked workspace for GitHub PR #123 (requires gh)
ggw cd feature/login                  # cd into a workspace (needs shell integration, see below)
ggw cd                                # interactive selector
ggw exec feature/login -- npm install # run a command inside a workspace
ggw delete feature/login              # remove a workspace and its branch (prompts to confirm)
ggw delete feature/login --without-branch --force  # keep the branch, skip confirm

# Print the installed version
ggw --version
```

To make every new worktree immediately ready to use, add a `.ggw.yaml` at your
repository root (a copy-on-write workspace needs far less of it — it already
has your untracked files):

```bash
ggw project-init   # scaffold .ggw.yaml in the current repo
```

`.ggw.yaml` can copy files (e.g. `.env`), create symlinks (e.g. `node_modules`),
and run setup commands after each `ggw create` or `ggw pr`. Pass `--bare` to
skip provisioning for a single run. In `cow` mode the `copy` and `symlink` steps
are skipped — the snapshot already contains them — and only `post_create` runs.
See [Project Provisioning](docs/configuration.md#project-provisioning-ggwyaml).

`ggw pr <id>` uses [GitHub CLI](https://cli.github.com/) to check out the PR
branch, so the created workspace keeps the tracking configuration that allows
`git push` when GitHub permits pushing to the PR branch.

## Shell integration

`ggw cd` needs a shell wrapper to actually change your shell's directory.
Add to your shell config (once):

```bash
eval "$(ggw shell-init bash)"   # in ~/.bashrc
eval "$(ggw shell-init zsh)"    # in ~/.zshrc
ggw shell-init fish | source    # in ~/.config/fish/config.fish
```

The wrapper intercepts `ggw cd` and turns it into a real `cd`. Every other
subcommand (`list`, `create`, ...) passes through unchanged.

Without the wrapper, `ggw cd <name>` simply prints the workspace path on
stdout — useful for `cd "$(ggw cd foo)"` or piping into other tools.

## Configuration

By default ggw derives the base directory from the environment
(`$XDG_DATA_HOME/worktrees`, or `~/.local/share/worktrees`). To store workspaces
somewhere else, create a config file:

```bash
ggw init   # writes ~/.config/ggw/config.yaml, seeded with the current default
```

Then edit it:

```yaml
# ~/.config/ggw/config.yaml
base_dir: ~/Worktrees
mode: cow          # or: worktree (the default)
```

With this, a workspace for `acme/api` on branch `feature/login` lives at
`~/Worktrees/acme/api/feature-login/`. A `base_dir` set here **overrides**
`XDG_DATA_HOME`. A leading `~` is expanded to your home directory.

`mode` decides what `ggw create` and `ggw pr` make by default. Override it for a
single run with `--cow` or `--wt`, or for a whole shell with `GGW_MODE`.

> **Copy-on-write needs both paths on one filesystem.** Reflinks cannot cross
> mount points, so `base_dir` has to live on the same filesystem as your repos.

## Docs

- [Commands Reference](docs/commands.md)
- [Configuration](docs/configuration.md)
- [Shell Integration](docs/shell-integration.md)
- [Storage Layout](docs/storage.md)

## See also

- [`ggg`](https://github.com/illegalstudio/ggg) — the sister project that inspired `ggw`.

## Status

All commands are operative, including the config file, copy-on-write
workspaces, project provisioning (`.ggw.yaml`), tab completion, and releases.
See [`ROADMAP.md`](ROADMAP.md) for the remaining backlog.

## License

MIT — see [`LICENSE`](LICENSE).
