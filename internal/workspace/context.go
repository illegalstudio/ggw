package workspace

import (
	"fmt"

	"github.com/illegalstudio/ggw/internal/cow"
	"github.com/illegalstudio/ggw/internal/layout"
	"github.com/illegalstudio/ggw/internal/worktree"
)

// Context is the repository ggw is operating on, resolved from the current
// directory.
//
// It exists because a copy-on-write workspace is an independent repository:
// asking git from inside one answers only about itself, so the rest of the
// family — the main worktree and the sibling workspaces — has to be recovered
// from the marker ggw wrote at creation time.
type Context struct {
	// RepoPath is the git root of the current directory. It may be the main
	// worktree, a linked worktree, or a copy-on-write workspace.
	RepoPath string
	// MainPath is the canonical main worktree of the repository. It is empty
	// only when RepoPath is a snapshot whose source repository is gone.
	MainPath string
	// Org and Repo place the repository in the storage layout. They may be
	// empty when the repository has no `origin` remote.
	Org  string
	Repo string
	// InCoW reports whether the current directory is itself a copy-on-write
	// workspace.
	InCoW bool

	recordedSource string // marker's source_repo, kept for error messages
	layoutErr      error  // why Org/Repo are unknown, if they are
}

// Resolve identifies the repository containing cwd.
func Resolve(cwd string) (*Context, error) {
	root, err := layout.RepoRoot(cwd)
	if err != nil {
		return nil, err
	}
	ctx := &Context{RepoPath: root}

	marker, isCoW, err := cow.Read(root)
	if err != nil {
		return nil, err
	}
	switch {
	case isCoW:
		ctx.InCoW = true
		ctx.Org, ctx.Repo = marker.Org, marker.Repo
		ctx.recordedSource = marker.SourceRepo
		// A snapshot outlives its source: a missing one is a degraded state to
		// report when it matters, not a reason to fail every command.
		if marker.SourceRepo != "" {
			if main, err := worktree.MainWorktree(marker.SourceRepo); err == nil {
				ctx.MainPath = main
			}
		}
	default:
		main, err := worktree.MainWorktree(root)
		if err != nil {
			return nil, err
		}
		ctx.MainPath = main
	}

	if ctx.Org == "" || ctx.Repo == "" {
		org, repo, err := layout.OriginOrgRepo(root)
		if err != nil {
			ctx.layoutErr = err
		} else {
			ctx.Org, ctx.Repo = org, repo
		}
	}
	return ctx, nil
}

// HasLayout reports whether the repository can be placed in the storage layout,
// which requires an `origin` remote.
func (c *Context) HasLayout() bool { return c.Org != "" && c.Repo != "" }

// RequireLayout returns the reason the repository cannot be placed in the
// storage layout, for the commands that cannot work without it.
func (c *Context) RequireLayout() error {
	if c.HasLayout() {
		return nil
	}
	if c.layoutErr != nil {
		return c.layoutErr
	}
	return fmt.Errorf("cannot determine org/repo for %s", c.RepoPath)
}

// WorktreeTarget returns the repository that git worktrees are registered in
// and removed from. Run from inside a snapshot, that is still the original
// repository — a worktree belongs to the repo, not to the copy you happen to
// be standing in.
func (c *Context) WorktreeTarget() (string, error) {
	if c.MainPath != "" {
		return c.MainPath, nil
	}
	return "", c.missingSourceError()
}

// SourceRepo returns the main worktree that copy-on-write snapshots are taken
// from and that .ggw.yaml provisioning reads from.
func (c *Context) SourceRepo() (string, error) {
	if c.MainPath != "" {
		return c.MainPath, nil
	}
	return "", c.missingSourceError()
}

// BranchRepo returns the repository whose refs a new branch name must not
// collide with. A copy-on-write workspace inherits every branch of the repo it
// was snapshotted from, so that repo is the right place to ask even when the
// workspace itself is what ggw is standing in.
func (c *Context) BranchRepo() string {
	if c.MainPath != "" {
		return c.MainPath
	}
	return c.RepoPath
}

func (c *Context) missingSourceError() error {
	if c.recordedSource != "" {
		return fmt.Errorf("the repository this workspace was created from is gone (%s); run ggw from a clone of the repository instead", c.recordedSource)
	}
	return fmt.Errorf("cannot locate the main worktree of %s", c.RepoPath)
}
