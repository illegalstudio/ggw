# Storage Layout

`ggw` derives the workspace location from the current repository's `origin` remote and the branch name. A [config file](configuration.md) can override the base directory.

## Default Location

Workspaces live under:

```text
~/.local/share/worktrees/<org>/<repo>/<branch-slug>/
```

For a repository with `origin` set to `git@github.com:acme/api.git`:

| Branch | Workspace path |
|--------|---------------|
| `feature/login` | `~/.local/share/worktrees/acme/api/feature-login/` |
| `hotfix-123` | `~/.local/share/worktrees/acme/api/hotfix-123/` |
| `BugFix/User A` | `~/.local/share/worktrees/acme/api/bugfix-user-a/` |

## `XDG_DATA_HOME`

If `XDG_DATA_HOME` is set, `ggw` stores workspaces below that directory instead:

```text
$XDG_DATA_HOME/worktrees/<org>/<repo>/<branch-slug>/
```

## One Namespace, Two Kinds

Git worktrees and copy-on-write workspaces share this directory, so a slug
belongs to whichever kind claimed it first. They are told apart on disk:

| | `.git` | ggw marker |
|---|---|---|
| worktree | a *file* (`gitdir: …`) | none — git tracks it |
| cow      | a *directory*          | `.git/ggw.json` |

`git worktree list` enumerates the worktrees. Copy-on-write workspaces are
invisible to git, so ggw finds them by scanning `<base>/<org>/<repo>/` for that
marker. Nothing else is stored: the layout *is* the index, so there is no
registry to drift out of sync.

The marker also records the repository the snapshot came from. That is how
`ggw list`, `ggw cd` and `ggw create` still see the whole family when run from
*inside* a snapshot — git in there answers only about the snapshot itself.

```json
{
  "kind": "cow",
  "version": 1,
  "source_repo": "/home/you/code/api",
  "org": "acme",
  "repo": "api",
  "branch": "feature/login",
  "backend": "reflink",
  "created_at": "2026-09-14T12:00:00Z"
}
```

## Directory Names

The directory name is the slugified branch, unless `--as` says otherwise:

```bash
ggw create feature/login             # .../acme/api/feature-login/
ggw create feature/login --as review # .../acme/api/review/
```

`--as` is what lets two copy-on-write workspaces share one branch — git refuses
to check out the same branch in two worktrees, but two independent repositories
have no such rule.

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

`ggw create` needs the current repository to have an `origin` remote so it can derive the `<org>/<repo>` path segment. Without one, `ggw list` still shows the repository's git worktrees — there is simply no layout directory to scan for copy-on-write workspaces.

Configure one with:

```bash
git remote add origin git@github.com:acme/api.git
```
