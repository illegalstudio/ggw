package cli

import (
	"os"
	"path/filepath"
	"testing"

	ggwskills "github.com/illegalstudio/ggw/skills"

	"github.com/spf13/cobra"
)

func TestVerifySkillTargetsReportsEachDestination(t *testing.T) {
	all := skillTargets(t.TempDir())

	if _, err := ggwskills.Install(all[0].Path, false); err != nil {
		t.Fatal(err)
	}

	result := verifySkillTargets(all)
	if result.Name != "ggw" {
		t.Fatalf("name = %q, want %q", result.Name, "ggw")
	}
	if len(result.Verifications) != 2 {
		t.Fatalf("verification count = %d, want 2", len(result.Verifications))
	}
	if got := result.Verifications[0].Status; got != ggwskills.VerifyUpToDate {
		t.Fatalf("agents status = %q, want %q", got, ggwskills.VerifyUpToDate)
	}
	if got := result.Verifications[1].Status; got != ggwskills.VerifyNotInstalled {
		t.Fatalf("claude status = %q, want %q", got, ggwskills.VerifyNotInstalled)
	}
	if result.anyStale() {
		t.Fatal("up-to-date and not-installed destinations must not count as stale")
	}
}

func TestVerifySkillTargetsFlagsStaleDestinations(t *testing.T) {
	all := skillTargets(t.TempDir())

	if _, err := ggwskills.Install(all[0].Path, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(all[0].Path, "SKILL.md"), []byte("hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := verifySkillTargets(all)
	if got := result.Verifications[0].Status; got != ggwskills.VerifyModified {
		t.Fatalf("agents status = %q, want %q", got, ggwskills.VerifyModified)
	}
	if !result.anyStale() {
		t.Fatal("a modified destination must count as stale")
	}
}

func TestSelectVerifyTargetsDefaultsToAll(t *testing.T) {
	all := skillTargets(t.TempDir())

	selected, err := selectVerifyTargets(all, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}

	selected, err = selectVerifyTargets(all, []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].Key != "claude" {
		t.Fatalf("selected = %+v, want only claude", selected)
	}
}

func TestSkillNoticeApplies(t *testing.T) {
	root := &cobra.Command{Use: "ggw"}
	skills := &cobra.Command{Use: "skills"}
	install := &cobra.Command{Use: "install"}
	skills.AddCommand(install)
	completion := &cobra.Command{Use: "completion"}
	completionBash := &cobra.Command{Use: "bash"}
	completion.AddCommand(completionBash)
	list := &cobra.Command{Use: "list"}
	cd := &cobra.Command{Use: "cd"}
	exec := &cobra.Command{Use: "exec"}
	shellInit := &cobra.Command{Use: "shell-init"}
	help := &cobra.Command{Use: "help"}
	root.AddCommand(skills, list, cd, exec, shellInit, completion, help)

	cases := []struct {
		name string
		cmd  *cobra.Command
		want bool
	}{
		{"nil command", nil, false},
		{"root command", rootCmd, false},
		{"list", list, true},
		{"skills install", install, false},
		{"skills", skills, false},
		{"cd", cd, false},
		{"exec", exec, false},
		{"shell-init", shellInit, false},
		{"completion", completion, false},
		{"completion bash", completionBash, false},
		{"help", help, false},
	}
	for _, tc := range cases {
		if got := skillNoticeApplies(tc.cmd); got != tc.want {
			t.Fatalf("%s: skillNoticeApplies = %v, want %v", tc.name, got, tc.want)
		}
	}
}
