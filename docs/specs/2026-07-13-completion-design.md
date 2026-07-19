# sxcli.dev/completion — Design Specification

> **Staleness note (2026-07-19):** this spec predates the fw v0.3.0
> composition release and is kept as designed. Known drift: blank-import
> integration (§2) is dead — compositions accept `bash.ID`/`zsh.ID`;
> `completionbash`/`completionzsh` are the applets' ALIASES (ids are
> `sxcli.dev/completion/bash` and `.../zsh`); registration now goes
> through the v0.3.0 chain; `engine` and `script` are PUBLIC packages
> (the §2 reversal supersedes §4/§7's `internal/` paths); the bash wire
> protocol grew `--line` and `--breaks` (§6). The code and README are
> the current truth.

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

REVERSED 2026-07-14, before first publishing: `engine` and `script`
are PUBLIC packages (`sxcli.dev/completion/engine`,
`sxcli.dev/completion/script`). Rationale: the fish adapter was
dropped from the roadmap (Plamen: not a shell he wants to support;
community territory) — but "the community can build fish" is only
true if the engine is reachable, and internal/ would have blocked
third-party adapters until a breaking reshuffle. Published public
from day one instead; the as-if-public discipline made the move a
git mv. The layering rule is unchanged: the engine still does not
know scripts exist — script (generation policy) is public alongside
it so dispatch truth is never reimplemented per shell.

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

## 6. Bash adapter (landed 2026-07-13)

Wire protocol — raw transport, smart Go; the script is deliberately
dumb and never needs to change:

    <cmd> completionbash [--applet <id>] --cword $COMP_CWORD -- "${COMP_WORDS[@]}"

Raw COMP_WORDS as positionals (command word included, =-splits
unrepaired), COMP_CWORD locating the cursor. The Go side drops word
zero, reassembles the =-splits (bash's COMP_WORDBREAKS tears
`--debug=fal` into three words; value candidates return bare because
bash replaces only the post-= word), slices at the cursor and calls
the engine. All fields env:"-": per-keystroke transport, not
configuration. Queries always exit 0.

`--script` generation happens THROUGH the name it serves — the
basename decision is made once, at generation, never per keystroke:
single-applet mode → nothing baked (any name runs the sole applet);
basename names a public applet (busybox symlink farm) → `--applet`
baked, mirroring dispatch rule 4; anything else — the real binary name
included — keeps selector logic live, so `mybin cat /tmp/z<TAB>`
completes via the engine's bare-first-word rule and an explicitly
typed Hidden id still completes its arguments while never being
offered by name. Note: generation cannot distinguish "real binary
name" from "symlink to a hidden applet" (Applets is public-only, by
design) and does not need to — selector-mode registration is exactly
what dispatch honors for both, so the refusal case from the earlier
draft dissolved. Installation: `eval "$(mybin completionbash
--script)"` per name, one eval per symlink actually created — a
single blanket registration for every applet id would hijack real
commands' completions (cat!).

Answer encoding: one candidate per line on stdout; KindFiles/KindDirs
arrive as \001-sentinel lines the script maps to `compgen -f`/`-d`;
`complete -o default` keeps undeclared values on the shell's own file
completion. Unit-tested in `z_bash_test.go` (fake Source: reassembly,
baked/selector queries, directives, script golden fragments);
end-to-end smoke verified against a real fw binary.

Colon/equals handling (2026-07-13): bash splits COMP_WORDS at every
COMP_WORDBREAKS character — ":" and "=" included — so true tokens like
`unix:/dev/log` arrive shredded, and COMP_WORDS alone cannot tell
`--addr:8080` from `--addr :8080`. The protocol therefore carries
`--line "$COMP_LINE"` and `--breaks "$COMP_WORDBREAKS"`, and
`reassemble` is a faithful Go port of bash-completion's
`_comp__reassemble_words`: the original line's whitespace decides
glued-vs-spaced separators, separators never join word 0, and only
characters actually present in the user's break set are excluded (a
shell with ":" stripped from COMP_WORDBREAKS — the documented
workaround — passes words through untouched). On output, bash replaces
only the segment after the last break it split on, so `answer` prints
candidates segment-relative (the `__ltrim_colon_completions`
treatment, generalized): `unix:<TAB>` answers `/dev/log`, not
`unix:/dev/log`, and a shell that does NOT break on "=" gets the full
`--name=value` token rebuilt. Assumption: breaks default to bash's
stock set when a query arrives without --breaks (manual invocation).

## 7. Zsh adapter (landed 2026-07-14)

Same shape as bash over the same engine; the shell changes four
things. Transport is SIMPLER — zsh tokenizes properly (no
COMP_WORDBREAKS shredding, quotes honored), so there is no line, no
breaks, no reassembly, and no output segmenting:

    <cmd> completionzsh [--applet <id>] --cword $((CURRENT-1)) \
        --current "$PREFIX" -- "${(@)words}"

$PREFIX (the pre-cursor part of the current word) is exactly the
engine's Current contract — better than bash, which only has the whole
word. Answers are _describe-ready `value:description` lines (colons in
values escaped), so the usage/Doc metadata finally renders — rendered
through Tr(), the module's first translation consumer; long-form Doc
is reduced to its first line. Directives stay the \001 sentinels
(mapped to `_files` / `_files -/`); zero candidates → `_default`,
mirroring bash's `-o default`. Registration via
`eval "$(mybin completionzsh --script)"`, requires compinit.

The --script baking decision (single-applet / busybox symlink /
selector mode) moved to a shared package, `internal/script`, used by
both adapters — the engine stays ignorant that script generation
exists (Plamen's call: layering, engine = "what completes here" only).
Unit tests in z_zsh_test.go; smoke-verified under a real zsh with
stubbed _describe/_files/_default sourcing the generated script.

## 8. Open questions (next discussion targets)

- fish adapter (same shape; native descriptions, `complete -c`).
- Integration (x_) tests: bash AND zsh DONE (x_bash_test.go +
  x_zsh_test.go, 2026-07-14 — the test binary re-execs itself as a
  real fw application via fw's personality pattern; six wire-level
  query assertions each, plus one test per shell sourcing the
  generated script under the real shell with stubbed completion
  machinery — bash reads COMPREPLY, zsh reads the _describe pairs;
  skipped cleanly when the shell is absent).
- Later: completing `--override` values (understanding the `from=to`
  pair form is engine-side knowledge, not a field hint).
