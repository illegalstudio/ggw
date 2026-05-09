# ggw roadmap

## Iterazione 1: Scaffold + comandi base

- [x] Setup `go.mod` e struttura cartelle
- [x] Cobra root command + sottocomandi stub
- [x] Logica risoluzione path worktree (`org/repo/slug`)
- [x] Slugificazione branch name
- [x] `ggw list` (+ `--json`)
- [x] `ggw create <name>` (auto-create branch se non esiste)
- [x] README minimale
- [x] Smoke test
- [x] Test unitari per slugificazione e path resolution

## Iterazione 2: Navigazione

- [ ] `ggw init zsh|bash|fish`
- [ ] Sottocomando interno `cd-path`
- [ ] `ggw cd [name]` con selector huh
- [ ] Shell function testata in zsh/bash

## Iterazione 3: Operazioni

- [ ] `ggw exec [name] -- <cmd>`
- [ ] `ggw delete [name]` con `--with-branch` e `--force`

## Backlog / future

- [ ] Config file (TOML) per override path, hooks post-create, ecc.
- [ ] Hook post-create (es. `npm install` automatico)
- [ ] Tab completion zsh/bash
- [ ] Goreleaser
- [ ] Homebrew tap
- [ ] Fallback configurabile quando manca remote `origin` (oggi: errore esplicito)
