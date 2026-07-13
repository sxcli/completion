# sxcli.dev/completion — Design Specification

Status: living document, design phase. Decisions recorded as made.

## 1. Purpose & Position

Shell completion for binaries built on `sxcli.dev/fw`. Deliberately a
**separate module** — never part of the core — for two reasons:

- it hammers home that the core stays simple: completion is an ordinary
  ecosystem package with no privileged access;
- it is the first external consumer of the core's Introspector API —
  any capability gap it hits is fixed as a core API improvement, never
  a backdoor.

The core prerequisites landed in fw (spec §4/§5, commit d28173e):
`Hidden()`/`System()` registration options; System applets are
selectable by an explicit first token in every dispatch mode (including
single-applet mode), invisible in listings/basename dispatch, and
excluded from single-applet counting.

## 2. Shape

One applet **per shell**, each in its **own package**, selected by
blank import exactly like the fw log sinks — a binary links only the
shells it supports:

```go
import (
    _ "sxcli.dev/completion/bash"
    _ "sxcli.dev/completion/zsh"
    _ "sxcli.dev/completion/fish"
)
```

Each package registers one applet, id `completionbash` /
`completionzsh` / `completionfish` (plain lowercase — conforms to the
fw service-id rule; these are System applets, humans never type them).

The applets are **thin adapters**. All candidate computation lives
once, in `internal/engine`; a shell package owns exactly two things:

- its registration **script template** (bash `complete -F`, zsh
  `compdef`, fish `complete -c` — genuinely different syntax), and
- its **answer encoding** (bash: bare newline-separated words; zsh and
  fish carry per-candidate descriptions, sourced from the fw usage/Doc
  metadata).

`internal/engine` starts internal but its API is designed as if
public — it may be promoted to an exported package when a third-party
shell package (powershell, elvish, …) materializes; foreign modules
cannot reach another module's `internal/`.

## 3. Applet contract (to be detailed)

Two operations per shell applet:

- **script emission** — print the shell registration script to stdout
  (sourced/eval-ed by the user's shell setup);
- **query answering** — given the target applet and the words before
  the cursor, print the candidates in the shell's encoding.

The generated script invokes `<binary> completion<shell> …` — an
explicit first-token System selector, valid in every fw dispatch mode.

## 4. Open questions (next discussion targets)

- Engine API shape: inputs (Introspector, applet id, words before
  cursor), output candidate model (value, kind, description), error
  semantics.
- Query argument protocol: how the script passes the target applet and
  words (`--applet <id>` + positionals? cursor index?), and how it
  behaves for single-applet binaries where no applet name is typed.
- Completing the applet name itself in multi-applet binaries
  (Introspector.Applets is already filtered to public applets).
- File/path fallback: when a value has no Allowed domain, defer to the
  shell's own file completion instead of guessing.
- Script installation UX: document `eval "$(bin completionbash --script)"`
  vs writing to the shell's completion directory.
- Testing: unit (z_) against a fake Introspector-shaped source vs
  integration (x_) driving a real fw binary; golden files for scripts.
