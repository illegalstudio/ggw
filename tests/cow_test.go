package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/illegalstudio/ggw/internal/cow"
)

// --- copy-on-write workspaces -----------------------------------------------

func TestCLICreateCoWWorkspace(t *testing.T) {
	home, repo := setupCoWRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), "node_modules/\n")
	writeFile(t, filepath.Join(repo, "node_modules", "dep.js"), "untracked\n")
	writeFile(t, filepath.Join(repo, ".env"), "SECRET=1\n")
	runGit(t, home, repo, "add", ".")
	runGit(t, home, repo, "commit", "-m", "add gitignore")

	out, err := runGGW(t, home, repo, "create", "--cow", "feature/some")
	if err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}

	ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-some")
	if _, err := os.Stat(ws); err != nil {
		t.Fatalf("workspace was not created at %s: %v", ws, err)
	}
	if branch := runGit(t, home, ws, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature/some" {
		t.Fatalf("workspace branch = %q, want feature/some", branch)
	}

	// The whole point: files git does not track come along.
	for _, rel := range []string{"node_modules/dep.js", ".env"} {
		if _, err := os.Stat(filepath.Join(ws, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("snapshot is missing untracked %s: %v", rel, err)
		}
	}

	// Its own repository, with a way back to the one it came from.
	if info, err := os.Stat(filepath.Join(ws, ".git")); err != nil || !info.IsDir() {
		t.Fatalf("snapshot .git is not a directory: %v", err)
	}
	if url := runGit(t, home, ws, "remote", "get-url", "main"); url != canonicalPath(t, repo) && url != repo {
		t.Fatalf("remote main = %q, want the source repository %q", url, repo)
	}

	// The source's worktree registrations must not have been inherited.
	if _, err := os.Stat(filepath.Join(ws, ".git", "worktrees")); !os.IsNotExist(err) {
		t.Fatalf("snapshot inherited .git/worktrees, stat err: %v", err)
	}

	assertKind(t, home, repo, canonicalPath(t, ws), "cow")

	out, err = runGGW(t, home, repo, "list")
	if err != nil {
		t.Fatalf("ggw list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "[cow]") {
		t.Fatalf("list output does not tag the snapshot:\n%s", out)
	}
}

// A snapshot must be free of the source's worktree registrations, or git in it
// refuses every branch those worktrees hold.
func TestCLICreateCoWOnABranchAlreadyCheckedOutInAWorktree(t *testing.T) {
	home, repo := setupCoWRepo(t)

	if out, err := runGGW(t, home, repo, "create", "--wt", "feature/shared"); err != nil {
		t.Fatalf("ggw create --wt failed: %v\n%s", err, out)
	}

	out, err := runGGW(t, home, repo, "create", "--cow", "feature/shared", "--as", "review")
	if err != nil {
		t.Fatalf("ggw create --cow on a checked-out branch failed: %v\n%s", err, out)
	}

	ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "review")
	if branch := runGit(t, home, ws, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature/shared" {
		t.Fatalf("workspace branch = %q, want feature/shared", branch)
	}
}

// --as is what makes two workspaces on one branch possible; each then has to
// stay individually addressable.
func TestCLICreateCoWAsGivesEachWorkspaceItsOwnName(t *testing.T) {
	home, repo := setupCoWRepo(t)

	for _, name := range []string{"", "review"} {
		args := []string{"create", "--cow", "feature/some"}
		if name != "" {
			args = append(args, "--as", name)
		}
		if out, err := runGGW(t, home, repo, args...); err != nil {
			t.Fatalf("ggw %v failed: %v\n%s", args, err, out)
		}
	}

	base := filepath.Join(home, ".local", "share", "worktrees", "acme", "api")
	for _, dir := range []string{"feature-some", "review"} {
		if _, err := os.Stat(filepath.Join(base, dir)); err != nil {
			t.Fatalf("workspace %s missing: %v", dir, err)
		}
	}

	// The shared branch name identifies neither, so it must not resolve.
	out, err := runGGW(t, home, repo, "--json", "cd", "feature/some")
	if err == nil {
		t.Fatalf("expected an ambiguous branch to refuse to resolve:\n%s", out)
	}

	// Each directory name still does.
	for _, dir := range []string{"feature-some", "review"} {
		out, err := runGGW(t, home, repo, "cd", dir)
		if err != nil {
			t.Fatalf("ggw cd %s failed: %v\n%s", dir, err, out)
		}
		if want := canonicalPath(t, filepath.Join(base, dir)); strings.TrimSpace(out) != want {
			t.Fatalf("cd %s = %q, want %q", dir, strings.TrimSpace(out), want)
		}
	}
}

