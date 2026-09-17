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

type skillVerifyItem struct {
	Target string                 `json:"target"`
	Path   string                 `json:"path"`
	Status ggwskills.VerifyStatus `json:"status,omitempty"`
	Error  string                 `json:"error,omitempty"`
}

type skillsVerifyResult struct {
	Name          string            `json:"name"`
	Verifications []skillVerifyItem `json:"verifications"`
}

// anyStale reports whether any installed destination disagrees with the
// bundled skill. Absent destinations and per-destination errors do not count.
func (r skillsVerifyResult) anyStale() bool {
	for _, item := range r.Verifications {
		if item.Error == "" && item.Status.Stale() {
			return true
		}
	}
	return false
}

// anyModified reports whether any destination was edited locally or not
// written by ggw, which makes `ggw skills install` refuse without --force.
func (r skillsVerifyResult) anyModified() bool {
	for _, item := range r.Verifications {
		if item.Error == "" && item.Status == ggwskills.VerifyModified {
			return true
		}
	}
	return false
}

// staleSkillsError makes a stale verification fail the command (exit code 1)
// after its report has been printed, and carries the verify payload into JSON
// mode, where root's error handler emits it instead of a bare {"error": ...}.
type staleSkillsError struct{ result skillsVerifyResult }

func (e staleSkillsError) Error() string {
	update := "`ggw skills install`"
	if e.result.anyModified() {
		update = "`ggw skills install --force` (an installed copy was modified)"
	}
	return fmt.Sprintf("one or more installed skills are not in sync with this ggw version (run %s to update)", update)
}

func (e staleSkillsError) JSONPayload() any { return e.result }

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

var skillsVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check whether the installed GGW skills match this binary",
	Long: `Compare each installed copy of the bundled AI agent skill with the skill
this ggw binary carries, using the recorded SHA-256 digests.

Without --target every known destination is checked; the command never
prompts. Exit code is 1 when an installed skill is outdated or was modified
locally, so the check can gate scripts and CI.`,
	Args:         cobra.NoArgs,
	SilenceUsage: true, // a stale skill fails the command but is not a usage error
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find user home directory: %w", err)
		}

		requested, _ := cmd.Flags().GetStringArray("target")
		targets, err := selectVerifyTargets(skillTargets(home), requested)
		if err != nil {
			return err
		}

		result := verifySkillTargets(targets)

		if jsonOutput {
			if result.anyStale() {
				return staleSkillsError{result: result}
			}
			return emitJSON(result)
		}

		printSkillsVerifyResult(result)
		if result.anyStale() {
			return staleSkillsError{result: result}
		}
		return nil
	},
}

// selectVerifyTargets resolves which destinations to check. Verification is
// read-only, so without --target it simply checks every destination instead of
// prompting.
func selectVerifyTargets(all []skillTarget, requested []string) ([]skillTarget, error) {
	if len(requested) > 0 {
		return filterSkillTargets(all, requested)
	}
	return all, nil
}

// verifySkillTargets checks each destination independently: a failure is
// recorded on its own item and never stops the remaining destinations.
func verifySkillTargets(targets []skillTarget) skillsVerifyResult {
	result := skillsVerifyResult{
		Name:          ggwskills.Name,
		Verifications: make([]skillVerifyItem, 0, len(targets)),
	}

	for _, target := range targets {
		status, err := ggwskills.Verify(target.Path)
		result.Verifications = append(result.Verifications, skillVerifyItem{
			Target: target.Key,
			Path:   target.Path,
			Status: status,
			Error:  errString(err),
		})
	}

	return result
}

func printSkillsVerifyResult(result skillsVerifyResult) {
	fmt.Println()
	for _, item := range result.Verifications {
		switch {
		case item.Error != "":
			fmt.Printf("  %s %s %s\n", ui.Error.Render("✗"), ui.Branch.Render(displayPath(item.Path)), ui.Error.Render(item.Error))
		case item.Status == ggwskills.VerifyUpToDate:
			fmt.Printf("  %s %s %s\n", ui.Success.Render("✓"), ui.Branch.Render(displayPath(item.Path)), ui.Muted.Render(string(item.Status)))
		case item.Status == ggwskills.VerifyNotInstalled:
			fmt.Printf("  %s %s %s\n", ui.Muted.Render("·"), ui.Branch.Render(displayPath(item.Path)), ui.Muted.Render(string(item.Status)))
		default:
			fmt.Printf("  %s %s %s\n", ui.Error.Render("✗"), ui.Branch.Render(displayPath(item.Path)), ui.Error.Render(string(item.Status)))
		}
	}
	fmt.Println()
}

func init() {
	skillsInstallCmd.Flags().Bool("force", false, "Replace an existing GGW skill that differs from the bundled version")
	skillsInstallCmd.Flags().StringArray("target", nil, "Install only to this destination (agents, claude); repeatable, not comma-separated")
	registerSkillTargetCompletion(skillsInstallCmd)

	skillsVerifyCmd.Flags().StringArray("target", nil, "Verify only this destination (agents, claude); repeatable, not comma-separated")
	registerSkillTargetCompletion(skillsVerifyCmd)

	skillsCmd.AddCommand(skillsInstallCmd, skillsVerifyCmd)
	rootCmd.AddCommand(skillsCmd)
}
