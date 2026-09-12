# Joss extension for VS Code

[Index](README.md)

The Joss distribution includes the official VSIX in the `jossecurity-vscode.zip` artifact. Install it from VS Code with **Extensions: Install from VSIX...**.

The extension recognizes `.joss` and `.joss.html`. Indexes classes, properties, functions and methods; offers autocomplete, signature help, hover, symbols, definition and references. Validates `Controller@method` routes and applies three text-based security heuristics: use of `eval`, interpolated SQL with `DB::query`, and low cost of bcrypt. It does not replace the parser or a security audit.

For a JP v2 plugin, it reads `META-INF/joss-symbols.json` from the package and adds classes, methods, parameters, and returns to the IntelliSense and analyzer. Older JPs without that file can run, but they don't offer their API to the editor.

After installing a plugin with `joss pub install`, reload the VS Code window if its symbols don't appear immediately. To create a plugin that offers good signing help, declare parameters and types in the public API before running `joss build package .`.