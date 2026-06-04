package cli

import (
	"reflect"
	"testing"

	"github.com/illegalstudio/ggw/internal/worktree"
)

func TestWorktreeCompletionItemsSkipsGeneratedBranchSlug(t *testing.T) {
	root := "/repo"
	list := []worktree.Worktree{
		{Path: root, Branch: "main"},
		{Path: "/worktrees/feature-some", Branch: "feature/some"},
	}

	got := worktreeCompletionItems(list, root, "")
	want := []string{"main", "feature/some"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWorktreeCompletionItemsKeepsCustomBasename(t *testing.T) {
	root := "/repo"
	list := []worktree.Worktree{
		{Path: "/worktrees/review-copy", Branch: "feature/some"},
	}

	got := worktreeCompletionItems(list, root, "")
	want := []string{"feature/some", "review-copy"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResolveOneWorktreeMatchesHandleNotMain(t *testing.T) {
	list := []worktree.Worktree{
		{Path: "/Volumes/x/elephc", Branch: "main"},
		{Path: "/home/u/.codex/worktrees/0e21/elephc", Detached: true},
	}

	got, err := resolveOneWorktree(list, "0e21/elephc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Path != "/home/u/.codex/worktrees/0e21/elephc" {
		t.Fatalf("resolved to %q, want the detached worktree", got.Path)
	}
}
