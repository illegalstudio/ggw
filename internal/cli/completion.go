package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/illegalstudio/ggw/internal/layout"
	"github.com/illegalstudio/ggw/internal/workspace"
	"github.com/illegalstudio/ggw/internal/worktree"
	"github.com/spf13/cobra"
)

// workspaceCompletion returns branch names and path basenames of all
// workspaces in the current repository for shell autocompletion.
func workspaceCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, list, err := completionList()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	comps := workspaceCompletionItems(list, ctx.RepoPath, toComplete)
	return comps, cobra.ShellCompDirectiveNoFileComp
}

// completionList resolves the current repository and its workspaces, the first
// step every completion function shares.
func completionList() (*workspace.Context, []workspace.Workspace, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	ctx, err := workspace.Resolve(cwd)
	if err != nil {
		return nil, nil, err
	}
	list, err := workspace.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	return ctx, list, nil
}

// deleteCompletion is like workspaceCompletion, but it filters out the main
// worktree and the current workspace to avoid accidental deletions.
func deleteCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, list, err := completionList()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	if len(list) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	handles := workspace.Handles(list)
	var comps []string
	for i, w := range list {
		if w.Path == ctx.RepoPath || w.Main {
			continue
		}
		comps = appendWorkspaceCompletionItems(comps, w, handles[i], ctx.RepoPath, toComplete)
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
}

// createCompletion suggests branches that do not yet have a workspace, so that
// `ggw create <branch>` can check out an existing branch into a fresh workspace
// instead of always creating a new branch. Local branches are listed first,
// then origin branches that have no local counterpart.
func createCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, list, err := completionList()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	branchRepo := ctx.BranchRepo()
	local, err := worktree.LocalBranches(branchRepo)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	remote, err := worktree.RemoteBranches(branchRepo)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	comps := createCompletionItems(local, remote, list, toComplete)
	return comps, cobra.ShellCompDirectiveNoFileComp
}

// createCompletionItems returns branch names from local and remote that match
// toComplete and are not already checked out in an existing worktree. The
// result preserves order (local before remote) and contains no duplicates.
func createCompletionItems(local, remote []string, list []workspace.Workspace, toComplete string) []string {
	taken := make(map[string]bool)
	for _, w := range list {
		if w.Branch != "" {
			taken[w.Branch] = true
		}
	}

	var comps []string
	seen := make(map[string]bool)
	add := func(branch string) {
		if branch == "" || taken[branch] || seen[branch] {
			return
		}
		if completionMatches(branch, toComplete) {
			comps = append(comps, branch)
			seen[branch] = true
		}
	}

	for _, b := range local {
		add(b)
	}
	for _, b := range remote {
		add(b)
	}
	return comps
}

func workspaceCompletionItems(list []workspace.Workspace, root, toComplete string) []string {
	handles := workspace.Handles(list)
	var comps []string
	for i, w := range list {
		comps = appendWorkspaceCompletionItems(comps, w, handles[i], root, toComplete)
	}
	return comps
}

func appendWorkspaceCompletionItems(comps []string, w workspace.Workspace, handle, root, toComplete string) []string {
	if w.Branch == "" {
		// Branchless (detached/bare): the only stable, unambiguous token is
		// the computed handle (e.g. "0e21/elephc"). The bare basename may
		// collide with the main worktree, so we never suggest it.
		if w.Path != root && completionMatches(handle, toComplete) {
			comps = append(comps, handle)
		}
		return comps
	}

	if completionMatches(w.Branch, toComplete) {
		comps = append(comps, w.Branch)
	}

	base := filepath.Base(w.Path)
	// Do not suggest the main worktree basename: it is just the project
	// directory name and the user can already refer to it by branch name.
	if base == "" || base == w.Branch || w.Path == root {
		return comps
	}
	// A directory named after its own branch adds nothing — except when --as
	// gave two workspaces the same branch, where the directory is the only way
	// to tell them apart.
	if base == layout.SlugifyBranch(w.Branch) && handle == w.Branch {
		return comps
	}
	if completionMatches(base, toComplete) {
		comps = append(comps, base)
	}
	return comps
}

func completionMatches(candidate, toComplete string) bool {
	return toComplete == "" || strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(toComplete))
}

// skillTargetCompletionItems returns the `ggw skills install --target` keys
// that match toComplete, in declaration order.
func skillTargetCompletionItems(toComplete string) []string {
	var comps []string
	for _, key := range skillTargetKeys() {
		if completionMatches(key, toComplete) {
			comps = append(comps, key)
		}
	}
	return comps
}

func skillTargetCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return skillTargetCompletionItems(toComplete), cobra.ShellCompDirectiveNoFileComp
}

func registerSkillTargetCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("target", skillTargetCompletion)
}
