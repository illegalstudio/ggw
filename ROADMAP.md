# ggw roadmap

## Iteration 1: Scaffold + basic commands

- [x] Set up `go.mod` and folder structure
- [x] Cobra root command + stub subcommands
- [x] Worktree path resolution logic (`org/repo/slug`)
- [x] Branch name slugification
- [x] `ggw list` (+ `--json`)
- [x] `ggw create <name>` (auto-create branch if it doesn't exist)
- [x] Minimal README
- [x] Smoke test
- [x] Unit tests for slugification and path resolution

## Iteration 2: Navigation

- [x] `ggw init zsh|bash|fish`
- [x] `ggw cd [name]` (prints path; shell wrapper intercepts and chdirs)
- [x] `huh` selector when `name` is omitted or ambiguous
- [x] Shell function tested in zsh/bash

## Iteration 3: Operations

- [x] `ggw exec [name] -- <cmd>` (propagates exit code, refuses `--json`)
- [x] `ggw delete [name]` with `--without-branch` and `--force` (`huh` confirm; `--force` skips confirmation)

## Backlog / future

- [ ] Config file (TOML) for path override, post-create hooks, etc.
- [ ] Post-create hook (e.g. automatic `npm install`)
- [ ] Tab completion zsh/bash
- [ ] Goreleaser
- [ ] Homebrew tap
- [ ] Configurable fallback when `origin` remote is missing (today: explicit error)
