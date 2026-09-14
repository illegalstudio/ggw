package worktree

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/illegalstudio/ggw/internal/layout"
)

// Worktree describes a single entry from `git worktree list --porcelain`.
type Worktree struct {
	Path     string `json:"path"`
	Head     string `json:"head,omitempty"`
	Branch   string `json:"branch,omitempty"` // empty if detached
	Detached bool   `json:"detached,omitempty"`
	Locked   bool   `json:"locked,omitempty"`
	Bare     bool   `json:"bare,omitempty"`
}

// MainWorktree returns the path of the repository's main worktree, which git
// always reports first. Every linked worktree of a repo resolves to the same
// answer, so it is the canonical identity of a repository on disk.
func MainWorktree(repoPath string) (string, error) {
	list, err := List(repoPath)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no worktrees found for %s", repoPath)
	}
	return list[0].Path, nil
}

// List returns all worktrees registered for the repo containing repoPath.
func List(repoPath string) ([]Worktree, error) {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, gitError("git worktree list", err)
	}
	return parseList(string(out)), nil
}

func parseList(s string) []Worktree {
	var result []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil && cur.Path != "" {
			result = append(result, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		if cur == nil {
			cur = &Worktree{}
		}
		key, val, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			cur.Path = val
		case "HEAD":
			cur.Head = val
		case "branch":
			cur.Branch = strings.TrimPrefix(val, "refs/heads/")
		case "detached":
			cur.Detached = true
		case "locked":
			cur.Locked = true
		case "bare":
			cur.Bare = true
		}
	}
	flush()
	return result
}

// BranchExistsLocal reports whether `branch` resolves to a local ref.
func BranchExistsLocal(repoPath, branch string) bool {
	cmd := exec.Command("git", "-C", repoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return cmd.Run() == nil
}

// RemoteBranchRef returns the cached remote ref (e.g. "origin/feature/x") for
// `branch` if it exists locally as a remote-tracking ref. Empty string if not.
func RemoteBranchRef(repoPath, branch string) string {
	candidate := "refs/remotes/origin/" + branch
	cmd := exec.Command("git", "-C", repoPath, "show-ref", "--verify", "--quiet", candidate)
	if cmd.Run() == nil {
		return "origin/" + branch
	}
	return ""
}

// DefaultBranch returns the branch pointed to by origin/HEAD, without the
// remote prefix. It is empty only if git returns an empty symbolic ref.
func DefaultBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", gitError("git symbolic-ref refs/remotes/origin/HEAD", err)
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "origin/"), nil
}

// LocalBranches returns the names of all local branches (refs/heads), in the
// order git reports them.
func LocalBranches(repoPath string) ([]string, error) {
	out, err := exec.Command("git", "-C", repoPath, "for-each-ref", "--format=%(refname:short)", "refs/heads").Output()
	if err != nil {
		return nil, gitError("git for-each-ref refs/heads", err)
	}
	return nonEmptyLines(string(out)), nil
}

// RemoteBranches returns the branches on the origin remote with the "origin/"
// prefix stripped. The symbolic origin/HEAD ref is skipped.
func RemoteBranches(repoPath string) ([]string, error) {
	out, err := exec.Command("git", "-C", repoPath, "for-each-ref", "--format=%(refname)", "refs/remotes/origin").Output()
	if err != nil {
		return nil, gitError("git for-each-ref refs/remotes/origin", err)
	}
	const prefix = "refs/remotes/origin/"
	var branches []string
	for _, line := range nonEmptyLines(string(out)) {
		name := strings.TrimPrefix(line, prefix)
		if name == line || name == "HEAD" {
			continue
		}
		branches = append(branches, name)
	}
	return branches, nil
}

