package cli

import (
	"fmt"
	"os"
	osexec "os/exec"

	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:                   "exec [name] -- <cmd>...",
	Short:                 "Run a command inside a worktree",
	GroupID:               GroupWorktree,
	DisableFlagsInUseLine: true,
	Long: `Run an arbitrary command inside a worktree's directory.

Everything after "--" is passed to the command verbatim.

  ggw exec feature/login -- npm install
  ggw exec -- ls -la                  # selector picks the worktree

Stdin/stdout/stderr are piped through. The command's exit code is
propagated as ggw's exit code.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonOutput {
			return fmt.Errorf("exec does not support --json")
		}

		dashAt := cmd.ArgsLenAtDash()
		if dashAt == -1 {
			return fmt.Errorf("missing `--` separator: usage: ggw exec [name] -- <cmd>...")
		}
		before := args[:dashAt]
		after := args[dashAt:]

		if len(after) == 0 {
			return fmt.Errorf("missing command after `--`")
		}
		if len(before) > 1 {
			return fmt.Errorf("expected at most one [name] argument, got %d", len(before))
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := worktree.RepoRoot(cwd)
		if err != nil {
			return err
		}

		list, err := worktree.List(root)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return fmt.Errorf("no worktrees registered for this repository")
		}

		query := ""
		if len(before) == 1 {
			query = before[0]
		}
		wt, err := resolveOneWorktree(list, query)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "%s in %s\n", ui.Muted.Render("ggw exec"), ui.Path.Render(displayPath(wt.Path)))

		c := osexec.Command(after[0], after[1:]...)
		c.Dir = wt.Path
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			if exitErr, ok := err.(*osexec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return fmt.Errorf("failed to run %q: %w", after[0], err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
}
