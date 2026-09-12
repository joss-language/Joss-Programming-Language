# Recursive functions

[Index](README.md)

Joss supports direct and mutual recursion. For useful static checking, declare the return type:```joss
public func factorial(int $n): int {
    ($n <= 1) ? {
        return 1
    } : {
        return $n * factorial($n - 1)
    }
}
```
The function declarations and their signatures are recorded before the bodies are analyzed. That's why a function can refer to itself or to another function declared later in the project.

Each call creates a separate frame for parameters, locales, inferred types, and constants. A named function cannot accidentally read the locals of its caller or top-level source variables; data must travel by parameters. Native and plugin bindings are visible. Instances, maps, and arrays passed as values ​​maintain the current reference semantics; Isolating local binding does not turn those objects into deep copies. Closures preserve a separate captured environment.

The runtime uses `Runtime.MaxCallDepth` and applies 1024 frames by default. Exceeding it produces `RecursionLimit` instead of letting the Go stack grow uncontrollably. This limit protects execution, but is not a substitute for a correct base case.

The analyzer validates the types of arguments, each explicit `return`, and that every provable path of an annotated function ends in `return` or `throw`. Recognizes blocks, both arms of ternaries, `match` with `default` and both arms of `try/catch`; does not assume that an arbitrary loop terminates.