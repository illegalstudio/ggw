package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/illegalstudio/ggw/internal/layout"
	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/workspace"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use:     "pr <id>",
	Short:   "Create a tracked workspace for a GitHub pull request",
	GroupID: GroupWorktree,
	Args:    cobra.ExactArgs(1),
	Long: `Create a workspace with a GitHub pull request checked out in it.

The workspace kind follows the same rules as ` + "`ggw create`" + `: the config
file's ` + "`mode`" + `, overridden by --cow or --wt.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		prID, err := normalizePRID(args[0])
		if err != nil {
			return err
		}
		bare, _ := cmd.Flags().GetBool("bare")
		as, _ := cmd.Flags().GetString("as")

		mode, err := resolveMode(cmd)
		if err != nil {
			return err
		}
		if err := requireGH(); err != nil {
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

		slug, err := workspaceSlug(as, "pr-"+prID)
		if err != nil {
			return err
		}
		dest, err := layout.WorkspacePath(ctx.Org, ctx.Repo, slug)
		if err != nil {
			return err
		}

		// The branch is `gh pr checkout`'s to pick, so the workspace is created
		// without one and adopts whatever lands in it.
		if err := createForPR(ctx, mode, dest); err != nil {
			return err
		}

		// Roll back the workspace on any failure; a branch that existed before
		// this command is always kept.
		success := false
		defer func() {
			if !success {
				if rmErr := workspace.Remove(ctx, workspace.Workspace{Kind: mode, Path: dest}, true); rmErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to roll back workspace %s: %v\n", dest, rmErr)
				}
			}
		}()

		if err := runGHPRCheckout(dest, prID); err != nil {
			return err
		}

		branch, err := worktree.CurrentBranch(dest)
		if err != nil {
			return err
		}

		if mode == workspace.KindCoW {
			if err := workspace.FinalizeCoW(ctx, dest); err != nil {
				return err
			}
		}

		if err := provisionWorkspace(ctx, dest, mode, bare, os.Stderr); err != nil {
			return err
		}
		success = true

		if done, err := maybeJSON(map[string]any{
			"kind":   string(mode),
			"pr":     prID,
			"branch": branch,
			"slug":   slug,
			"path":   dest,
			"org":    ctx.Org,
			"repo":   ctx.Repo,
		}); done {
			return err
		}

		fmt.Printf("%s PR #%s %s created: %s → %s\n",
			ui.Success.Render("✓"),
			prID,
			strings.ToLower(kindLabel(mode)),
			ui.Branch.Render(branch),
			ui.Path.Render(displayPath(dest)),
		)
		return nil
	},
}

// createForPR makes an empty workspace for `gh pr checkout` to fill in. A
// worktree starts detached at HEAD; a copy-on-write workspace is snapshotted
// and left exactly where the source repository's HEAD was.
func createForPR(ctx *workspace.Context, mode workspace.Kind, dest string) error {
	if mode == workspace.KindCoW {
		return workspace.PrepareCoW(ctx, dest)
	}
	target, err := ctx.WorktreeTarget()
	if err != nil {
		return err
	}
	return worktree.CreateDetached(target, dest, "HEAD")
}

func init() {
	prCmd.Flags().Bool("bare", false, "Create the workspace without running .ggw.yaml provisioning")
	registerModeFlags(prCmd)
	registerAsFlag(prCmd)
	rootCmd.AddCommand(prCmd)
}

func normalizePRID(raw string) (string, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return "", fmt.Errorf("PR id must be a positive number")
	}
	return strconv.Itoa(id), nil
}

func requireGH() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh is required for `ggw pr`; install GitHub CLI from https://cli.github.com/ and run `gh auth login`")
	}
	return nil
}

func runGHPRCheckout(workspacePath, prID string) error {
	cmd := exec.Command("gh", "pr", "checkout", prID)
	cmd.Dir = workspacePath
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("gh pr checkout %s failed: %w: %s", prID, err, detail)
		}
		return fmt.Errorf("gh pr checkout %s failed: %w", prID, err)
	}
	return nil
}
