package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPaths(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "tmp", "ggw-home")
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "Agent Skills",
			got:  AgentSkillsInstallPath(home),
			want: filepath.Join(home, ".agents", "skills", "ggw"),
		},
		{
			name: "Claude Code",
			got:  ClaudeSkillsInstallPath(home),
			want: filepath.Join(home, ".claude", "skills", "ggw"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("path = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestInstallRequiresDestination(t *testing.T) {
	if _, err := Install("", false); err == nil {
		t.Fatal("expected an error for an empty destination")
	}
}

func TestInstallNewAndIdempotent(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())

	status, err := Install(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusInstalled {
		t.Fatalf("status = %q, want %q", status, StatusInstalled)
	}

	for _, relative := range []string{"SKILL.md", filepath.Join("agents", "openai.yaml"), markerFileName} {
		if _, err := os.Stat(filepath.Join(destination, relative)); err != nil {
			t.Fatalf("installed %s: %v", relative, err)
		}
	}

	status, err = Install(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusUnchanged {
		t.Fatalf("status = %q, want %q", status, StatusUnchanged)
	}
}

func TestInstallRefusesModifiedSkillWithoutForce(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(destination, false); err == nil {
		t.Fatal("expected a conflict for the modified skill")
	}
	if got := string(mustRead(t, filepath.Join(destination, "SKILL.md"))); got != "modified\n" {
		t.Fatalf("refused install still overwrote SKILL.md: %q", got)
	}
}

func TestInstallRefusesUnmanagedDirectoryWithoutForce(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("someone else\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(destination, false); err == nil {
		t.Fatal("expected a conflict for the unmanaged directory")
	}
}

func TestInstallUpdatesUnmodifiedManagedSkill(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}

	// Simulate a previously bundled version: change the content, then record
	// that content in the marker so it still looks untouched by the user.
	if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("previous bundled skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previousDigest, err := digestInstalledSkill(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeMarker(destination, previousDigest); err != nil {
		t.Fatal(err)
	}

	status, err := Install(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusUpdated {
		t.Fatalf("status = %q, want %q", status, StatusUpdated)
	}

	status, err = Install(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusUnchanged {
		t.Fatalf("status = %q, want %q", status, StatusUnchanged)
	}
}

func TestInstallForceReplacesModifiedSkill(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "extra.txt"), []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := Install(destination, true)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusReplaced {
		t.Fatalf("status = %q, want %q", status, StatusReplaced)
	}
	if _, err := os.Stat(filepath.Join(destination, "extra.txt")); !os.IsNotExist(err) {
		t.Fatalf("extra file survived the replace: %v", err)
	}

	status, err = Install(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusUnchanged {
		t.Fatalf("status = %q, want %q", status, StatusUnchanged)
	}
}

func TestInstallLeavesNoStagingDirectories(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "ggw")
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "ggw" {
		t.Fatalf("parent directory has leftovers: %v", entries)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func TestInstallRefusesNonDirectoryDestinationWithoutForce(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Install(destination, false)
	if err == nil {
		t.Fatal("expected a conflict for the non-directory destination")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("error = %v, want it to say the destination is not a directory", err)
	}
}

// A destination symlinked at an already-installed copy holds byte-identical
// files, so it must never be described as containing different files.
func TestInstallDoesNotClaimSymlinkedDestinationDiffers(t *testing.T) {
	home := t.TempDir()
	real := AgentSkillsInstallPath(home)
	if _, err := Install(real, false); err != nil {
		t.Fatal(err)
	}

	linked := ClaudeSkillsInstallPath(home)
	if err := os.MkdirAll(filepath.Dir(linked), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, linked); err != nil {
		t.Fatal(err)
	}

	_, err := Install(linked, false)
	if err == nil {
		t.Fatal("expected a conflict for the symlinked destination")
	}
	if strings.Contains(err.Error(), "contains different files") {
		t.Fatalf("error falsely claims the contents differ: %v", err)
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("error = %v, want it to say the destination is not a directory", err)
	}

	// The symlink itself must survive a refused install.
	info, statErr := os.Lstat(linked)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("refused install replaced the symlink")
	}
}
