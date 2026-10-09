package worktree

import (
	"errors"
	"os/exec"
	"strings"
)

// configurePRBase remembers an explicit starting branch for gh pr create.
// Call it only after creating a new branch, in the repository that owns it.
func configurePRBase(repoPath, branch, from string) error {
	if from == "" {
		return nil
	}
	base, err := prBaseBranch(repoPath, from)
	if err != nil || base == "" {
		return err
	}
	return savePRBase(repoPath, branch, base)
}

// savePRBase preserves any base already configured for the new branch.
func savePRBase(repoPath, branch, base string) error {
	// A second independent workspace on the same branch is not a stacked PR.
	if branch == base {
		return nil
	}
	key := "branch." + branch + ".gh-merge-base"
	if _, err := exec.Command("git", "-C", repoPath, "config", "--get", key).Output(); err == nil {
		return nil
	} else {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return gitError("read PR base", err)
		}
	}
	_, err := exec.Command("git", "-C", repoPath, "config", "--local", key, base).Output()
	if err != nil {
		return gitError("save PR base", err)
	}
	return nil
}

// prBaseBranch accepts named branch refs, excluding tags, commit expressions,
// HEAD and ambiguous names. Remote prefixes are removed for GitHub CLI.
func prBaseBranch(repoPath, from string) (string, error) {
	out, err := exec.Command("git", "-C", repoPath, "rev-parse", "--symbolic-full-name", "--verify", "--end-of-options", from).Output()
	if err != nil {
		return "", gitError("resolve PR base", err)
	}
	ref := strings.TrimSpace(string(out))
	if name, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		if from == name || from == ref || from == "heads/"+name {
			return name, nil
		}
		return "", nil
	}
	name, ok := strings.CutPrefix(ref, "refs/remotes/")
	if !ok || (from != name && from != ref && from != "remotes/"+name) {
		return "", nil
	}
	out, err = exec.Command("git", "-C", repoPath, "remote").Output()
	if err != nil {
		return "", gitError("list remotes for PR base", err)
	}
	// Remote names may contain slashes. Match the longest configured prefix.
	var prefix string
	for _, remote := range nonEmptyLines(string(out)) {
		candidate := remote + "/"
		if strings.HasPrefix(name, candidate) && len(candidate) > len(prefix) {
			prefix = candidate
		}
	}
	if prefix == "" {
		return "", nil
	}
	base := strings.TrimPrefix(name, prefix)
	if base == "HEAD" {
		return "", nil
	}
	return base, nil
}
