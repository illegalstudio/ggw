# Configurable worktrees directory — design

Date: 2026-06-07
Status: Approved (pending spec review)

## Problem

ggw stores every worktree under a base directory derived automatically from the
environment: `$XDG_DATA_HOME/worktrees`, falling back to
`~/.local/share/worktrees`. There is no way for a user to relocate that base
(e.g. to `~/Worktrees` or a dedicated volume) without exporting
`XDG_DATA_HOME`, which is a blunt, global instrument.

We want an optional per-user configuration file that sets the worktrees base
directory. When set, it overrides the default derivation. We also want a
`ggw init` command that materializes a global config file seeded with the
current system's default path, ready to edit.

The configuration approach (location, format, how it is read, and the `init`
command) mirrors the sister project `../ggg` for consistency across the two
tools.

## Decisions (locked)

- **Config wins over `XDG_DATA_HOME`.** When the config file sets `base_dir`,
  it is used directly; `XDG_DATA_HOME` only matters when the config is absent or
  does not set the key.
- **File:** `~/.config/ggw/config.yaml`, key `base_dir` — same filename pattern,
  location, and (for `base_dir`) key name as ggg, for maximum coherence between
  the twin tools.
- **Read with `viper`**, written from a commented template string — exactly as
  ggg does. This adds `github.com/spf13/viper` as a direct dependency
  (ggw is otherwise lean: cobra, huh, lipgloss). Accepted for ggg parity.
- **Wiring approach (A):** make `worktree.WorktreesBase()` config-aware at the
  single chokepoint, rather than injecting an override from the CLI layer.

## Semantics of `base_dir`

When `base_dir` is set, a worktree for `(org, repo, slug)` lives at:

```
<base_dir>/<org>/<repo>/<branch-slug>
```

The `base_dir` value **is** the worktrees base — no `/worktrees` suffix is
appended. (The `worktrees` suffix exists only in the *default* derivation,
because `~/.local/share` is a shared XDG data directory and ggw carves out a
`worktrees` subdir within it. An explicit `base_dir` is already dedicated.)

A leading `~/` (or a bare `~`) in `base_dir` is expanded to the user's home
directory. No other expansion (env vars, etc.) is performed.

## Effective resolution order

```
config base_dir  →  $XDG_DATA_HOME/worktrees  →  ~/.local/share/worktrees
```

## Architecture

### New package: `internal/config`

Leaf utility package. Imports `viper`, `os`, `path/filepath`, `errors`,
`io/fs`. Imports nothing from `internal/worktree` (no cycle).

```go
type Config struct {
    BaseDir string `mapstructure:"base_dir"`
}

// ConfigPath returns ~/.config/ggw/config.yaml.
func ConfigPath() (string, error)

// Load reads and parses the config via viper, expanding ~ in BaseDir.
// A missing file yields an error for which errors.Is(err, fs.ErrNotExist) is true.
func Load() (*Config, error)

// BaseDir returns the configured, ~-expanded base dir and whether it is set.
//   - no config file / empty base_dir -> ("", false, nil)
//   - base_dir set                     -> (absPath, true, nil)
//   - malformed config                 -> ("", false, err)  (fails loudly)
func BaseDir() (string, bool, error)

// WriteDefault writes a commented template seeded with seedBaseDir,
// creating ~/.config/ggw as needed. Does not overwrite (caller checks existence).
func WriteDefault(path, seedBaseDir string) error
```

Design notes:
- `Load()` does **not** memoize across calls (no process-global cache). This
  keeps unit tests that override `HOME` per-test correct. Per-command file I/O
  cost is negligible (small YAML, read a handful of times per invocation; the
  CLI already caches the resolved base once per process via the `sync.Once` in
  `internal/cli/helpers.go`).
- A broken/unparseable config surfaces the error to the user rather than
  silently falling back to the default — a misconfigured base should not
  silently scatter worktrees in the default location.

### Modified: `internal/worktree/path.go`

`WorktreesBase()` consults config first:

