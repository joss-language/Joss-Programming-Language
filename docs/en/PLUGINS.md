# Plugins and packages

[Index](README.md) · Before: [structure](ESTRUCTURA_PROYECTO.md) · After: [contribute](CONTRIBUIR.md)

A **package** brings together files and metadata to distribute a capability.
A **plugin** incorporates functions, classes or commands to an application. Josh
loads your `.jp` packages automatically; an import is not written in the program.

## Create a Joss plugin

With the CLI installed, in a working folder:```sh
joss new plugin calculadora
cd calculadora
joss plugin compile .
```
The template contains `joss.yaml`, `src/plugin.joss` and a workflow of
publication. Read the generated manifest before changing names or exports.
The `joss new package calculadora` variant produces a smaller package;
is built with `joss build package .`. The tests
`TestNewPackageAndPluginTemplatesCompileEndToEnd` build both variants,
They verify the signature and decode the content.```sh
joss plugin inspect calculadora.jp
joss plugin verify calculadora.jp
```
`inspect` allows you to discover names, exports, permissions and symbols.
`verify` checks integrity and signature Ed25519. The public key included in
a file demonstrates consistency with its signature; by itself **does not establish
that the editor is someone you trust**.

## Install and use```sh
joss pub add nombre_del_paquete ^1.0.0
joss pub install
```
Replaces the name and version with an existing package in the registry
configured. `pub` maintains dependencies; does not add source syntax. Consultation
[CLI](CLI.md) and the package documentation for your particular API.

Packages declared in `joss.yaml` or present in `plugins/` are discovered
when preparing runtime. Your `SymbolIndex` publishes parameters and returns to
the analyzer. An old package with no published return provides `unknown`:
means lack of information, not a guarantee of compatibility.

Example of integration **dependent on a plugin that exports these symbols**:```joss
$resultado = calculadora::sumar(2, 3)
```
Don't copy that name without checking it with `inspect`. An exported function
may also be available by direct name; the exported classes
are instantiated with `new`. There are no source namespaces or modules with imports.

## What does a JP actually contain?

The signed container stores metadata, files, symbol index, and a
bytecode input. The runtime detects two different formats:

| Content | Executor | Scope |
|---|---|---|
| `JOSSBC2Z` | `pkg/core` AST Adapter | Serialized and compressed Joss tree, interpreted. |
| `JPBC` | `pkg/pluginruntime.JPBCVM` | Plugin-specific instructions machine. |

None convert the main program to LLVM/Cranelift machine code.
The experimental VM in `pkg/vm` is a third component and is not the executor
default of `joss run` or `joss build native`.

## Compilation from other languages: state and limits

`joss plugin compile archivo --lang=... --name=... --exports=...` select
a `pkg/plugincompiler` backend. Have a name accepted by the CLI
It doesn't mean that all that language is implemented.

| Entry | Current implementation |
|---|---|
| Joss / generated project | Joss AST Packaging. |
| Python | Translator of a subset of expressions and functions to IR/JPBC; it does not run CPython nor does it incorporate its entire ecosystem. |
| PHP | Partial translator to IR/JPBC; It is not equivalent to a PHP runtime. |
| Java/Kotlin | Reading `.class` or `.jar` and partial translation; it does not offer the entire JVM. |
| Rust, C, C++, Dart, Flutter, Wasm | The backend checks the `\\0asm` header and generates a function by export that returns a demo text. **Does not interpret or translate Wasm instructions.** |

Therefore do not use the Wasm path to encrypt, transform files or run
a real library. A signed packet produced by that route can be
structurally valid and still not implement the requested operation.

Plugin optimization removes functions not reachable from exports
according to the built IR. Does not demonstrate equivalence with any program
of the source language. The configured size limit generates a warning,
not a maximum size guarantee.

## Permissions and isolation

`PermissionGuard` checks for calls to the host that are mapped:

| Host operation | Permission |
|---|---|
| `http_get`, `http_post`, `fetch` | `network.http` |
| `file_read`, `file_write` | `filesystem.read`, `filesystem.write` |
| `env_read`, `env_write` | `env.read`, `env.write` |
| `db_query`, `db_exec` | `database.query`, `database.exec` |

Check the `pkg/pluginruntime/jpbc_vm.go` table when extending the host.
Declared permissions are granted when building the guard; there is no dialogue
user approval. Wildcards expand permissions by prefix and a
Exact revocation does not void an awarded wildcard.

This is **not a WASI sandbox nor process or operating system isolation**.
The verification is limited to operations integrated at that border. The
ABI C drivers, external processes and the AST executor have different mechanisms.The JPBC statement budget is applied per function invocation;
It should not be advertised as a global resource limit for an entire application.

## Bridges and life cycle

`Plugin::call`, `stream`, `path` and `platform` connect plugin formats;
`System::load_driver` and `driver_call` load specific ABI C v1 libraries
of platform. Their contracts are in the [catalog](CATALOGO_NATIVO.md) and
the [native reference](MODULOS_NATIVOS.md).

A runtime fork shares the plugin registry and other resources.
When freeing an instance from the pool, `Runtime.Free()` also cleans up
`PluginRegistry`; keeping the record and deleting only classes/symbols breaks
the next request. Plugins should not assume that variables of a
previous request are still available.

Sources: [compiler](../../pkg/plugincompiler/plugincompiler.go),
[Wasm backend](../../pkg/plugincompiler/backends/nativewasm/nativewasm_backend.go),
[runtime](../../pkg/pluginruntime/), [container](../../pkg/pluginpkg/).