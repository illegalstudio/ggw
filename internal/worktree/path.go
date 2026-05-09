package worktree

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OriginOrgRepo returns the (org, repo) pair derived from the `origin` remote
// of the git repository at repoPath.
//
// If the repo has no `origin` remote configured, an explicit error is returned
// — the caller is expected to surface it. (We intentionally do not fall back
// to using the directory name: the storage layout assumes a stable
// org/repo pair, and silently inventing one is more surprising than failing.)
func OriginOrgRepo(repoPath string) (string, string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("no `origin` remote configured for %s (configure one with `git remote add origin …`)", repoPath)
	}
	return ParseOrgRepo(strings.TrimSpace(string(out)))
}

// ParseOrgRepo extracts the (org, repo) pair from a git remote URL.
//
// Supports SSH form (git@github.com:org/repo.git) and HTTPS form
// (https://github.com/org/repo.git). The trailing `.git` is optional.
//
// For URLs with deeper paths (e.g. GitLab subgroups: group/sub/repo), the
// last segment is taken as the repo and the second-to-last as the org. This
// loses the subgroup but keeps things flat under the worktrees dir.
func ParseOrgRepo(rawURL string) (string, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", fmt.Errorf("empty remote URL")
	}

	var path string
	if strings.Contains(rawURL, "@") && strings.Contains(rawURL, ":") && !strings.Contains(rawURL, "://") {
		// SSH form: git@host:org/repo.git
		colonIdx := strings.Index(rawURL, ":")
		path = rawURL[colonIdx+1:]
	} else {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return "", "", fmt.Errorf("invalid remote URL %q: %w", rawURL, err)
		}
		path = parsed.Path
	}

	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")

	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[len(parts)-1] == "" {
		return "", "", fmt.Errorf("cannot extract org/repo from %q", rawURL)
	}

	org := parts[len(parts)-2]
	repo := parts[len(parts)-1]
	return org, repo, nil
}

// SlugifyBranch turns a git branch name into a directory-safe slug:
// lowercased, alphanumerics and underscores preserved, every other rune
// folded to a single `-`, with no leading/trailing dashes.
//
// Examples:
//
//	"feature/login"  -> "feature-login"
//	"BugFix/User Auth" -> "bugfix-user-auth"
//	"hotfix-123"     -> "hotfix-123"
func SlugifyBranch(branch string) string {
	var b strings.Builder
	b.Grow(len(branch))

	lastDash := false
	for _, r := range strings.ToLower(branch) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	return strings.TrimRight(b.String(), "-")
}

// WorktreePath returns the absolute path where a worktree for (org, repo, slug)
// should live, honoring XDG_DATA_HOME when set.
func WorktreePath(org, repo, slug string) (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	if org == "" || repo == "" || slug == "" {
		return "", fmt.Errorf("worktree path components must be non-empty (org=%q repo=%q slug=%q)", org, repo, slug)
	}
	return filepath.Join(base, "worktrees", org, repo, slug), nil
}

// RepoRoot returns the absolute path of the git repo containing cwd.
// Returns an error if cwd is not inside a git work tree.
func RepoRoot(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository (run from within a repo)")
	}
	return strings.TrimSpace(string(out)), nil
}
