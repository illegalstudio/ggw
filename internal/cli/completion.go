package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/illegalstudio/ggw/internal/worktree"
	"github.com/spf13/cobra"
)

// worktreeCompletion returns branch names and path basenames of all worktrees
// in the current repository for shell autocompletion.
func worktreeCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	root, err := worktree.RepoRoot(cwd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	list, err := worktree.List(root)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	comps := worktreeCompletionItems(list, root, toComplete)
	return comps, cobra.ShellCompDirectiveNoFileComp
}

// deleteCompletion is like worktreeCompletion, but it filters out the main
// worktree and the current worktree to avoid accidental deletions.
func deleteCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	root, err := worktree.RepoRoot(cwd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	list, err := worktree.List(root)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	if len(list) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	mainWorktreePath := list[0].Path

	handles := worktree.Handles(list)
	var comps []string
	for i, w := range list {
		if w.Path == root || w.Path == mainWorktreePath {
			continue
		}
		comps = appendWorktreeCompletionItems(comps, w, handles[i], root, toComplete)
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
}

func worktreeCompletionItems(list []worktree.Worktree, root, toComplete string) []string {
	handles := worktree.Handles(list)
	var comps []string
	for i, w := range list {
		comps = appendWorktreeCompletionItems(comps, w, handles[i], root, toComplete)
	}
	return comps
}

func appendWorktreeCompletionItems(comps []string, w worktree.Worktree, handle, root, toComplete string) []string {
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
	if base == worktree.SlugifyBranch(w.Branch) {
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