// localBranchRefs returns the full refnames of every local branch. Full names
// avoid the branch/tag ambiguity a short name can carry.
func localBranchRefs(repoPath string) []string {
	out, err := exec.Command("git", "-C", repoPath, "for-each-ref", "--format=%(refname)", "refs/heads").Output()
	if err != nil {
		return nil
	}
	return nonEmptyLines(string(out))
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// CreateOptions configures a `git worktree add` invocation.
type CreateOptions struct {
	RepoPath string // git repo to operate from
	Branch   string // branch name (passed verbatim to git)
	DestPath string // absolute filesystem path for the new worktree
	From     string // optional base ref; only used when creating a new branch
}

// Create creates a new worktree for opts.Branch at opts.DestPath.
//
// Behavior:
//   - if the branch exists locally → checkout that branch
//   - else if a tracking ref `origin/<branch>` exists → create a tracking branch
//   - else → create a new branch from opts.From (or HEAD)
func Create(opts CreateOptions) error {
	if err := layout.EnsureFreeDestination(opts.DestPath); err != nil {
		return err
	}

	args := []string{"-C", opts.RepoPath, "worktree", "add"}
	switch {
	case BranchExistsLocal(opts.RepoPath, opts.Branch):
		args = append(args, opts.DestPath, opts.Branch)
	case RemoteBranchRef(opts.RepoPath, opts.Branch) != "":
		args = append(args, "--track", "-b", opts.Branch, opts.DestPath, RemoteBranchRef(opts.RepoPath, opts.Branch))
	default:
		base := opts.From
		if base == "" {
			base = "HEAD"
		}
		args = append(args, "-b", opts.Branch, opts.DestPath, base)
	}

	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree add failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// CreateDetached creates a detached worktree at destPath from ref.
func CreateDetached(repoPath, destPath, ref string) error {
	if err := layout.EnsureFreeDestination(destPath); err != nil {
		return err
	}
	if ref == "" {
		ref = "HEAD"
	}

	cmd := exec.Command("git", "-C", repoPath, "worktree", "add", "--detach", destPath, ref)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree add --detach failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// CurrentBranch returns the checked-out branch for repoPath.
func CurrentBranch(repoPath string) (string, error) {
	out, err := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output()
	if err != nil {
		return "", gitError("git branch --show-current", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("worktree at %s is detached", repoPath)
	}
	return branch, nil
}

// Status captures the per-worktree git state shown by `ggw list`.
type Status struct {
	Dirty       bool `json:"dirty"`
	Ahead       int  `json:"ahead"`
	Behind      int  `json:"behind"`
	HasUpstream bool `json:"has_upstream"`
}

// GetStatus runs `git status --porcelain` and (if an upstream is configured)
// `git rev-list --left-right --count HEAD...@{upstream}` for the worktree at
// worktreePath. A missing upstream is not an error — Ahead/Behind are simply
// left at zero and HasUpstream stays false.
func GetStatus(worktreePath string) (Status, error) {
	var s Status

	out, err := exec.Command("git", "-C", worktreePath, "status", "--porcelain").Output()
	if err != nil {
		return s, gitError("git status", err)
	}
	s.Dirty = len(strings.TrimSpace(string(out))) > 0

	out, err = exec.Command("git", "-C", worktreePath, "rev-list", "--left-right", "--count", "HEAD...@{upstream}").Output()
	if err == nil {
		var ahead, behind int
		if _, scanErr := fmt.Sscanf(strings.TrimSpace(string(out)), "%d\t%d", &ahead, &behind); scanErr == nil {
			s.Ahead = ahead
			s.Behind = behind
			s.HasUpstream = true
		}
	}
	return s, nil
}

// Remove invokes `git worktree remove` for the given worktree path.
// If force is true, dirty worktrees are removed too.
func Remove(repoPath, worktreePath string, force bool) error {
	args := []string{"-C", repoPath, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)

	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree remove failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// DeleteBranch deletes a local branch (force, to also remove unmerged branches).
func DeleteBranch(repoPath, branch string) error {
	cmd := exec.Command("git", "-C", repoPath, "branch", "-D", branch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git branch -D %s failed: %w: %s", branch, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitError(label string, err error) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if stderr != "" {
			return fmt.Errorf("%s failed: %s", label, stderr)
		}
	}
	return fmt.Errorf("%s failed: %w", label, err)
}

// HeadInfo reports the HEAD commit and checked-out branch of the repository at
// repoPath. A detached HEAD yields an empty branch; a repository without
// commits yields both empty. Errors are folded into empty values on purpose —
// this is used while listing many workspaces, where one unreadable repo must
// not fail the whole listing.
func HeadInfo(repoPath string) (head, branch string) {
	if out, err := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output(); err == nil {
		head = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}
	return head, branch
}

// CheckoutOptions configures Checkout.
type CheckoutOptions struct {
	RepoPath string // repository to check out in
	Branch   string // branch name (passed verbatim to git)
	From     string // optional base ref; only used when creating a new branch
}

// Checkout switches the repository at opts.RepoPath onto opts.Branch, applying
// the same branch resolution as Create: an existing local branch is checked
// out, an `origin/<branch>` tracking ref becomes a new tracking branch, and
// anything else is a new branch off opts.From (or HEAD).
//
// It is Create's counterpart for copy-on-write workspaces, where the workspace
// directory already exists and only its HEAD has to move.
func Checkout(opts CheckoutOptions) error {
	args := []string{"-C", opts.RepoPath, "checkout"}
	switch {
	case BranchExistsLocal(opts.RepoPath, opts.Branch):
		args = append(args, opts.Branch)
	case RemoteBranchRef(opts.RepoPath, opts.Branch) != "":
		args = append(args, "--track", "-b", opts.Branch, RemoteBranchRef(opts.RepoPath, opts.Branch))
	default:
		base := opts.From
		if base == "" {
			base = "HEAD"
		}
		args = append(args, "-b", opts.Branch, base)
	}

	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git checkout %s failed: %w: %s", opts.Branch, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// HasRemote reports whether the repository at repoPath already has a remote
// called name.
func HasRemote(repoPath, name string) bool {
	return exec.Command("git", "-C", repoPath, "remote", "get-url", name).Run() == nil
}

// AddRemote adds a remote called name pointing at url.
func AddRemote(repoPath, name, url string) error {
	cmd := exec.Command("git", "-C", repoPath, "remote", "add", name, url)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git remote add %s failed: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// UnpushedCommits counts the commits on HEAD that exist nowhere but this
// repository — the work that would be lost if it were deleted.
//
// Commits reachable from a remote-tracking ref are safe, and so are commits
// reachable from another local branch: a copy-on-write workspace inherits every
// branch of the repository it was snapshotted from, so the history it starts
// out with still lives in the original. Only what was committed on top of that
// is genuinely at risk.
func UnpushedCommits(repoPath string) (int, error) {
	// A repository with no commits has no HEAD to walk, and nothing at risk.
	if exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "--quiet", "HEAD").Run() != nil {
		return 0, nil
	}

	args := []string{"-C", repoPath, "rev-list", "--count", "HEAD", "--not", "--remotes"}
	// Everything after --not is excluded, so these are plain refs, never ^refs.
	_, current := HeadInfo(repoPath)
	for _, ref := range localBranchRefs(repoPath) {
		if current == "" || ref != "refs/heads/"+current {
			args = append(args, ref)
		}
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return 0, gitError("git rev-list --count HEAD --not --remotes", err)
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n); err != nil {
		return 0, fmt.Errorf("cannot parse unpushed commit count: %w", err)
	}
	return n, nil
}
