package worktree

import (
	"path/filepath"
	"testing"
)

func TestParseOrgRepo(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantOrg string
		wantRep string
		wantErr bool
	}{
		{"ssh github", "git@github.com:acme/api.git", "acme", "api", false},
		{"ssh github no .git", "git@github.com:acme/api", "acme", "api", false},
		{"https github", "https://github.com/acme/api.git", "acme", "api", false},
		{"https github no .git", "https://github.com/acme/api", "acme", "api", false},
		{"https github trailing slash", "https://github.com/acme/api/", "acme", "api", false},
		{"ssh gitlab subgroup uses last two", "git@gitlab.com:group/sub/repo.git", "sub", "repo", false},
		{"https gitlab subgroup uses last two", "https://gitlab.com/group/sub/repo.git", "sub", "repo", false},
		{"empty", "", "", "", true},
		{"single segment", "git@github.com:repo.git", "", "", true},
		{"https no path", "https://github.com/", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, repo, err := ParseOrgRepo(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got org=%q repo=%q", org, repo)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if org != tt.wantOrg || repo != tt.wantRep {
				t.Fatalf("got (%q, %q), want (%q, %q)", org, repo, tt.wantOrg, tt.wantRep)
			}
		})
	}
}

func TestSlugifyBranch(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"feature/login", "feature-login"},
		{"BugFix/User Auth", "bugfix-user-auth"},
		{"hotfix-123", "hotfix-123"},
		{"feat/can't-do", "feat-can-t-do"},
		{"foo//bar", "foo-bar"},
		{"  spaces  ", "spaces"},
		{"---leading-and-trailing---", "leading-and-trailing"},
		{"underscore_kept", "underscore_kept"},
		{"UPPER", "upper"},
		{"!!!", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := SlugifyBranch(tt.in)
			if got != tt.want {
				t.Fatalf("SlugifyBranch(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWorktreePathHonorsXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	got, err := WorktreePath("acme", "api", "feature-login")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/tmp/xdg", "worktrees", "acme", "api", "feature-login")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWorktreePathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/tmp/fakehome")
	got, err := WorktreePath("acme", "api", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/tmp/fakehome", ".local", "share", "worktrees", "acme", "api", "x")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWorktreePathRejectsEmptyComponents(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	if _, err := WorktreePath("", "api", "x"); err == nil {
		t.Fatal("expected error for empty org")
	}
	if _, err := WorktreePath("acme", "", "x"); err == nil {
		t.Fatal("expected error for empty repo")
	}
	if _, err := WorktreePath("acme", "api", ""); err == nil {
		t.Fatal("expected error for empty slug")
	}
}