// Removing a snapshot removes its branch, so work that exists nowhere else has
// to survive a plain `ggw delete`.
func TestCLIDeleteCoWRefusesToDestroyUnsavedWork(t *testing.T) {
	home, repo := setupCoWRepo(t)

	if out, err := runGGW(t, home, repo, "create", "--cow", "feature/some"); err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}
	ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-some")

	// A snapshot nobody has touched carries only inherited history, which the
	// source still holds: deleting it loses nothing.
	if out, err := runGGW(t, home, repo, "delete", "--force", "feature/some"); err != nil {
		t.Fatalf("deleting an untouched snapshot failed: %v\n%s", err, out)
	}

	if out, err := runGGW(t, home, repo, "create", "--cow", "feature/some"); err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}
	runGit(t, home, ws, "commit", "--allow-empty", "-m", "work that exists nowhere else")

	out, err := runGGW(t, home, repo, "delete", "feature/some")
	if err == nil {
		t.Fatalf("expected delete to refuse a snapshot with unpushed commits:\n%s", out)
	}
	if !strings.Contains(out, "1 commit that exists nowhere else") {
		t.Fatalf("refusal does not name what is at stake:\n%s", out)
	}
	if !strings.Contains(out, "git -C") || !strings.Contains(out, "fetch") {
		t.Fatalf("refusal does not say how to save the branch:\n%s", out)
	}
	if _, err := os.Stat(ws); err != nil {
		t.Fatalf("workspace was removed despite the refusal: %v", err)
	}

	if out, err := runGGW(t, home, repo, "delete", "--force", "feature/some"); err != nil {
		t.Fatalf("ggw delete --force failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(ws); !os.IsNotExist(err) {
		t.Fatalf("workspace still exists after --force, stat err: %v", err)
	}
}

// Uncommitted changes cannot be confirmed away when there is nobody to ask.
func TestCLIDeleteCoWRefusesDirtyWorkspaceUnderJSON(t *testing.T) {
	home, repo := setupCoWRepo(t)

	if out, err := runGGW(t, home, repo, "create", "--cow", "feature/some"); err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}
	ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-some")
	writeFile(t, filepath.Join(ws, "README.md"), "# edited\n")

	out, err := runGGW(t, home, repo, "--json", "delete", "feature/some")
	if err == nil {
		t.Fatalf("expected --json delete to refuse a dirty snapshot:\n%s", out)
	}
	if !strings.Contains(out, "uncommitted changes") {
		t.Fatalf("refusal does not name what is at stake:\n%s", out)
	}
	if _, statErr := os.Stat(ws); statErr != nil {
		t.Fatalf("workspace was removed despite the refusal: %v", statErr)
	}
}

// A snapshot inherits whatever the source had in progress, so it can be dirty
// from birth with nothing of its own at stake. A guard that fired on that would
// fire on almost every workspace and teach people to reach for --force.
func TestCLIDeleteCoWAcceptsInheritedDirtiness(t *testing.T) {
	home, repo := setupCoWRepo(t)
	writeFile(t, filepath.Join(repo, "scratch.txt"), "work in progress\n")
	writeFile(t, filepath.Join(repo, "README.md"), "# edited in the source\n")

	if out, err := runGGW(t, home, repo, "create", "--cow", "feature/some"); err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}

	out, err := runGGW(t, home, repo, "--json", "delete", "feature/some", "--force")
	if err != nil {
		t.Fatalf("deleting a snapshot of a dirty source failed: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "scratch.txt")); statErr != nil {
		t.Fatalf("the source's own work in progress was disturbed: %v", statErr)
	}
}

