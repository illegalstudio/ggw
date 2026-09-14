package cli

import (
	"fmt"
	"os"

	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/workspace"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [branch]",
	Short: "Create a workspace for a branch (creates the branch if needed)",
	Long: `Create a workspace for a branch.

If branch is omitted, ggw generates a random Docker-style name
(adjective-noun), e.g. intelligent-elephant.

Two kinds of workspace are available, both stored in the same place:

  worktree  a git worktree — git tracks it, untracked files are not carried over
  cow       a copy-on-write snapshot of the repository, untracked files included
            (needs btrfs, XFS with reflink=1, bcachefs or APFS)

The default comes from ` + "`mode`" + ` in ~/.config/ggw/config.yaml and is
"worktree"; --cow and --wt override it for a single run.`,
	GroupID:           GroupWorktree,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: createCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		bare, _ := cmd.Flags().GetBool("bare")
		as, _ := cmd.Flags().GetString("as")

		mode, err := resolveMode(cmd)
		if err != nil {
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, err := workspace.Resolve(cwd)
		if err != nil {
			return err
		}
		if err := ctx.RequireLayout(); err != nil {
			return err
		}

		var branch string
		if len(args) == 1 {
			branch = args[0]
		} else {
			branch, err = workspace.UniqueRandomName(ctx.BranchRepo(), ctx.Org, ctx.Repo)
			if err != nil {
				return err
			}
		}

		slug, err := workspaceSlug(as, branch)
		if err != nil {
			return err
		}

		dest, err := workspace.Create(workspace.CreateOptions{
			Ctx:    ctx,
			Kind:   mode,
			Branch: branch,
			Slug:   slug,
			From:   from,
		})
		if err != nil {
			return err
		}

		// Provision transactionally: on any failure, remove the workspace
		// (force=true allows dirty trees) but keep the branch so pre-existing
		// work is never lost. A copy-on-write workspace has no branch outside
		// itself, and its branch was created moments ago, so removing the
		// directory loses nothing either.
		provisioned := false
		defer func() {
			if !provisioned {
				if rmErr := workspace.Remove(ctx, workspace.Workspace{Kind: mode, Path: dest}, true); rmErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to roll back workspace %s: %v\n", dest, rmErr)
				}
			}
		}()
		if err := provisionWorkspace(ctx, dest, mode, bare, os.Stderr); err != nil {
			return err
		}
		provisioned = true

		if done, err := maybeJSON(map[string]any{
			"kind":   string(mode),
			"branch": branch,
			"slug":   slug,
			"path":   dest,
			"org":    ctx.Org,
			"repo":   ctx.Repo,
		}); done {
			return err
		}

		fmt.Printf("%s %s created: %s → %s\n",
			ui.Success.Render("✓"),
			kindLabel(mode),
			ui.Branch.Render(branch),
			ui.Path.Render(displayPath(dest)),
		)
		if mode == workspace.KindCoW {
			fmt.Printf("  %s\n", ui.Muted.Render(fmt.Sprintf(
				"snapshot of %s — `git fetch %s` reaches it",
				displayPath(ctx.MainPath), workspace.SourceRemoteName,
			)))
		}
		return nil
	},
}

func init() {
	createCmd.Flags().String("from", "", "Base ref for new branches (default: HEAD)")
	createCmd.Flags().Bool("bare", false, "Create the workspace without running .ggw.yaml provisioning")
	registerModeFlags(createCmd)
	registerAsFlag(createCmd)
	rootCmd.AddCommand(createCmd)
}

// kindLabel names a workspace kind the way it is shown to a human.
func kindLabel(k workspace.Kind) string {
	if k == workspace.KindCoW {
		return "Copy-on-write workspace"
	}
	return "Worktree"
}
