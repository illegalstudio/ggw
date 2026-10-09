package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLICoWParentImportsOnlyCommits(t *testing.T) {
	for _, mode := range []string{"wt", "cow"} {
		t.Run(mode, func(t *testing.T) {
			home, repo := setupCoWRepo(t)
			parent := createCowParent(t, home, repo, "parent")
			writeFile(t, filepath.Join(parent, "feature.txt"), "committed parent feature\n")
			runGit(t, home, parent, "add", "feature.txt")
			runGit(t, home, parent, "commit", "-m", "parent feature")
			parentSHA := runGit(t, home, parent, "rev-parse", "HEAD")
			runGit(t, home, parent, "tag", "parent-only-tag")
			writeFile(t, filepath.Join(parent, "README.md"), "uncommitted parent changes\n")
			writeFile(t, filepath.Join(parent, "parent-only.txt"), "untracked parent file\n")
			writeFile(t, filepath.Join(parent, ".env"), "from parent\n")
			writeFile(t, filepath.Join(repo, ".env"), "from main repository\n")
			writeFile(t, filepath.Join(repo, ".git", "FETCH_HEAD"), "preserve fetch state\n")
			mainSHA := runGit(t, home, repo, "rev-parse", "HEAD")
			parentStatus := runGit(t, home, parent, "status", "--porcelain")

			// Run from the original repository to exercise workspace discovery.
			if out, err := runGGW(t, home, repo, "create", "--"+mode, "child", "--from", "refs/heads/parent"); err != nil {
				t.Fatalf("create child failed: %v\n%s", err, out)
			}
			child := filepath.Join(filepath.Dir(parent), "child")
			if got := runGit(t, home, child, "rev-parse", "HEAD"); got != parentSHA {
				t.Fatalf("child HEAD = %s, want %s", got, parentSHA)
			}
			assertPRBase(t, home, child, "child", "parent")
			assertFileContents(t, filepath.Join(child, "feature.txt"), "committed parent feature\n")
			assertFileContents(t, filepath.Join(child, "README.md"), "# api\n")
			if _, err := os.Stat(filepath.Join(child, "parent-only.txt")); !os.IsNotExist(err) {
				t.Fatalf("child inherited an untracked parent file: %v", err)
			}
			if mode == "cow" {
				assertFileContents(t, filepath.Join(child, ".env"), "from main repository\n")
				assertFileContents(t, filepath.Join(child, ".git", "FETCH_HEAD"), "preserve fetch state\n")
				assertPRBase(t, home, repo, "child", "")
			}
			if _, err := runGitCommand(t, home, child, "show-ref", "--verify", "refs/tags/parent-only-tag"); err == nil {
				t.Fatal("fetch imported an unrelated parent tag")
			}
			assertFileContents(t, filepath.Join(repo, ".git", "FETCH_HEAD"), "preserve fetch state\n")
			if got := runGit(t, home, repo, "rev-parse", "HEAD"); got != mainSHA {
				t.Fatalf("original HEAD changed to %s", got)
			}
			if got := runGit(t, home, parent, "status", "--porcelain"); got != parentStatus {
				t.Fatalf("parent work changed: %q, want %q", got, parentStatus)
			}
			if _, err := runGitCommand(t, home, repo, "show-ref", "--verify", "refs/heads/parent"); err == nil {
				t.Fatal("parent branch was imported into the original repository")
			}

			// A second child must use the parent's new tip, not a cached import.
			runGit(t, home, parent, "commit", "--allow-empty", "-m", "next parent commit")
			latest := runGit(t, home, parent, "rev-parse", "HEAD")
			if out, err := runGGW(t, home, repo, "create", "--"+mode, "child2", "--from", "parent"); err != nil {
				t.Fatalf("create second child failed: %v\n%s", err, out)
			}
			if got := runGit(t, home, filepath.Join(filepath.Dir(parent), "child2"), "rev-parse", "HEAD"); got != latest {
				t.Fatalf("second child HEAD = %s, want latest %s", got, latest)
			}
		})
	}
}

func TestCLICoWParentAmbiguity(t *testing.T) {
	home, repo := setupCoWRepo(t)
	parent := createCowParent(t, home, repo, "parent")
	if out, err := runGGW(t, home, repo, "create", "--cow", "parent", "--as", "other-parent"); err != nil {
		t.Fatalf("create duplicate parent failed: %v\n%s", err, out)
	}
	runGit(t, home, parent, "commit", "--allow-empty", "-m", "choose this parent")
	parentSHA := runGit(t, home, parent, "rev-parse", "HEAD")
	for _, mode := range []string{"wt", "cow"} {
		branch := "child-" + mode
		args := []string{"--json", "create", "--" + mode, branch, "--from", "parent"}
		if out, err := runGGW(t, home, repo, args...); err == nil || !strings.Contains(out, "multiple copy-on-write workspaces") {
			t.Fatalf("ambiguous parent should fail: %v\n%s", err, out)
		}
		child := filepath.Join(filepath.Dir(parent), branch)
		if _, err := os.Stat(child); !os.IsNotExist(err) {
			t.Fatalf("ambiguous create left a workspace: %v", err)
		}
		if out, err := runGGW(t, home, parent, args...); err != nil {
			t.Fatalf("create from explicit current parent failed: %v\n%s", err, out)
		}
		if got := runGit(t, home, child, "rev-parse", "HEAD"); got != parentSHA {
			t.Fatalf("wrong parent selected: %s, want %s", got, parentSHA)
		}
	}
}

