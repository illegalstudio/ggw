package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	GroupWorktree = "worktree"
	GroupShell    = "shell"
)

var rootCmd = &cobra.Command{
	Use:           "ggw",
	Short:         "ggw — git worktrees, ergonomic",
	SilenceErrors: true,
	Long: `ggw — git worktrees, ergonomic.

Stores all worktrees of all your repos in a single predictable location:

  ~/.local/share/worktrees/<org>/<repo>/<branch-slug>/

Use "ggw <command> --help" for details on any command.`,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format (suppresses spinners and prompts)")
}

// Execute is the entry point invoked from cmd/ggw/main.go.
func Execute() {
	rootCmd.SilenceUsage = jsonFlagRequested(os.Args[1:])

	rootCmd.AddGroup(
		&cobra.Group{ID: GroupWorktree, Title: "Worktree Operations:"},
		&cobra.Group{ID: GroupShell, Title: "Shell Integration:"},
	)
	rootCmd.SetHelpCommandGroupID(GroupShell)
	rootCmd.SetCompletionCommandGroupID(GroupShell)

	if err := rootCmd.Execute(); err != nil {
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
