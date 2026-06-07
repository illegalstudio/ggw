# Configurable Worktrees Directory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users set the worktrees base directory via `~/.config/ggw/config.yaml`, overriding the auto-derived default, with a `ggw init` command that seeds the file with the current system default.

**Architecture:** A new leaf package `internal/config` reads the YAML config with viper (mirroring the sister project `../ggg`). `worktree.WorktreesBase()` — the single chokepoint every command routes through — consults `config.BaseDir()` first, so a configured `base_dir` wins over `XDG_DATA_HOME`. A new `ggw init` command writes the config seeded with this system's resolved default.

**Tech Stack:** Go 1.26, cobra, lipgloss, `github.com/spf13/viper` (new dep), Go's `testing`.

**Spec:** `docs/superpowers/specs/2026-06-07-configurable-worktrees-dir-design.md`

---

## File Structure

- **Create** `internal/config/config.go` — config location, load/parse (viper), `BaseDir()` accessor, `WriteDefault()`. One responsibility: reading/writing ggw's config.
- **Create** `internal/config/config_test.go` — unit tests for the above.
- **Create** `internal/cli/init.go` — the `ggw init` command.
- **Modify** `internal/worktree/path.go` — make `WorktreesBase()` config-aware.
- **Modify** `internal/worktree/path_test.go` — new precedence tests; isolate `HOME` in the existing XDG test.
- **Modify** `internal/cli/root.go` — add the `config` command group.
- **Modify** `tests/e2e_test.go` — e2e for `init` and for `create` honoring `base_dir`.
- **Modify** `README.md`, `docs/commands.md`, **create** `docs/configuration.md` — docs.

Resolution precedence (effective): `config base_dir` → `$XDG_DATA_HOME/worktrees` → `~/.local/share/worktrees`.

---

## Task 1: `config` package — path, load, BaseDir accessor

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Modify: `go.mod`, `go.sum` (add viper)

- [ ] **Step 1: Add the viper dependency**

Run (from repo root):
```bash
go get github.com/spf13/viper@v1.19.0
```
Expected: `go.mod` gains `github.com/spf13/viper v1.19.0` and indirect deps; `go.sum` updated.

- [ ] **Step 2: Write the failing tests**

Create `internal/config/config_test.go`:
```go
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, home, content string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "ggw")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := ConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, ".config", "ggw", "config.yaml")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoadMissingFileIsNotExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := Load(); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestLoadParsesAndExpandsBaseDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeConfig(t, home, "base_dir: ~/code/wt\n")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, "code", "wt")
	if cfg.BaseDir != want {
		t.Fatalf("got %q, want %q", cfg.BaseDir, want)
	}
}

func TestBaseDirUnsetWhenNoFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, ok, err := BaseDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || dir != "" {
		t.Fatalf("expected unset, got (%q, %v)", dir, ok)
	}
}

func TestBaseDirUnsetWhenEmptyKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeConfig(t, home, "base_dir: \"\"\n")
	dir, ok, err := BaseDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || dir != "" {
		t.Fatalf("expected unset, got (%q, %v)", dir, ok)
	}
}

func TestBaseDirSetAndExpanded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeConfig(t, home, "base_dir: ~/Worktrees\n")
	dir, ok, err := BaseDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, "Worktrees")
	if !ok || dir != want {
		t.Fatalf("got (%q, %v), want (%q, true)", dir, ok, want)
	}
}

func TestBaseDirErrorsOnMalformedYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeConfig(t, home, "base_dir: [unclosed\n")
	if _, _, err := BaseDir(); err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/config/...`
Expected: FAIL — `undefined: ConfigPath`, `undefined: Load`, `undefined: BaseDir`.

- [ ] **Step 4: Write the implementation**

Create `internal/config/config.go`:
```go
// Package config reads ggw's user configuration from ~/.config/ggw/config.yaml.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config is the ggw configuration.
type Config struct {
	// BaseDir is the directory under which all worktrees live, nested as
	// <BaseDir>/<org>/<repo>/<branch-slug>. Empty means "use the default".
	BaseDir string `mapstructure:"base_dir"`
}

// ConfigPath returns the path to the ggw config file: ~/.config/ggw/config.yaml.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "ggw", "config.yaml"), nil
}

// Load reads and parses the config file via viper, expanding a leading ~ in
// BaseDir. A missing file yields an error for which errors.Is(err, fs.ErrNotExist)
// is true, so callers can treat "no config" as "use defaults".
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("config file %s: %w", path, fs.ErrNotExist)
		}
		return nil, fmt.Errorf("cannot stat config file: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file: %w", err)
	}

	expanded, err := expandTilde(cfg.BaseDir)
	if err != nil {
		return nil, err
	}
	cfg.BaseDir = expanded
	return &cfg, nil
}

// BaseDir returns the configured worktrees base directory (~ expanded) and
// whether it is set. A missing config file or an empty base_dir yields
// ("", false, nil); a malformed config yields ("", false, err).
func BaseDir() (string, bool, error) {
	cfg, err := Load()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	if cfg.BaseDir == "" {
		return "", false, nil
	}
	return cfg.BaseDir, true, nil
}

// expandTilde expands a leading ~ or ~/ in p to the user's home directory.
func expandTilde(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot expand ~ in path %q: %w", p, err)
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, p[2:]), nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/config/...`
Expected: PASS (all 7 tests).