// Commits at risk are counted against the repository the snapshot came from,
// not against the snapshot's own refs — a second local branch is not a safe
// harbour, it lives in the very directory about to be removed.
func TestCLIDeleteCoWCountsRiskAgainstTheSourceRepository(t *testing.T) {
	home, repo := setupCoWRepo(t)

	if out, err := runGGW(t, home, repo, "create", "--cow", "feature/some"); err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}
	ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-some")
	runGit(t, home, ws, "commit", "--allow-empty", "-m", "work that exists nowhere else")

	// A second ref onto the same commit keeps it just as unreachable from
	// anywhere outside this directory.
	runGit(t, home, ws, "branch", "backup")

	out, err := runGGW(t, home, repo, "delete", "feature/some")
	if err == nil {
		t.Fatalf("a second local branch defeated the guard:\n%s", out)
	}
	if !strings.Contains(out, "1 commit that exists nowhere else") {
		t.Fatalf("refusal does not name what is at stake:\n%s", out)
	}

	// Once the branch is in the source repository, it is genuinely safe.
	runGit(t, home, repo, "fetch", ws, "feature/some:feature/some")
	if out, err := runGGW(t, home, repo, "--json", "delete", "feature/some", "--without-branch"); err != nil {
		t.Fatalf("deleting a rescued snapshot failed: %v\n%s", err, out)
	}
}

// A snapshot that never had a remote must not report its whole inherited
// history as work at risk.
func TestCLIDeleteCoWWithoutRemotesCountsOnlyItsOwnWork(t *testing.T) {
	home, repo := setupCoWRepo(t)
	runGit(t, home, repo, "commit", "--allow-empty", "-m", "second")
	runGit(t, home, repo, "commit", "--allow-empty", "-m", "third")

	if out, err := runGGW(t, home, repo, "create", "--cow", "feature/some"); err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}
	ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-some")
	runGit(t, home, ws, "remote", "remove", "origin")
	runGit(t, home, ws, "commit", "--allow-empty", "-m", "one commit of its own")

	out, err := runGGW(t, home, repo, "delete", "feature/some")
	if err == nil {
		t.Fatalf("expected delete to refuse:\n%s", out)
	}
	if !strings.Contains(out, "1 commit that exists nowhere else") {
		t.Fatalf("inherited history was counted as at risk:\n%s", out)
	}
}

// Run from inside a snapshot, ggw must still see the whole family: git alone
// answers only about the snapshot itself.
func TestCLICoWWorkspaceSeesItsSiblings(t *testing.T) {
	home, repo := setupCoWRepo(t)

	if out, err := runGGW(t, home, repo, "create", "--cow", "feature/one"); err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}
	ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-one")

	if out, err := runGGW(t, home, ws, "create", "--cow", "feature/two"); err != nil {
		t.Fatalf("creating a sibling from inside a snapshot failed: %v\n%s", err, out)
	}

	out, err := runGGW(t, home, ws, "list")
	if err != nil {
		t.Fatalf("ggw list from inside a snapshot failed: %v\n%s", err, out)
	}
	for _, want := range []string{"feature/one", "feature/two", "main"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list from inside a snapshot is missing %q:\n%s", want, out)
		}
	}

	// And a worktree created from in there belongs to the real repository.
	if out, err := runGGW(t, home, ws, "create", "--wt", "feature/three"); err != nil {
		t.Fatalf("creating a worktree from inside a snapshot failed: %v\n%s", err, out)
	}
	if got := runGit(t, home, repo, "worktree", "list"); !strings.Contains(got, "feature-three") {
		t.Fatalf("worktree was not registered in the source repository:\n%s", got)
	}
}

// A snapshot already contains what copy and symlink exist to reproduce; running
// them would only collide with what is there.
func TestCLICreateCoWSkipsCopyAndSymlinkProvisioning(t *testing.T) {
	home, repo := setupCoWRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), "node_modules/\n")
	writeFile(t, filepath.Join(repo, "node_modules", "dep.js"), "untracked\n")
	writeFile(t, filepath.Join(repo, ".env"), "SECRET=1\n")
	writeFile(t, filepath.Join(repo, ".ggw.yaml"), `copy:
  - .env
symlink:
  - node_modules
post_create:
  - echo provisioned > provisioned.txt
`)
	runGit(t, home, repo, "add", ".")
	runGit(t, home, repo, "commit", "-m", "add provisioning")

	out, err := runGGW(t, home, repo, "create", "--cow", "feature/some")
	if err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}

	ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-some")
	if _, err := os.Stat(filepath.Join(ws, "provisioned.txt")); err != nil {
		t.Fatalf("post_create did not run: %v", err)
	}
	info, err := os.Lstat(filepath.Join(ws, "node_modules"))
	if err != nil {
		t.Fatalf("node_modules missing from the snapshot: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("node_modules was symlinked; a snapshot should carry its own copy")
	}
}

