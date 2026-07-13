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

## 4. Engine API (decided 2026-07-13; implementation pending)

`internal/engine`, see `engine.go`. Decisions:

- **Single entry point** — `Complete(src, q) []Candidate` answers
  everything: applet-name completion, argument names, declared value
  domains, file/directory directives. The adapter never decides *what*
  is being completed, only how to print it.
- **`Source` is a locally-defined interface** (`Applets`, `Arguments`)
  — the honest ledger of what the module needs from the core;
  `*sxclifw.Introspector` satisfies it implicitly, tests use fakes.
- **`Query{Applet, Words, Current}`** — the words-before-cursor
  contract of the core Introspector is baked into the type system: the
  half-typed token has its own field (`Current`), used only as a
  filter prefix, never planned.
- **`Kind` directives, declared not guessed** — `KindFiles`/`KindDirs`
  are emitted only for fields with a declared `HintFile`/
  `HintDirectory` (fw `FieldMetadata.Hint`, landed fw@4b7edd4 with the
  core's own `--config` declaring `HintFile`). Undeclared plain string
  values yield no candidates; the generated bash script uses
  `complete -o default` so the shell's own file completion is the
  natural fallback.
- **No error return** — best-effort like the Introspector itself: a
  shell script cannot render an error; an unanswerable query yields no
  candidates. (Confirmed 2026-07-13.)
- **Service-id completion** — fields declared `HintServiceID` (fw
  d7232f2; the core's `--disable`/`--enable` declare it, `--override`
  does not — `from=to` pairs fit no honest hint) complete as
  `KindValue` candidates drawn from `Source.Services()`.

Dependency note: requires the fw hint API — unreleased at the time of
writing; `go.mod` carries `replace sxcli.dev/fw => ../sxcli-fw` until
fw v0.1.1 is tagged, when the replace is dropped.

## 5. Engine implementation (decided + landed 2026-07-13)

`Complete` resolves the target like core dispatch (explicit `q.Applet`
→ `SingleApplet()` — a core Introspector method added for exactly this,
fw d302f36, because `Applets()` is public-only while a Hidden
non-System applet still counts for the mode → bare first word as
selector; no target + no words = completing the first word: public
applet names). It then plans the schema via `Arguments(target, words)`
and replays the words exactly as the parser would: `--` puts the
cursor in positional land (silent), a non-bool long or final bundled
short with no `=`-joined value leaves a pending value, consumed longs
are recorded. Emission rules:

- pending value → `Allowed` domain, else bool `true`/`false`
  (reachable only through the `=` form — bools never consume the next
  word, mirroring the parser), else the hint (`KindFiles`/`KindDirs`
  directives; `HintServiceID` → `Services()`), else nothing (shell
  default).
- `Current` of the form `--name=prefix` → the semantic `=` split is
  the engine's (parser semantics); adapters only reassemble
  shell-mangled tokens. Value candidates are returned bare.
- `Current` starting with `-` → long argument names only (shorts are
  for people who know what they are doing), used scalars suppressed,
  used slices still offered (repetition is their append mechanism),
  `Doc` falling back to the usage one-liner.
- fresh bare word → silent; the shell's own default (files) is the
  honest fallback.

Unit-tested in `z_engine_test.go` against a fake `Source`.

## 6. Open questions (next discussion targets)

- Query argument protocol: how the script passes the target applet and
  words (`--applet <id>` + positionals? cursor index?), and how it
  behaves for single-applet binaries where no applet name is typed.
- Script installation UX: document `eval "$(bin completionbash --script)"`
  vs writing to the shell's completion directory.
- Testing: unit (z_) against a fake Source vs integration (x_) driving
  a real fw binary; golden files for scripts.
- Later: completing `--override` values (understanding the `from=to`
  pair form is engine-side knowledge, not a field hint).
