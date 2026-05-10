package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:               "delete [name]",
	Short:             "Delete a worktree",
	GroupID:           GroupWorktree,
	ValidArgsFunction: deleteCompletion,
	Long: `Delete a worktree. Without arguments, opens an interactive selector.

Flags:
  --force         remove even if the worktree is dirty (skips confirmation)
  --with-branch   also delete the local branch the worktree pointed to`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		withBranch, _ := cmd.Flags().GetBool("with-branch")

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
		if len(args) == 1 {
			query = args[0]
		}
		wt, err := resolveOneWorktree(list, query)
		if err != nil {
			return err
		}

		if wt.Path == root {
			return fmt.Errorf("cannot delete the main worktree (current directory)")
		}

		if !force {
			ok, err := confirmDelete(wt, withBranch)
			if err != nil {
				return err
			}
			if !ok {
				if done, err := maybeJSON(map[string]any{"deleted": false, "aborted": true}); done {
					return err
				}
				fmt.Println(ui.Muted.Render("Aborted."))
				return nil
			}
		}

		if err := worktree.Remove(root, wt.Path, force); err != nil {
			return err
		}

		var branchDeleted bool
		var branchErr string
		if withBranch && wt.Branch != "" {
			if err := worktree.DeleteBranch(root, wt.Branch); err != nil {
				branchErr = err.Error()
			} else {
				branchDeleted = true
			}
		}

		if done, err := maybeJSON(map[string]any{
			"deleted":        true,
			"path":           wt.Path,
			"branch":         wt.Branch,
			"with_branch":    withBranch,
			"branch_deleted": branchDeleted,
			"branch_error":   branchErr,
		}); done {
			return err
		}

		fmt.Printf("%s Worktree removed: %s\n",
			ui.Success.Render("✓"),
			ui.Path.Render(displayPath(wt.Path)),
		)
		if withBranch && wt.Branch != "" {
			if branchDeleted {
				fmt.Printf("%s Branch deleted: %s\n", ui.Success.Render("✓"), ui.Branch.Render(wt.Branch))
			} else {
				fmt.Printf("%s Branch not deleted: %s\n", ui.Error.Render("✗"), branchErr)
			}
		}
		return nil
	},
}

func init() {
	deleteCmd.Flags().Bool("force", false, "Remove even if the worktree is dirty (also skips confirmation)")
	deleteCmd.Flags().Bool("with-branch", false, "Also delete the local branch the worktree pointed to")
	rootCmd.AddCommand(deleteCmd)
}

func confirmDelete(w *worktree.Worktree, withBranch bool) (bool, error) {
	if jsonOutput {
		return true, nil
	}

	label := w.Branch
	if label == "" {
		label = filepath.Base(w.Path)
	}
	title := fmt.Sprintf("Delete worktree %q at %s?", label, displayPath(w.Path))
	if withBranch && w.Branch != "" {
		title += fmt.Sprintf(" (branch %q will also be deleted)", w.Branch)
	}

	var choice string
	err := huh.NewSelect[string]().
		Title(title).
		Options(
			huh.NewOption("Yes, delete", "yes"),
			huh.NewOption("No, abort", "no"),
		).
		Value(&choice).
		Run()
	if err != nil {
		return false, err
	}
	return choice == "yes", nil
}