// --- mode selection ---------------------------------------------------------

func TestCLIModePrecedence(t *testing.T) {
	home, repo := setupCoWRepo(t)
	base := filepath.Join(home, ".local", "share", "worktrees", "acme", "api")

	configDir := filepath.Join(home, ".config", "ggw")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(configDir, "config.yaml"), "mode: cow\n")

	// The config decides when nothing else does.
	if out, err := runGGW(t, home, repo, "create", "from/config"); err != nil {
		t.Fatalf("ggw create failed: %v\n%s", err, out)
	}
	assertKind(t, home, repo, canonicalPath(t, filepath.Join(base, "from-config")), "cow")

	// The environment beats the config.
	if out, err := runGGWWithEnv(t, home, repo, []string{"GGW_MODE=worktree"}, "create", "from/env"); err != nil {
		t.Fatalf("ggw create failed: %v\n%s", err, out)
	}
	assertKind(t, home, repo, canonicalPath(t, filepath.Join(base, "from-env")), "worktree")

	// The flag beats everything.
	if out, err := runGGWWithEnv(t, home, repo, []string{"GGW_MODE=cow"}, "create", "from/flag", "--wt"); err != nil {
		t.Fatalf("ggw create failed: %v\n%s", err, out)
	}
	assertKind(t, home, repo, canonicalPath(t, filepath.Join(base, "from-flag")), "worktree")
}

func TestCLIModeRejectsNonsense(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	out, err := runGGWWithEnv(t, home, repo, []string{"GGW_MODE=bogus"}, "create", "feature/some")
	if err == nil {
		t.Fatalf("expected an unknown GGW_MODE to fail:\n%s", out)
	}
	if !strings.Contains(out, "unknown workspace mode") {
		t.Fatalf("unhelpful error:\n%s", out)
	}

	configDir := filepath.Join(home, ".config", "ggw")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(configDir, "config.yaml"), "mode: nonsense\n")

	out, err = runGGW(t, home, repo, "create", "feature/some")
	if err == nil {
		t.Fatalf("expected an unknown config mode to fail:\n%s", out)
	}
	if !strings.Contains(out, "unknown workspace mode") {
		t.Fatalf("unhelpful error:\n%s", out)
	}
}

func TestCLIModeFlagsAreMutuallyExclusive(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	out, err := runGGW(t, home, repo, "create", "--cow", "--wt", "feature/some")
	if err == nil {
		t.Fatalf("expected --cow and --wt together to fail:\n%s", out)
	}
}

// --- helpers ----------------------------------------------------------------

// assertKind checks what `ggw list` reports for the workspace at path.
func assertKind(t *testing.T, home, repo, path, want string) {
	t.Helper()

	out, err := runGGW(t, home, repo, "--json", "list")
	if err != nil {
		t.Fatalf("ggw --json list failed: %v\n%s", err, out)
	}
	var payload struct {
		Worktrees []struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("list JSON is invalid: %v\n%s", err, out)
	}
	for _, e := range payload.Worktrees {
		if e.Path == path {
			if e.Kind != want {
				t.Fatalf("kind of %s = %q, want %q", path, e.Kind, want)
			}
			return
		}
	}
	t.Fatalf("list JSON has no entry for %s: %+v", path, payload.Worktrees)
}

// setupCoWRepo builds a home and a repository on a filesystem that can share
// blocks, skipping the test when none is available. Go's temp directory is
// frequently tmpfs, which cannot, so $HOME is tried as well.
func setupCoWRepo(t *testing.T) (string, string) {
	t.Helper()

	base := cowCapableDir(t)
	home := filepath.Join(base, "home")
	for _, dir := range []string{
		filepath.Join(home, ".config"),
		filepath.Join(home, ".githooks"),
		filepath.Join(home, ".local", "share"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create test home dir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), nil, 0o644); err != nil {
		t.Fatalf("create git config: %v", err)
	}

	repo := filepath.Join(base, "api")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	runGit(t, home, repo, "init", "-b", "main")
	writeFile(t, filepath.Join(repo, "README.md"), "# api\n")
	runGit(t, home, repo, "add", ".")
	runGit(t, home, repo, "commit", "-m", "initial commit")
	runGit(t, home, repo, "remote", "add", "origin", "git@github.com:acme/api.git")

	return home, repo
}

