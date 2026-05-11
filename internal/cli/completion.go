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

// deleteCompletion è come worktreeCompletion ma filtra via il main worktree
// e il worktree corrente per evitare cancellazioni accidentali.
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

	var comps []string
	for _, w := range list {
		if w.Path == root || w.Path == mainWorktreePath {
			continue
		}
		comps = appendWorktreeCompletionItems(comps, w, root, toComplete)
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
}

func worktreeCompletionItems(list []worktree.Worktree, root, toComplete string) []string {
	var comps []string
	for _, w := range list {
		comps = appendWorktreeCompletionItems(comps, w, root, toComplete)
	}
	return comps
}

func appendWorktreeCompletionItems(comps []string, w worktree.Worktree, root, toComplete string) []string {
	if w.Branch != "" && completionMatches(w.Branch, toComplete) {
		comps = append(comps, w.Branch)
	}

	base := filepath.Base(w.Path)
	// Non suggerire il basename del main worktree (quello il cui path è la root del repo)
	// perché è solo il nome della directory del progetto e crea confusione.
	// L'utente può già riferirsi al main worktree tramite il suo branch name.
	if base == "" || base == w.Branch || w.Path == root {
		return comps
	}
	if w.Branch != "" && base == worktree.SlugifyBranch(w.Branch) {
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
