package cli

import (
	"fmt"
	"io"

	"github.com/illegalstudio/ggw/internal/project"
	"github.com/illegalstudio/ggw/internal/worktree"
)

// mainWorktreePath returns the path of the repository's main worktree, which
// git always lists first. .ggw.yaml lives there and is the source for
// copy/symlink provisioning.
func mainWorktreePath(root string) (string, error) {
	list, err := worktree.List(root)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no worktrees found for %s", root)
	}
	return list[0].Path, nil
}

// provisionWorktree applies the repo's .ggw.yaml to a freshly created worktree
// at dest. It is a no-op when bare is true or no .ggw.yaml exists. On error the
// caller is responsible for rolling back the worktree.
func provisionWorktree(root, dest string, bare bool, out io.Writer) error {
	if bare {
		return nil
	}
	mainPath, err := mainWorktreePath(root)
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
	return project.Provision(project.ProvisionOptions{
		MainPath: mainPath,
		DestPath: dest,
		Config:   cfg,
		Out:      out,
	})
}
