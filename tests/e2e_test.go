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

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = cwd
	cmd.Env = childEnv(home)
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