func TestCLICoWParentRefPrecedence(t *testing.T) {
	for _, mode := range []string{"wt", "cow"} {
		t.Run(mode, func(t *testing.T) {
			home, repo := setupCoWRepo(t)
			runGit(t, home, repo, "branch", "parent")
			original := runGit(t, home, repo, "rev-parse", "parent")
			parent := createCowParent(t, home, repo, "parent")
			runGit(t, home, parent, "commit", "--allow-empty", "-m", "cow parent diverges")
			updated := runGit(t, home, parent, "rev-parse", "HEAD")
			for _, tc := range []struct{ cwd, branch, want string }{
				{repo, "from-original", original},
				{parent, "from-cow", updated},
			} {
				if out, err := runGGW(t, home, tc.cwd, "create", "--"+mode, tc.branch, "--from", "parent"); err != nil {
					t.Fatalf("create %s failed: %v\n%s", tc.branch, err, out)
				}
				ws := filepath.Join(filepath.Dir(parent), tc.branch)
				if got := runGit(t, home, ws, "rev-parse", "HEAD"); got != tc.want {
					t.Fatalf("%s HEAD = %s, want %s", tc.branch, got, tc.want)
				}
				assertPRBase(t, home, ws, tc.branch, "parent")
			}
			if got := runGit(t, home, repo, "rev-parse", "parent"); got != original {
				t.Fatalf("original parent branch moved to %s", got)
			}
		})
	}
}

func TestCLICoWParentFetchFailure(t *testing.T) {
	for _, mode := range []string{"wt", "cow"} {
		t.Run(mode, func(t *testing.T) {
			home, repo := setupCoWRepo(t)
			parent := createCowParent(t, home, repo, "parent")
			runGit(t, home, parent, "commit", "--allow-empty", "-m", "parent feature")
			runGit(t, home, repo, "config", "protocol.file.allow", "never")
			out, err := runGGW(t, home, repo, "create", "--"+mode, "child", "--from", "parent")
			if err == nil || !strings.Contains(out, "fetch local parent commit") {
				t.Fatalf("expected local fetch failure: %v\n%s", err, out)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(parent), "child")); !os.IsNotExist(err) {
				t.Fatalf("fetch failure left a workspace: %v", err)
			}
			if _, err := runGitCommand(t, home, repo, "show-ref", "--verify", "refs/heads/child"); err == nil {
				t.Fatal("fetch failure created the child branch")
			}
		})
	}
}

func TestCLICoWParentSameBranchDoesNotSetItselfAsPRBase(t *testing.T) {
	home, repo := setupCoWRepo(t)
	parent := createCowParent(t, home, repo, "parent")
	runGit(t, home, parent, "commit", "--allow-empty", "-m", "parent feature")
	if out, err := runGGW(t, home, parent, "create", "--cow", "parent", "--as", "second-parent", "--from", "parent"); err != nil {
		t.Fatalf("create second workspace on parent failed: %v\n%s", err, out)
	}
	second := filepath.Join(filepath.Dir(parent), "second-parent")
	assertPRBase(t, home, second, "parent", "")
	if got, want := runGit(t, home, second, "rev-parse", "HEAD"), runGit(t, home, parent, "rev-parse", "HEAD"); got != want {
		t.Fatalf("second workspace HEAD = %s, want %s", got, want)
	}
}

func TestCLICoWParentStackedHistory(t *testing.T) {
	for _, mode := range []string{"wt", "cow"} {
		t.Run(mode, func(t *testing.T) {
			home, repo := setupCoWRepo(t)
			parent := createCowParent(t, home, repo, "parent")
			writeFile(t, filepath.Join(parent, "parent.txt"), "parent feature\n")
			runGit(t, home, parent, "add", "parent.txt")
			runGit(t, home, parent, "commit", "-m", "parent feature")
			if out, err := runGGW(t, home, repo, "create", "--"+mode, "child", "--from", "parent"); err != nil {
				t.Fatalf("create child failed: %v\n%s", err, out)
			}
			child := filepath.Join(filepath.Dir(parent), "child")
			writeFile(t, filepath.Join(child, "child.txt"), "child feature\n")
			runGit(t, home, child, "add", "child.txt")
			runGit(t, home, child, "commit", "-m", "child feature")

			// Model publishing both independent branches using a local bare repo.
			remote := t.TempDir()
			runGit(t, home, remote, "init", "--bare")
			runGit(t, home, repo, "push", remote, "main")
			runGit(t, home, parent, "push", remote, "parent")
			runGit(t, home, child, "push", remote, "child")
			if got := runGit(t, home, remote, "diff", "--name-only", "main...parent"); got != "parent.txt" {
				t.Fatalf("parent PR diff = %q, want parent.txt", got)
			}
			if got := runGit(t, home, remote, "diff", "--name-only", "parent...child"); got != "child.txt" {
				t.Fatalf("child PR diff = %q, want only child.txt", got)
			}
			assertPRBase(t, home, child, "child", "parent")
		})
	}
}

func createCowParent(t *testing.T, home, repo, branch string) string {
	t.Helper()
	if out, err := runGGW(t, home, repo, "create", "--cow", branch); err != nil {
		t.Fatalf("create cow parent failed: %v\n%s", err, out)
	}
	return filepath.Join(home, ".local", "share", "worktrees", "acme", "api", branch)
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s = %q (%v), want %q", path, got, err, want)
	}
}
