# list-create-cd.tape — VHS demo of `ggw list`, `ggw create` and `ggw cd`.
#
# Requirements: vhs, ggw (on PATH), git, bash.
# Generate from the repo root:
#
#     vhs demo/list-create-cd.tape
#
# Output: demo/list-create-cd.gif
#
# The demo runs inside a throwaway sandbox ($HOME and the worktrees base are
# redirected to a temp dir) so it never touches your real repos. All the
# scaffolding happens between `Hide` and `Show`, so only the three commands
# the demo is about — list, create, cd — appear in the recording.

Output demo/list-create-cd.gif

Require ggw
Require git

Set Shell "bash"
Set FontSize 22
Set Width 1200
Set Height 768
Set Padding 28
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

# ── Sandbox setup (hidden) ───────────────────────────────────────────────
Hide
Type 'export GGW_DEMO="$(mktemp -d)"' Enter
Type 'export HOME="$GGW_DEMO"' Enter
Type 'export GIT_CONFIG_GLOBAL="$GGW_DEMO/.gitconfig"' Enter
Type 'export GIT_AUTHOR_NAME=demo GIT_AUTHOR_EMAIL=demo@example.com' Enter
Type 'export GIT_COMMITTER_NAME=demo GIT_COMMITTER_EMAIL=demo@example.com' Enter
Type 'export PS1="\[\e[1;35m\]\W\[\e[0m\] \[\e[2m\]❯\[\e[0m\] "' Enter
Type 'mkdir -p "$GGW_DEMO/work" && cd "$GGW_DEMO/work"' Enter
Type 'git init -q demo-repo && cd demo-repo' Enter
Type 'git remote add origin git@github.com:acme/api.git' Enter
Type 'git commit -q --allow-empty -m init && git branch -M main' Enter
Type 'command ggw create feature/api >/dev/null 2>&1' Enter
Type 'command ggw create hotfix/payments >/dev/null 2>&1' Enter
Type 'eval "$(command ggw shell-init bash)"' Enter
Type 'clear' Enter
Show

# ── Demo ─────────────────────────────────────────────────────────────────
Sleep 800ms

# 1. List every worktree of the current repo, with git status.
Type "ggw list" Sleep 500ms Enter
Sleep 2.5s

# 2. Create a worktree (and its branch) in a single, predictable step.
Type "ggw create feature/login" Sleep 500ms Enter
Sleep 2.5s

# 3. It shows up immediately in the list.
Type "ggw list" Sleep 500ms Enter
Sleep 2.5s

# 4. Jump into it — shell integration turns `ggw cd` into a real cd.
Type "ggw cd feature/login" Sleep 500ms Enter
Sleep 1.2s
Type "git branch --show-current" Sleep 400ms Enter
Sleep 3s

# ── Cleanup (hidden) ─────────────────────────────────────────────────────
Hide
Type 'rm -rf "$GGW_DEMO"' Enter
Show
