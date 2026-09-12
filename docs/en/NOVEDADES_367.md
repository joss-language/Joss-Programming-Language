# What's new in Joss v3.6.7.2

[Index](README.md)

This release consolidates the engine around verifiable guarantees: analysis
before running, stable types after inferring them, autoloading without
imports source and tooling that validates the generated projects.

## Language and type system

- `$x = valor` and `var $x = valor` infer a fixed type. A reassignment must
  keep it.
- `let $x = valor` declares explicit dynamism using `mixed`.
- Implemented constants, nullable/union types and return types.
- Functions and methods use isolated lexical frames; direct recursion and
  Mutual works with a configurable limit of 1024 calls by default.
- `func` is the only function keyword. `function`, `import`, `@import`,
  `use` and source namespaces were removed.

## Static analysis

- `joss analyze` loads the same project and native surface as the runtime.
- Resolves top-level classes and functions in two passes, including references
  advances necessary for mutual recursion.
- Check symbols, scopes, known arity, operators, assignments,
  arguments, returns and reachable flow.
- Structured diagnostics include code, severity, file, range,
  explanation and suggestion; unknown information does not become a
  error without evidence.

## Runtime, bytecode and plugins

- Execution frames prevent accidental dynamic scope and runtime
  protects the recursion limit.
- The main format `JOSSBC2Z` contains serialized and compressed AST. The
  runner continues to play him; It is not presented as machine code.
- JP v2 packages contain manifest, symbol table, bytecode and signature
  Ed25519 verifiable.
- Python, Java, PHP and WASM backends depend on their actual hosts/protocols
  when applicable; no promise is made to remove external runtimes.
- Plugins declared in `joss.yaml` or installed in `plugins/` are loaded
  automatically, without import statements in the Joss code.

## CLI and generated projects

- `joss new web`, `console`, `package` and `plugin` produce projects that pass
  parser, parsing and their representative execution/compilation flows.
- `make:migration` normalizes names like `create_products_table`, applies the
  migration and only reports success after registering the batch.
- `make:crud` validates the schema, distinguishes existing relationships, limits
  writable fields, avoids deletions by GET and does not duplicate routes or navigation.

## Validation

The repository validates build, tests, analysis, language catalog and examples
representative through CI. For the exact status and its limits consult
[Implementation status](ESTADO_IMPLEMENTACION.md) and the
[Technical audit](AUDITORIA_TECNICA_2026.md).