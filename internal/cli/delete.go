package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/workspace"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:               "delete [name]",
	Short:             "Delete a workspace",
	GroupID:           GroupWorktree,
	ValidArgsFunction: deleteCompletion,
	Long: `Delete a workspace. Without arguments, opens an interactive selector.

A worktree is unregistered from git and its branch is deleted too, unless
--without-branch is given — the branch survives in the repository either way if
you keep it.

A copy-on-write workspace is a repository of its own: removing it removes its
branch and every commit that exists nowhere else. ggw therefore refuses to
delete one that holds uncommitted changes or unpushed commits, and tells you how
to save the branch first. --without-branch has no meaning for this kind.

Flags:
  --force            remove even if the workspace holds unsaved work (skips confirmation)
  --without-branch   keep the local branch a worktree pointed to`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		withoutBranch, _ := cmd.Flags().GetBool("without-branch")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, err := workspace.Resolve(cwd)
		if err != nil {
			return err
		}

		list, err := workspace.List(ctx)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return fmt.Errorf("no workspaces registered for this repository")
		}

		query := ""
		if len(args) == 1 {
			query = args[0]
		}
		ws, err := resolveOneWorkspace(list, query)
		if err != nil {
			return err
		}

		if ws.Path == ctx.RepoPath {
			if ws.Main {
				return fmt.Errorf("cannot delete the main worktree (current directory)")
			}
			return fmt.Errorf("cannot delete the current workspace")
		}
		if ws.Main {
			return fmt.Errorf("cannot delete the main worktree")
		}

		// A worktree's branch lives in the repository and outlives the
		// directory, so deleting it is a separate, optional step. A
		// copy-on-write workspace has no branch anywhere else — there is
		// nothing left to delete once the directory is gone.
		deleteBranch := ws.Kind == workspace.KindWorktree &&
			!withoutBranch && ws.Branch != "" &&
			!isProtectedMainBranch(ctx, list, ws.Branch)

		handle := handleFor(list, ws.Path)

		if ws.Kind == workspace.KindCoW && !force {
			// The command was typed correctly from here on; printing the usage
			// block under a multi-line rescue instruction only buries it.
			cmd.SilenceUsage = true

			unsaved, err := workspace.Inspect(*ws)
			if err != nil {
				return fmt.Errorf("cannot tell whether %q holds unsaved work, refusing to delete it: %w\n\nPass --force to delete it anyway.", handle, err)
			}
			if unsaved.Any() {
				return unsavedWorkError(ctx, *ws, handle, unsaved)
			}
		}

		if !force {
			ok, err := confirmDelete(ws, handle, deleteBranch)
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

		if err := workspace.Remove(ctx, *ws, force); err != nil {
			return err
		}

		var branchDeleted bool
		var branchErr string
		if deleteBranch {
			target, err := ctx.WorktreeTarget()
			if err != nil {
				return err
			}
			if err := worktree.DeleteBranch(target, ws.Branch); err != nil {
				branchErr = err.Error()
			} else {
				branchDeleted = true
			}
		}

		if done, err := maybeJSON(map[string]any{
			"deleted":        true,
			"kind":           string(ws.Kind),
			"path":           ws.Path,
			"branch":         ws.Branch,
			"without_branch": withoutBranch,
			"branch_deleted": branchDeleted,
			"branch_error":   branchErr,
		}); done {
			return err
		}

		fmt.Printf("%s %s removed: %s\n",
			ui.Success.Render("✓"),
			kindLabel(ws.Kind),
			ui.Path.Render(displayPath(ws.Path)),
		)
		if deleteBranch {
			if branchDeleted {
				fmt.Printf("%s Branch deleted: %s\n", ui.Success.Render("✓"), ui.Branch.Render(ws.Branch))
			} else {
				fmt.Printf("%s Branch not deleted: %s\n", ui.Error.Render("✗"), branchErr)
			}
		}
		return nil
	},
}

func init() {
	deleteCmd.Flags().Bool("force", false, "Remove even if the workspace holds unsaved work (also skips confirmation)")
	deleteCmd.Flags().Bool("without-branch", false, "Keep the local branch a worktree pointed to")
	rootCmd.AddCommand(deleteCmd)
}

// unsavedWorkError refuses to destroy work that exists in exactly one place.
// The message names what is at stake and the exact command that rescues it,
// because "use --force" on its own is a dead end when the alternative is losing
// commits.
func unsavedWorkError(ctx *workspace.Context, ws workspace.Workspace, handle string, unsaved workspace.UnsavedWork) error {
	var lost []string
	if unsaved.Unpushed == 1 {
		lost = append(lost, "1 unpushed commit")
	} else if unsaved.Unpushed > 1 {
		lost = append(lost, fmt.Sprintf("%d unpushed commits", unsaved.Unpushed))
	}
	if unsaved.Dirty {
		lost = append(lost, "uncommitted changes")
	}

	msg := fmt.Sprintf("refusing to delete %q: %s would be lost\n\n"+
		"A copy-on-write workspace is a repository of its own, so its branch\n"+
		"exists nowhere else.", handle, strings.Join(lost, " and "))

	if rescue := workspace.RescueCommand(ctx, ws); rescue != "" && unsaved.Unpushed > 0 {
		msg += fmt.Sprintf("\n\nSave the branch into the repository first:\n\n  %s", rescue)
	}
	return fmt.Errorf("%s\n\nOr pass --force to delete it anyway.", msg)
}

// handleFor returns the handle of the workspace at path, as computed over the
// whole list.
func handleFor(list []workspace.Workspace, path string) string {
	handles := workspace.Handles(list)
	for i := range list {
		if list[i].Path == path {
			return handles[i]
		}
	}
	return ""
}

func confirmDelete(w *workspace.Workspace, handle string, deleteBranch bool) (bool, error) {
	if jsonOutput {
		return true, nil
	}

	label := w.Branch
	if label == "" {
		label = handle
	}
	title := fmt.Sprintf("Delete %s %q at %s?", strings.ToLower(kindLabel(w.Kind)), label, displayPath(w.Path))
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

func isProtectedMainBranch(ctx *workspace.Context, list []workspace.Workspace, branch string) bool {
	if branch == "" {
		return false
	}
	if defaultBranch, err := worktree.DefaultBranch(ctx.BranchRepo()); err == nil && defaultBranch != "" {
		return branch == defaultBranch
	}
	if branch == "main" || branch == "master" {
		return true
	}
	// The main worktree's branch is the local fallback.
	for _, w := range list {
		if w.Main {
			return branch == w.Branch
		}
	}
	return false
}
