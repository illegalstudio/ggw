# Project Provisioning (`.ggw.yaml`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-project worktree provisioning via a `.ggw.yaml` file (copy files, symlink files, run post-create commands), created by a new `ggw project-init` command and applied transactionally on `ggw create` and `ggw pr`.

**Architecture:** A new isolated `internal/project` package loads `.ggw.yaml` and performs provisioning (copy → symlink → commands). The `create` and `pr` CLI commands call a shared `provisionWorktree` helper after the worktree is created; on any failure the command's deferred rollback removes the worktree with `git worktree remove --force` while keeping the branch. `worktree.Create` and the PR checkout logic are untouched.

**Tech Stack:** Go, spf13/cobra (CLI), spf13/viper (YAML config), standard library (`os`, `os/exec`, `path/filepath`, `io`).

## Global Constraints

- Config file name: `.ggw.yaml`, located at the **main worktree root** (resolved via `worktree.List(root)[0].Path`).
- `copy`/`symlink` sources are relative paths resolved from the main worktree; destination is the same relative path in the new worktree. Symlinks point at the **absolute** path in the main worktree.
- Provisioning order is fixed: copy → symlink → post_create.
- `post_create` commands run via `sh -c`, cwd = new worktree, environment inherited, sequential, **stop at first non-zero exit code**.
- Transactional: any failure removes the worktree (`git worktree remove --force`) and **keeps the branch**.
- Failure cases that trigger rollback: absolute/`..`-escaping path, missing source, pre-existing destination, command non-zero exit.
- A missing `.ggw.yaml` is **not** an error (no provisioning configured).
- `--bare` on `create` and `pr` skips all provisioning.
- Provisioning output streams to **stderr** (keeps stdout clean for `--json` and the `cd` shell wrapper).
- Commit messages: no `Co-Authored-By` line (user preference).

---

### Task 1: `internal/project` config — load & template

**Files:**
- Create: `internal/project/project.go`
- Test: `internal/project/project_test.go`

**Interfaces:**
- Consumes: nothing (new package).
- Produces:
  - `const ConfigFileName = ".ggw.yaml"`
  - `type Config struct { Copy []string; Symlink []string; PostCreate []string }`
  - `func Load(mainWorktreePath string) (*Config, bool, error)` — bool reports whether the file exists; missing file returns `(nil, false, nil)`.
  - `func WriteTemplate(path string, force bool) error` — writes the commented template to the full file `path`; errors if the file exists unless `force`.

- [ ] **Step 1: Write the failing tests**

Create `internal/project/project_test.go`:

```go
package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadMissingFileReportsNotExists(t *testing.T) {
	cfg, exists, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected exists=false for a missing config")
	}
	if cfg != nil {
		t.Fatalf("expected nil config, got %+v", cfg)
	}
}

func TestLoadParsesAllSections(t *testing.T) {
	dir := t.TempDir()
	content := "copy:\n  - .env\nsymlink:\n  - node_modules\n  - vendor\npost_create:\n  - composer install\n"
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, exists, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
	if !reflect.DeepEqual(cfg.Copy, []string{".env"}) {
		t.Fatalf("copy = %v", cfg.Copy)
	}
	if !reflect.DeepEqual(cfg.Symlink, []string{"node_modules", "vendor"}) {
		t.Fatalf("symlink = %v", cfg.Symlink)
	}
	if !reflect.DeepEqual(cfg.PostCreate, []string{"composer install"}) {
		t.Fatalf("post_create = %v", cfg.PostCreate)
	}
}

func TestWriteTemplateCreatesAndGuardsOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), ConfigFileName)

	if err := WriteTemplate(path, false); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	if !strings.Contains(string(data), "copy:") || !strings.Contains(string(data), "post_create:") {
		t.Fatalf("template missing sections:\n%s", data)
	}

	if err := WriteTemplate(path, false); err == nil {
		t.Fatal("expected error overwriting existing file without force")
	}
	if err := WriteTemplate(path, true); err != nil {
		t.Fatalf("force write failed: %v", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/project/`
