# Evolution of Joss 2026: phases 0 and 1

This document records executable decisions following the
[critical evaluation](JOSS_LANGUAGE_REVIEW_2026.md) .The code and tests are
the source of truth.

## 1. Initial state

Baseline of `main` in `362c26a` , before changing semantics:

- `go test ./...` : correct.
- `go vet ./...` : correct.
- `go build ./...` : correct.
- `cataloggen --check` and `docgen --check`: correct.
- The parser gave higher precedence to `%` than to `*` / `/` , and treated `&&` and `||`
as equivalent.
- `??` recovered lookup panics, types and other errors and converted them to null.
- `0.0` and `{}` were true, although `0` , decimal zero and `[]` were false.
- `int → float` and `float → decimal` were implicitly assignable.

## 2. Semantic decisions

### Precedence

The canonical table lives in `pkg/parser/parser.go` and is published in
[Syntax](SINTAXIS.md) .From lowest to highest: assignment, ternary, coalescence,
pipeline, OR, AND, equality, comparison, shifts, range, sum, product,
prefixes, call and access.`*` , `/` and `%` share a level;`&&` outperforms `||` .

### Null coalescing

`izquierda ?? derecha` evaluates `derecha` only when `izquierda` terminates
normally with `null` .It does not catch exceptions, panics or program errors.
Explicit recovery belongs to `try` / `catch` .

### Truthiness

Truthiness is preserved because it is already part of existing ternaries, guards, filters and
APIs.The table is closed and is based on categories:

|Value |False |
|---|---|
|Absence |`null` |
|Boolean |`false` |
|Number |zero in `int` , `float` or `decimal` |
|String |empty |
|Collection |empty array or map |

All other values ​​are true.`"0"` is true because it is non-empty.

### Numeric Security

The only implicit promotion between numeric classes is `int → decimal` , which is exact
.`int → float` , `float → decimal` and mixed operations that depend on
require explicit casting.The integer division, the result of which is
`float` , rejects operands that cannot be represented exactly.The stable
code is `JOSS-ARITH-003` .

`const` continues to mean immutable binding.It does not promise immutability of the
object, of a collection or of the reachable graph.

## 3. Compatibility and migration

|Change |Class |Automatic migration |Action |
|---|---|---:|---|
|Conventional precedence |semantic breaking |Not in general |Add parentheses to preserve the previous result.|
|`??` only for null |breaking and security fix |No |Use `try` / `catch` if the recovery was intentional.|
|Uniform Truthiness |breaking for `0.0` , `{}` and `"0"` |No |Explicitly compare when the domain uses another rule.|
|Strict numerical conversions |static breaking |Partial |Insert `floatval(...)` or `decimal(...)` only with revised intent.|

A permanent branch of legacy semantics is not introduced.Projects that
depend on the above behavior must make their intent visible through
parentheses, comparisons, or conversions.

## 4. Bugs blocked by regression

- Complete arithmetic, logic, comparison, range, shifts,
pipeline, coalescence and ternary pipeline.
- Propagation of division by zero and invalid index via `??` .
- Conceptual equality of zero between `int` , `float` and `decimal` .
- Conceptual void equality between arrays and maps.
- Static rejection and runtime defense of known precision losses.

## 5. Surfaces prepared for phase 2

The next phase will consolidate `var` , explicit type, `mixed` and `const` , with
finite deprecation of`let` and implied statements.Before modifying the
parser, there must be an inventory of uses, a specific diagnosis and a codemod that
only transforms semantically demonstrable cases.

Also inventoried for later phases: mandatory returns and
`void` , closure inference, `unknown` reduction, native metadata,
namespaces, canonical constructor, structured concurrency and
profiles capabilities.None alter PHASE 1.

## 6. Open risks

- `floatval` expresses deliberate acceptance of approach;Your callers must
decide if the domain allows that loss.
- External sources can deliver already approximated floats before converting them
to decimal.The suffix `m` or decimal text avoids that border.
- The VM and plugins have their own testers.
equivalence will not be announced until differential testing covers these rules.
- Aggregations, JSON and DB drivers require extended boundary testing without
silently altering external contracts.

[Index](README.md)
