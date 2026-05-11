package worktree

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// branchExistsLocal reports whether `branch` resolves to a local ref.
func branchExistsLocal(repoPath, branch string) bool {
	cmd := exec.Command("git", "-C", repoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return cmd.Run() == nil
}

// remoteBranchRef returns the cached remote ref (e.g. "origin/feature/x") for
// `branch` if it exists locally as a remote-tracking ref. Empty string if not.
func remoteBranchRef(repoPath, branch string) string {
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
	if err := prepareDestination(opts.DestPath); err != nil {
		return err
	}

	args := []string{"-C", opts.RepoPath, "worktree", "add"}
	switch {
	case branchExistsLocal(opts.RepoPath, opts.Branch):
		args = append(args, opts.DestPath, opts.Branch)
	case remoteBranchRef(opts.RepoPath, opts.Branch) != "":
		args = append(args, "--track", "-b", opts.Branch, opts.DestPath, remoteBranchRef(opts.RepoPath, opts.Branch))
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
	if err := prepareDestination(destPath); err != nil {
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

func prepareDestination(destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("path already exists: %s", destPath)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("cannot create parent directory: %w", err)
	}
	return nil
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
