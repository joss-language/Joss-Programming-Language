# Glossary

[Index](README.md) · [First steps](PRIMEROS_PASOS.md) · [Reference](SINTAXIS.md)

These words are also explained where they are introduced. You can come back here
without interrupting the learning journey.

| Term | Meaning in this documentation |
|---|---|
| Program | Instructions saved in files for a computer to perform a task. |
| Project | Organized folder created with `joss new` or `joss new console` that contains your code, configuration and structure ready to work. |
| `main.joss` | Main entry file of every Joss project. It is the first file Joss looks for when running or compiling your application. |
| `Init main()` | Startup block inside `public class Main` in `main.joss`. Everything you write inside its braces `{ ... }` runs when the program starts. |
| Value | A concrete piece of data: `12`, `"Ana"`, `true`. |
| Variable | Name prefixed with `$` that allows you to store, read or change a value in memory; for example `$edad`. |
| Declaration | Action of creating a variable for the first time by specifying `var` (`var $x = 1`), an explicit type (`int $x = 1`), `mixed` or `const`. |
| Reassignment | Action of changing the value of a variable that was previously declared (`$x = 2`). |
| Binding | Association between a name and its value. A constant (`const`) protects that association so it cannot be reassigned. |
| Type | Category of data that an operation or variable accepts. For example, `int` represents integer numbers and `decimal` represents exact monetary amounts. |
| Inference | Automatic deduction of a variable's type from its initial value (such as in `var $edad = 20`, where Joss deduces `int`). |
| Conversion / casting | Obtaining a value in another type through functions such as `intval(...)`, `decimal(...)` or `strval(...)`. |
| Interpolation | Embedding variables or calculations inside a double-quoted string using `${nombre}` or `${$precio * 2}`. |
| `mixed` | Explicit decision to allow a variable or parameter to accept values of different types over time. |
| `void` | Return type that explicitly indicates that a function performs actions but returns no value. |
| `unknown` | Internal state of the analyzer when it does not yet know a type; it is not a keyword for declaring variables. |
| `null` / `nil` | Absence of value; `null` is the recommended canonical form and `nil` is accepted as a compatibility synonym. |
| Nullable | Type that allows a concrete value or `null`, written as `string|null` or with the shorthand `string?`. |
| Expression | Code fragment that evaluates and produces a value: `2 + 3`, `"Hola " . $nombre` or `($edad >= 18) ? "Mayor" : "Menor"`. |
| Statement | Instruction that performs a step within the program: declaring a variable, displaying a message, repeating a loop or returning. |
| Block | Group of instructions delimited by braces `{` and `}`. |
| Function | Reusable code machine (`public func` or `private func`) that receives input data and can return a result. |
| Parameter | Typed variable declared inside the parentheses of a function (`int $a`) that defines what data it expects to receive. |
| Argument | Actual value passed inside the parentheses when you call a function (`sumar(5, 10)`). |
| Return | Result that a function hands back to its caller using the keyword `return`. |
| `guard` | Guard clause (`guard (condicion) : { return ... }`) that requires a condition to be met or immediately exits the function. |
| `defer` | Instruction that schedules a block to execute right before the current function or file finishes, useful for closing resources. |
| Callable | Any value that can be invoked with parentheses `()`: a function, a method or a closure. |
| Scope | Area of the program where a variable exists and can be used. Each function call has its own isolated scope. |
| Closure | Anonymous function (`func(...) { ... }`) that can be stored in a variable and remember variables from the environment where it was created. |
| Recursion | Technique in which a function calls itself to solve a problem in smaller steps. |
| Class | Blueprint or mold (`public class Nombre`) that groups data (properties) and behaviors (methods). |
| `record` | Concise way (`public record Nombre(...)`) to declare a class whose data remains protected as constants after creation. |
| Instance / Object | Concrete entity created in memory from a class using `new Nombre(...)`. |
| Property | Variable that belongs to a class or instance (accessed with `$this->propiedad` or `$objeto->propiedad`). |
| Method | Function that belongs to a class; invoked with `->` on an instance or with `::` if it is a static method of the class. |
| Constructor | Special method (`public func constructor(...)` or `Init(...)`) that prepares initial data when creating an object with `new`. |
| Inheritance | Ability of a class to reuse and extend another base class using `extends`. |
| Interface | Formal contract (`public interface`) that lists which public methods a class must implement using `implements`. |
| Encapsulation | Protection of class members using `public` (open), `protected` (class and subclasses) or `private` (only the same class). |
| Array | Ordered list of elements inside brackets `[10, 20]`, whose positions (indices) start counting from `0`. |
| Map | Dictionary of key → value pairs written as `{"clave": valor}` or `["clave" => valor]`. |
| Reference | Shared access to the same data. With `ref` (`incrementar(ref $x)`), it allows a function to temporarily modify a caller variable. |
| Mutability | Ability to modify the value of a variable or the internal elements of a collection. |
| Shallow copy | Duplicate of the outer container of a collection whose complex inner elements remain shared. |
| Error / Diagnostic | Structured notice (`JOSS-...`) with file, line, explanation and suggestion that points out a problem in the code. |
| Exception | Runtime failure thrown with `throw` that can be caught with `try { ... } catch ($e) { ... }`. |
| Call stack / stack | Chain of waiting functions while an inner call executes. Not to be confused with the data structure class `Stack`. |
| Heap | Automatically managed memory where objects, arrays and maps created during execution live. |
| Synchrony | Step-by-step execution in which each instruction waits for the previous one to finish. |
| Concurrency | Coordinated progress of multiple tasks at the same time. |
| Asynchrony | Delegating a task with `async { ... }` so it runs without blocking the main program. |
| Future | Object returned by an `async` block that represents the future result of that task. |
| `await` | Keyword (`await $tarea` or `await($tarea)`) that waits for and obtains the final value of a `Future`. |
| Channel | Typed channel (`channel<T>` / `make_chan()`) for safely sending (`<<`) and receiving (`<-`) messages between concurrent tasks. |
| Runtime | Joss execution engine that manages memory, concurrency and built-in native classes. |
| Lexer | First step of the Joss processor: converts the text of the `.joss` file into indivisible pieces called tokens. |
| Token | Recognized lexical unit: a keyword, a variable, a number or an operator. |
| Parser | Second step of the processor: organizes tokens into a syntax tree while checking writing rules. |
| AST | Abstract Syntax Tree that represents the logical structure of your code. |
| Analyzer / analyzer | Static safety net that verifies types, names, contracts and returns before running or compiling, generating a `PreparedProgram`. |
| Interpretation (`joss run`) | Immediate execution of the validated program directly from the terminal. |
| Native compilation (`joss build`) | Transformation of the same Joss program into a standalone native executable file (`.exe` on Windows or binary on Linux/macOS/Android). |
| Bytecode (`.jp`) | Cryptographically signed binary representation used to distribute Joss packages and plugins. |
| Module | Automatic organization of files within the project (`app/`); in Joss public declarations are discovered without writing `import`. |
| Package | Reusable library distributed and managed with `joss pub`. |
| Plugin | Compiled extension in `.jp` format that adds capabilities or commands to the project. |
| CLI | Command-line tool (`joss`) that you control by typing commands in the terminal. |
| Terminal / Console | Operating system window (such as PowerShell or Terminal) where you type text commands and see the results. |
| HTTP Route | Rule in `routes.joss` or `api.joss` that connects a web address (URL) to the controller responsible for answering it. |
| Controller | Class located in `app/controllers/` that receives the user's request, processes the logic and returns a view or JSON. |
| Middleware | Intermediate filter that checks or protects a web request before it reaches the controller (for example, verifying if the user is logged in). |
| View | Dynamic HTML template (`.joss.html`) in `app/views/` that displays data to the user while automatically escaping malicious code. |
| Migration | File in `app/database/migrations/` that creates or updates database tables in an orderly manner with `joss migrate`. |
| Query builder / ORM | Built-in system (`Model` and `GranDB`) for querying and saving data in databases without concatenating unsafe SQL. |
| API | Set of functions, methods or web routes that a program offers to communicate with another. |
