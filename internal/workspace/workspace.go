// Package workspace is ggw's unified view of the places you can work on a
// repository.
//
// Two kinds share one namespace on disk. A *worktree* is a git worktree, which
// git tracks itself and which `git worktree list` can enumerate. A *cow*
// workspace is a copy-on-write snapshot: an independent repository that git
// knows nothing about, discovered by scanning the layout directory for ggw's
// marker. Every command above this package works in terms of Workspace and
// stays indifferent to which kind it got.
package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/illegalstudio/ggw/internal/cow"
	"github.com/illegalstudio/ggw/internal/layout"
	"github.com/illegalstudio/ggw/internal/worktree"
)

// Kind distinguishes the two ways a workspace can exist on disk.
type Kind string

const (
	KindWorktree Kind = "worktree"
	KindCoW      Kind = "cow"
)

// ParseKind validates a mode name coming from a flag, an environment variable
// or the config file.
func ParseKind(s string) (Kind, error) {
	switch s {
	case string(KindWorktree), "wt":
		return KindWorktree, nil
	case string(KindCoW):
		return KindCoW, nil
	default:
		return "", fmt.Errorf("unknown workspace mode %q (want %q or %q)", s, KindWorktree, KindCoW)
	}
}

// SourceRemoteName is the remote a copy-on-write workspace gets pointing back
// at the repository it was snapshotted from, so `git fetch main` reaches the
// original without anyone having to remember its path.
const SourceRemoteName = "main"

// Workspace is one place to work on a repository, of either kind.
type Workspace struct {
	Kind     Kind   `json:"kind"`
	Path     string `json:"path"`
	Head     string `json:"head,omitempty"`
	Branch   string `json:"branch,omitempty"` // empty if detached
	Detached bool   `json:"detached,omitempty"`
	Locked   bool   `json:"locked,omitempty"`
	Bare     bool   `json:"bare,omitempty"`
	// Main marks the repository's main worktree — the one that may not be
	// deleted and that copy-on-write snapshots are taken from.
	Main bool `json:"main,omitempty"`
}

// List returns every workspace of the repository ctx belongs to: its git
// worktrees first (main worktree first, as git reports them), then its
// copy-on-write snapshots ordered by path.
//
// Discovery of the two kinds is independent, and so is their failure: a
// repository without an `origin` remote has no layout directory to scan, but
// its worktrees still list fine.
func List(ctx *Context) ([]Workspace, error) {
	var out []Workspace

	if ctx.MainPath != "" {
		wts, err := worktree.List(ctx.MainPath)
		if err != nil {
			return nil, err
		}
		for i, w := range wts {
			out = append(out, Workspace{
				Kind:     KindWorktree,
				Path:     w.Path,
				Head:     w.Head,
				Branch:   w.Branch,
				Detached: w.Detached,
				Locked:   w.Locked,
				Bare:     w.Bare,
				Main:     i == 0,
			})
		}
	}

	if ctx.HasLayout() {
		snapshots, err := scanCoW(ctx.Org, ctx.Repo)
		if err != nil {
			return nil, err
		}
		out = append(out, snapshots...)
	}

	return ensureCurrent(ctx, out), nil
}

// ensureCurrent adds the workspace we are standing in when neither source found
// it.
//
// Only a snapshot can go missing this way, by being moved out of the layout
// directory or left behind by a change of base_dir: git enumerates every
// worktree of a repository, but nothing enumerates a snapshot except the scan.
// Leaving it out would make `ggw cd` and `ggw delete` unable to name the very
// place the user is in.
func ensureCurrent(ctx *Context, list []Workspace) []Workspace {
	if !ctx.InCoW {
		return list
	}
	for _, w := range list {
		if w.Path == ctx.RepoPath {
			return list
		}
	}

	head, branch := worktree.HeadInfo(ctx.RepoPath)
	return append(list, Workspace{
		Kind:     KindCoW,
		Path:     ctx.RepoPath,
		Head:     head,
		Branch:   branch,
		Detached: branch == "",
	})
}