func cowCapableDir(t *testing.T) string {
	t.Helper()

	candidates := []string{t.TempDir()}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, home)
	}

	for _, parent := range candidates {
		dir, err := os.MkdirTemp(parent, ".ggw-cow-e2e-*")
		if err != nil {
			continue
		}
		if supportsCoW(dir) {
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			return dir
		}
		_ = os.RemoveAll(dir)
	}

	t.Skip("no filesystem available that supports copy-on-write clones")
	return ""
}

func supportsCoW(dir string) bool {
	src := filepath.Join(dir, "probe-src")
	if err := os.WriteFile(src, []byte("probe\n"), 0o644); err != nil {
		return false
	}
	defer func() { _ = os.Remove(src) }()

	dst := filepath.Join(dir, "probe-dst")
	defer func() { _ = os.Remove(dst) }()
	return cow.Clone(src, dst) == nil
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// `ggw pr --cow` snapshots first and lets `gh pr checkout` choose the branch,
// so the marker and the source remote have to be written after the fact.
func TestCLIPRCreatesCoWWorkspaceViaGH(t *testing.T) {
	home, repo := setupCoWRepo(t)
	binDir := t.TempDir()
	fakeGH := filepath.Join(binDir, "gh")

	script := `#!/bin/sh
if [ "$1" != "pr" ] || [ "$2" != "checkout" ] || [ "$3" != "123" ]; then
	echo "unexpected gh args: $*" >&2
	exit 1
fi
git checkout -b contributor/feature
`
	if err := os.WriteFile(fakeGH, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}

	pathEnv := "PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	out, err := runGGWWithEnv(t, home, repo, []string{pathEnv}, "pr", "123", "--cow")
	if err != nil {
		t.Fatalf("ggw pr --cow failed: %v\n%s", err, out)
	}

	ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "pr-123")
	if branch := runGit(t, home, ws, "rev-parse", "--abbrev-ref", "HEAD"); branch != "contributor/feature" {
		t.Fatalf("workspace branch = %q, want contributor/feature", branch)
	}
	if url := runGit(t, home, ws, "remote", "get-url", "main"); url != canonicalPath(t, repo) && url != repo {
		t.Fatalf("remote main = %q, want the source repository %q", url, repo)
	}
	assertKind(t, home, repo, canonicalPath(t, ws), "cow")
}

// A snapshot whose source repository is gone must still list its siblings and
// say something useful when asked to do what needs the source.
func TestCLICoWWorkspaceSurvivesTheSourceRepository(t *testing.T) {
	home, repo := setupCoWRepo(t)

	if out, err := runGGW(t, home, repo, "create", "--cow", "feature/one"); err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}
	if out, err := runGGW(t, home, repo, "create", "--cow", "feature/two"); err != nil {
		t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
	}
	ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-one")

	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}

	out, err := runGGW(t, home, ws, "list")
	if err != nil {
		t.Fatalf("ggw list without the source repository failed: %v\n%s", err, out)
	}
	for _, want := range []string{"feature/one", "feature/two"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list is missing %q:\n%s", want, out)
		}
	}

	out, err = runGGW(t, home, ws, "create", "--cow", "feature/three")
	if err == nil {
		t.Fatalf("expected create to fail without the source repository:\n%s", out)
	}
	if !strings.Contains(out, "is gone") {
		t.Fatalf("unhelpful error:\n%s", out)
	}
}

