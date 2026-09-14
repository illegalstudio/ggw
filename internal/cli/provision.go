package cli

import (
	"io"

	"github.com/illegalstudio/ggw/internal/project"
	"github.com/illegalstudio/ggw/internal/workspace"
)

// provisionWorkspace applies the repo's .ggw.yaml to a freshly created
// workspace at dest. It is a no-op when bare is true or no .ggw.yaml exists.
// On error the caller is responsible for rolling the workspace back.
//
// A copy-on-write workspace is a snapshot of the main worktree, so it already
// contains everything `copy` and `symlink` exist to reproduce — running them
// would only fail on destinations that are already there. Those steps are
// skipped for that kind, and only post_create runs.
func provisionWorkspace(ctx *workspace.Context, dest string, kind workspace.Kind, bare bool, out io.Writer) error {
	if bare {
		return nil
	}
	mainPath, err := ctx.SourceRepo()
	if err != nil {
		return err
	}
	cfg, exists, err := project.Load(mainPath)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if kind == workspace.KindCoW {
		cfg = &project.Config{PostCreate: cfg.PostCreate}
	}
	return project.Provision(project.ProvisionOptions{
		MainPath: mainPath,
		DestPath: dest,
		Config:   cfg,
		Out:      out,
	})
}
