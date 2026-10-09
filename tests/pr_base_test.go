package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLICreatePRBase(t *testing.T) {
	for _, mode := range []string{"wt", "cow"} {
		t.Run(mode, func(t *testing.T) {
			for _, tc := range []struct {
				name     string
				from     string
				preset   string
				existing string
				want     string
			}{
				{name: "unpublished local branch", from: "feature/parent", want: "feature/parent"},
				{name: "full local ref", from: "refs/heads/feature/parent", want: "feature/parent"},
				{name: "remote branch", from: "origin/feature/parent", want: "feature/parent"},
				{name: "full remote ref", from: "refs/remotes/origin/feature/parent", want: "feature/parent"},
				{name: "remote with slash", from: "team/origin/feature/parent", want: "feature/parent"},
				{name: "tag", from: "v1"},
				{name: "annotated tag", from: "v2"},
				{name: "branch sharing a tag name", from: "refs/heads/ambiguous", want: "ambiguous"},
				{name: "commit", from: "<sha>"},
				{name: "commit expression", from: "feature/parent~0"},
				{name: "explicit HEAD", from: "HEAD"},
				{name: "remote HEAD", from: "origin/HEAD"},
				{name: "implicit HEAD"},
				{name: "preserve configured base", from: "feature/parent", preset: "custom-base", want: "custom-base"},
				{name: "existing branch", from: "missing", existing: "local"},
				{name: "existing branch with base", from: "feature/parent", existing: "local", preset: "custom-base", want: "custom-base"},
				{name: "existing remote branch", from: "feature/parent", existing: "remote"},
				{name: "existing remote branch with base", from: "feature/parent", existing: "remote", preset: "custom-base", want: "custom-base"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					home, repo := setupPRBaseRepo(t, mode)
					mainSHA := runGit(t, home, repo, "rev-parse", "HEAD")
					runGit(t, home, repo, "checkout", "-b", "feature/parent")
					runGit(t, home, repo, "commit", "--allow-empty", "-m", "parent feature")
					parentSHA := runGit(t, home, repo, "rev-parse", "HEAD")
					runGit(t, home, repo, "checkout", "main")
					runGit(t, home, repo, "tag", "v1", parentSHA)
					runGit(t, home, repo, "tag", "-a", "v2", "-m", "release", parentSHA)
					if tc.from == "refs/heads/ambiguous" {
						runGit(t, home, repo, "tag", "ambiguous", parentSHA)
						runGit(t, home, repo, "branch", "ambiguous", parentSHA)
					}
					if strings.Contains(tc.from, "origin/") {
						runGit(t, home, repo, "update-ref", "refs/remotes/origin/feature/parent", parentSHA)
						runGit(t, home, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/feature/parent")
						runGit(t, home, repo, "remote", "add", "team/origin", "git@github.com:acme/api.git")
						runGit(t, home, repo, "update-ref", "refs/remotes/team/origin/feature/parent", parentSHA)
					}

					const branch = "feature/child.v2"
					const key = "branch." + branch + ".gh-merge-base"
					switch tc.existing {
					case "local":
						runGit(t, home, repo, "branch", branch)
					case "remote":
						runGit(t, home, repo, "update-ref", "refs/remotes/origin/"+branch, mainSHA)
					}
					if tc.preset != "" {
						runGit(t, home, repo, "config", key, tc.preset)
					}
					from := tc.from
					if from == "<sha>" {
						from = parentSHA
					}
					args := []string{"create", "--" + mode, branch}
					if from != "" {
						args = append(args, "--from", from)
					}
					if out, err := runGGW(t, home, repo, args...); err != nil {
						t.Fatalf("ggw %v failed: %v\n%s", args, err, out)
					}
					ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "feature-child-v2")
					assertPRBase(t, home, ws, branch, tc.want)
					wantSHA := parentSHA
					if from == "" || from == "HEAD" || tc.existing != "" {
						wantSHA = mainSHA
					}
					if got := runGit(t, home, ws, "rev-parse", "HEAD"); got != wantSHA {
						t.Fatalf("HEAD = %s, want %s", got, wantSHA)
					}
					if mode == "cow" {
						assertPRBase(t, home, repo, branch, tc.preset)
					}
				})
			}
		})
	}
}

func TestCLICreatePRBaseFromParentWorktree(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)
	if out, err := runGGW(t, home, repo, "create", "--wt", "parent", "--from", "main"); err != nil {
		t.Fatalf("create parent failed: %v\n%s", err, out)
	}
	baseDir := filepath.Join(home, ".local", "share", "worktrees", "acme", "api")
	parent := filepath.Join(baseDir, "parent")
	runGit(t, home, parent, "commit", "--allow-empty", "-m", "parent feature")
	parentSHA := runGit(t, home, parent, "rev-parse", "HEAD")
	if out, err := runGGW(t, home, parent, "create", "--wt", "child", "--from", "parent"); err != nil {
		t.Fatalf("create child from parent worktree failed: %v\n%s", err, out)
	}
	child := filepath.Join(baseDir, "child")
	assertPRBase(t, home, parent, "parent", "main")
	assertPRBase(t, home, child, "child", "parent")
	if got := runGit(t, home, child, "rev-parse", "HEAD"); got != parentSHA {
		t.Fatalf("child starts at %s, want parent commit %s", got, parentSHA)
	}
}

