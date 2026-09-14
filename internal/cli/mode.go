package cli

import (
	"fmt"
	"os"

	"github.com/illegalstudio/ggw/internal/config"
	"github.com/illegalstudio/ggw/internal/layout"
	"github.com/illegalstudio/ggw/internal/workspace"

	"github.com/spf13/cobra"
)

// ModeEnvVar overrides the configured default for one invocation, which is
// mostly useful to scripts and tests that should not depend on the user's
// config file.
const ModeEnvVar = "GGW_MODE"

// registerModeFlags adds the mutually exclusive pair that forces one kind of
// workspace for a single run.
func registerModeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("cow", false, "Make a copy-on-write workspace: a snapshot of the repository, untracked files included")
	cmd.Flags().Bool("wt", false, "Make a git worktree")
	cmd.MarkFlagsMutuallyExclusive("cow", "wt")
}

// registerAsFlag adds the flag that decouples the directory name from the
// branch name.
func registerAsFlag(cmd *cobra.Command) {
	cmd.Flags().String("as", "", "Directory name for the workspace (default: the slugified branch)")
}

// resolveMode picks the kind of workspace to create, in decreasing priority:
// the --cow/--wt flags, $GGW_MODE, the config file's `mode`, and finally
// worktrees — the behaviour ggw had before copy-on-write existed.
func resolveMode(cmd *cobra.Command) (workspace.Kind, error) {
	if forced, _ := cmd.Flags().GetBool("cow"); forced {
		return workspace.KindCoW, nil
	}
	if forced, _ := cmd.Flags().GetBool("wt"); forced {
		return workspace.KindWorktree, nil
	}

	if env := os.Getenv(ModeEnvVar); env != "" {
		kind, err := workspace.ParseKind(env)
		if err != nil {
			return "", fmt.Errorf("%s: %w", ModeEnvVar, err)
		}
		return kind, nil
	}

	raw, ok, err := config.Mode()
	if err != nil {
		return "", err
	}
	if ok {
		kind, err := workspace.ParseKind(raw)
		if err != nil {
			return "", fmt.Errorf("config file `mode`: %w", err)
		}
		return kind, nil
	}

	return workspace.KindWorktree, nil
}

// workspaceSlug picks the directory name for a new workspace: the --as value
// when given, the slugified branch otherwise.
//
// --as is what makes two workspaces on one branch possible — which only a
// copy-on-write workspace can actually be, since git refuses to check out the
// same branch in two worktrees.
func workspaceSlug(as, branch string) (string, error) {
	source, label := branch, "branch"
	if as != "" {
		source, label = as, "--as value"
	}
	slug := layout.SlugifyBranch(source)
	if slug == "" {
		return "", fmt.Errorf("%s %q produces an empty directory name", label, source)
	}
	return slug, nil
}
