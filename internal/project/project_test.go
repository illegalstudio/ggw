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
