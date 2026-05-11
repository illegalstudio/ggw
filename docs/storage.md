# Storage Layout

`ggw` does not use a config file today. It derives the worktree location from the current repository's `origin` remote and the branch name.

## Default Location

Worktrees live under:

```text
~/.local/share/worktrees/<org>/<repo>/<branch-slug>/
```

For a repository with `origin` set to `git@github.com:acme/api.git`:

| Branch | Worktree path |
|--------|---------------|
| `feature/login` | `~/.local/share/worktrees/acme/api/feature-login/` |
| `hotfix-123` | `~/.local/share/worktrees/acme/api/hotfix-123/` |
| `BugFix/User A` | `~/.local/share/worktrees/acme/api/bugfix-user-a/` |

## `XDG_DATA_HOME`

If `XDG_DATA_HOME` is set, `ggw` stores worktrees below that directory instead:

```text
$XDG_DATA_HOME/worktrees/<org>/<repo>/<branch-slug>/
```

## Branch Slugs

The branch name is not changed before it is passed to git. Slugification is only used for the directory name.

Slug rules:

- uppercase letters become lowercase;
- letters, numbers, and underscores are preserved;
- every other run of characters becomes a single `-`;
- leading and trailing `-` characters are removed.

Examples:

| Branch | Slug |
|--------|------|
| `feature/login` | `feature-login` |
| `BugFix/User Auth` | `bugfix-user-auth` |
| `feat/can't-do` | `feat-can-t-do` |

## Remote Requirement

`ggw create` needs the current repository to have an `origin` remote so it can derive the `<org>/<repo>` path segment.

Configure one with:

```bash
git remote add origin git@github.com:acme/api.git
```
