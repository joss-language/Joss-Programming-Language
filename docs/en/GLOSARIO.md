# Glossary

[Index](README.md) · [Getting started](PRIMEROS_PASOS.md) · [Reference](SINTAXIS.md)

These words are also explained where they are introduced. You can come back here
without interrupting the learning journey.

| Term | Meaning in this documentation |
|---|---|
| Program | Instructions saved in files for a computer to perform a task. |
| Value | A specific piece of information: `12`, `"Ana"`, `true`. |
| Variable | Name that allows you to save and consult a value; for example `$edad`. |
| Binding | Association between a name and its value. A constant protects that association, not necessarily the contents of a collection. |
| Type | Type of data that an operation or variable admits. `int` represents integers. |
| Inference | Deduction of a type from the value, without writing it explicitly. |
| Conversion/casting | Obtaining a value in another type using specific rules; you may lose information. |
| `mixed` | Explicit decision to accept values ​​of different types. |
| `unknown` | Lack of information from the analyzer; It is not a source type for declaring variables. |
| `null` / `nil` | Absence of value; both forms produce the same null value. |
| Nullable | Type that also allows `null`, such as `string|null` or `string?`. |
| Expression | Code that produces a value: `2 + 3`. |
| Sentence | Instruction that performs a step: declare, return or repeat. |
| Block | Group of statements enclosed in braces in a context that expects a body. |
| Function | Reusable operation that receives data and can return a result. |
| Parameter | Name and type of data received in the declaration of a function. |
| Argument | Value delivered when the function is called. |
| Return | Result that a function returns with `return`. |
| Callable | Value that the runtime can invoke: function, method or closure, among others. It is not a source keyword. |
| Scope / scope | Region where a name can be resolved. |
| Closure | Anonymous function that preserves an environment of variables of its creation. |
| Recursion | Function that calls itself, directly or indirectly. |
| Class | Declaration that groups properties and methods. |
| Instance | Object created from a class using `new`. |
| Property | Data saved within an instance. |
| Method | Function associated with a class; called with `->` in an instance or `::` in static context. |
| Builder | Initialization when creating an instance; Joss supports `Init` in his class contract. |
| Inheritance | Reusing a base class using `extends`. |
| Encapsulation | Access control with `public`, `protected` or `private`. |
| Array | Ordered collection of elements, indexed from scratch; “list” is an explanation, not the type alias `list`. |
| Map | Collection of keys and values; It is also called a dictionary. |
| Reference | Access to the same storage. `ref` is also a temporary and restricted ability to modify a caller variable. |
| Mutability | Possibility of changing a value or the content of a structure. |
| Shallow copy | Copy of the container that can continue sharing interior elements. |
| Error / diagnosis | Problem detected; a diagnosis includes location, code and explanation. |
| Exception | Failure that interrupts the flow and can be recovered with `try/catch`. |
| Call stack / stack | Sequence of functions that are waiting for another call to finish. Not to be confused with the wrapper class `Stack`. |
| Heap | Memory for objects whose life is not limited to one call; It is managed by Go, not manually by the Joss program. |
| Synchrony | An operation ends before continuing with the next. |
| Concurrency | Various tasks progress during overlapping periods; does not guarantee simultaneous execution. |
| Asynchrony | An operation allows you to obtain its result later. |
| Future | Runtime object representing a pending result of `async`; It is not a keyword or canonical source type. |
| `await` | Blocking wait for the result of a Future in the current execution. |
| Channel | Channel to send data between tasks and coordinate them. |
| Runtime | Engine that runs the program and offers integrated services. |
| Lexer | Component that converts characters into tokens. |
| Token | Recognized unit: a name, a number, an operator, etc. |
| Parser | Component that organizes tokens according to syntax. |
| AST | Tree that represents the structure of the program. |
| Analyzer / analyzer | Component that checks names, types, and flow before executing. |
| Interpretation | Execution by reading a representation of the program, such as its AST. |
| Compilation | Transformation to another representation. In Joss it does not necessarily imply machine code. |
| Bytecode | Intermediate format. Joss main contains compressed AST; JPBC has instructions for plugins. |
| Module | Integrated capacity or physical organization; Joss does not have source modules with imports. |
| Package | Distributable unit with metadata and files. |
| Plugins | Extension loaded by runtime from a package. |
| CLI | Program that is controlled by typing commands in a terminal. |
| Terminal | Window to execute commands and view their output. |
| HTTP Route | Association between a URL, an HTTP method and responding code. |
| Controller | Class that prepares the response to a web request. |
| Middleware | Verification or transformation around a request. |
| View | Template that converts data to HTML. |
| Migration | Versioned change of the structure of a database. |
| Query builder | Object that constructs a query before executing it. |
| API | A set of operations that another program or component can use. |
