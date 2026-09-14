# Contribute to Joss

[Index](README.md) · Before: [architecture](ARQUITECTURA.md) · [Document audit](DOCUMENTATION_AUDIT.md)

Start with [AGENTS.md](../../AGENTS.md), the architecture, types, diagnostics and the
tests of the subsystem. The code and the tests executed are the source of truth;
a thesis, commentary, or historical document may describe a different objective.

## Flow of a contribution

1. Write the observable contract and its limits.
2. Locate the canonical source, without creating another parallel list.
3. Add the valid case and the invalid neighbor where appropriate.
4. Implement the same rule on the affected layers.
5. Update related tutorial, reference, catalog and diagnosis.
6. Run the validations on this page.

## Change language

New syntax typically loops through token/lexer, Pratt parser, AST, parser, and
evaluator. Defines precedence and associativity, error recovery and examples.
An operator requires type rules and runtime defense. Do not add `if`, imports,
namespaces or retired aliases as a compatibility shortcut: these are explicit
language decisions.

For a type, start at `pkg/typesystem`: Kind, canonical name, assignability,
inference and coercion. Then integrate it into analyzer/runtime and only touch parser if
there is new syntax. Updates [types](SISTEMA_TIPOS.md) and regenerates the catalog.

A public diagnosis uses stable code `JOSS-...`, severity, file/range,
explanation and suggestion. Adds sufficient evidence and protects against false positive neighbor
. Document the code at [diagnostics](DIAGNOSTICOS.md).

## Add native APIs

For a global built-in, the name lives at `pkg/core/builtins.go`; There must be
a reachable case in one of the dispatchers and a return in
`native_signatures.go`. For a class use the registry run by
`Runtime.RegisterNativeClasses()` and `GetNativeClassMethods()`. Publishing just a
name without a handler creates a ghost API; implementing just one case without registering it
creates unreachable code.

Add parameters to the metadata when supported; don't invent arity in the
parser. Update [contracts](MODULOS_NATIVOS.md), run
`go run ./tools/docgen` and check the example in real context.

For plugins, keep JOSSBC2Z, JPBC and the experimental VM separate. All new
Runtime mutable state needs a copy/share decision, cleanup at
`Free` and a concurrent test. A sensitive host operation must cross an actual
permissions boundary; Declaring it in metadata is not enough.

## Views, editor and publication

Keywords are projected with `parser.KeywordNames()`. VS Code consumes
`vscode-joss/src/server/generated/languageCatalog.json`; It is never edited by hand.
The canonical guides in Spanish live at `docs/*.md`; English and Portuguese keep
the same file name at `docs/en` and `docs/pt`. The versioned
JosSecurity release uses `assets/docs/{es,en,pt}` and must match byte-for-byte with each
source language. After modifying Spanish, run
`go run ./tools/docsi18n -translate -sync`; The hash manifest avoids unnecessary
work and `go run ./tools/docsi18n -check` detects obsolete translations, missing
, broken local links, and public mirror differences.

Full verifiable examples use `joss-run`, `joss-check` or
`joss-error` markers immediately before their fence. `documentation_test.go` parses
all three and runs `joss-run`, comparing output lines. Use `joss-check` for
fragments that depend on server, DB or plugins and explain that context.

## Validation

```sh
gofmt -w archivos_go_modificados
go run ./tools/cataloggen --check
go run ./tools/docgen --check
go run ./tools/docsi18n -check
go vet ./...
go test ./...
go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core
go build ./...
```

At `vscode-joss`: `npm ci` and `npm run compile`. Build a temporary binary of
`./cmd/joss` and analyze the actual integration project. Template changes,
migrations, or CRUD must pass the arrays named in AGENTS.md.

A documentary review should look for local links, Joss fences, legacy terms,
indexes and differences between registry and dispatcher. Avoid claiming portability,
atomicity, security, or full compatibility without proof to prove it.
