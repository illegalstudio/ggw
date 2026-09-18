package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/illegalstudio/ggw/internal/version"

	"github.com/spf13/cobra"
)

const (
	GroupWorktree = "worktree"
	GroupShell    = "shell"
	GroupConfig   = "config"
)

var rootCmd = &cobra.Command{
	Use:           "ggw",
	Short:         "ggw — git worktrees, ergonomic",
	Version:       version.String(),
	SilenceErrors: true,
	Long: `ggw — git worktrees, ergonomic.

Stores every workspace of every repo in a single predictable location:

  ~/.local/share/worktrees/<org>/<repo>/<branch-slug>/

A workspace is either a git worktree or a copy-on-write snapshot of the
repository, which carries your untracked files along at no disk cost. Pick one
per run with --cow or --wt, or set "mode" in ~/.config/ggw/config.yaml.

Use "ggw <command> --help" for details on any command.`,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format (suppresses spinners and prompts)")
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

// Execute is the entry point invoked from cmd/ggw/main.go.
func Execute() {
	rootCmd.SilenceUsage = jsonFlagRequested(os.Args[1:])

	rootCmd.AddGroup(
		&cobra.Group{ID: GroupWorktree, Title: "Workspace Operations:"},
		&cobra.Group{ID: GroupShell, Title: "Shell Integration:"},
		&cobra.Group{ID: GroupConfig, Title: "Configuration:"},
	)
	rootCmd.SetHelpCommandGroupID(GroupShell)
	rootCmd.SetCompletionCommandGroupID(GroupShell)

	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		if jsonOutput {
			var payloadErr interface{ JSONPayload() any }
			if errors.As(err, &payloadErr) {
				_ = emitJSON(payloadErr.JSONPayload())
			} else {
				_ = emitJSON(map[string]any{"error": err.Error()})
			}
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}

	maybeNoticeStaleSkill(cmd)
}

func jsonFlagRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
		if strings.HasPrefix(arg, "--json=") {
			value := strings.TrimPrefix(arg, "--json=")
			return value != "false" && value != "0"
		}
	}
	return false
}
