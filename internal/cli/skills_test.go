package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSkillTargets(t *testing.T) {
	home := t.TempDir()
	targets := skillTargets(home)

	if len(targets) != 2 {
		t.Fatalf("target count = %d, want 2", len(targets))
	}
	if targets[0].Key != "agents" || targets[1].Key != "claude" {
		t.Fatalf("target keys = %q, %q", targets[0].Key, targets[1].Key)
	}
	if want := filepath.Join(home, ".agents", "skills", "ggw"); targets[0].Path != want {
		t.Fatalf("agents path = %q, want %q", targets[0].Path, want)
	}
	if want := filepath.Join(home, ".claude", "skills", "ggw"); targets[1].Path != want {
		t.Fatalf("claude path = %q, want %q", targets[1].Path, want)
	}
}

func TestSkillTargetKeys(t *testing.T) {
	keys := skillTargetKeys()
	if !reflect.DeepEqual(keys, []string{"agents", "claude"}) {
		t.Fatalf("keys = %v, want [agents claude]", keys)
	}
}

func TestFilterSkillTargetsPreservesDeclarationOrder(t *testing.T) {
	all := skillTargets(t.TempDir())

	selected, err := filterSkillTargets(all, []string{"claude", "agents"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}
	if selected[0].Key != "agents" || selected[1].Key != "claude" {
		t.Fatalf("selected = %q, %q; want agents, claude", selected[0].Key, selected[1].Key)
	}
}

func TestFilterSkillTargetsDeduplicatesAndNormalizes(t *testing.T) {
	all := skillTargets(t.TempDir())

	selected, err := filterSkillTargets(all, []string{"Claude", " claude ", "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].Key != "claude" {
		t.Fatalf("selected = %+v, want one claude target", selected)
	}
}

func TestFilterSkillTargetsRejectsUnknownKey(t *testing.T) {
	all := skillTargets(t.TempDir())

	_, err := filterSkillTargets(all, []string{"cursor"})
	if err == nil {
		t.Fatal("expected an error for an unknown target")
	}
	if !strings.Contains(err.Error(), "unknown skill target") {
		t.Fatalf("error = %v, want it to mention an unknown skill target", err)
	}
	if !strings.Contains(err.Error(), "agents, claude") {
		t.Fatalf("error = %v, want it to list the valid targets", err)
	}
}

func TestFilterSkillTargetsRejectsEmptySelection(t *testing.T) {
	all := skillTargets(t.TempDir())

	if _, err := filterSkillTargets(all, []string{"  "}); err == nil {
		t.Fatal("expected an error when no target remains")
	}
}

func TestSelectSkillTargetsUsesAllTargetsInJSONMode(t *testing.T) {
	previous := jsonOutput
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = previous })

	all := skillTargets(t.TempDir())
	selected, err := selectSkillTargets(all, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}
}

func TestSelectSkillTargetsPrefersExplicitTargetsOverJSONMode(t *testing.T) {
	previous := jsonOutput
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = previous })

	all := skillTargets(t.TempDir())
	selected, err := selectSkillTargets(all, []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].Key != "claude" {
		t.Fatalf("selected = %+v, want only claude", selected)
	}
}

func TestInstallSkillTargetsKeepsDestinationsIndependent(t *testing.T) {
	all := skillTargets(t.TempDir())

	if result := installSkillTargets(all[:1], false); result.Installations[0].Error != "" {
		t.Fatalf("first install failed: %s", result.Installations[0].Error)
	}
	if err := os.WriteFile(filepath.Join(all[0].Path, "SKILL.md"), []byte("hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := installSkillTargets(all, false)
	if result.Name != "ggw" {
		t.Fatalf("name = %q, want %q", result.Name, "ggw")
	}
	if len(result.Installations) != 2 {
		t.Fatalf("installation count = %d, want 2", len(result.Installations))
	}
	if result.Installations[0].Error == "" {
		t.Fatal("modified destination did not report a conflict")
	}
	if result.Installations[1].Error != "" {
		t.Fatalf("second destination failed: %s", result.Installations[1].Error)
	}
	if _, err := os.Stat(filepath.Join(all[1].Path, "SKILL.md")); err != nil {
		t.Fatalf("second destination was not installed: %v", err)
	}
}

func TestSkillTargetCompletionItemsFiltersByPrefix(t *testing.T) {
	if got := skillTargetCompletionItems(""); !reflect.DeepEqual(got, []string{"agents", "claude"}) {
		t.Fatalf("got %v, want [agents claude]", got)
	}
	if got := skillTargetCompletionItems("cl"); !reflect.DeepEqual(got, []string{"claude"}) {
		t.Fatalf("got %v, want [claude]", got)
	}
	if got := skillTargetCompletionItems("zz"); len(got) != 0 {
		t.Fatalf("got %v, want no completions", got)
	}
}
