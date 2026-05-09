package cli

import (
	"fmt"
	"os"

	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:     "create <branch>",
	Short:   "Create a worktree for a branch (creates the branch if needed)",
	GroupID: GroupWorktree,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]
		from, _ := cmd.Flags().GetString("from")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := worktree.RepoRoot(cwd)
		if err != nil {
			return err
		}

		org, repo, err := worktree.OriginOrgRepo(root)
		if err != nil {
			return err
		}

		slug := worktree.SlugifyBranch(branch)
		if slug == "" {
			return fmt.Errorf("branch %q produces an empty slug", branch)
		}

		dest, err := worktree.WorktreePath(org, repo, slug)
		if err != nil {
			return err
		}

		if err := worktree.Create(worktree.CreateOptions{
			RepoPath: root,
			Branch:   branch,
			DestPath: dest,
			From:     from,
		}); err != nil {
			return err
		}

		if done, err := maybeJSON(map[string]any{
			"branch": branch,
			"slug":   slug,
			"path":   dest,
			"org":    org,
			"repo":   repo,
		}); done {
			return err
		}

		fmt.Printf("%s Worktree created: %s → %s\n",
			ui.Success.Render("✓"),
			ui.Branch.Render(branch),
			ui.Path.Render(dest),
		)
		return nil
	},
}

func init() {
	createCmd.Flags().String("from", "", "Base ref for new branches (default: HEAD)")
	rootCmd.AddCommand(createCmd)
}
