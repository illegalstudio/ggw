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
  --force            remove even if the worktree is dirty (skips confirmation)
  --without-branch   keep the local branch the worktree pointed to`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		withoutBranch, _ := cmd.Flags().GetBool("without-branch")

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
		mainWorktreePath := list[0].Path

		query := ""
		if len(args) == 1 {
			query = args[0]
		}
		wt, err := resolveOneWorktree(list, query)
		if err != nil {
			return err
		}

		if wt.Path == root {
			if wt.Path == mainWorktreePath {
				return fmt.Errorf("cannot delete the main worktree (current directory)")
			}
			return fmt.Errorf("cannot delete the current worktree")
		}
		if wt.Path == mainWorktreePath {
			return fmt.Errorf("cannot delete the main worktree")
		}

		deleteBranch := !withoutBranch && wt.Branch != "" && !isProtectedMainBranch(root, list, wt.Branch)

		if !force {
			ok, err := confirmDelete(wt, deleteBranch)
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
		if deleteBranch {
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
			"without_branch": withoutBranch,
			"branch_deleted": branchDeleted,
			"branch_error":   branchErr,
		}); done {
			return err
		}

		fmt.Printf("%s Worktree removed: %s\n",
			ui.Success.Render("✓"),
			ui.Path.Render(displayPath(wt.Path)),
		)
		if deleteBranch {
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
	deleteCmd.Flags().Bool("without-branch", false, "Keep the local branch the worktree pointed to")
	rootCmd.AddCommand(deleteCmd)
}

func confirmDelete(w *worktree.Worktree, deleteBranch bool) (bool, error) {
	if jsonOutput {
		return true, nil
	}

	label := w.Branch
	if label == "" {
		label = filepath.Base(w.Path)
	}
	title := fmt.Sprintf("Delete worktree %q at %s?", label, displayPath(w.Path))
	if deleteBranch {
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

func isProtectedMainBranch(repoPath string, list []worktree.Worktree, branch string) bool {
	if branch == "" {
		return false
	}
	if defaultBranch, err := worktree.DefaultBranch(repoPath); err == nil && defaultBranch != "" {
		return branch == defaultBranch
	}
	if branch == "main" || branch == "master" {
		return true
	}
	// git worktree list reports the main worktree first; use it as a local fallback.
	return branch == list[0].Branch
}
