# Joss Native Memory Model

## 1. Design Goals
- No mandatory general-purpose stop-the-world GC.
- Memory safety over binary size.
- Deterministic deallocation and bounded memory growth.

## 2. Value Representation
### 2.1 Statically Typed Values
- `int` (`i64`), `float` (`f64`), `bool` (`i1`/`i8`), `byte` (`u8`): unboxed in registers/stack.
- `struct` / `record`: value layout with deterministic field offsets and alignment.

### 2.2 Dynamic Values (`mixed`)
Replace the unsafe `val < 0x10000` heuristic with an explicit **Tagged Value** (`JossValue`, 16 bytes):
```c
typedef enum {
    JOSS_TAG_NULL   = 0,
    JOSS_TAG_INT    = 1,
    JOSS_TAG_FLOAT  = 2,
    JOSS_TAG_BOOL   = 3,
    JOSS_TAG_STRING = 4,
    JOSS_TAG_OBJECT = 5,
    JOSS_TAG_ARRAY  = 6,
    JOSS_TAG_MAP    = 7,
    JOSS_TAG_BYTES  = 8
} JossTag;

typedef struct {
    uint64_t tag;
    uint64_t payload; // int64, bitcast double, bool (0/1), or heap pointer
} JossValue;
```
For the legacy 64-bit register-only emitter, heap-allocated boxed `JossBoxedValue` with a magic header (`0x4A4F5353` + tag + payload) or tagged pointer/arena slot ensures integers `>= 65536` never masquerade as string pointers.

## 3. Allocation & Lifetime Strategy
1. **Stack Allocation**: Primitives, fixed-size structs, and non-escaping locals.
2. **Region / Arena Allocator**: Request-scoped or function-scoped arenas for temporary strings and collections, freed deterministically at scope/request exit.
3. **Explicit Ownership Types**: `ByteSlice` (`Span<u8>`), `ByteBuffer`, `StringBuilder` with bounds checking and explicit `.free()` / RAII scope cleanup.
4. **ARC/RC for Shared Heap Objects**: Reference-counted headers on long-lived objects/arrays/maps when escaping scopes.
