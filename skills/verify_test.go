package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRequiresDestination(t *testing.T) {
	if _, err := Verify(""); err == nil {
		t.Fatal("expected an error for an empty destination")
	}
}

func TestVerifyNotInstalled(t *testing.T) {
	status, err := Verify(AgentSkillsInstallPath(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if status != VerifyNotInstalled {
		t.Fatalf("status = %q, want %q", status, VerifyNotInstalled)
	}
}

func TestVerifyUpToDate(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}

	status, err := Verify(destination)
	if err != nil {
		t.Fatal(err)
	}
	if status != VerifyUpToDate {
		t.Fatalf("status = %q, want %q", status, VerifyUpToDate)
	}
}

func TestVerifyOutdated(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}

	// Simulate a skill bundled with another ggw version: change the content,
	// then record that content in the marker so it still looks untouched.
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

	status, err := Verify(destination)
	if err != nil {
		t.Fatal(err)
	}
	if status != VerifyOutdated {
		t.Fatalf("status = %q, want %q", status, VerifyOutdated)
	}
}

func TestVerifyModified(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := Verify(destination)
	if err != nil {
		t.Fatal(err)
	}
	if status != VerifyModified {
		t.Fatalf("status = %q, want %q", status, VerifyModified)
	}
}

func TestVerifyNonDirectoryDestination(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Verify(destination)
	if err == nil {
		t.Fatal("expected an error for the non-directory destination")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("error = %v, want it to say the destination is not a directory", err)
	}
}

func TestVerifyStatusStale(t *testing.T) {
	stale := map[VerifyStatus]bool{
		VerifyNotInstalled: false,
		VerifyUpToDate:     false,
		VerifyOutdated:     true,
		VerifyModified:     true,
	}
	for status, want := range stale {
		if got := status.Stale(); got != want {
			t.Fatalf("%q.Stale() = %v, want %v", status, got, want)
		}
	}
}
