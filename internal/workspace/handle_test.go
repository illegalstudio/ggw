package workspace

import (
	"reflect"
	"testing"
)

func TestHandles(t *testing.T) {
	tests := []struct {
		name string
		list []Workspace
		want []string
	}{
		{
			name: "branch worktrees use the branch name",
			list: []Workspace{
				{Path: "/Volumes/x/elephc", Branch: "main"},
				{Path: "/data/worktrees/org/elephc/feature-x", Branch: "feature/x"},
			},
			want: []string{"main", "feature/x"},
		},
		{
			name: "detached with unique basename uses the basename",
			list: []Workspace{
				{Path: "/Volumes/x/elephc", Branch: "main"},
				{Path: "/home/u/scratch", Detached: true},
			},
			want: []string{"main", "scratch"},
		},
		{
			name: "detached basename colliding with main grows to be unique",
			list: []Workspace{
				{Path: "/Volumes/x/elephc", Branch: "main"},
				{Path: "/home/u/.codex/worktrees/0e21/elephc", Detached: true},
			},
			want: []string{"main", "0e21/elephc"},
		},
		{
			name: "two colliding detached worktrees both grow",
			list: []Workspace{
				{Path: "/a/0e21/elephc", Detached: true},
				{Path: "/b/9f33/elephc", Detached: true},
			},
			want: []string{"0e21/elephc", "9f33/elephc"},
		},
		{
			name: "detached basename equal to a branch name is avoided",
			list: []Workspace{
				{Path: "/repo", Branch: "elephc"},
				{Path: "/home/u/0e21/elephc", Detached: true},
			},
			want: []string{"elephc", "0e21/elephc"},
		},
		{
			name: "bare worktree falls back to a path-based handle",
			list: []Workspace{
				{Path: "/srv/project", Branch: "main"},
				{Path: "/srv/project/.bare", Bare: true},
			},
			want: []string{"main", ".bare"},
		},
		{
			name: "a branch shared by two workspaces falls back to paths",
			list: []Workspace{
				{Kind: KindCoW, Path: "/wt/acme/api/feature-x", Branch: "feature/x"},
				{Kind: KindCoW, Path: "/wt/acme/api/review", Branch: "feature/x"},
			},
			want: []string{"feature-x", "review"},
		},
		{
			name: "only the shared branch loses its name",
			list: []Workspace{
				{Path: "/repo", Branch: "main"},
				{Kind: KindCoW, Path: "/wt/acme/api/feature-x", Branch: "feature/x"},
				{Kind: KindCoW, Path: "/wt/acme/api/review", Branch: "feature/x"},
			},
			want: []string{"main", "feature-x", "review"},
		},
		{
			name: "a path handle never shadows a branch name",
			list: []Workspace{
				{Path: "/repo", Branch: "review"},
				{Kind: KindCoW, Path: "/wt/acme/api/feature-x", Branch: "feature/x"},
				{Kind: KindCoW, Path: "/wt/acme/api/review", Branch: "feature/x"},
			},
			want: []string{"review", "feature-x", "api/review"},
		},
		{
			name: "empty list returns empty handles",
			list: []Workspace{},
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
