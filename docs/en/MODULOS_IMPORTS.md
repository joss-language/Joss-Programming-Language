# Modules, files and plugins

[Index](README.md)

## Current status

The project is organized as a set of `.joss` files, but each command
Choose a specific surface. `joss analyze` loads the indicated entry and everything
`app/`; it does not automatically add `routes.joss` or `api.joss`. The server loads
their routes through their own infrastructure. The `joss run` runtime preloads only the
standard domains under `app/` and today omits `app/libs`, although the parser does
go. This asymmetry is recorded as debt.

The historical forms `import`, `use`, `@import` and `Namespace` were removed from the token set, AST, executor and plugin compiler. The parser rejects them with a migration message. This absence is a permanent decision of the language: there will be no exports, source namespaces or imports graph.

##Plugins

The current modular extensibility uses `.jp` packages:

- They are declared in `joss.yaml` or placed in `plugins/`.
- The runtime verifies and loads them automatically.
- Each package publishes `META-INF/joss-symbols.json`.
- The analyzer consumes that index to resolve exported classes, methods and functions without running the plugin.

An external symbol that does not appear in the index is not invented or added to a manual list of the analyzer: the package or its symbol generation must be corrected.

## Files included by infrastructure

Routes, controllers, models and middleware are discovered based on the project layout and CLI/runtime. This physical inclusion does not create a namespace per file; Top-level functions and classes share the project declaration space and their duplicates are diagnosed.

## Decision regarding the thesis

Chapter 11 of the thesis describes source modules with interfaces, exports and a DAG. The implementation takes a different decision: “modular” in ALIM means decoupled integrated capabilities and isolated plugins, not imports written by the application. This discrepancy is deliberate and the design of source modules for the thesis is not part of Joss's roadmap.

The top-level functions and classes form a single project declaration space. Top-level source variables are not dynamically inherited within named functions; They must be passed as parameters. A closure does preserve the lexical environment it captures.