package workspace

import (
	"fmt"
	"strings"

	"github.com/illegalstudio/ggw/internal/cow"
	"github.com/illegalstudio/ggw/internal/worktree"
)

// cowParentForCreate resolves an explicit branch living in a cow workspace.
// The current cow branch takes precedence, then the original repository's
// refs, then an exact branch match in the other managed cow workspaces.
func cowParentForCreate(opts CreateOptions) (*Workspace, error) {
	if opts.From == "" || opts.From == "HEAD" {
		return nil, nil
	}
	source, err := opts.Ctx.SourceRepo()
	if err != nil {
		return nil, err
	}
	// Existing destination branches ignore --from, as in ordinary creation.
	if worktree.BranchExistsLocal(source, opts.Branch) || worktree.RemoteBranchRef(source, opts.Branch) != "" {
		return nil, nil
	}
	branch := strings.TrimPrefix(opts.From, "refs/heads/")
	if opts.Ctx.InCoW {
		head, current := worktree.HeadInfo(opts.Ctx.RepoPath)
		if current == branch && head != "" {
			return &Workspace{Kind: KindCoW, Path: opts.Ctx.RepoPath, Branch: current, Head: head}, nil
		}
	}
	if worktree.RefExists(source, opts.From) {
		return nil, nil
	}
	list, err := List(opts.Ctx)
	if err != nil {
		return nil, err
	}
	var matches []Workspace
	for _, ws := range list {
		if ws.Kind != KindCoW || ws.Branch != branch || ws.Head == "" {
			continue
		}
		marker, ok, err := cow.Read(ws.Path)
		if err != nil {
			return nil, err
		}
		if ok && marker.Org == opts.Ctx.Org && marker.Repo == opts.Ctx.Repo {
			matches = append(matches, ws)
		}
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		paths := make([]string, len(matches))
		for i, ws := range matches {
			paths[i] = ws.Path
		}
		return nil, fmt.Errorf("multiple copy-on-write workspaces contain branch %q: %s; run create from inside the intended workspace", branch, strings.Join(paths, ", "))
	}
}
