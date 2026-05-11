# ggw

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Follow @nahime0 on X](https://img.shields.io/badge/Follow%20@nahime0-black?logo=x)](https://x.com/nahime0)

Git worktrees, ergonomic. Stores all worktrees of all your repos in a single
predictable location, derived from the repo's `origin` remote and the branch
name.

```
~/.local/share/worktrees/<org>/<repo>/<branch-slug>/
```

For example, in a repo whose `origin` is `git@github.com:acme/api.git`:

| Branch          | Worktree path                                          |
|-----------------|--------------------------------------------------------|
| `feature/login` | `~/.local/share/worktrees/acme/api/feature-login/`     |
| `hotfix-123`    | `~/.local/share/worktrees/acme/api/hotfix-123/`        |
| `BugFix/User A` | `~/.local/share/worktrees/acme/api/bugfix-user-a/`     |

`XDG_DATA_HOME` is respected: if set, worktrees live under
`$XDG_DATA_HOME/worktrees/...`.

The branch name is **not** slugified — it is passed unchanged to git.
Only the directory name is slugified.

## Install

```bash
go install github.com/illegalstudio/ggw/cmd/ggw@latest
```

Or build from source:

```bash
git clone https://github.com/illegalstudio/ggw.git
cd ggw
make && make install
```

## Usage

```bash
# from inside a repo:
ggw list                              # show all worktrees of the current repo
ggw list --json                       # machine-readable output
ggw create feature/login              # create worktree at .../<org>/<repo>/feature-login/
                                      # creates the branch from HEAD if it does not exist
ggw create fix --from main            # create branch from a specific base
ggw cd feature/login                  # cd into a worktree (needs shell integration, see below)
ggw cd                                # interactive selector
ggw exec feature/login -- npm install # run a command inside a worktree
ggw delete feature/login              # remove a worktree and its branch (prompts to confirm)
ggw delete feature/login --without-branch --force  # keep the branch, skip confirm
```

## Shell integration

`ggw cd` needs a shell wrapper to actually change your shell's directory.
Add to your shell config (once):

```bash
eval "$(ggw init bash)"   # in ~/.bashrc
eval "$(ggw init zsh)"    # in ~/.zshrc
ggw init fish | source    # in ~/.config/fish/config.fish
```

The wrapper intercepts `ggw cd` and turns it into a real `cd`. Every other
subcommand (`list`, `create`, ...) passes through unchanged.

Without the wrapper, `ggw cd <name>` simply prints the worktree path on
stdout — useful for `cd "$(ggw cd foo)"` or piping into other tools.

## Docs

- [Commands Reference](docs/commands.md)
- [Shell Integration](docs/shell-integration.md)
- [Storage Layout](docs/storage.md)

## See also

- [`ggg`](https://github.com/illegalstudio/ggg) — the sister project that inspired `ggw`.

## Status

All commands are operative. See [`ROADMAP.md`](ROADMAP.md) for the
backlog (config file, post-create hooks, completions, releases).

## License

MIT — see [`LICENSE`](LICENSE).
