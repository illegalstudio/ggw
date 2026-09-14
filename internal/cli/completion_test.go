package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/illegalstudio/ggw/internal/workspace"
)

func TestWorkspaceCompletionItemsSkipsGeneratedBranchSlug(t *testing.T) {
	root := "/repo"
	list := []workspace.Workspace{
		{Path: root, Branch: "main"},
		{Path: "/worktrees/feature-some", Branch: "feature/some"},
	}

	got := workspaceCompletionItems(list, root, "")
	want := []string{"main", "feature/some"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWorkspaceCompletionItemsKeepsCustomBasename(t *testing.T) {
	root := "/repo"
	list := []workspace.Workspace{
		{Path: "/worktrees/review-copy", Branch: "feature/some"},
	}

	got := workspaceCompletionItems(list, root, "")
	want := []string{"feature/some", "review-copy"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWorkspaceCompletionItemsSuggestsHandleForDetached(t *testing.T) {
	root := "/Volumes/x/elephc"
	list := []workspace.Workspace{
		{Path: root, Branch: "main"},
		{Path: "/home/u/.codex/worktrees/0e21/elephc", Detached: true},
	}

	got := workspaceCompletionItems(list, root, "")
	want := []string{"main", "0e21/elephc"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Two copy-on-write workspaces on one branch (via --as) are only tellable
// apart by their directory, so both basenames have to be suggested even though
// one of them matches the branch slug.
func TestWorkspaceCompletionItemsKeepsBasenamesForSharedBranch(t *testing.T) {
	root := "/repo"
	list := []workspace.Workspace{
		{Kind: workspace.KindCoW, Path: "/worktrees/feature-some", Branch: "feature/some"},
		{Kind: workspace.KindCoW, Path: "/worktrees/review", Branch: "feature/some"},
	}

	got := workspaceCompletionItems(list, root, "")
	want := []string{"feature/some", "feature-some", "feature/some", "review"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCreateCompletionItemsExcludesBranchesWithWorkspaces(t *testing.T) {
	list := []workspace.Workspace{
		{Path: "/repo", Branch: "main"},
		{Path: "/worktrees/feature-x", Branch: "feature/x"},
	}
	local := []string{"main", "feature/x", "feature/y", "bugfix/z"}

	got := createCompletionItems(local, nil, list, "")
	want := []string{"feature/y", "bugfix/z"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCreateCompletionItemsIncludesRemoteWithoutLocalCounterpart(t *testing.T) {
	list := []workspace.Workspace{
		{Path: "/repo", Branch: "main"},
	}
	local := []string{"main", "feature/x"}
	remote := []string{"main", "feature/x", "colleague/feature"}

	got := createCompletionItems(local, remote, list, "")
	want := []string{"feature/x", "colleague/feature"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCreateCompletionItemsFiltersByPrefix(t *testing.T) {
	list := []workspace.Workspace{{Path: "/repo", Branch: "main"}}
	local := []string{"main", "feature/x", "feature/y", "bugfix/z"}

	got := createCompletionItems(local, nil, list, "feat")
	want := []string{"feature/x", "feature/y"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResolveOneWorkspaceMatchesHandleNotMain(t *testing.T) {
	list := []workspace.Workspace{
		{Path: "/Volumes/x/elephc", Branch: "main"},
		{Path: "/home/u/.codex/worktrees/0e21/elephc", Detached: true},
	}

	got, err := resolveOneWorkspace(list, "0e21/elephc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Path != "/home/u/.codex/worktrees/0e21/elephc" {
		t.Fatalf("resolved to %q, want the detached worktree", got.Path)
	}

	// The bare basename "elephc" is ambiguous: it must fall through to the
	// basename match and resolve to the main worktree, not the detached one.
	gotMain, err := resolveOneWorkspace(list, "elephc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMain.Path != "/Volumes/x/elephc" {
		t.Fatalf("query %q resolved to %q, want the main worktree", "elephc", gotMain.Path)
	}
}

// A branch shared by two workspaces identifies neither, so it must not resolve
// to whichever happens to come first.
func TestResolveOneWorkspaceRefusesAmbiguousBranch(t *testing.T) {
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	list := []workspace.Workspace{
		{Kind: workspace.KindCoW, Path: "/worktrees/feature-x", Branch: "feature/x"},
		{Kind: workspace.KindCoW, Path: "/worktrees/review", Branch: "feature/x"},
	}

	if _, err := resolveOneWorkspace(list, "feature/x"); err == nil {
		t.Fatal("expected an ambiguous branch to refuse to resolve")
	} else if !strings.Contains(err.Error(), "Multiple workspaces are on") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each is still addressable by its directory name.
	got, err := resolveOneWorkspace(list, "review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Path != "/worktrees/review" {
		t.Fatalf("resolved to %q, want /worktrees/review", got.Path)
	}
}
