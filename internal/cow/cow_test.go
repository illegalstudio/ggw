package cow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSourceAcceptsARepository(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".git"))

	if err := ValidateSource(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// A linked worktree's .git is a file pointing into the original repository, so
// a copy of it would be a shell with no objects of its own.
func TestValidateSourceRejectsLinkedWorktree(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".git"), "gitdir: /elsewhere/.git/worktrees/x\n")

	err := ValidateSource(dir)
	if err == nil {
		t.Fatal("expected a linked worktree to be rejected")
	}
	if !strings.Contains(err.Error(), "linked worktree") {
		t.Fatalf("unhelpful error: %v", err)
	}
}

func TestValidateSourceRejectsNonRepository(t *testing.T) {
	if err := ValidateSource(t.TempDir()); err == nil {
		t.Fatal("expected a directory without .git to be rejected")
	}
}

// The inherited registrations are what make git in a snapshot refuse to check
// out branches that belong to the source's worktrees.
func TestSanitizeRemovesInheritedState(t *testing.T) {
	ws := t.TempDir()
	gitDir := filepath.Join(ws, ".git")
	mustMkdir(t, filepath.Join(gitDir, "worktrees", "feature-x"))
	mustWrite(t, filepath.Join(gitDir, "worktrees", "feature-x", "gitdir"), "/elsewhere\n")
	mustWrite(t, filepath.Join(gitDir, "index.lock"), "")
	mustWrite(t, filepath.Join(gitDir, "HEAD"), "ref: refs/heads/main\n")

	if err := Sanitize(ws); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(gitDir, "worktrees")); !os.IsNotExist(err) {
		t.Fatal("inherited worktree registrations survived Sanitize")
	}
	if _, err := os.Stat(filepath.Join(gitDir, "index.lock")); !os.IsNotExist(err) {
		t.Fatal("inherited index lock survived Sanitize")
	}
	if _, err := os.Stat(filepath.Join(gitDir, "HEAD")); err != nil {
		t.Fatalf("Sanitize removed more than it should: %v", err)
	}
}

// Sanitize runs on every snapshot, including the overwhelming majority that
// inherit neither of those files.
func TestSanitizeIsAContentedNoOp(t *testing.T) {
	ws := t.TempDir()
	mustMkdir(t, filepath.Join(ws, ".git"))

	if err := Sanitize(ws); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkerRoundTrip(t *testing.T) {
	ws := t.TempDir()
	mustMkdir(t, filepath.Join(ws, ".git"))

	if Is(ws) {
		t.Fatal("an unmarked directory must not look like a workspace")
	}

	if err := Write(ws, Marker{
		SourceRepo: "/repos/api",
		Org:        "acme",
		Repo:       "api",
		Branch:     "feature/x",
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !Is(ws) {
		t.Fatal("a marked directory must be recognised as a workspace")
	}

	got, ok, err := Read(ws)
	if err != nil || !ok {
		t.Fatalf("Read: ok=%v err=%v", ok, err)
	}
	if got.Kind != "cow" || got.Version != MarkerVersion || got.Backend != Backend {
		t.Fatalf("Write did not stamp the marker: %+v", got)
	}
	if got.SourceRepo != "/repos/api" || got.Org != "acme" || got.Repo != "api" || got.Branch != "feature/x" {
		t.Fatalf("round trip lost data: %+v", got)
	}
	if got.CreatedAt == "" {
		t.Fatal("Write did not stamp created_at")
	}
}

func TestReadReportsMissingMarkerWithoutError(t *testing.T) {
	ws := t.TempDir()
	mustMkdir(t, filepath.Join(ws, ".git"))

	m, ok, err := Read(ws)
	if err != nil || ok || m != nil {
		t.Fatalf("got (%v, %v, %v), want (nil, false, nil)", m, ok, err)
	}
}

// A marker that exists but cannot be parsed is a real problem; treating the
// directory as a plain worktree would quietly hide it.
func TestReadRejectsCorruptMarker(t *testing.T) {
	ws := t.TempDir()
	mustMkdir(t, filepath.Join(ws, ".git"))
	mustWrite(t, MarkerPath(ws), "{not json")

	if _, ok, err := Read(ws); err == nil {
		t.Fatalf("expected an error for a corrupt marker (ok=%v)", ok)
	}
}

func TestCloneCarriesEveryFileAndSharesBlocks(t *testing.T) {
	base := reflinkDir(t)

	src := filepath.Join(base, "src")
	mustMkdir(t, filepath.Join(src, ".git"))
	mustMkdir(t, filepath.Join(src, "node_modules"))
	mustWrite(t, filepath.Join(src, ".git", "HEAD"), "ref: refs/heads/main\n")
	mustWrite(t, filepath.Join(src, "node_modules", "dep.js"), "untracked\n")
	mustWrite(t, filepath.Join(src, ".env"), "SECRET=1\n")

	dst := filepath.Join(base, "dst")
	if err := Clone(src, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	for _, rel := range []string{".git/HEAD", "node_modules/dep.js", ".env"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("snapshot is missing %s: %v", rel, err)
		}
	}

	// The point of a snapshot is that the two sides are independent copies,
	// not shared files.
	mustWrite(t, filepath.Join(dst, ".env"), "SECRET=2\n")
	if got := read(t, filepath.Join(src, ".env")); got != "SECRET=1\n" {
		t.Fatalf("writing to the snapshot changed the source: %q", got)
	}
}

func TestCloneRefusesAnExistingDestination(t *testing.T) {
	base := reflinkDir(t)
	src := filepath.Join(base, "src")
	mustMkdir(t, src)
	dst := filepath.Join(base, "dst")
	mustMkdir(t, dst)

	if err := Clone(src, dst); err == nil {
		t.Fatal("expected Clone to refuse an existing destination")
	}
}

// reflinkDir returns a directory on a filesystem that can share blocks, or
// skips the test. Temporary directories are often on tmpfs, which cannot, so
// $HOME is tried as well before giving up.
func reflinkDir(t *testing.T) string {
	t.Helper()
	if cloneName == "" {
		t.Skipf("copy-on-write is not supported on this platform")
	}

	candidates := []string{t.TempDir()}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, home)
	}

	for _, parent := range candidates {
		dir, err := os.MkdirTemp(parent, ".ggw-cow-test-*")
		if err != nil {
			continue
		}
		if supportsReflink(dir) {
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			return dir
		}
		_ = os.RemoveAll(dir)
	}

	t.Skip("no filesystem available that supports copy-on-write clones")
	return ""
}

func supportsReflink(dir string) bool {
	src := filepath.Join(dir, "probe-src")
	if err := os.WriteFile(src, []byte("probe\n"), 0o644); err != nil {
		return false
	}
	defer func() { _ = os.Remove(src) }()

	dst := filepath.Join(dir, "probe-dst")
	defer func() { _ = os.Remove(dst) }()
	return Clone(src, dst) == nil
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
