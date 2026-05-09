package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Stub commands for features planned in iterazione 2 and 3 (see ROADMAP.md).
// They are registered so they show up in `ggw --help`, but error out when run.

func stubRunE(name string) func(*cobra.Command, []string) error {
	return func(*cobra.Command, []string) error {
		msg := fmt.Sprintf("`ggw %s` is not implemented yet — see ROADMAP.md", name)
		if done, err := maybeJSON(map[string]any{"error": msg}); done {
			return err
		}
		return fmt.Errorf("%s", msg)
	}
}

// TODO(iter 2): implement shell integration (`ggw init`, `ggw cd-path`, `ggw cd`).
var initCmd = &cobra.Command{
	Use:     "init <bash|zsh|fish>",
	Short:   "Print shell integration script (stub)",
	GroupID: GroupShell,
	RunE:    stubRunE("init"),
}

var cdCmd = &cobra.Command{
	Use:     "cd [name]",
	Short:   "Change directory into a worktree (stub — needs shell integration)",
	GroupID: GroupShell,
	RunE:    stubRunE("cd"),
}

// TODO(iter 3): implement `ggw exec` and `ggw delete`.
var execCmd = &cobra.Command{
	Use:     "exec [name] -- <cmd>",
	Short:   "Run a command inside a worktree (stub)",
	GroupID: GroupWorktree,
	RunE:    stubRunE("exec"),
}

var deleteCmd = &cobra.Command{
	Use:     "delete [name]",
	Short:   "Delete a worktree (stub)",
	GroupID: GroupWorktree,
	RunE:    stubRunE("delete"),
}

func init() {
	rootCmd.AddCommand(initCmd, cdCmd, execCmd, deleteCmd)
}
