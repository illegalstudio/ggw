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
