package worktree

import (
	"reflect"
	"testing"
)

func TestHandles(t *testing.T) {
	tests := []struct {
		name string
		list []Worktree
		want []string
	}{
		{
			name: "branch worktrees use the branch name",
			list: []Worktree{
				{Path: "/Volumes/x/elephc", Branch: "main"},
				{Path: "/data/worktrees/org/elephc/feature-x", Branch: "feature/x"},
			},
			want: []string{"main", "feature/x"},
		},
		{
			name: "detached with unique basename uses the basename",
			list: []Worktree{
				{Path: "/Volumes/x/elephc", Branch: "main"},
				{Path: "/home/u/scratch", Detached: true},
			},
			want: []string{"main", "scratch"},
		},
		{
			name: "detached basename colliding with main grows to be unique",
			list: []Worktree{
				{Path: "/Volumes/x/elephc", Branch: "main"},
				{Path: "/home/u/.codex/worktrees/0e21/elephc", Detached: true},
			},
			want: []string{"main", "0e21/elephc"},
		},
		{
			name: "two colliding detached worktrees both grow",
			list: []Worktree{
				{Path: "/a/0e21/elephc", Detached: true},
				{Path: "/b/9f33/elephc", Detached: true},
			},
			want: []string{"0e21/elephc", "9f33/elephc"},
		},
		{
			name: "detached basename equal to a branch name is avoided",
			list: []Worktree{
				{Path: "/repo", Branch: "elephc"},
				{Path: "/home/u/0e21/elephc", Detached: true},
			},
			want: []string{"elephc", "0e21/elephc"},
		},
		{
			name: "bare worktree falls back to a path-based handle",
			list: []Worktree{
				{Path: "/srv/project", Branch: "main"},
				{Path: "/srv/project/.bare", Bare: true},
			},
			want: []string{"main", ".bare"},
		},
		{
			name: "empty list returns empty handles",
			list: []Worktree{},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Handles(tt.list)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Handles() = %v, want %v", got, tt.want)
			}
		})
	}
}
