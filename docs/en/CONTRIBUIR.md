# Contribute to Joss

[Index](README.md) · Before: [architecture](ARQUITECTURA.md) · [Document audit](DOCUMENTATION_AUDIT.md)

Start at [AGENTS.md](../../AGENTS.md), the architecture, types, diagnostics and the
subsystem tests. The code and the tests executed are the source of truth;
a thesis, commentary, or historical document may describe a different objective.

## Flow of a contribution

1. Write the observable contract and its limits.
2. Locate the canonical source, without creating another parallel list.
3. Add the valid case and the invalid neighbor where appropriate.
4. Implement the same rule on the affected layers.
5. Update related tutorial, reference, catalog and diagnosis.
6. Run the validations on this page.

## Change the language

New syntax typically loops through token/lexer, Pratt parser, AST, parser, and
evaluator. Defines precedence and associativity, error recovery and examples.
An operator requires type rules and runtime defense. Do not add `if`, imports,
namespaces or aliases removed as a compatibility shortcut: they are decisions
explicit language.

For a type, start at `pkg/typesystem`: Kind, canonical name, assignability,
inference and coercion. Then integrate it into analyzer/runtime and only touch parser if
there is new syntax. Updates [types](SISTEMA_TIPOS.md) and regenerates the catalog.

A public diagnosis uses stable code `JOSS-...`, severity, file/range,
explanation and suggestion. Add sufficient evidence and protect against false
positive neighbor. Document the code at [diagnostics](DIAGNOSTICOS.md).

## Add native APIs

For a global built-in, the name lives at `pkg/core/builtins.go`; must exist
a reachable case in one of the dispatchers and a return in
`native_signatures.go`. For a class use the registry executed by
`Runtime.RegisterNativeClasses()` and `GetNativeClassMethods()`. Post only one
name without handler creates a ghost API; implement only one case without registering it
creates unreachable code.

Add parameters to the metadata when supported; do not invent aridity in the
analyzer. Update [contracts](MODULOS_NATIVOS.md), run
`go run ./tools/docgen` and check the example in real context.

For plugins, keep JOSSBC2Z, JPBC, and the experimental VM separate. Everything new
Runtime mutable state needs a copy/share decision, cleanup in
`Free` and a concurrent test. A sensitive host operation must cross a
actual permit border; Declaring it in metadata is not enough.

## Views, editor and publication

The keywords are projected with `parser.KeywordNames()`. VS Code consumes
`vscode-joss/src/server/generated/languageCatalog.json`; It is never edited by hand.
The canonical guides in Spanish live at `docs/*.md`; English and Portuguese retain
the same file name at `docs/en` and `docs/pt`. The versioned publication of
JosSecurity uses `assets/docs/{es,en,pt}` and must match byte for byte with each
source language. After modifying Spanish, execute
`go run ./tools/docsi18n -translate -sync`; hash manifest avoids work
unnecessary and `go run ./tools/docsi18n -check` detects obsolete translations,
missing links, broken local links and public mirror differences.

Full verifiable examples use bookmarks `joss-run`, `joss-check` or
`joss-error` immediately before your fence. `documentation_test.go` analyzes
all three and run `joss-run`, comparing output lines. Use `joss-check` to
fragments that depend on server, DB or plugins and explains that context.

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

At `vscode-joss`: `npm ci` and `npm run compile`. Build a temporary binary
`./cmd/joss` and analyze the actual integration project. Template changes,
Migrations or CRUD must pass the arrays named in AGENTS.md.

A documentary review should look for local links, Joss fences, legacy terms,
indexes and differences between registry and dispatcher. Avoid claiming portability,
atomicity, security or complete compatibility without proof to prove it.
