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
