# Review: sxcli.dev/completion v0.2.0 (read-only, Fable agent, 2026-07-19)

`go vet ./...` and `go test ./...` both pass cleanly (bash, engine, zsh all ok; x_ tests executed real bash and zsh).

## Ranked findings

**1. MAJOR — zsh/zsh.go:113-125 — all `=`-joined completions are dead under zsh (values, bools, and files after `--name=`).**
`answer` prints candidate values bare, and the script template (zsh.go:65-87) never calls `compset -P` and never rebuilds the token. zsh's compadd matches candidates against the whole current word `$PREFIX` — for `--level=de` the candidate `debug` doesn't match, so `_describe` adds nothing. Bash handles exactly this case (bash.go:146-150 "rebuild the full token", pinned by TestAnswerEqualsRebuiltWhenShellDoesNotBreakOnIt for "a shell whose breaks lack `=`") — zsh IS that shell, and got no equivalent.
Verified empirically under genuine zsh 5.9.1 (zpty harness, real compinit/compadd):
- control `srv --le<TAB>` → `srv --level ` (harness works);
- `srv --level=<TAB>` and `srv --level=de<TAB>` → nothing completed, though the applet emitted `debug\ninfo\nwarn`;
- emitting the full token `--level=debug` instead → completes correctly.
The same PREFIX mechanism kills `_files` for `--config=/et<TAB>` and the bool `--debug=t<TAB>` path. Fix: bash-style full-token rebuild in `answer`, or `compset -P '1*='` in the template.

**2. MAJOR — script/script.go:37-47 — `BakedApplet` mirrors only part of dispatch rule 4.**
fw basename dispatch resolves ANY alias, System applets included (main.go:181-189), but `BakedApplet` matches only against `src.Applets()` — public applets' PRIMARY aliases. A symlink named by a secondary alias (or a non-Hidden System applet's alias) dispatches to that applet, yet the generated script stays in selector mode, completing against the wrong schema. `engine.Source` cannot even express the needed lookup (no alias resolution) — an API-shape gap too. (The hidden-applet-symlink case is correctly dissolved; the secondary-alias case is not.)

**3. MINOR — engine/engine.go:106-118 — argument names offered where the parser forbids them.**
`walk` skips bare words and keeps offering `--` names afterwards, but the strict parser refuses flags after pending bare tokens ("positionals must come last"). `solo datafile --<TAB>` offers names whose acceptance produces a parse error. Completion should go silent once a bare positional precedes the cursor (except after `--`).

**4. MINOR — bash/bash.go:137 — `strings.LastIndex(q.Current, "=")` disagrees with the engine's first-`=` Cut.** For Allowed values containing `=`, the rebuilt token duplicates the prefix. Should be `strings.Index`.

**5. MINOR — bash/bash.go:82-86 — unquoted expansions leave candidates subject to pathname expansion.** `IFS=$'\n'` stops word-splitting but not globbing: a candidate containing `*`/`?`/`[...]` gets glob-expanded against the cwd. `mapfile`/`while read` or `set -f` is the standard defense.

**6. MINOR (documentation truth, public API) — engine/types.go:26-39, 69 — the `Source` contract says "ids" where the Introspector delivers primary ALIASES.** For the documented third-party-adapter entry point this is exactly the id/alias distinction the composition model made load-bearing.

**7. MINOR (documentation truth) — docs/specs/2026-07-13-completion-design.md — stale vs the v0.3.0 composition model:** blank-import integration (§2) — dead; "id `completionbash`" — that's the alias now; no spec entry records the registration-chain migration; §4/§7 "internal/engine"/"internal/script" contradict the §2 reversal note making both public; §6 wire-protocol block omits --line/--breaks.

**8. NIT — README.md:99 — "each is one template plus ~60 lines of Go"** — bash.go is ~240 lines; understates the adapter effort.

**9. TEST GAPS** — the real-shell x tests stub the matching layer, which is exactly why finding #1 was invisible (no test drives `--level=de<TAB>` through genuine compadd/readline); `KindDirs`/`HintDirectory` zero coverage anywhere; the `script` package has no tests of its own (finding #2 territory); no end-to-end for the bash `--breaks`-absent default path.

## Verified-correct highlights

Core filter matches actual fw behavior (`--disable core` fails resolution; CoreAlias leads Services()); the `pos:"rest"` Words transport is sound end to end (incl. `--` routing empty strings); `walk` mirrors the parser faithfully where it claims to (non-bool longs consume unconditionally, bool-= rules, bundle rules); `reassemble` + segment-relative trim check out against `_comp__reassemble_words` semantics in every traced case; both adapters register proper v0.3.0 chains; README/doc.go speak the composition model correctly; single-applet baking, System-selector carve-out, hidden-symlink dissolution all match fw dispatch.

## Overall assessment

A disciplined, well-layered module: the engine/script/adapter split keeps dispatch truth and candidate logic in one testable place, and the bash transport — the hardest part — is done carefully with real unit coverage. The two findings that matter: the zsh `=`-joined dead zone (empirically confirmed, user-visible, in the shell users most associate with rich completion — ironically a case bash documents and tests explicitly), and `BakedApplet`'s partial mirror of basename dispatch, which is both a bug and a hint that `engine.Source` needs alias resolution. The rest is polish: parser-conformance edges, bash quoting hygiene, and documentation (spec staleness, id-vs-alias vocabulary in the public Source contract) that should be trued up before a third-party adapter takes the README's invitation seriously.
