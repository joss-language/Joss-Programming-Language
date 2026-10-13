# JOSS NATIVE AOT: The Native ABI

This document defines the formal Application Binary Interface (ABI) used for cross-boundary communication between the raw AOT machine code and the Canonical Host Bridge.

## Calling Convention (Joss Native x86_64)

- **Registers:** The first 4 arguments are passed in `RCX`, `RDX`, `R8`, `R9`.
- **Stack:** Additional arguments are passed on the stack starting at `[RSP + 32]`.
- **Return Value:** Integer/Pointer return values are passed in `RAX`.
- **Shadow Space:** 32 bytes of shadow space are reserved above the return address for all calls. 
- **Stack Alignment:** 16-byte alignment is maintained immediately before any `CALL` instruction.

## Memory Model

- Native execution uses a structured heap for `JossObject`, `JossArray`, and `JossMap`.
- Strings are immutable byte buffers with explicit lengths.
- Closures (`JossClosure`) maintain a function pointer and an environment pointer representing captured state.

## Host Bridge Cross-Call (`_joss_host_bridge`)

When raw machine code needs to invoke a complex canonical Host Service (like `GranDB` or `HTTP`), it does not execute a Go AST interpreter. Instead, it delegates to the bridge:

```asm
; Example Call to GranDB::where(query, column, value)
lea rcx, [GranDB::where_signature_id]
mov rdx, [arg_array_pointer]
call _joss_host_bridge
; Return value is populated in rax
```

The bridge statically resolves the target ID to the canonical Go implementation function in `pkg/core`, unwraps the ABI values, calls the function at native speeds, wraps the return value according to the Native ABI, and returns execution back to the AOT caller.
