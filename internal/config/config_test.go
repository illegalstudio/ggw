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

func TestLoadAbsoluteBaseDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeConfig(t, home, "base_dir: /data/wt\n")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseDir != "/data/wt" {
		t.Fatalf("got %q, want %q", cfg.BaseDir, "/data/wt")
	}
}

func TestLoadTildeAloneExpandsToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeConfig(t, home, "base_dir: \"~\"\n")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseDir != home {
		t.Fatalf("got %q, want %q", cfg.BaseDir, home)
	}
}

func TestLoadRejectsRelativeBaseDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeConfig(t, home, "base_dir: rel/path\n")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for relative base_dir")
	}
}

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
