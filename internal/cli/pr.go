package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/illegalstudio/ggw/internal/ui"
	"github.com/illegalstudio/ggw/internal/worktree"

	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use:     "pr <id>",
	Short:   "Create a tracked worktree for a GitHub pull request",
	GroupID: GroupWorktree,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prID, err := normalizePRID(args[0])
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
		root, err := worktree.RepoRoot(cwd)
		if err != nil {
			return err
		}

		org, repo, err := worktree.OriginOrgRepo(root)
		if err != nil {
			return err
		}

		slug := "pr-" + prID
		dest, err := worktree.WorktreePath(org, repo, slug)
		if err != nil {
			return err
		}

		if err := worktree.CreateDetached(root, dest, "HEAD"); err != nil {
			return err
		}

		success := false
		defer func() {
			if !success {
				_ = worktree.Remove(root, dest, true)
			}
		}()

		if err := runGHPRCheckout(dest, prID); err != nil {
			return err
		}

		branch, err := worktree.CurrentBranch(dest)
		if err != nil {
			return err
		}
		success = true

		if done, err := maybeJSON(map[string]any{
			"pr":     prID,
			"branch": branch,
			"slug":   slug,
			"path":   dest,
			"org":    org,
			"repo":   repo,
		}); done {
			return err
		}

		fmt.Printf("%s PR #%s worktree created: %s → %s\n",
			ui.Success.Render("✓"),
			prID,
			ui.Branch.Render(branch),
			ui.Path.Render(displayPath(dest)),
		)
		return nil
	},
}

func init() {
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

func runGHPRCheckout(worktreePath, prID string) error {
	cmd := exec.Command("gh", "pr", "checkout", prID)
	cmd.Dir = worktreePath
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