func TestCLICreatePRBaseFromCoWParent(t *testing.T) {
	for _, mode := range []string{"wt", "cow"} {
		t.Run(mode, func(t *testing.T) {
			home, repo := setupCoWRepo(t)
			if out, err := runGGW(t, home, repo, "create", "--cow", "parent", "--from", "main"); err != nil {
				t.Fatalf("create cow parent failed: %v\n%s", err, out)
			}
			baseDir := filepath.Join(home, ".local", "share", "worktrees", "acme", "api")
			parent := filepath.Join(baseDir, "parent")
			runGit(t, home, parent, "commit", "--allow-empty", "-m", "parent feature")
			parentSHA := runGit(t, home, parent, "rev-parse", "HEAD")
			args := []string{"create", "--" + mode, "child", "--from", "parent"}
			child := filepath.Join(baseDir, "child")
			if out, err := runGGW(t, home, parent, args...); err != nil {
				t.Fatalf("create child from the cow parent failed: %v\n%s", err, out)
			}
			assertPRBase(t, home, child, "child", "parent")
			if got := runGit(t, home, child, "rev-parse", "HEAD"); got != parentSHA {
				t.Fatalf("child starts at %s, want cow parent commit %s", got, parentSHA)
			}
			if _, err := runGitCommand(t, home, repo, "show-ref", "--verify", "refs/heads/parent"); err == nil {
				t.Fatal("creating child unexpectedly imported the parent branch into the original repository")
			}
		})
	}
}

func TestCLICreatePRBaseInvalidFrom(t *testing.T) {
	for _, mode := range []string{"wt", "cow"} {
		t.Run(mode, func(t *testing.T) {
			home, repo := setupPRBaseRepo(t, mode)
			if out, err := runGGW(t, home, repo, "create", "--"+mode, "child", "--from", "missing"); err == nil {
				t.Fatalf("create from a missing ref succeeded:\n%s", out)
			}
			assertPRBase(t, home, repo, "child", "")
			ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "child")
			if _, err := os.Stat(ws); !os.IsNotExist(err) {
				t.Fatalf("failed create left a workspace: %v", err)
			}
		})
	}
}

func TestCLICreatePRBaseWriteFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell checkout hook")
	}
	for _, mode := range []string{"wt", "cow"} {
		t.Run(mode, func(t *testing.T) {
			home, repo := setupPRBaseRepo(t, mode)
			hooks := filepath.Join(home, ".githooks")
			runGit(t, home, repo, "config", "core.hooksPath", hooks)
			hook := "#!/bin/sh\nlock=$(git rev-parse --git-path config.lock)\n: > \"$lock\"\n"
			if err := os.WriteFile(filepath.Join(hooks, "post-checkout"), []byte(hook), 0o755); err != nil {
				t.Fatal(err)
			}
			out, err := runGGW(t, home, repo, "create", "--"+mode, "child", "--from", "main")
			if err == nil || !strings.Contains(out, "save PR base") {
				t.Fatalf("expected PR base write failure: %v\n%s", err, out)
			}
			ws := filepath.Join(home, ".local", "share", "worktrees", "acme", "api", "child")
			if _, err := os.Stat(ws); !os.IsNotExist(err) {
				t.Fatalf("failed create left a workspace: %v", err)
			}
			assertPRBase(t, home, repo, "child", "")
			if got := runGit(t, home, repo, "branch", "--show-current"); got != "main" {
				t.Fatalf("source branch changed to %q", got)
			}
			if mode == "wt" {
				if out := runGit(t, home, repo, "worktree", "list", "--porcelain"); strings.Contains(out, "refs/heads/child") {
					t.Fatalf("failed create left a worktree registration:\n%s", out)
				}
			}
		})
	}
}

func setupPRBaseRepo(t *testing.T, mode string) (string, string) {
	t.Helper()
	if mode == "cow" {
		return setupCoWRepo(t)
	}
	home := setupHome(t)
	return home, initGitRepo(t, home)
}

func assertPRBase(t *testing.T, home, repo, branch, want string) {
	t.Helper()
	out, err := runGitCommand(t, home, repo, "config", "--get", "branch."+branch+".gh-merge-base")
	if want == "" {
		if err == nil || strings.TrimSpace(out) != "" {
			t.Fatalf("expected no PR base, got %q (%v)", out, err)
		}
	} else if err != nil || strings.TrimSpace(out) != want {
		t.Fatalf("PR base = %q (%v), want %q", out, err, want)
	}
}