Expected: FAIL — `undefined: Load`, `undefined: WriteTemplate`, `undefined: ConfigFileName`.

- [ ] **Step 3: Write the implementation**

Create `internal/project/project.go`:

```go
// Package project reads and applies a repository's .ggw.yaml provisioning file.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// ConfigFileName is the per-project provisioning file, read from the main
// worktree root.
const ConfigFileName = ".ggw.yaml"

// Config describes how to provision a freshly created worktree.
type Config struct {
	Copy       []string `mapstructure:"copy"`
	Symlink    []string `mapstructure:"symlink"`
	PostCreate []string `mapstructure:"post_create"`
}

// Load reads .ggw.yaml from mainWorktreePath. The bool reports whether the file
// exists; a missing file yields (nil, false, nil) so callers treat it as "no
// provisioning configured".
func Load(mainWorktreePath string) (*Config, bool, error) {
	path := filepath.Join(mainWorktreePath, ConfigFileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("cannot stat %s: %w", path, err)
	}

	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, true, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, true, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	return &cfg, true, nil
}

// WriteTemplate writes a commented .ggw.yaml template to path. It errors if the
// file already exists unless force is true.
func WriteTemplate(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}
	return os.WriteFile(path, []byte(template), 0o644)
}

const template = `# .ggw.yaml — per-project worktree provisioning.
#
# Paths are relative to the repository root. ` + "`copy`" + ` and ` + "`symlink`" + ` sources are
# taken from the main worktree; the destination is the same relative path in
# the newly created worktree.

# Files/directories copied (recursively) into each new worktree.
copy: []
  # - .env

# Files/directories symlinked into each new worktree. The symlink points at the
# absolute path in the main worktree.
symlink: []
  # - node_modules
  # - vendor

# Shell commands run (via ` + "`sh -c`" + `) inside each new worktree, in order, after
# copy and symlink. Execution stops at the first non-zero exit code.
post_create: []
  # - composer install
  # - npm ci
`
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/project/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/project/project.go internal/project/project_test.go
git commit -m "feat(project): load and template .ggw.yaml config"
```

---

### Task 2: `internal/project` provisioning engine

**Files:**
- Create: `internal/project/provision.go`
- Test: `internal/project/provision_test.go`

**Interfaces:**
- Consumes: `Config` (Task 1).
- Produces:
  - `type ProvisionOptions struct { MainPath string; DestPath string; Config *Config; Out io.Writer }`
  - `func Provision(opts ProvisionOptions) error` — runs copy → symlink → post_create; returns the first error encountered.

- [ ] **Step 1: Write the failing tests**

Create `internal/project/provision_test.go`:

