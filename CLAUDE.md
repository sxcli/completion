# sxcli-completion — project conventions

Same house rules as sxcli-fw:

## Test file naming

- Unit test sources must be named `z_*_test.go`.
- Integration test sources must be named `x_*_test.go`.

## Code style

- Nested `if` statements are mandatory — do not flatten them into guard
  clauses / early returns.
- Complex boolean expressions are allowed.
- We prefer C-style `for` loops. Not a hard rule — ranged `for` loops
  have their uses — but blindly enforcing them everywhere is stupid and
  not worth our attention.

## Design discipline

- Design is discussed BEFORE implementation, point by point; wait for
  an explicit go.
- The living spec is `docs/specs/2026-07-13-completion-design.md`;
  every decision lands there.
- The engine stays in `internal/` for now but its API is written as if
  public — it may be promoted once third-party shells appear.
