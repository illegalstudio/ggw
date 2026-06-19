package cli

import (
	"fmt"

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
