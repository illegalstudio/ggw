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

	var comps []string
	for _, w := range list {
		if w.Branch != "" {
			if toComplete == "" || strings.HasPrefix(strings.ToLower(w.Branch), strings.ToLower(toComplete)) {
				comps = append(comps, w.Branch)
			}
		}
		base := filepath.Base(w.Path)
		// Non suggerire il basename del main worktree (quello il cui path è la root del repo)
		// perché è solo il nome della directory del progetto e crea confusione.
		// L'utente può già riferirsi al main worktree tramite il suo branch name.
		if base != "" && base != w.Branch && w.Path != root {
			if toComplete == "" || strings.HasPrefix(strings.ToLower(base), strings.ToLower(toComplete)) {
				comps = append(comps, base)
			}
		}
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
}

// deleteCompletion è come worktreeCompletion ma filtra via il main worktree
// (quello il cui path è la root del repo) per evitare cancellazioni accidentali.
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

	var comps []string
	for _, w := range list {
		if w.Path == root {
			continue
		}
		if w.Branch != "" {
			if toComplete == "" || strings.HasPrefix(strings.ToLower(w.Branch), strings.ToLower(toComplete)) {
				comps = append(comps, w.Branch)
			}
		}
		base := filepath.Base(w.Path)
		if base != "" && base != w.Branch {
			if toComplete == "" || strings.HasPrefix(strings.ToLower(base), strings.ToLower(toComplete)) {
				comps = append(comps, base)
			}
		}
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
}
