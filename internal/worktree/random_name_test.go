package worktree

import (
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRandomNameFormat(t *testing.T) {
	adjIdx := indexOf(adjectives, "intelligent")
	nounIdx := indexOf(nouns, "elephant")
	calls := 0
	randIntN = func(n int) int {
		calls++
		if calls%2 == 1 {
			return adjIdx
		}
		return nounIdx
	}
	t.Cleanup(func() { randIntN = rand.IntN })

	if got := RandomName(); got != "intelligent-elephant" {
		t.Fatalf("RandomName() = %q, want intelligent-elephant", got)
	}
}

func TestRandomNameShape(t *testing.T) {
	randIntN = rand.IntN
	t.Cleanup(func() { randIntN = rand.IntN })

	re := regexp.MustCompile(`^[a-z]+-[a-z]+$`)
	seen := map[string]bool{}
	for range 50 {
		name := RandomName()
		if !re.MatchString(name) {
			t.Fatalf("RandomName() = %q, want adjective-noun", name)
		}
		parts := strings.Split(name, "-")
		if len(parts) != 2 {
			t.Fatalf("expected 2 parts, got %q", name)
		}
		if !contains(adjectives, parts[0]) {
			t.Fatalf("adjective %q not in list", parts[0])
		}
		if !contains(nouns, parts[1]) {
			t.Fatalf("noun %q not in list", parts[1])
		}
		seen[name] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected some variety in RandomName, got only %v", seen)
	}
}

func TestUniqueRandomNameSkipsTakenNames(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "README")
	runGit(t, repo, "commit", "-m", "init")
	runGit(t, repo, "branch", "busy-bee")

	// Force candidates: first busy-bee (taken), then clever-cat (free).
	names := []string{"busy-bee", "clever-cat"}
	call := 0
	randIntN = func(n int) int {
		name := names[call/2]
		part := call % 2
		call++
		if part == 0 {
			return indexOf(adjectives, strings.Split(name, "-")[0])
		}
		return indexOf(nouns, strings.Split(name, "-")[1])
	}
	t.Cleanup(func() { randIntN = rand.IntN })

	// Isolate path checks from the developer's real config/home.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	got, err := UniqueRandomName(repo, "acme", "api")
	if err != nil {
		t.Fatalf("UniqueRandomName: %v", err)
	}
	if got != "clever-cat" {
		t.Fatalf("got %q, want clever-cat (busy-bee should be skipped)", got)
	}
}

func TestUniqueRandomNameSkipsExistingPath(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "README")
	runGit(t, repo, "commit", "-m", "init")

	home := t.TempDir()
	t.Setenv("HOME", home)
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	// Pre-create destination for first candidate.
	dest := filepath.Join(xdg, "worktrees", "acme", "api", "busy-bee")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	names := []string{"busy-bee", "clever-cat"}
	call := 0
	randIntN = func(n int) int {
		name := names[call/2]
		part := call % 2
		call++
		if part == 0 {
			return indexOf(adjectives, strings.Split(name, "-")[0])
		}
		return indexOf(nouns, strings.Split(name, "-")[1])
	}
	t.Cleanup(func() { randIntN = rand.IntN })

	got, err := UniqueRandomName(repo, "acme", "api")
	if err != nil {
		t.Fatalf("UniqueRandomName: %v", err)
	}
	if got != "clever-cat" {
		t.Fatalf("got %q, want clever-cat", got)
	}
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	panic("not found: " + s)
}
