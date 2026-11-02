# Modules, Files, and Plugins

[Index](README.md)

## Current State: Automatic Discovery Without `import`

In Joss, a project is organized as a set of `.joss` files that share a global space of public declarations without needing to write manual `import` statements:

- The canonical loader `semanticanalyzer.LoadProject("main.joss", "app")` builds the project unit by automatically discovering the entry file (`main.joss`), the route files (`routes.joss`, `api.joss`), and all `.joss` files inside `app/` (including `controllers`, `models`, `middleware`, `services`, `libs`, `database`, etc.).
- That validated set produces a single `PreparedProgram` that is consumed directly by `joss run` (`Runtime.ExecutePrepared`), `joss server`, and `joss build`.
- Cross-file visibility:
  - Classes, interfaces, enums, and functions declared as `public` are visible across all files in the project.
  - Those declared as `private` are restricted exclusively to the `.joss` file where they were defined (`JOSS-ACCESS-001`).

The historical forms `import`, `use`, `@import`, and `Namespace` were removed from the token set, AST, executor, and plugin compiler. The parser rejects them with a migration message (`JOSS-PARSE-001`). This absence is a permanent language decision: there will be no exports, source namespaces, or manual import graph.

## Plugins

Current external modular extensibility uses `.jp` packages:

- They are declared in `joss.yaml` or placed in `plugins/`.
- The runtime verifies them cryptographically and loads them automatically.
- Each package publishes `META-INF/joss-symbols.json`.
- The analyzer consumes that index to resolve exported classes, methods, and functions without executing the plugin.

An external symbol missing from the index is neither invented nor added to a manual analyzer list: the package or its symbol generation must be fixed.

## Files Included by Infrastructure

Routes, controllers, models, services, libraries, and middleware under `app/` are discovered automatically according to the project layout. This physical inclusion does not create a per-file namespace; top-level `public` functions and classes share the project declaration space, and duplicates are diagnosed (`JOSS-SYM-002`, `JOSS-DECL-004`).

## Standard Library (`stdlib/`)

Joss includes a standard library written in pure Joss located in the `stdlib/` directory. All files and submodules within `stdlib/` are automatically discovered and analyzed in every project without manual imports or path configurations.

### Available Modules and Capabilities

1. **`MathUtils` (`stdlib/math/math.joss`):**
   - `MathUtils::clamp(int $val, int $min, int $max): int`: Restricts a value within an inclusive minimum and maximum range.
   - `MathUtils::sign(int $n): int`: Returns `1` if positive, `-1` if negative, and `0` if zero.
   - `MathUtils::gcd(int $a, int $b): int`: Computes the greatest common divisor of two integers using Euclid's algorithm.

2. **`StringUtils` (`stdlib/strings/strings.joss`):**
   - `StringUtils::isEmpty(string $s): bool`: Efficiently checks if a string has a length of 0.
   - `StringUtils::isNotEmpty(string $s): bool`: Checks if a string contains at least one character.
   - `StringUtils::padZero(int $num, int $width): string`: Left-pads a number with zeros until reaching the specified width.

### Usage Example

<!-- joss-check: uso directo de la biblioteca estándar -->
```joss
int $minimo = MathUtils::clamp(150, 0, 100);
echo $minimo; // 100

int $signo = MathUtils::sign(-45);
echo $signo; // -1

string $codigo = StringUtils::padZero(42, 5);
echo $codigo; // "00042"
```

## Foreign Function Interface (C FFI)

To interact with native dynamic libraries (`.dll`, `.so`, `.dylib`), Joss provides the `extern` block syntax:

<!-- joss-check: declaración FFI externa -->
```joss
extern "C" library "user32.dll" {
    public func MessageBoxA(int $hWnd, string $lpText, string $lpCaption, int $uType): int;
}
```

Functions declared within an `extern` block are registered in the global symbol table and share the direct native machine code convention of the host CPU.

## Decision Regarding the Thesis

Chapter 11 of the thesis describes source modules with interfaces, exports, and a DAG. The implementation adopts a different decision: "modular" in ALIM means decoupled built-in capabilities and isolated `.jp` plugins, not application-written imports. This discrepancy is deliberate, and the source-module design from the thesis is not part of the Joss roadmap.

Top-level functions and classes form a single project declaration space. Top-level source variables are not dynamically inherited inside named functions; they must be passed as parameters. A closure does preserve the lexical environment it captures.