```go
package project

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestProvisionCopiesFileAndDirectory(t *testing.T) {
	main, dest := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(main, ".env"), []byte("X=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(main, "cfg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "cfg", "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Provision(ProvisionOptions{
		MainPath: main, DestPath: dest,
		Config: &Config{Copy: []string{".env", "cfg"}}, Out: io.Discard,
	})
	if err != nil {
		t.Fatalf("provision failed: %v", err)
	}

	if got, _ := os.ReadFile(filepath.Join(dest, ".env")); string(got) != "X=1" {
		t.Fatalf(".env content = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "cfg", "a.txt")); string(got) != "a" {
		t.Fatalf("cfg/a.txt content = %q", got)
	}
}

func TestProvisionSymlinksToAbsoluteMainPath(t *testing.T) {
	main, dest := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(main, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := Provision(ProvisionOptions{
		MainPath: main, DestPath: dest,
		Config: &Config{Symlink: []string{"node_modules"}}, Out: io.Discard,
	})
	if err != nil {
		t.Fatalf("provision failed: %v", err)
	}

	link := filepath.Join(dest, "node_modules")
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected a symlink")
	}
	target, _ := os.Readlink(link)
	if target != filepath.Join(main, "node_modules") {
		t.Fatalf("symlink target = %q, want %q", target, filepath.Join(main, "node_modules"))
	}
}

func TestProvisionRunsCommandsInOrderStoppingOnError(t *testing.T) {
	main, dest := t.TempDir(), t.TempDir()

	err := Provision(ProvisionOptions{
		MainPath: main, DestPath: dest,
		Config: &Config{PostCreate: []string{
			"echo first > marker1",
			"exit 3",
			"echo third > marker3",
		}}, Out: io.Discard,
	})
	if err == nil {
		t.Fatal("expected error from failing command")
	}
	if _, err := os.Stat(filepath.Join(dest, "marker1")); err != nil {
		t.Fatal("first command should have run")
	}
	if _, err := os.Stat(filepath.Join(dest, "marker3")); err == nil {
		t.Fatal("third command should not have run after failure")
	}
}

func TestProvisionMissingSourceErrors(t *testing.T) {
	main, dest := t.TempDir(), t.TempDir()
	err := Provision(ProvisionOptions{
		MainPath: main, DestPath: dest,
		Config: &Config{Copy: []string{".env"}}, Out: io.Discard,
	})
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestProvisionRejectsAbsoluteAndEscapingPaths(t *testing.T) {
	main, dest := t.TempDir(), t.TempDir()
	for _, bad := range []string{"/etc/passwd", "../outside"} {
		err := Provision(ProvisionOptions{
			MainPath: main, DestPath: dest,
			Config: &Config{Copy: []string{bad}}, Out: io.Discard,
		})
		if err == nil {
			t.Fatalf("expected error for path %q", bad)
		}
	}
}

func TestProvisionRejectsExistingDestination(t *testing.T) {
	main, dest := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(main, ".env"), []byte("X=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, ".env"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Provision(ProvisionOptions{
		MainPath: main, DestPath: dest,
		Config: &Config{Copy: []string{".env"}}, Out: io.Discard,
	})
	if err == nil {
		t.Fatal("expected error for pre-existing destination")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/project/`
Expected: FAIL — `undefined: Provision`, `undefined: ProvisionOptions`.

- [ ] **Step 3: Write the implementation**

Create `internal/project/provision.go`:

```go
package project

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProvisionOptions configures a single Provision run.
type ProvisionOptions struct {
	MainPath string    // absolute path of the main worktree (source root)
	DestPath string    // absolute path of the new worktree (destination root)
	Config   *Config   // parsed .ggw.yaml
	Out      io.Writer // where post_create command output is streamed
}

// Provision applies the config to the new worktree: copy, then symlink, then
// post_create commands. It returns the first error encountered; the caller is
// responsible for rolling back the worktree.
func Provision(opts ProvisionOptions) error {
	for _, rel := range opts.Config.Copy {
		if err := copyEntry(opts.MainPath, opts.DestPath, rel); err != nil {
			return err
		}
	}
	for _, rel := range opts.Config.Symlink {
		if err := symlinkEntry(opts.MainPath, opts.DestPath, rel); err != nil {
			return err
		}
	}
	for _, command := range opts.Config.PostCreate {
		if err := runCommand(opts.DestPath, command, opts.Out); err != nil {
			return err
		}
	}
	return nil
}

func copyEntry(mainPath, destPath, rel string) error {
	src, dst, err := resolveEntry(mainPath, destPath, rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy %q: %w", rel, err)
	}
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("copy %q: destination already exists: %s", rel, dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("copy %q: %w", rel, err)
	}
	if info.IsDir() {
		return copyTree(src, dst)
	}
	return copyFile(src, dst, info.Mode())
}

func symlinkEntry(mainPath, destPath, rel string) error {
	src, dst, err := resolveEntry(mainPath, destPath, rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("symlink %q: %w", rel, err)
	}
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("symlink %q: destination already exists: %s", rel, dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("symlink %q: %w", rel, err)
	}
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("symlink %q: %w", rel, err)
	}
	return nil
}

func runCommand(destPath, command string, out io.Writer) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = destPath
	cmd.Env = os.Environ()
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %q failed: %w", command, err)
	}
	return nil
}

// resolveEntry validates a relative entry and returns its absolute source
// (under mainPath) and destination (under destPath).
func resolveEntry(mainPath, destPath, rel string) (string, string, error) {
	if rel == "" {
		return "", "", fmt.Errorf("empty path in .ggw.yaml")
	}
	if filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("path %q must be relative", rel)
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("path %q escapes the repository", rel)
	}
	return filepath.Join(mainPath, clean), filepath.Join(destPath, clean), nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/project/`
Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add internal/project/provision.go internal/project/provision_test.go
git commit -m "feat(project): add worktree provisioning engine"
```

---

### Task 3: `ggw project-init` command

**Files:**
- Create: `internal/cli/project_init.go`
- Modify: `tests/e2e_test.go` (add a test)

**Interfaces:**
- Consumes: `project.ConfigFileName`, `project.WriteTemplate` (Task 1); `worktree.RepoRoot`, `worktree.List` (existing); `maybeJSON`, `displayPath`, `ui`, `GroupConfig` (existing CLI helpers).
- Produces: a `project-init` cobra command registered on `rootCmd`.

- [ ] **Step 1: Write the failing e2e test**

Add to `tests/e2e_test.go` (place near the other command tests, e.g. before `TestCLIPRRequiresGH`):

```go
func TestCLIProjectInitCreatesConfig(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	out, err := runGGW(t, home, repo, "project-init")
	if err != nil {
		t.Fatalf("ggw project-init failed: %v\n%s", err, out)
	}
	cfgPath := filepath.Join(repo, ".ggw.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf(".ggw.yaml not created: %v", err)
	}
	if !strings.Contains(string(data), "post_create:") {
		t.Fatalf("template missing sections:\n%s", data)
	}

	// Without --force, a second run must fail.
	if out, err := runGGW(t, home, repo, "project-init"); err == nil {
		t.Fatalf("expected project-init to fail on existing file:\n%s", out)
	}

	// With --force, it regenerates.
	if out, err := runGGW(t, home, repo, "project-init", "--force"); err != nil {
		t.Fatalf("ggw project-init --force failed: %v\n%s", err, out)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./tests/ -run TestCLIProjectInitCreatesConfig`
Expected: FAIL — `unknown command "project-init"`.

- [ ] **Step 3: Write the command**

Create `internal/cli/project_init.go`:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/illegalstudio/ggw/internal/project"
	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/spf13/cobra"
)

var projectInitCmd = &cobra.Command{
	Use:     "project-init",
	Short:   "Create a .ggw.yaml provisioning file at the repository root",
	GroupID: GroupConfig,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := worktree.RepoRoot(cwd)
		if err != nil {
			return err
		}
		mainPath, err := mainWorktreePath(root)
		if err != nil {
			return err
		}
		path := filepath.Join(mainPath, project.ConfigFileName)

		if err := project.WriteTemplate(path, force); err != nil {
			return err
		}

		if done, err := maybeJSON(map[string]any{"created": true, "path": path}); done {
			return err
		}

		fmt.Println(ui.Success.Render("✓") + " Project config created at " + ui.Path.Render(displayPath(path)))
		return nil
	},
}

func init() {
	projectInitCmd.Flags().Bool("force", false, "Overwrite an existing .ggw.yaml")
	rootCmd.AddCommand(projectInitCmd)
}
```

- [ ] **Step 4: Add the shared `mainWorktreePath` helper**

Create `internal/cli/provision.go` (this file is extended in Task 4; start it here with the helper):

```go
package cli

import (
	"fmt"

	"github.com/illegalstudio/ggw/internal/worktree"
)

// mainWorktreePath returns the path of the repository's main worktree, which
// git always lists first. .ggw.yaml lives there and is the source for
// copy/symlink provisioning.
func mainWorktreePath(root string) (string, error) {
	list, err := worktree.List(root)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no worktrees found for %s", root)
	}
	return list[0].Path, nil
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./tests/ -run TestCLIProjectInitCreatesConfig`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/project_init.go internal/cli/provision.go tests/e2e_test.go
git commit -m "feat(cli): add ggw project-init command"
```

---

### Task 4: Provisioning + `--bare` on `ggw create`

**Files:**
- Modify: `internal/cli/provision.go` (add `provisionWorktree`)
- Modify: `internal/cli/create.go`
- Modify: `tests/e2e_test.go` (add tests)

**Interfaces:**
- Consumes: `project.Load`, `project.Provision`, `project.ProvisionOptions` (Tasks 1–2); `mainWorktreePath` (Task 3); `worktree.Remove` (existing).
- Produces: `func provisionWorktree(root, dest string, bare bool, out io.Writer) error`.

- [ ] **Step 1: Write the failing e2e tests**

Add to `tests/e2e_test.go`:

```go
func TestCLICreateProvisionsWorktree(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	// Untracked source files + a .ggw.yaml committed to the repo.
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("TOKEN=abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "deps"), 0o755); err != nil {
		t.Fatal(err)
	}
	ggwYaml := "copy:\n  - .env\nsymlink:\n  - deps\npost_create:\n  - echo provisioned > .provisioned\n"
	if err := os.WriteFile(filepath.Join(repo, ".ggw.yaml"), []byte(ggwYaml), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, repo, "add", ".ggw.yaml")
	runGit(t, home, repo, "commit", "-m", "add ggw config")

	out, err := runGGW(t, home, repo, "create", "feature/work")
	if err != nil {
		t.Fatalf("ggw create failed: %v\n%s", err, out)
	}
	dest := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-work")

	if got, _ := os.ReadFile(filepath.Join(dest, ".env")); string(got) != "TOKEN=abc" {
		t.Fatalf("copied .env = %q", got)
	}
	fi, err := os.Lstat(filepath.Join(dest, "deps"))
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("deps should be a symlink: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".provisioned")); err != nil {
		t.Fatalf("post_create marker missing: %v", err)
	}
}

func TestCLICreateBareSkipsProvisioning(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("TOKEN=abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	ggwYaml := "copy:\n  - .env\n"
	if err := os.WriteFile(filepath.Join(repo, ".ggw.yaml"), []byte(ggwYaml), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, repo, "add", ".ggw.yaml")
	runGit(t, home, repo, "commit", "-m", "add ggw config")

	out, err := runGGW(t, home, repo, "create", "feature/bare", "--bare")
	if err != nil {
		t.Fatalf("ggw create --bare failed: %v\n%s", err, out)
	}
	dest := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-bare")
	if _, err := os.Stat(filepath.Join(dest, ".env")); err == nil {
		t.Fatal(".env should NOT be copied with --bare")
	}
}

func TestCLICreateRollsBackOnFailureKeepingBranch(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	ggwYaml := "post_create:\n  - exit 7\n"
	if err := os.WriteFile(filepath.Join(repo, ".ggw.yaml"), []byte(ggwYaml), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, repo, "add", ".ggw.yaml")
	runGit(t, home, repo, "commit", "-m", "add ggw config")

	out, err := runGGW(t, home, repo, "create", "feature/fail")
	if err == nil {
		t.Fatalf("expected create to fail on provisioning error:\n%s", out)
	}
	dest := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-fail")
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("worktree should have been removed, stat err: %v", statErr)
	}
	// The branch must survive the rollback.
	if _, err := runGitCommand(t, home, repo, "show-ref", "--verify", "--quiet", "refs/heads/feature/fail"); err != nil {
		t.Fatal("branch feature/fail should still exist after rollback")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tests/ -run TestCLICreate`
Expected: FAIL — `unknown flag: --bare` and provisioning not performed.

- [ ] **Step 3: Add `provisionWorktree` to `internal/cli/provision.go`**

Append to `internal/cli/provision.go` (add `io` and the project import to the import block):

```go
import (
	"fmt"
	"io"

	"github.com/illegalstudio/ggw/internal/project"
	"github.com/illegalstudio/ggw/internal/worktree"
)

// provisionWorktree applies the repo's .ggw.yaml to a freshly created worktree
// at dest. It is a no-op when bare is true or no .ggw.yaml exists. On error the
// caller is responsible for rolling back the worktree.
func provisionWorktree(root, dest string, bare bool, out io.Writer) error {
	if bare {
		return nil
	}
	mainPath, err := mainWorktreePath(root)
	if err != nil {
		return err
	}
	cfg, exists, err := project.Load(mainPath)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return project.Provision(project.ProvisionOptions{
		MainPath: mainPath,
		DestPath: dest,
		Config:   cfg,
		Out:      out,
	})
}
```

- [ ] **Step 4: Wire provisioning into `create.go`**

In `internal/cli/create.go`, add the `--bare` flag in `init()`:

```go
func init() {
	createCmd.Flags().String("from", "", "Base ref for new branches (default: HEAD)")
	createCmd.Flags().Bool("bare", false, "Create the worktree without running .ggw.yaml provisioning")
	rootCmd.AddCommand(createCmd)
}
```

In the `RunE`, read the flag near the top (alongside `from`):

```go
			from, _ := cmd.Flags().GetString("from")
			bare, _ := cmd.Flags().GetBool("bare")
```

Then replace the block that currently runs `worktree.Create(...)` and returns its error:

```go
			if err := worktree.Create(worktree.CreateOptions{
				RepoPath: root,
				Branch:   branch,
				DestPath: dest,
				From:     from,
			}); err != nil {
				return err
			}
```

with the create-plus-provisioning-plus-rollback sequence:

```go
			if err := worktree.Create(worktree.CreateOptions{
				RepoPath: root,
				Branch:   branch,
				DestPath: dest,
				From:     from,
			}); err != nil {
				return err
			}

			// Provision transactionally: on any failure, remove the worktree
			// but keep the branch so pre-existing work is never lost.
			provisioned := false
			defer func() {
				if !provisioned {
					_ = worktree.Remove(root, dest, true)
				}
			}()
			if err := provisionWorktree(root, dest, bare, os.Stderr); err != nil {
				return err
			}
			provisioned = true
```

(The existing `maybeJSON` / `fmt.Printf` success block stays as-is below this.)

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./tests/ -run TestCLICreate`
Expected: PASS (provision, bare, rollback).

- [ ] **Step 6: Run the full suite and build**

Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/provision.go internal/cli/create.go tests/e2e_test.go
git commit -m "feat(cli): provision worktrees on create with transactional rollback"
```

---

### Task 5: Provisioning + `--bare` on `ggw pr`

**Files:**
- Modify: `internal/cli/pr.go`
- Modify: `tests/e2e_test.go` (add a test)

**Interfaces:**
- Consumes: `provisionWorktree` (Task 4).
- Produces: nothing new.

- [ ] **Step 1: Write the failing e2e test**

Add to `tests/e2e_test.go`. This mirrors the existing `TestCLIPRCreatesTrackedWorktreeViaGH` fake-`gh` harness but adds a `.ggw.yaml` and asserts provisioning ran:

```go
func TestCLIPRProvisionsWorktree(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)
	binDir := t.TempDir()
	fakeGH := filepath.Join(binDir, "gh")

	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("TOKEN=pr"), 0o644); err != nil {
		t.Fatal(err)
	}
	ggwYaml := "copy:\n  - .env\npost_create:\n  - echo ok > .provisioned\n"
	if err := os.WriteFile(filepath.Join(repo, ".ggw.yaml"), []byte(ggwYaml), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, repo, "add", ".ggw.yaml")
	runGit(t, home, repo, "commit", "-m", "add ggw config")

	script := `#!/bin/sh
git checkout -b contributor/feature
`
	if err := os.WriteFile(fakeGH, []byte(script), 0755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}

	pathEnv := "PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	out, err := runGGWWithEnv(t, home, repo, []string{pathEnv}, "pr", "321")
	if err != nil {
		t.Fatalf("ggw pr failed: %v\n%s", err, out)
	}
	dest := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "pr-321")
	if got, _ := os.ReadFile(filepath.Join(dest, ".env")); string(got) != "TOKEN=pr" {
		t.Fatalf("copied .env = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dest, ".provisioned")); err != nil {
		t.Fatalf("post_create marker missing: %v", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./tests/ -run TestCLIPRProvisionsWorktree`
Expected: FAIL — `.env`/`.provisioned` not present (provisioning not wired into `pr`).

- [ ] **Step 3: Wire provisioning into `pr.go`**

In `internal/cli/pr.go`, add the `--bare` flag in `init()`:

```go
func init() {
	prCmd.Flags().Bool("bare", false, "Create the worktree without running .ggw.yaml provisioning")
	rootCmd.AddCommand(prCmd)
}
```

Read the flag at the top of `RunE` (after `prID` is normalized):

```go
		bare, _ := cmd.Flags().GetBool("bare")
```

`pr.go` already has the `success`/deferred-rollback pattern. Insert the provisioning call **after** `branch, err := worktree.CurrentBranch(dest)` succeeds and **before** `success = true`:

```go
			branch, err := worktree.CurrentBranch(dest)
			if err != nil {
				return err
			}

			if err := provisionWorktree(root, dest, bare, os.Stderr); err != nil {
				return err
			}
			success = true
```

(The existing `defer` already calls `worktree.Remove(root, dest, true)` when `success` is false, which keeps the branch — matching the create rollback.)

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./tests/ -run TestCLIPR`
Expected: PASS (both the existing PR test and the new provisioning test).

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/pr.go tests/e2e_test.go
git commit -m "feat(cli): provision worktrees on pr with --bare opt-out"
```

---

### Task 6: Documentation

**Files:**
- Modify: `docs/commands.md`
- Modify: `docs/configuration.md`
- Modify: `README.md`

**Interfaces:** none (docs only).

- [ ] **Step 1: Document `project-init` and `--bare` in `docs/commands.md`**

Add a `## ggw project-init` section (after the `ggw init` or config section) and note the `--bare` flag under `ggw create` and `ggw pr`:

```markdown
## `ggw project-init`

Create a `.ggw.yaml` provisioning file at the repository root.

```bash
ggw project-init
ggw project-init --force   # overwrite an existing file
```

`.ggw.yaml` declares how each new worktree is set up:

```yaml
copy:            # files/dirs copied from the main worktree
  - .env
symlink:         # files/dirs symlinked to the main worktree
  - node_modules
  - vendor
post_create:     # shell commands run in the new worktree, in order
  - composer install
```

Provisioning runs after `ggw create` and `ggw pr`. If any step fails, the new
worktree is removed (the branch is kept) so you can fix the issue and re-run.
Pass `--bare` to `create`/`pr` to skip provisioning for a single run.
```

- [ ] **Step 2: Add a `.ggw.yaml` section to `docs/configuration.md`**

Document the schema, that paths are relative to the repo root, that `copy`/`symlink` sources come from the main worktree, the fixed copy→symlink→post_create order, the transactional rollback (worktree removed, branch kept), and the failure cases (absolute/`..` paths, missing source, existing destination).

- [ ] **Step 3: Mention provisioning in `README.md`**

Add a short bullet/paragraph near the usage section pointing to `ggw project-init` and `.ggw.yaml` for making every worktree ready to use.

- [ ] **Step 4: Commit**

```bash
git add docs/commands.md docs/configuration.md README.md
git commit -m "docs: document .ggw.yaml project provisioning"
```

---

## Self-Review Notes

- **Spec coverage:** project-init (Task 3), schema + Load (Task 1), provisioning flow & ordering + path validation + missing-source/existing-destination (Task 2), rollback + --bare on create (Task 4) and pr (Task 5), docs (Task 6). All spec sections map to a task.
- **Type consistency:** `Config{Copy,Symlink,PostCreate}`, `Load(string)->(*Config,bool,error)`, `WriteTemplate(string,bool)error`, `ProvisionOptions{MainPath,DestPath,Config,Out}`, `Provision(ProvisionOptions)error`, `mainWorktreePath(string)->(string,error)`, `provisionWorktree(root,dest string,bare bool,out io.Writer)error` are used consistently across tasks.
- **Decisions locked from the design:** provisioning output → stderr; rollback uses `worktree.Remove(root,dest,true)`; branch always preserved.