// scanCoW finds the copy-on-write workspaces of (org, repo) by looking for
// ggw's marker in every entry of the layout directory. A missing directory
// simply means none were created yet.
func scanCoW(org, repo string) ([]Workspace, error) {
	dir, err := layout.RepoDir(org, repo)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot scan %s: %w", dir, err)
	}

	var out []Workspace
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if !cow.Is(path) {
			continue
		}
		// git reports worktree paths with symlinks resolved, and so does
		// RepoRoot. A scanned path that keeps them would compare unequal to the
		// same directory named by git — which is how "do not delete the
		// workspace you are standing in" would stop working.
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		head, branch := worktree.HeadInfo(path)
		out = append(out, Workspace{
			Kind:     KindCoW,
			Path:     path,
			Head:     head,
			Branch:   branch,
			Detached: branch == "",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// CreateOptions configures Create.
type CreateOptions struct {
	Ctx    *Context
	Kind   Kind
	Branch string // branch to end up on (passed verbatim to git)
	Slug   string // directory name under <base>/<org>/<repo>
	From   string // optional base ref, only used when creating a new branch
}

// Create makes a workspace of the requested kind and returns its path.
//
// Both kinds land in the same place — <base>/<org>/<repo>/<slug> — so a slug
// belongs to whichever kind claimed it first.
func Create(opts CreateOptions) (string, error) {
	dest, err := layout.WorkspacePath(opts.Ctx.Org, opts.Ctx.Repo, opts.Slug)
	if err != nil {
		return "", err
	}

	switch opts.Kind {
	case KindWorktree:
		target, err := opts.Ctx.WorktreeTarget()
		if err != nil {
			return "", err
		}
		if err := worktree.Create(worktree.CreateOptions{
			RepoPath: target,
			Branch:   opts.Branch,
			DestPath: dest,
			From:     opts.From,
		}); err != nil {
			return "", err
		}
		return dest, nil

	case KindCoW:
		return createCoW(opts, dest)

	default:
		return "", fmt.Errorf("unknown workspace kind %q", opts.Kind)
	}
}

// createCoW snapshots the repository's main worktree and moves the copy onto
// the requested branch. Everything after the snapshot is rolled back on
// failure: a half-built snapshot has nothing worth keeping, since its branch
// only ever existed inside it.
func createCoW(opts CreateOptions, dest string) (string, error) {
	if err := PrepareCoW(opts.Ctx, dest); err != nil {
		return "", err
	}

	done := false
	defer func() {
		if !done {
			_ = os.RemoveAll(dest)
		}
	}()

	if opts.Branch != "" {
		if err := worktree.Checkout(worktree.CheckoutOptions{
			RepoPath: dest,
			Branch:   opts.Branch,
			From:     opts.From,
		}); err != nil {
			return "", err
		}
	}
	if err := FinalizeCoW(opts.Ctx, dest); err != nil {
		return "", err
	}

	done = true
	return dest, nil
}

// PrepareCoW snapshots the repository's main worktree into dest and strips the
// state the copy must not inherit, leaving HEAD exactly where the source had
// it. Callers that decide the branch themselves — `ggw pr`, where `gh pr
// checkout` does — start here and finish with FinalizeCoW.
func PrepareCoW(ctx *Context, dest string) error {
	src, err := ctx.SourceRepo()
	if err != nil {
		return err
	}
	if err := cow.ValidateSource(src); err != nil {
		return err
	}
	if err := layout.EnsureFreeDestination(dest); err != nil {
		return err
	}
	if err := cow.Clone(src, dest); err != nil {
		return err
	}
	if err := cow.Sanitize(dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	return nil
}

// FinalizeCoW writes the bookkeeping that turns a sanitized snapshot into a
// workspace ggw can find again: the remote back to its source, and the marker
// that identifies it on disk. The caller rolls the workspace back on failure.
func FinalizeCoW(ctx *Context, dest string) error {
	src, err := ctx.SourceRepo()
	if err != nil {
		return err
	}
	if err := linkSourceRemote(dest, src); err != nil {
		return err
	}
	_, branch := worktree.HeadInfo(dest)
	return cow.Write(dest, cow.Marker{
		SourceRepo: src,
		Org:        ctx.Org,
		Repo:       ctx.Repo,
		Branch:     branch,
	})
}

// linkSourceRemote points a remote at the repository the snapshot came from, so
// work can flow back with `git fetch main` / `git push main HEAD`. An existing
// remote of that name is left alone: clobbering a user's remote to add a
// convenience is never worth it.
func linkSourceRemote(dest, src string) error {
	if worktree.HasRemote(dest, SourceRemoteName) {
		return nil
	}
	return worktree.AddRemote(dest, SourceRemoteName, src)
}

// Remove deletes a workspace.
//
// A worktree is handed back to git, which unregisters it and refuses to drop a
// dirty one unless forced. A copy-on-write workspace is a plain directory and
// is simply removed — there is no registration to clean up, and no branch left
// behind anywhere else.
func Remove(ctx *Context, ws Workspace, force bool) error {
	if ws.Kind == KindCoW {
		if err := os.RemoveAll(ws.Path); err != nil {
			return fmt.Errorf("cannot remove %s: %w", ws.Path, err)
		}
		return nil
	}

	target, err := ctx.WorktreeTarget()
	if err != nil {
		return err
	}
	return worktree.Remove(target, ws.Path, force)
}

// Status reports the git state of a workspace. Both kinds are ordinary git
// checkouts, so the question — and the answer — is the same for either.
func Status(ws Workspace) (worktree.Status, error) {
	return worktree.GetStatus(ws.Path)
}

// UnsavedWork is what removing a workspace would destroy.
type UnsavedWork struct {
	// Dirty reports uncommitted changes in the work tree. For a copy-on-write
	// workspace this is a weak signal: a snapshot inherits whatever the source
	// had in progress, so it can be dirty from birth with nothing of its own
	// at stake.
	Dirty bool
	// AtRisk counts commits that exist in no remote and in no other repository
	// — for a copy-on-write workspace, the work that vanishes with the
	// directory, silently and with no way back.
	AtRisk int
}

// Any reports whether there is anything to lose.
func (u UnsavedWork) Any() bool { return u.Dirty || u.AtRisk > 0 }

// Inspect reports the work that removing ws would destroy.
//
// A failure to measure is returned rather than swallowed. The caller is about
// to delete something irreplaceable, and "git would not answer" is not the same
// answer as "there is nothing to lose".
func Inspect(ctx *Context, ws Workspace) (UnsavedWork, error) {
	var u UnsavedWork

	st, err := worktree.GetStatus(ws.Path)
	if err != nil {
		return u, err
	}
	u.Dirty = st.Dirty

	// History the snapshot merely inherited is still in the repository it came
	// from. Only tips that repository still has are worth excluding — it may
	// have moved on, and rev-list cannot be handed a commit this one never saw.
	var elsewhere []string
	if ctx.MainPath != "" {
		elsewhere = worktree.KnownObjects(ws.Path, worktree.BranchTips(ctx.MainPath))
	}

	u.AtRisk, err = worktree.CommitsAtRisk(ws.Path, elsewhere)
	if err != nil {
		return u, err
	}
	return u, nil
}

// RescueCommand returns the git invocation that saves a copy-on-write
// workspace's branch into the repository it came from, so a refusal to delete
// can tell the user exactly how to keep their work.
//
// Paths are quoted: this is a command meant to be pasted into a shell, and a
// directory with a space in it would otherwise produce one that silently does
// something else.
func RescueCommand(ctx *Context, ws Workspace) string {
	if ws.Branch == "" || ctx.MainPath == "" {
		return ""
	}
	return fmt.Sprintf("git -C %s fetch %s %s:%s",
		shellQuote(ctx.MainPath), shellQuote(ws.Path), ws.Branch, ws.Branch)
}

// shellQuote renders s as a single POSIX shell word.
func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n\"'\\$`&;|<>()*?[]#~!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
