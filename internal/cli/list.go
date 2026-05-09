package cli

import (
	"fmt"
	"os"

	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List worktrees of the current repository",
	GroupID: GroupWorktree,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := worktree.RepoRoot(cwd)
		if err != nil {
			return err
		}

		entries, err := worktree.List(root)
		if err != nil {
			return err
		}

		if done, err := maybeJSON(map[string]any{"worktrees": entries}); done {
			return err
		}

		if len(entries) == 0 {
			fmt.Println(ui.Info.Render("No worktrees registered."))
			return nil
		}

		fmt.Println(ui.Title.Render("Worktrees"))
		fmt.Println()
		for _, w := range entries {
			label := w.Branch
			if label == "" {
				if w.Detached {
					label = "(detached)"
				} else if w.Bare {
					label = "(bare)"
				} else {
					label = "(unknown)"
				}
			}
			suffix := ""
			if w.Locked {
				suffix = " " + ui.Muted.Render("[locked]")
			}
			fmt.Printf("  %s %s → %s%s\n", ui.Success.Render("●"), ui.Branch.Render(label), ui.Path.Render(w.Path), suffix)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
