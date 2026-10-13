# JOSS NATIVE AOT: Native Artifact Linking

This document outlines how `joss build` links the generated machine code to form a final executable (PE32+ or ELF64) without relying on external host toolchains.

## Integrated Linker

`joss build` includes a self-contained in-memory linker (`pe_emitter.go` and `elf_emitter.go`).

1. **IR to Machine Code:** The Native Backend lowers Joss Native IR into x86_64 raw machine bytes, maintaining internal relocation tables for branches and calls.
2. **Host Bridge Integration:** For calls requiring canonical host services (e.g. GranDB), the linker sets up cross-references to the embedded Host Bridge stub.
3. **Executable Emitter:**
   - On Windows, the `pe_emitter` wraps the `.text`, `.rdata`, and `.data` sections into a valid PE32+ executable importing only `KERNEL32.dll`.
   - On Linux, the `elf_emitter` generates a static ELF64 executable using direct syscalls.

## Clean Environment Guarantee

To ensure true autonomy:
- `os/exec` is never used to invoke external linkers like `ld`, `lld`, `link.exe`, `gcc`, or `clang` during `joss build`.
- The final artifact does not dynamically link against `libc` or require a Go runtime unless explicitly bundled via the pre-compiled Host Bridge object.
