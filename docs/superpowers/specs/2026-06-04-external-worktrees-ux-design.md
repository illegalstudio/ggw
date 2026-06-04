# Design: indirizzare worktree esterni/branchless in list, delete e cd

Data: 2026-06-04

## Problema

`ggw` gestisce worktree creati dal comando stesso (sotto `WorktreesBase()`), ma
la repo può contenere anche worktree creati da altri strumenti — ad esempio una
sessione Codex in `~/.codex/worktrees/0e21/elephc`. Questi worktree:

1. sono spesso **detached** → `Branch` vuoto, quindi il completion non ha un nome
   da suggerire;
2. hanno un **basename che collide** con quello del worktree principale (Codex usa
   il layout `<hash>/<repo>`, quindi il basename è il nome della repo);
3. non sono filtrati da `deleteCompletion` (che esclude solo main e current), ma
   il token che il completion propone (`elephc`) è ambiguo.

Conseguenza concreta: il completion di `delete` suggerisce `elephc`, ma
`ggw delete elephc` viene risolto da `resolveOneWorktree` sul **worktree
principale** (stesso basename, match prima nella catena) e fallisce con
"cannot delete the main worktree". Il worktree di Codex è di fatto non
indirizzabile per nome.

Possono essercene più di uno contemporaneamente.

## Obiettivo

Rendere i worktree branchless/esterni visibili in `list` e indirizzabili in
`delete` e `cd`, tramite un **handle univoco e digitabile**, senza introdurre
collisioni con il worktree principale.

## Concetto centrale: l'handle

Una funzione pura nel package `worktree` calcola, data la lista dei worktree, un
**handle univoco** per ciascuno:

- worktree con branch → handle = nome del branch (comportamento attuale,
  invariato);
- worktree branchless (detached / bare) → handle = **minimo numero di segmenti
  finali del path** che rende l'handle univoco rispetto a tutti gli altri handle
  e basename della lista.

Esempio (caso Codex):

```
~/.codex/worktrees/0e21/elephc
  → prova "elephc"        (collide col basename del main) ✗
  → sale di un livello
  → "0e21/elephc"         univoco ✓
```

L'handle è la **singola fonte di verità** usata da completion, matching esatto e
label della lista.

## Innesto nei comandi

### `resolveOneWorktree` (condivisa da `cd` e `delete`)

Si aggiunge l'handle nella catena di match esatto, con priorità:

```
branch esatto → handle esatto → basename esatto → substring su branch/path
```

Così `ggw delete 0e21/elephc` e `ggw cd 0e21/elephc` risolvono in modo
deterministico al worktree giusto, senza collidere col main.

### Completion

- `worktreeCompletion` (usata da `cd`) e `deleteCompletion` (usata da `delete`)
  suggeriscono **l'handle calcolato** invece di branch+basename grezzi. Per i
  branchless viene proposto `0e21/elephc` anziché l'ambiguo `elephc`.
- `delete` continua a filtrare main e current; `cd` li include come oggi.

### `selectWorktree` (selettore interattivo `huh`)

Per i branchless l'etichetta usa l'handle invece di `(detached)`, così la voce è
coerente con ciò che si digita.

### `list`

- Per i branchless la label diventa l'handle; resta un `(detached)` in muted come
  contesto.
- Suffisso `[external]` in muted, accanto all'eventuale `[locked]`, per i
  worktree esterni (vedi sotto).

## Rilevamento "external"

Un worktree è *external* quando:

- il suo path **non sta sotto `WorktreesBase()`**, **e**
- è **diverso dal worktree principale** (`list[0]`).

Un helper (nel package `worktree`, vicino a `WorktreesBase`) risponde sì/no dato
il path. Il main e i worktree creati da ggw (sotto
`WorktreesBase/<org>/<repo>/<slug>`) non vengono mai marcati.

Esposizione:

- `list` testuale → suffisso `[external]` in muted;
- `list --json` → nuovo campo `external bool` in `listEntry`;
- `delete` → **nessuna conferma extra**: stessa conferma standard, `--force` la
  salta (comportamento attuale invariato).

## Test (TDD)

Package `worktree`:

- calcolo handle: branch presente; detached con basename univoco; detached con
  basename che collide (caso Codex → `0e21/elephc`); due detached che collidono
  tra loro; bare;
- rilevamento `external`: path sotto `WorktreesBase`, path fuori, main escluso.

Package `cli` (estende `completion_test.go`):

- il completion propone l'handle e non il basename ambiguo;
- `resolveOneWorktree` con l'handle risolve al worktree corretto e non al main.

E2E (`tests/e2e_test.go`):

- worktree detached fuori da `WorktreesBase` → `list` lo mostra con handle +
  `[external]`; `delete <handle>` lo rimuove; `cd <handle>` ne stampa il path.

## Fuori scope (YAGNI)

- indici numerici per riga;
- conferme speciali o warning per i worktree external;
- gestione dei lock di altri strumenti;
- refactor non correlati.
