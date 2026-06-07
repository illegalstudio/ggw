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
