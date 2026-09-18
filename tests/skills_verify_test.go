package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGGWSplit is runGGW with stdout and stderr captured separately, which the
// stale-skill notice contract requires: the notice lives on stderr and must
// never leak into stdout.
func runGGWSplit(t *testing.T, home, cwd string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = cwd
	cmd.Env = childEnv(home)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func installSkill(t *testing.T, home, cwd, target string) {
	t.Helper()

	out, err := runGGW(t, home, cwd, "--json", "skills", "install", "--target", target)
	if err != nil {
		t.Fatalf("ggw --json skills install --target %s failed: %v\n%s", target, err, out)
	}
}

// TestCLISkillsVerifyEmitsPayloadOnStale covers the visible JSON contract:
// `ggw --json skills verify` exits 1 when an installed skill is stale, but
// still emits the full verifications payload instead of {"error": ...}.
func TestCLISkillsVerifyEmitsPayloadOnStale(t *testing.T) {
	home := setupHome(t)
	cwd := t.TempDir()

	installSkill(t, home, cwd, "agents")
	skillPath := filepath.Join(home, ".agents", "skills", "ggw", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runGGWSplit(t, home, cwd, "--json", "skills", "verify")
	if err == nil {
		t.Fatalf("ggw --json skills verify succeeded on a stale skill:\n%s", stdout)
	}
	if stderr != "" {
		t.Fatalf("stale --json verify wrote to stderr: %q", stderr)
	}

	var payload struct {
		Name          string `json:"name"`
		Verifications []struct {
			Target string `json:"target"`
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"verifications"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("stale verify did not emit the verifications payload: %v\n%s", err, stdout)
	}
	if payload.Name != "ggw" || len(payload.Verifications) != 2 {
		t.Fatalf("unexpected payload shape: %+v\n%s", payload, stdout)
	}
	if got := payload.Verifications[0]; got.Target != "agents" || got.Status != "modified" || got.Error != "" {
		t.Fatalf("agents verification = %+v, want modified without error\n%s", got, stdout)
	}
	if got := payload.Verifications[1]; got.Target != "claude" || got.Status != "not-installed" {
		t.Fatalf("claude verification = %+v, want not-installed\n%s", got, stdout)
	}

	// The human run fails too, and its closing message must recommend --force
	// for a modified copy, not the plain install that would conflict.
	out, err := runGGW(t, home, cwd, "skills", "verify")
	if err == nil {
		t.Fatalf("ggw skills verify succeeded on a stale skill:\n%s", out)
	}
	if !strings.Contains(out, "modified") || !strings.Contains(out, "--force") {
		t.Fatalf("human verify output missing status or --force advice:\n%s", out)
	}

	// A command-level failure (unknown target) still uses the {"error": ...}
	// shape, with no verifications payload.
	stdout, _, err = runGGWSplit(t, home, cwd, "--json", "skills", "verify", "--target", "cursor")
	if err == nil {
		t.Fatalf("ggw skills verify with an unknown target succeeded:\n%s", stdout)
	}
	var errPayload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &errPayload); err != nil || !strings.Contains(errPayload.Error, "unknown skill target") {
		t.Fatalf("unknown target did not emit a JSON error: %v\n%s", err, stdout)
	}
}

// TestCLISkillNoticeFollowsInteractiveCommands covers the post-success notice
// contract end to end: when it appears, where it appears, and where it must
// never appear.
func TestCLISkillNoticeFollowsInteractiveCommands(t *testing.T) {
	home := setupHome(t)
	repo := initGitRepo(t, home)
	const notice = "not in sync"

	// Nothing installed: no notice.
	_, stderr, err := runGGWSplit(t, home, repo, "list")
	if err != nil {
		t.Fatalf("ggw list failed: %v", err)
	}
	if strings.Contains(stderr, notice) {
		t.Fatalf("notice appeared with no skill installed:\n%s", stderr)
	}

	// Install, then make the copy stale by editing it.
	installSkill(t, home, repo, "agents")
	skillPath := filepath.Join(home, ".agents", "skills", "ggw", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Interactive command: notice on stderr, stdout untouched, and the
	// message explains update, verify, and suppression.
	stdout, stderr, err := runGGWSplit(t, home, repo, "list")
	if err != nil {
		t.Fatalf("ggw list failed: %v", err)
	}
	if !strings.Contains(stderr, notice) ||
		!strings.Contains(stderr, "ggw skills install --force") ||
		!strings.Contains(stderr, "ggw skills verify") ||
		!strings.Contains(stderr, "suppress_skills_notice") {
		t.Fatalf("stderr notice missing parts:\n%s", stderr)
	}
	if strings.Contains(stdout, notice) {
		t.Fatalf("notice leaked into stdout:\n%s", stdout)
	}

	// --json: machine output stays clean.
	stdout, stderr, err = runGGWSplit(t, home, repo, "--json", "list")
	if err != nil {
		t.Fatalf("ggw --json list failed: %v", err)
	}
	if strings.Contains(stderr, notice) {
		t.Fatalf("notice appeared under --json:\n%s", stderr)
	}
	var listPayload struct {
		Worktrees []struct {
			Path string `json:"path"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal([]byte(stdout), &listPayload); err != nil {
		t.Fatalf("--json list output is not clean JSON: %v\n%s", err, stdout)
	}

	// Per-command help and shell completion internals: no notice.
	if _, stderr, err = runGGWSplit(t, home, repo, "list", "--help"); err != nil {
		t.Fatalf("ggw list --help failed: %v", err)
	}
	if strings.Contains(stderr, notice) {
		t.Fatalf("notice appeared on --help:\n%s", stderr)
	}
	if _, stderr, err = runGGWSplit(t, home, repo, "__complete", "de"); err != nil {
		t.Fatalf("ggw __complete failed: %v", err)
	}
	if strings.Contains(stderr, notice) {
		t.Fatalf("notice appeared on __complete:\n%s", stderr)
	}

	// suppress_skills_notice: true silences the reminder.
	cfgDir := filepath.Join(home, ".config", "ggw")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("suppress_skills_notice: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err = runGGWSplit(t, home, repo, "list"); err != nil {
		t.Fatalf("ggw list failed: %v", err)
	}
	if strings.Contains(stderr, notice) {
		t.Fatalf("notice appeared despite suppress_skills_notice:\n%s", stderr)
	}
}
