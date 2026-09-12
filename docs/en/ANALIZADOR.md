# Static analysis (`joss analyze`)

[Index](README.md)

`joss analyze [archivo.joss]` parses the input and files `.joss` under `app/` without running the application. The pipeline loads each file as a source unit, registers global declarations, and then parses each callable with its own scope.

```bash
joss analyze
joss analyze main.joss
```

The command returns non-zero exit code if there are errors. The warnings are displayed, but they do not block. `joss run` applies the same analysis before running and only continues if there are no errors.

## Current checks

- Undefined variables, redeclarations and unused premises.
- Functions, classes, superclasses and methods that do not exist when their receiver is known.
- Scope of parameters, functions, methods, `Init` and closures.
- Fixed inference on first assignment and reassignment compatibility.
- Typed initializers, unions/nullables, constants, defaults, arguments, returns, exhaustive paths and arity of Joss functions.
- Non-existent class types in variables, parameters and returns.
- Incompatible operators and indexes.
- Code after an unconditional `return`.
- Duplicate declarations at the project level.
- Symbols of native classes and JP v2 plugins loaded by the project.

## Evidence and unknown information

The parser differentiates `unknown` from invalid and from `mixed`. The returns from all core APIs have explicit metadata, but some entries do not match all runtime results (see [audit](DOCUMENTATION_AUDIT.md)). `mixed` represents intentional polymorphism. A native API without parameter metadata does not produce a speculative arity error; a dynamic receiver does not produce a member error; `isset` and `empty` can query a missing variable.

## Exit

The diagnostics use the model from `pkg/diagnostics`:

```text
error[JOSS-TYPE-001] app/example.joss:3:2: Cannot use `string` as assignment for `$age` of type `int`.
  suggestion: Convert the value explicitly or use `let $name` only when dynamic typing is intentional.
```

See [Diagnostics](DIAGNOSTICOS.md) for codes and severities, and [Type system](SISTEMA_TIPOS.md) for inference rules.

## Architecture

`pkg/analyzer` does not depend on `pkg/core`. The `pkg/core/analyzer.go` adapter builds your `Environment` from the actual records of built-ins, native classes and plugins. The CLI uses `analyzer.LoadProject`, so it preserves file, line, and column instead of concatenating ASTs.

## Known limits

- Exhaustive return testing covers blocks, ternaries, `match` with `default` and `try/catch`; does not yet demonstrate mathematical loop termination.
- Parameters of many native APIs remain variadic/unknown to avoid arity errors; Their returns are explicit.
- There is no branch-sensitive refinement analysis, formal taint/escape, or database schema contracts.
- Joss will not have a source import graph: the project uses automatic loading and a single declaration space.
- Parser recovery can still issue more than one derived diagnostic after an invalid token; each finding already preserves the line and column structured from the original token.

These limitations are not reported as user errors.