```go
func WorktreesBase() (string, error) {
    if dir, ok, err := config.BaseDir(); err != nil {
        return "", err
    } else if ok {
        return dir, nil // config wins; used directly, no /worktrees suffix
    }
    base := os.Getenv("XDG_DATA_HOME")
    if base == "" {
        home, err := os.UserHomeDir()
        if err != nil {
            return "", fmt.Errorf("cannot resolve home directory: %w", err)
        }
        base = filepath.Join(home, ".local", "share")
    }
    return filepath.Join(base, "worktrees"), nil
}
```

`WorktreePath()` is unchanged — it builds on `WorktreesBase()`. Because every
consumer (`create`, `pr`, and the CLI's `isExternalPath` / `compactPath` via
`resolveWorktreesBase`) routes through `WorktreesBase()`, they all honor the
config automatically with no further changes.

### New command: `ggw init` (`internal/cli/init.go`)

Mirrors ggg's `init`:

1. `path, err := config.ConfigPath()` (return on error).
2. If `path` already exists → error: `config file already exists at <path>`.
3. Compute the system default base via `worktree.WorktreesBase()`. At this
   moment no config file exists, so it returns the real default
   (`~/.local/share/worktrees` or the `XDG_DATA_HOME`-based path). Collapse
   `$HOME` → `~` for readability using the existing `displayPath` helper.
4. `config.WriteDefault(path, seed)`.
5. `--json` output `{"created": true, "path": path}` (via `maybeJSON`), matching
   every other ggw command; otherwise a success line styled like ggg's.

`init` is therefore behavior-preserving: it writes the current default
explicitly, which the user can then edit.

Generated file:

```yaml
# GGW configuration
#
# base_dir: directory under which all worktrees live, nested as
# <base_dir>/<org>/<repo>/<branch-slug>. A leading ~ is expanded to $HOME.
base_dir: ~/.local/share/worktrees
```

(The seeded `base_dir` value is this system's resolved default, not a hardcoded
constant.)

### Modified: `internal/cli/root.go`

Add a command group `GroupConfig = "config"` with title `"Configuration:"`, and
register `initCmd` under it.

## Testing (TDD)

`internal/config/config_test.go` (temp `HOME` via `t.Setenv`):
- `ConfigPath` returns `<home>/.config/ggw/config.yaml`.
- `Load` parses a written file's `base_dir`; missing file → `fs.ErrNotExist`.
- `BaseDir`: unset when no file; unset when key empty; set + `~`-expanded when
  present; error on malformed YAML.
- `WriteDefault` creates the dir and writes the seeded value; produced file is
  re-loadable.

`internal/worktree/path_test.go` (additions; existing tests stay green):
- Config `base_dir` wins over `XDG_DATA_HOME` (set both; expect config value,
  used directly with no `/worktrees` suffix).
- Falls back to `XDG_DATA_HOME`/home default when no config file exists.

`internal/cli` (init + e2e):
- `ggw init` creates the file seeded with the default; errors when it already
  exists; `--json` shape.
- One e2e: with `base_dir` configured, `ggw create <branch>` places the worktree
  under `<base_dir>/<org>/<repo>/<slug>` and `ggw list` shows it as internal
  (compacted `[...]`, not `[external]`).

## Files

- New: `internal/config/config.go`, `internal/config/config_test.go`,
  `internal/cli/init.go`.
- Edit: `internal/worktree/path.go`, `internal/worktree/path_test.go`,
  `internal/cli/root.go`, CLI/e2e tests.
- Docs: `README.md` (config section + `ggw init`), `docs/commands.md` (the
  `init` command), and a new `docs/configuration.md`.
- Deps: add `github.com/spf13/viper`.

## Out of scope (YAGNI)

- A `ggw config` show/get/set command (ggg has one; not requested here).
- Per-repo overrides, multiple config keys, `XDG_CONFIG_HOME` support.
- Migration/moving of existing worktrees when `base_dir` changes.