- [ ] **Step 6: Tidy and verify the module builds**

Run: `go mod tidy && go build ./...`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): read worktrees base_dir from ~/.config/ggw/config.yaml"
```

---

## Task 2: `config.WriteDefault`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/config/config_test.go`:
```go
func TestWriteDefaultCreatesSeededReloadableFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	// The ~/.config/ggw directory does not exist yet; WriteDefault must create it.
	if err := WriteDefault(path, "~/.local/share/worktrees"); err != nil {
		t.Fatalf("WriteDefault: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load after WriteDefault: %v", err)
	}
	want := filepath.Join(home, ".local", "share", "worktrees")
	if cfg.BaseDir != want {
		t.Fatalf("got %q, want %q", cfg.BaseDir, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestWriteDefault`
Expected: FAIL — `undefined: WriteDefault`.

- [ ] **Step 3: Add the implementation**

Append to `internal/config/config.go`:
```go
// WriteDefault writes a commented config template to path, seeding base_dir with
// seedBaseDir. It creates the parent directory (~/.config/ggw) as needed. It does
// not check for an existing file — callers must guard against overwriting.
func WriteDefault(path, seedBaseDir string) error {
	content := fmt.Sprintf(`# GGW configuration
#
# base_dir: directory under which all worktrees live, nested as
# <base_dir>/<org>/<repo>/<branch-slug>. A leading ~ is expanded to $HOME.
base_dir: %s
`, seedBaseDir)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/...`
Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add WriteDefault to seed the config file"
```

---

## Task 3: Make `worktree.WorktreesBase()` config-aware

**Files:**
- Modify: `internal/worktree/path.go:98-110`
- Modify: `internal/worktree/path_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/worktree/path_test.go` (new functions):
```go
func TestWorktreePathConfigBaseDirWinsOverXDG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")

	cfgDir := filepath.Join(home, ".config", "ggw")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(home, "my-worktrees")
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"),
		[]byte("base_dir: "+custom+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := WorktreePath("acme", "api", "feature-login")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(custom, "acme", "api", "feature-login")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWorktreePathNoConfigFallsBackToXDG(t *testing.T) {
	home := t.TempDir() // clean home: no config file
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")

	got, err := WorktreePath("acme", "api", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/tmp/xdg", "worktrees", "acme", "api", "x")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
```

Then add the `os` import to the test file's import block (it currently imports only `path/filepath` and `testing`):
```go
import (
	"os"
	"path/filepath"
	"testing"
)
```

Also isolate `HOME` in the existing `TestWorktreePathHonorsXDG` so a developer's real `~/.config/ggw/config.yaml` cannot leak in. Change its body's first lines from:
```go
func TestWorktreePathHonorsXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
```
to:
```go
func TestWorktreePathHonorsXDG(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
```

- [ ] **Step 2: Run tests to verify the new ones fail**

Run: `go test ./internal/worktree/ -run 'TestWorktreePathConfigBaseDirWinsOverXDG|TestWorktreePathNoConfigFallsBackToXDG'`
Expected: FAIL — `TestWorktreePathConfigBaseDirWinsOverXDG` returns `/tmp/xdg/worktrees/...` instead of the custom dir (config not yet consulted).

- [ ] **Step 3: Make `WorktreesBase` consult config**

In `internal/worktree/path.go`, add the import:
```go
"github.com/illegalstudio/ggw/internal/config"
```

Replace the body of `WorktreesBase` (currently `internal/worktree/path.go:100-110`) with:
```go
func WorktreesBase() (string, error) {
	if dir, ok, err := config.BaseDir(); err != nil {
		return "", err
	} else if ok {
		// Config wins: the configured directory is the worktrees base, used
		// directly (no /worktrees suffix — that belongs to the default only).
		return dir, nil
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
(Leave the doc comment above it; update its second sentence to note config precedence:)
```go
// WorktreesBase returns the directory under which all ggw-managed worktrees
// live. A base_dir set in ~/.config/ggw/config.yaml wins; otherwise
// $XDG_DATA_HOME/worktrees, or ~/.local/share/worktrees as a fallback.
```

- [ ] **Step 4: Run the full worktree + config suites**

Run: `go test ./internal/worktree/... ./internal/config/...`
Expected: PASS (new precedence tests pass; existing `TestWorktreePathHonorsXDG`, `TestWorktreePathFallsBackToHome`, `TestWorktreePathRejectsEmptyComponents` still pass).

- [ ] **Step 5: Commit**

```bash
git add internal/worktree/path.go internal/worktree/path_test.go
git commit -m "feat(worktree): honor config base_dir over XDG in WorktreesBase"
```

---

## Task 4: `ggw init` command + config command group

**Files:**
- Create: `internal/cli/init.go`
- Modify: `internal/cli/root.go:14-17` (const block) and `internal/cli/root.go:42-45` (AddGroup)
- Test: `tests/e2e_test.go`

- [ ] **Step 1: Write the failing e2e tests**

Append to `tests/e2e_test.go`:
```go
func TestCLIInitCreatesConfig(t *testing.T) {
	home := setupHome(t)
	cwd := t.TempDir()

	out, err := runGGW(t, home, cwd, "init")
	if err != nil {
		t.Fatalf("ggw init failed: %v\n%s", err, out)
	}

	cfgPath := filepath.Join(home, ".config", "ggw", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config not created: %v", err)
	}
	// childEnv sets XDG_DATA_HOME=$home/.local/share, so the seeded default,
	// with $HOME collapsed to ~, is ~/.local/share/worktrees.
	if !strings.Contains(string(data), "base_dir: ~/.local/share/worktrees") {
		t.Fatalf("unexpected config contents:\n%s", data)
	}

	// Running again must fail because the file already exists.
	out, err = runGGW(t, home, cwd, "init")
	if err == nil {
		t.Fatalf("expected ggw init to fail when config exists; output:\n%s", out)
	}
	if !strings.Contains(out, "already exists") {
		t.Fatalf("unexpected error output:\n%s", out)
	}
}

func TestCLIInitJSON(t *testing.T) {
	home := setupHome(t)
	cwd := t.TempDir()

	out, err := runGGW(t, home, cwd, "--json", "init")
	if err != nil {
		t.Fatalf("ggw --json init failed: %v\n%s", err, out)
	}
	var payload struct {
		Created bool   `json:"created"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("init JSON invalid: %v\n%s", err, out)
	}
	if !payload.Created || !strings.HasSuffix(payload.Path, filepath.Join(".config", "ggw", "config.yaml")) {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
```
(`os`, `json`, `strings`, `filepath` are already imported in `tests/e2e_test.go`.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./tests/ -run 'TestCLIInit'`
Expected: FAIL — `ggw init` is an unknown command (non-zero exit), so the success path errors out.

- [ ] **Step 3: Add the `config` command group**

In `internal/cli/root.go`, change the const block:
```go
const (
	GroupWorktree = "worktree"
	GroupShell    = "shell"
)
```
to:
```go
const (
	GroupWorktree = "worktree"
	GroupShell    = "shell"
	GroupConfig   = "config"
)
```

And change the `AddGroup` call:
```go
	rootCmd.AddGroup(
		&cobra.Group{ID: GroupWorktree, Title: "Worktree Operations:"},
		&cobra.Group{ID: GroupShell, Title: "Shell Integration:"},
	)
```
to:
```go
	rootCmd.AddGroup(
		&cobra.Group{ID: GroupWorktree, Title: "Worktree Operations:"},
		&cobra.Group{ID: GroupShell, Title: "Shell Integration:"},
		&cobra.Group{ID: GroupConfig, Title: "Configuration:"},
	)
```

- [ ] **Step 4: Create the `init` command**

Create `internal/cli/init.go`:
```go
package cli

import (
	"fmt"
	"os"

	"github.com/illegalstudio/ggw/internal/config"
	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:     "init",
	Short:   "Create the global config file seeded with this system's default worktrees directory",
	GroupID: GroupConfig,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ConfigPath()
		if err != nil {
			return err
		}
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists at %s", path)
		}

		// No config exists yet, so WorktreesBase returns this system's default.
		base, err := worktree.WorktreesBase()
		if err != nil {
			return err
		}
		seed := displayPath(base) // collapse $HOME -> ~ for a portable, readable value

		if err := config.WriteDefault(path, seed); err != nil {
			return err
		}

		if done, err := maybeJSON(map[string]any{"created": true, "path": path}); done {
			return err
		}

		fmt.Println(ui.Success.Render("✓") + " Config file created at " + ui.Path.Render(path))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./tests/ -run 'TestCLIInit'`
Expected: PASS (both init tests).

- [ ] **Step 6: Commit**

```bash
git add internal/cli/init.go internal/cli/root.go tests/e2e_test.go
git commit -m "feat(cli): add ggw init to create the config file"
```

---

## Task 5: e2e — `create` honors `base_dir`

**Files:**
- Test: `tests/e2e_test.go`

- [ ] **Step 1: Write the failing test**

Append to `tests/e2e_test.go`:
```go
func TestCLICreateHonorsConfigBaseDir(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	// Configure a custom base dir. childEnv also sets XDG_DATA_HOME, so this
	// asserts config wins over XDG.
	base := filepath.Join(home, "custom-worktrees")
	cfgDir := filepath.Join(home, ".config", "ggw")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"),
		[]byte("base_dir: "+base+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runGGW(t, home, repo, "create", "feature/login")
	if err != nil {
		t.Fatalf("ggw create failed: %v\n%s", err, out)
	}
	want := filepath.Join(base, "acme", "api", "feature-login")
	if _, err := os.Stat(filepath.Join(want, ".git")); err != nil {
		t.Fatalf("worktree not created at %s: %v\noutput:\n%s", want, err, out)
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./tests/ -run TestCLICreateHonorsConfigBaseDir`
Expected: PASS (the wiring from Task 3 already makes `create` honor `base_dir`; this test locks in the end-to-end behavior).

- [ ] **Step 3: Run the entire test suite**

Run: `go test ./...`
Expected: PASS (all packages).

- [ ] **Step 4: Commit**

```bash
git add tests/e2e_test.go
git commit -m "test(e2e): cover create honoring configured base_dir"
```

---

## Task 6: Documentation

**Files:**
- Modify: `README.md`
- Create: `docs/configuration.md`
- Modify: `docs/commands.md`

- [ ] **Step 1: Add a Configuration section to the README**

In `README.md`, insert a new section immediately before the `## Docs` section:
```markdown
## Configuration

By default ggw derives the worktrees base directory from the environment
(`$XDG_DATA_HOME/worktrees`, or `~/.local/share/worktrees`). To store worktrees
somewhere else, create a config file:

```bash
ggw init   # writes ~/.config/ggw/config.yaml, seeded with the current default
```

Then edit `base_dir`:

```yaml
# ~/.config/ggw/config.yaml
base_dir: ~/Worktrees
```

With this, a worktree for `acme/api` on branch `feature/login` lives at
`~/Worktrees/acme/api/feature-login/`. A `base_dir` set here **overrides**
`XDG_DATA_HOME`. A leading `~` is expanded to your home directory.
```

Then add a bullet to the existing `## Docs` list:
```markdown
- [Configuration](docs/configuration.md)
```

- [ ] **Step 2: Create `docs/configuration.md`**

Create `docs/configuration.md`:
```markdown
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
```

- [ ] **Step 3: Document `ggw init` in the commands reference**

In `docs/commands.md`, add a new section (place it after the `ggw list` section, before `ggw create`, to keep config/setup commands near the top — or at the end of the command list if that better matches the file's ordering):
```markdown
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
```

- [ ] **Step 4: Verify docs render and links resolve**

Run: `grep -n "configuration.md" README.md docs/commands.md`
Expected: both files reference `configuration.md`; the new `docs/configuration.md` exists.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/configuration.md docs/commands.md
git commit -m "docs: document configurable worktrees directory and ggw init"
```

---

## Final verification

- [ ] **Run the full suite and build**

Run: `go build ./... && go test ./...`
Expected: build succeeds; all tests pass.

- [ ] **Manual smoke test (optional)**

```bash
go run ./cmd/ggw init        # in a throwaway HOME, e.g. HOME=$(mktemp -d)
cat "$HOME/.config/ggw/config.yaml"
```
Expected: file created with a `base_dir:` line.

---

## Self-Review notes

- **Spec coverage:** config location/format/read (Task 1), `BaseDir` accessor + precedence (Tasks 1, 3), `WriteDefault` (Task 2), config-wins-over-XDG (Tasks 3, 5), `ggw init` + seed-with-system-default + `--json` + no-overwrite (Task 4), command group (Task 4), docs (Task 6). All spec sections map to a task.
- **No placeholders:** every code/edit step shows the full code or the exact before/after.
- **Type consistency:** `Config.BaseDir`, `ConfigPath()`, `Load()`, `BaseDir()`, `WriteDefault(path, seed)` are referenced identically across Tasks 1–4.
