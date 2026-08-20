package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	testAuthorName  = "GGW Test"
	testAuthorEmail = "ggw@example.com"
)

var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "ggw-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}

	binaryName := "ggw"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath = filepath.Join(tmp, binaryName)

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get working dir: %v\n", err)
		_ = os.RemoveAll(tmp)
		os.Exit(1)
	}
	repoRoot := filepath.Dir(wd)

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/ggw")
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build ggw: %v\n%s", err, out)
		_ = os.RemoveAll(tmp)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

func TestCLIShellInitShellIntegration(t *testing.T) {
	home := setupHome(t)
	cwd := t.TempDir()

	out, err := runGGW(t, home, cwd, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("ggw shell-init zsh failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ggw()") || !strings.Contains(out, "turn 'ggw cd' into a real chdir") {
		t.Fatalf("unexpected shell-init output:\n%s", out)
	}

	out, err = runGGW(t, home, cwd, "--json", "shell-init", "bash")
	if err != nil {
		t.Fatalf("ggw --json shell-init bash failed: %v\n%s", err, out)
	}

	var payload struct {
		Shell  string `json:"shell"`
		Script string `json:"script"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("shell-init JSON is invalid: %v\n%s", err, out)
	}
	if payload.Shell != "bash" || !strings.Contains(payload.Script, "ggw()") {
		t.Fatalf("unexpected shell-init JSON payload: %+v", payload)
	}
}

func TestCLICreateRandomName(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	out, err := runGGW(t, home, repo, "--json", "create")
	if err != nil {
		t.Fatalf("ggw create (no name) failed: %v\n%s", err, out)
	}
	var payload struct {
		Branch string `json:"branch"`
		Slug   string `json:"slug"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("create JSON invalid: %v\n%s", err, out)
	}
	if !strings.Contains(payload.Branch, "-") {
		t.Fatalf("expected adjective-noun branch, got %q", payload.Branch)
	}
	parts := strings.Split(payload.Branch, "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		t.Fatalf("expected two-word name, got %q", payload.Branch)
	}
	if payload.Slug != payload.Branch {
		t.Fatalf("slug = %q, want same as branch %q", payload.Slug, payload.Branch)
	}
	if _, err := os.Stat(filepath.Join(payload.Path, ".git")); err != nil {
		t.Fatalf("worktree not created at %s: %v\noutput:\n%s", payload.Path, err, out)
	}
	if branch := runGit(t, home, payload.Path, "rev-parse", "--abbrev-ref", "HEAD"); branch != payload.Branch {
		t.Fatalf("worktree branch = %q, want %q", branch, payload.Branch)
	}
}

func TestCLICreateListCDExecDelete(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)
	worktreePath := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-some")

	out, err := runGGW(t, home, repo, "create", "feature/some")
	if err != nil {
		t.Fatalf("ggw create failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("worktree was not created at %s: %v", worktreePath, err)
	}
	canonicalWorktreePath := canonicalPath(t, worktreePath)
	if branch := runGit(t, home, worktreePath, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature/some" {
		t.Fatalf("worktree branch = %q, want feature/some", branch)
	}

	out, err = runGGW(t, home, repo, "list")
	if err != nil {
		t.Fatalf("ggw list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "feature/some") || !strings.Contains(out, "feature-some") {
		t.Fatalf("list output missing worktree branch or slug:\n%s", out)
	}

	out, err = runGGW(t, home, repo, "--json", "list")
	if err != nil {
		t.Fatalf("ggw --json list failed: %v\n%s", err, out)
	}
	var listPayload struct {
		Worktrees []struct {
			Path   string `json:"path"`
			Branch string `json:"branch"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal([]byte(out), &listPayload); err != nil {
		t.Fatalf("list JSON is invalid: %v\n%s", err, out)
	}
	if !hasWorktree(listPayload.Worktrees, canonicalWorktreePath, "feature/some") {
		t.Fatalf("list JSON missing created worktree: %+v", listPayload.Worktrees)
	}

	out, err = runGGW(t, home, repo, "cd", "feature/some")
	if err != nil {
		t.Fatalf("ggw cd failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != canonicalWorktreePath {
		t.Fatalf("cd output = %q, want %q", strings.TrimSpace(out), canonicalWorktreePath)
	}

	out, err = runGGW(t, home, repo, "exec", "feature/some", "--", "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("ggw exec failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "feature/some") {
		t.Fatalf("exec output missing command output:\n%s", out)
	}

	out, err = runGGW(t, home, repo, "__complete", "delete", "")
	if err != nil {
		t.Fatalf("ggw delete completion failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "feature/some") {
		t.Fatalf("completion output missing branch name:\n%s", out)
	}
	if strings.Contains(out, "feature-some") {
		t.Fatalf("completion output contains duplicate generated slug:\n%s", out)
	}

	out, err = runGGW(t, home, repo, "delete", "--force", "feature/some")
	if err != nil {
		t.Fatalf("ggw delete failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("worktree still exists after delete, stat err: %v", err)
	}
	if _, err := runGitCommand(t, home, repo, "show-ref", "--verify", "--quiet", "refs/heads/feature/some"); err == nil {
		t.Fatal("feature/some branch still exists after delete")
	}
}

func TestCLICreateCompletionExcludesWorktreeBranches(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	// Two extra local branches; "feature/done" will get a worktree.
	runGit(t, home, repo, "branch", "feature/todo")
	runGit(t, home, repo, "branch", "feature/done")

	if out, err := runGGW(t, home, repo, "create", "feature/done"); err != nil {
		t.Fatalf("ggw create failed: %v\n%s", err, out)
	}

	out, err := runGGW(t, home, repo, "__complete", "create", "")
	if err != nil {
		t.Fatalf("ggw create completion failed: %v\n%s", err, out)
	}

	lines := strings.Split(out, "\n")
	has := func(branch string) bool {
		for _, l := range lines {
			if strings.TrimSpace(l) == branch {
				return true
			}
		}
		return false
	}

	// feature/todo has no worktree → suggested.
	if !has("feature/todo") {
		t.Fatalf("completion missing branch without a worktree:\n%s", out)
	}
	// main and feature/done are checked out in worktrees → excluded.
	if has("feature/done") {
		t.Fatalf("completion includes branch that already has a worktree:\n%s", out)
	}
	if has("main") {
		t.Fatalf("completion includes the main branch (already checked out):\n%s", out)
	}
}

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

func TestCLIPRRequiresGH(t *testing.T) {
	home := setupHome(t)
	cwd := t.TempDir()

	out, err := runGGWWithEnv(t, home, cwd, []string{"PATH=" + t.TempDir()}, "pr", "123")
	if err == nil {
		t.Fatalf("ggw pr succeeded without gh:\n%s", out)
	}
	if !strings.Contains(out, "gh is required for `ggw pr`") || !strings.Contains(out, "https://cli.github.com/") {
		t.Fatalf("missing gh error did not explain how to install gh:\n%s", out)
	}
}

func TestCLIPRCreatesTrackedWorktreeViaGH(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)
	binDir := t.TempDir()
	ghLog := filepath.Join(binDir, "gh.log")
	fakeGH := filepath.Join(binDir, "gh")
	worktreePath := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "pr-123")

	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" > %q
if [ "$1" != "pr" ] || [ "$2" != "checkout" ] || [ "$3" != "123" ]; then
	echo "unexpected gh args: $*" >&2
	exit 1
fi
git checkout -b contributor/feature
git config branch.contributor/feature.remote fork
git config branch.contributor/feature.merge refs/heads/contributor/feature
`, ghLog)
	if err := os.WriteFile(fakeGH, []byte(script), 0755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}

	pathEnv := "PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	out, err := runGGWWithEnv(t, home, repo, []string{pathEnv}, "pr", "123")
	if err != nil {
		t.Fatalf("ggw pr failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("PR worktree was not created at %s: %v", worktreePath, err)
	}
	if branch := runGit(t, home, worktreePath, "rev-parse", "--abbrev-ref", "HEAD"); branch != "contributor/feature" {
		t.Fatalf("worktree branch = %q, want contributor/feature", branch)
	}
	if remote := runGit(t, home, worktreePath, "config", "branch.contributor/feature.remote"); remote != "fork" {
		t.Fatalf("branch remote = %q, want fork", remote)
	}
	if merge := runGit(t, home, worktreePath, "config", "branch.contributor/feature.merge"); merge != "refs/heads/contributor/feature" {
		t.Fatalf("branch merge ref = %q, want refs/heads/contributor/feature", merge)
	}

	logBytes, err := os.ReadFile(ghLog)
	if err != nil {
		t.Fatalf("read fake gh log: %v", err)
	}
	if strings.TrimSpace(string(logBytes)) != "pr checkout 123" {
		t.Fatalf("gh invocation = %q, want %q", strings.TrimSpace(string(logBytes)), "pr checkout 123")
	}
}

func hasWorktree(entries []struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
}, path, branch string) bool {
	for _, entry := range entries {
		if entry.Path == path && entry.Branch == branch {
			return true
		}
	}
	return false
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve path %s: %v", path, err)
	}
	return resolved
}

func setupHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	for _, dir := range []string{
		filepath.Join(home, ".config"),
		filepath.Join(home, ".githooks"),
		filepath.Join(home, ".local", "share"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create test home dir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), nil, 0644); err != nil {
		t.Fatalf("create git config: %v", err)
	}
	return home
}

func initGitRepo(t *testing.T, home string) string {
	t.Helper()

	repo := filepath.Join(t.TempDir(), "api")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}

	runGit(t, home, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# api\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, home, repo, "add", ".")
	runGit(t, home, repo, "commit", "-m", "initial commit")
	runGit(t, home, repo, "remote", "add", "origin", "git@github.com:acme/api.git")

	return repo
}

func runGGW(t *testing.T, home, cwd string, args ...string) (string, error) {
	t.Helper()
	return runGGWWithEnv(t, home, cwd, nil, args...)
}

func runGGWWithEnv(t *testing.T, home, cwd string, extraEnv []string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = cwd
	cmd.Env = append(childEnv(home), extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runGit(t *testing.T, home, dir string, args ...string) string {
	t.Helper()

	out, err := runGitCommand(t, home, dir, args...)
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(out)
}

func runGitCommand(t *testing.T, home, dir string, args ...string) (string, error) {
	t.Helper()

	gitArgs := []string{
		"-c", "commit.gpgsign=false",
		"-c", "tag.gpgsign=false",
		"-c", "core.hooksPath=" + filepath.Join(home, ".githooks"),
	}
	gitArgs = append(gitArgs, args...)

	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = dir
	cmd.Env = append(childEnv(home),
		"GIT_AUTHOR_NAME="+testAuthorName,
		"GIT_AUTHOR_EMAIL="+testAuthorEmail,
		"GIT_COMMITTER_NAME="+testAuthorName,
		"GIT_COMMITTER_EMAIL="+testAuthorEmail,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func childEnv(home string) []string {
	return append(os.Environ(),
		"HOME="+home,
		"USERPROFILE="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(home, ".gitconfig"),
		"GIT_TERMINAL_PROMPT=0",
		"NO_COLOR=1",
	)
}

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

	// ggw list must treat the configured-base_dir worktree as internal,
	// not [external] — both create and list route through WorktreesBase.
	out, err = runGGW(t, home, repo, "--json", "list")
	if err != nil {
		t.Fatalf("ggw --json list failed: %v\n%s", err, out)
	}
	var listed struct {
		Worktrees []struct {
			Path     string `json:"path"`
			Branch   string `json:"branch"`
			External bool   `json:"external"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		t.Fatalf("list JSON invalid: %v\n%s", err, out)
	}
	canonicalWant := canonicalPath(t, want)
	var found bool
	for _, w := range listed.Worktrees {
		if w.Branch == "feature/login" {
			found = true
			if w.External {
				t.Fatalf("worktree under configured base_dir wrongly tagged external: %+v", w)
			}
			if w.Path != canonicalWant {
				t.Fatalf("listed path = %q, want %q", w.Path, canonicalWant)
			}
		}
	}
	if !found {
		t.Fatalf("feature/login worktree not found in list output:\n%s", out)
	}
}

func TestCLIExternalDetachedWorktree(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	// A detached worktree created outside ggw's base (e.g. by another tool),
	// whose leaf "api" collides with the repo basename so the handle must
	// grow to "0e21/api".
	extPath := filepath.Join(home, "external", "0e21", "api")
	if err := os.MkdirAll(filepath.Dir(extPath), 0755); err != nil {
		t.Fatalf("create external parent dir: %v", err)
	}
	runGit(t, home, repo, "worktree", "add", "--detach", extPath)
	canonicalExt := canonicalPath(t, extPath)

	// list: shows the handle and the [external] tag.
	out, err := runGGW(t, home, repo, "list")
	if err != nil {
		t.Fatalf("ggw list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "0e21/api") || !strings.Contains(out, "[external]") {
		t.Fatalf("list output missing handle or external tag:\n%s", out)
	}

	// list --json: external flag is set for the detached worktree.
	out, err = runGGW(t, home, repo, "--json", "list")
	if err != nil {
		t.Fatalf("ggw --json list failed: %v\n%s", err, out)
	}
	var listPayload struct {
		Worktrees []struct {
			Path     string `json:"path"`
			External bool   `json:"external"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal([]byte(out), &listPayload); err != nil {
		t.Fatalf("list JSON is invalid: %v\n%s", err, out)
	}
	foundExternal := false
	for _, w := range listPayload.Worktrees {
		if w.Path == canonicalExt {
			foundExternal = w.External
		}
	}
	if !foundExternal {
		t.Fatalf("external worktree not flagged in JSON: %+v", listPayload.Worktrees)
	}

	// completion: offers the handle, never the ambiguous bare basename.
	out, err = runGGW(t, home, repo, "__complete", "delete", "")
	if err != nil {
		t.Fatalf("ggw delete completion failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "0e21/api") {
		t.Fatalf("completion missing handle:\n%s", out)
	}

	// cd: resolves the handle to the external worktree path.
	out, err = runGGW(t, home, repo, "cd", "0e21/api")
	if err != nil {
		t.Fatalf("ggw cd failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != canonicalExt {
		t.Fatalf("cd output = %q, want %q", strings.TrimSpace(out), canonicalExt)
	}

	// delete: removes the external worktree by handle.
	out, err = runGGW(t, home, repo, "delete", "--force", "0e21/api")
	if err != nil {
		t.Fatalf("ggw delete failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(extPath); !os.IsNotExist(err) {
		t.Fatalf("external worktree still exists after delete, stat err: %v", err)
	}
}

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

// skillsInstallPayload mirrors the JSON emitted by `ggw skills install`.
//
// Always decode into a fresh value via decodeSkillsInstall: encoding/json
// reuses existing slice elements and does not clear fields that are absent
// from the new document, so a reused variable can carry a stale `status` or
// `error` (both omitempty) into a later assertion and mask a regression.
type skillsInstallPayload struct {
	Name          string `json:"name"`
	Installations []struct {
		Target string `json:"target"`
		Path   string `json:"path"`
		Status string `json:"status"`
		Error  string `json:"error"`
	} `json:"installations"`
}

func decodeSkillsInstall(t *testing.T, out string) skillsInstallPayload {
	t.Helper()

	var payload skillsInstallPayload
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("skills install did not emit valid JSON: %v\n%s", err, out)
	}
	return payload
}

func TestCLISkillsInstallToSelectedTarget(t *testing.T) {
	home := setupHome(t)
	cwd := t.TempDir()

	out, err := runGGW(t, home, cwd, "--json", "skills", "install", "--target", "claude")
	if err != nil {
		t.Fatalf("ggw --json skills install failed: %v\n%s", err, out)
	}

	payload := decodeSkillsInstall(t, out)
	if payload.Name != "ggw" {
		t.Fatalf("name = %q, want %q", payload.Name, "ggw")
	}
	if len(payload.Installations) != 1 {
		t.Fatalf("installations = %d, want 1\n%s", len(payload.Installations), out)
	}
	if got := payload.Installations[0]; got.Target != "claude" || got.Status != "installed" || got.Error != "" {
		t.Fatalf("unexpected installation %+v\n%s", got, out)
	}

	skillPath := filepath.Join(home, ".claude", "skills", "ggw", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("SKILL.md not installed at %s: %v", skillPath, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "ggw")); !os.IsNotExist(err) {
		t.Fatalf("unselected destination was installed: %v", err)
	}

	// Reinstalling an untouched copy is a no-op.
	out, err = runGGW(t, home, cwd, "--json", "skills", "install", "--target", "claude")
	if err != nil {
		t.Fatalf("second ggw --json skills install failed: %v\n%s", err, out)
	}
	second := decodeSkillsInstall(t, out)
	if got := second.Installations[0].Status; got != "up-to-date" {
		t.Fatalf("second install status = %q, want %q\n%s", got, "up-to-date", out)
	}
	if got := second.Installations[0].Error; got != "" {
		t.Fatalf("second install reported an error: %q\n%s", got, out)
	}
}

func TestCLISkillsInstallRejectsUnknownTarget(t *testing.T) {
	home := setupHome(t)
	cwd := t.TempDir()

	out, err := runGGW(t, home, cwd, "--json", "skills", "install", "--target", "cursor")
	if err == nil {
		t.Fatalf("ggw skills install with an unknown target succeeded:\n%s", out)
	}

	// A command-level failure must arrive as the documented JSON error object,
	// not as plain text.
	var errPayload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &errPayload); err != nil {
		t.Fatalf("unknown target did not emit valid JSON error: %v\n%s", err, out)
	}
	if !strings.Contains(errPayload.Error, "unknown skill target") {
		t.Fatalf("unexpected JSON error payload: %+v\n%s", errPayload, out)
	}

	for _, dir := range []string{
		filepath.Join(home, ".claude", "skills", "ggw"),
		filepath.Join(home, ".agents", "skills", "ggw"),
	} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("%s was installed despite the error: %v", dir, err)
		}
	}
}

func TestCLISkillsInstallReportsConflictPerDestination(t *testing.T) {
	home := setupHome(t)
	cwd := t.TempDir()

	out, err := runGGW(t, home, cwd, "--json", "skills", "install", "--target", "agents")
	if err != nil {
		t.Fatalf("ggw --json skills install failed: %v\n%s", err, out)
	}

	skillPath := filepath.Join(home, ".agents", "skills", "ggw", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("hand edited\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// A conflict on one destination must not fail the command or block the other.
	out, err = runGGW(t, home, cwd, "--json", "skills", "install")
	if err != nil {
		t.Fatalf("ggw --json skills install exited non-zero on a per-item conflict: %v\n%s", err, out)
	}

	conflict := decodeSkillsInstall(t, out)
	if len(conflict.Installations) != 2 {
		t.Fatalf("installations = %d, want 2\n%s", len(conflict.Installations), out)
	}
	if conflict.Installations[0].Error == "" {
		t.Fatalf("modified destination did not report a conflict\n%s", out)
	}
	if conflict.Installations[1].Error != "" {
		t.Fatalf("second destination failed: %s\n%s", conflict.Installations[1].Error, out)
	}
	if got := string(mustReadFile(t, skillPath)); got != "hand edited\n" {
		t.Fatalf("conflicting destination was overwritten: %q", got)
	}

	out, err = runGGW(t, home, cwd, "--json", "skills", "install", "--force")
	if err != nil {
		t.Fatalf("ggw --json skills install --force failed: %v\n%s", err, out)
	}
	forced := decodeSkillsInstall(t, out)
	if got := forced.Installations[0].Status; got != "replaced" {
		t.Fatalf("forced install status = %q, want %q\n%s", got, "replaced", out)
	}
	if got := forced.Installations[0].Error; got != "" {
		t.Fatalf("forced install reported an error: %q\n%s", got, out)
	}
	if got := string(mustReadFile(t, skillPath)); got == "hand edited\n" {
		t.Fatal("--force did not replace the modified skill")
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return contents
}