// Every command resolves the repository from the current directory, and a
// linked worktree is one of the places that directory can be.
func TestCLIRunsFromInsideALinkedWorktree(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)

	if out, err := runGGW(t, home, repo, "create", "--wt", "feature/one"); err != nil {
		t.Fatalf("ggw create failed: %v\n%s", err, out)
	}
	wt := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-one")

	out, err := runGGW(t, home, wt, "list")
	if err != nil {
		t.Fatalf("ggw list from inside a worktree failed: %v\n%s", err, out)
	}
	for _, want := range []string{"main", "feature/one"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list from inside a worktree is missing %q:\n%s", want, out)
		}
	}

	if out, err := runGGW(t, home, wt, "create", "--wt", "feature/two"); err != nil {
		t.Fatalf("ggw create from inside a worktree failed: %v\n%s", err, out)
	}

	// It is also the one workspace this invocation must refuse to remove.
	out, err = runGGW(t, home, wt, "delete", "--force", "feature/one")
	if err == nil {
		t.Fatalf("expected delete to refuse the current workspace:\n%s", out)
	}
	if _, statErr := os.Stat(wt); statErr != nil {
		t.Fatalf("current workspace was removed: %v", statErr)
	}
}

// Reached through a symlink, a workspace is still the one you are standing in.
func TestCLIDeleteRefusesTheCurrentWorkspaceBehindASymlink(t *testing.T) {
	home, repo := setupCoWRepo(t)

	real := filepath.Join(filepath.Dir(repo), "real-workspaces")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(repo), "linked-workspaces")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	configDir := filepath.Join(home, ".config", "ggw")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(configDir, "config.yaml"), "base_dir: "+link+"\n")

	workspaces := map[string]string{"--cow": "snapshot", "--wt": "tree"}
	for flag, name := range workspaces {
		if out, err := runGGW(t, home, repo, "create", flag, name); err != nil {
			t.Fatalf("ggw create %s failed: %v\n%s", flag, err, out)
		}
		if _, err := os.Stat(filepath.Join(real, "acme", "api", name)); err != nil {
			t.Fatalf("workspace %s was not created through the symlink: %v", name, err)
		}
	}

	for _, name := range workspaces {
		// Entered through the symlink, so the path ggw computes and the one git
		// reports differ unless one of them is resolved.
		ws := filepath.Join(link, "acme", "api", name)
		out, err := runGGW(t, home, ws, "delete", "--force", name)
		if err == nil {
			t.Fatalf("expected delete to refuse the current workspace %s:\n%s", name, out)
		}
		if !strings.Contains(out, "current workspace") {
			t.Fatalf("unexpected error deleting %s:\n%s", name, out)
		}
		if _, statErr := os.Stat(filepath.Join(real, "acme", "api", name)); statErr != nil {
			t.Fatalf("current workspace %s was removed: %v", name, statErr)
		}
	}
}

// Moved out of the layout directory, a snapshot is discoverable from nowhere
// but itself — and standing in a workspace ggw cannot name is worse than
// showing it.
func TestCLIListIncludesTheWorkspaceYouAreStandingIn(t *testing.T) {
	home, repo := setupCoWRepo(t)

	for _, branch := range []string{"feature/stays", "feature/moves"} {
		if out, err := runGGW(t, home, repo, "create", "--cow", branch); err != nil {
			t.Fatalf("ggw create --cow failed: %v\n%s", err, out)
		}
	}

	moved := filepath.Join(filepath.Dir(repo), "moved-out")
	if err := os.Rename(filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-moves"), moved); err != nil {
		t.Fatal(err)
	}

	out, err := runGGW(t, home, moved, "--json", "list")
	if err != nil {
		t.Fatalf("ggw list from a moved snapshot failed: %v\n%s", err, out)
	}
	var payload struct {
		Worktrees []struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
			Main bool   `json:"main"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("list JSON is invalid: %v\n%s", err, out)
	}

	var found bool
	for _, e := range payload.Worktrees {
		if e.Path != canonicalPath(t, moved) {
			continue
		}
		found = true
		if e.Kind != "cow" {
			t.Fatalf("moved snapshot reported as kind %q, want cow", e.Kind)
		}
		if e.Main {
			t.Fatal("moved snapshot reported as the main worktree")
		}
	}
	if !found {
		t.Fatalf("list does not include the workspace it was run from: %+v", payload.Worktrees)
	}

	// And it must still be deletable, which "cannot delete the main worktree"
	// would have prevented.
	if out, err := runGGW(t, home, repo, "--json", "list"); err != nil {
		t.Fatalf("ggw list from the repo failed: %v\n%s", err, out)
	} else if strings.Contains(out, canonicalPath(t, moved)) {
		t.Fatalf("a workspace outside the layout must not show up from elsewhere:\n%s", out)
	}
}
