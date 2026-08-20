package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/illegalstudio/ggw/internal/ui"
	ggwskills "github.com/illegalstudio/ggw/skills"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// skillTargetSpec declares a supported installation destination. skillTargets
// turns it into concrete paths for a home directory; skillTargetKeys exposes
// the keys for --target validation and shell completion.
type skillTargetSpec struct {
	Key   string
	Label string
	path  func(home string) string
}

var skillTargetSpecs = []skillTargetSpec{
	{Key: "agents", Label: "~/.agents/skills/ggw", path: ggwskills.AgentSkillsInstallPath},
	{Key: "claude", Label: "~/.claude/skills/ggw", path: ggwskills.ClaudeSkillsInstallPath},
}

type skillTarget struct {
	Key   string
	Label string
	Path  string
}

type skillInstallItem struct {
	Target string                  `json:"target"`
	Path   string                  `json:"path"`
	Status ggwskills.InstallStatus `json:"status,omitempty"`
	Error  string                  `json:"error,omitempty"`
}

type skillsInstallResult struct {
	Name          string             `json:"name"`
	Installations []skillInstallItem `json:"installations"`
}

var skillsCmd = &cobra.Command{
	Use:     "skills",
	Short:   "Manage the bundled AI agent skill",
	GroupID: GroupConfig,
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the bundled GGW skill for AI agents",
	Long: `Install the AI agent skill bundled with this ggw binary.

Destinations:
  agents    ~/.agents/skills/ggw    Codex and other Agent Skills hosts
  claude    ~/.claude/skills/ggw    Claude Code

Without --target the command opens a multi-select menu with both destinations
preselected. With --json it never prompts and installs every destination that
--target does not narrow.

Reinstalling is safe: an unchanged copy is left alone, a copy this command
installed and you have not edited is updated in place, and anything else is
reported as a conflict until you pass --force.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find user home directory: %w", err)
		}

		force, _ := cmd.Flags().GetBool("force")
		requested, _ := cmd.Flags().GetStringArray("target")

		targets, err := selectSkillTargets(skillTargets(home), requested)
		if err != nil {
			return err
		}

		result := installSkillTargets(targets, force)

		if done, err := maybeJSON(result); done {
			return err
		}

		printSkillsInstallResult(result)
		return nil
	},
}

func skillTargets(home string) []skillTarget {
	targets := make([]skillTarget, len(skillTargetSpecs))
	for i, spec := range skillTargetSpecs {
		targets[i] = skillTarget{Key: spec.Key, Label: spec.Label, Path: spec.path(home)}
	}
	return targets
}

func skillTargetKeys() []string {
	keys := make([]string, len(skillTargetSpecs))
	for i, spec := range skillTargetSpecs {
		keys[i] = spec.Key
	}
	return keys
}

// selectSkillTargets resolves which destinations to install to. Explicit
// --target values win; otherwise JSON mode takes every destination without
// prompting, and interactive mode asks.
func selectSkillTargets(all []skillTarget, requested []string) ([]skillTarget, error) {
	if len(requested) > 0 {
		return filterSkillTargets(all, requested)
	}
	if jsonOutput {
		return all, nil
	}
	return promptSkillTargets(all)
}

func filterSkillTargets(all []skillTarget, requested []string) ([]skillTarget, error) {
	wanted := make(map[string]bool, len(requested))
	for _, key := range requested {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if !skillTargetExists(all, key) {
			return nil, fmt.Errorf("unknown skill target %q (valid targets: %s)", key, strings.Join(skillTargetKeys(), ", "))
		}
		wanted[key] = true
	}
	if len(wanted) == 0 {
		return nil, fmt.Errorf("no skill target selected")
	}

	selected := make([]skillTarget, 0, len(wanted))
	for _, target := range all {
		if wanted[target.Key] {
			selected = append(selected, target)
		}
	}
	return selected, nil
}

func skillTargetExists(all []skillTarget, key string) bool {
	for _, target := range all {
		if target.Key == key {
			return true
		}
	}
	return false
}

func promptSkillTargets(all []skillTarget) ([]skillTarget, error) {
	options := make([]huh.Option[int], len(all))
	for i, target := range all {
		options[i] = huh.NewOption(fmt.Sprintf("%s  (%s)", target.Key, target.Label), i).Selected(true)
	}

	var chosen []int
	err := huh.NewMultiSelect[int]().
		Title("Select the AI agent skill destinations").
		Description("space toggle · enter confirm").
		Options(options...).
		Value(&chosen).
		Run()
	if err != nil {
		return nil, fmt.Errorf("cannot prompt for skill destinations (use --target to choose them without a prompt): %w", err)
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("no skill target selected")
	}

	sort.Ints(chosen)
	selected := make([]skillTarget, 0, len(chosen))
	for _, index := range chosen {
		selected = append(selected, all[index])
	}
	return selected, nil
}

// installSkillTargets installs each destination independently: a failure is
// recorded on its own item and never stops the remaining destinations.
func installSkillTargets(targets []skillTarget, force bool) skillsInstallResult {
	result := skillsInstallResult{
		Name:          ggwskills.Name,
		Installations: make([]skillInstallItem, 0, len(targets)),
	}

	for _, target := range targets {
		status, err := ggwskills.Install(target.Path, force)
		result.Installations = append(result.Installations, skillInstallItem{
			Target: target.Key,
			Path:   target.Path,
			Status: status,
			Error:  errString(err),
		})
	}

	return result
}

func printSkillsInstallResult(result skillsInstallResult) {
	fmt.Println()
	installed := false
	for _, item := range result.Installations {
		if item.Error != "" {
			fmt.Printf("  %s %s %s\n", ui.Error.Render("✗"), ui.Branch.Render(displayPath(item.Path)), ui.Error.Render(item.Error))
			continue
		}
		installed = true
		fmt.Printf("  %s %s %s\n", ui.Success.Render("✓"), ui.Branch.Render(displayPath(item.Path)), ui.Muted.Render(string(item.Status)))
	}
	if installed {
		fmt.Println()
		fmt.Println(ui.Muted.Render("  Restart an AI agent if the skill does not appear automatically."))
	}
	fmt.Println()
}

func init() {
	skillsInstallCmd.Flags().Bool("force", false, "Replace an existing GGW skill that differs from the bundled version")
	skillsInstallCmd.Flags().StringArray("target", nil, "Install only to this destination (agents, claude); repeatable, not comma-separated")
	registerSkillTargetCompletion(skillsInstallCmd)

	skillsCmd.AddCommand(skillsInstallCmd)
	rootCmd.AddCommand(skillsCmd)
}
