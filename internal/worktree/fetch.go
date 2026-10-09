package worktree

import "os/exec"

// RefExists reports whether ref resolves to an object in this repository.
func RefExists(repoPath, ref string) bool {
	return exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "--quiet", "--end-of-options", ref).Run() == nil
}

// FetchCommit imports objects from a local repository without creating or
// moving refs, changing FETCH_HEAD, fetching tags, or recursing into submodules.
// The caller must make commit reachable by creating the requested branch.
func FetchCommit(repoPath, sourcePath, commit string) error {
	_, err := exec.Command("git", "-C", repoPath, "fetch",
		"--no-tags", "--no-prune", "--no-prune-tags", "--refmap=",
		"--no-write-fetch-head", "--no-recurse-submodules",
		"--no-auto-maintenance", "--no-write-commit-graph",
		"--", sourcePath, commit).Output()
	if err != nil {
		return gitError("fetch local parent commit", err)
	}
	return nil
}
