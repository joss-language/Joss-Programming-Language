# Getting started: from a file to a program

Before: [Index](README.md). After: [Values, variables and operations](FUNDAMENTOS.md).
Quick Reference: [Command Line (CLI)](CLI.md).

---

## What are you going to learn here?

If you've never written a single line of code in your life, this is the right place to start. You do not need prior programming knowledge or experience in other languages.

In this guide you will learn:
1. What is programming and what is an instruction for a computer.
2. What is the **Joss** language and what components make up its ecosystem.
3. How to install Joss on your operating system (Windows, Linux or macOS).
4. How to write your first program ("Hello World"), analyze it and run it.
5. What happens internally from when you save the text file until the screen shows the result.
6. How to solve the most common mistakes when taking the first steps.

---

## 1. What is programming and what is a program?

A computer is extraordinarily fast at doing calculations, but it can't figure out what you want to do. You need a clear, orderly and unambiguous sequence of orders. This sequence of instructions is called **program** or **algorithm**.

Imagine a cooking recipe:
1. Weigh 200 grams of flour.
2. Add two eggs.
3. Mix for five minutes.

A computer program works under the same logic: it performs tasks step by step. The exact text that you write to give those commands to the computer is called **source code**.

In **Joss**, source code is written in plain text and saved in files whose extension ends in `.joss` (for example, `hola.joss` or `app.joss`). You shouldn't use rich word processors like Microsoft Word or Google Docs, because they add invisible formatting that the computer doesn't understand; **code editors** such as Visual Studio Code are used.

---

## 2. What is Joss?

**Joss** is a modern programming language designed especially for backend applications, web development, command line utilities, and high-performance concurrent services.

Its design philosophy is based on four pillars:

1. **Zero imports in the source code (Zero Imports)**: In many languages you must write dozens of lines like `import X from Y` at the beginning of each file. Joss automatically discovers and organizes the public files, classes, and functions of your project and its plugins, allowing you to focus on business logic.
2. **Safety and rigorous static analysis**: Before executing a single statement, Joss' **semantic analyzer** reviews your variables, data types, return paths, and visibilities to detect errors before they reach production.
3. **Full Stack Included**: Joss natively includes high-performance HTTP server, routing system, HTML templating engine, ORM and database schema generator (GranDB / Schema), support for real-time WebSockets, cryptographic hashing and asynchronous tasks.4. **Clean concurrency and channels**: You can delegate heavy tasks to the background in a non-blocking way with `async` and communicate processes with secure channels (`channel`).

### The layers of the Joss ecosystem

It is important to distinguish three concepts that are sometimes confused:```text
┌─────────────────────────────────────────────────────────────┐
│ 1. Tu código fuente (.joss)                                 │
│    El texto legible que tú escribes con tus instrucciones.  │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. El analizador semántico (Semantic Analyzer)              │
│    Revisa tipos, nombres, visibilidad y coherencia.          │
└──────────────────────────────┬──────────────────────────────┘
                               │ Si no hay errores
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. El motor de ejecución (Runtime de Joss en Go)             │
│    Ejecuta las instrucciones reales en tu máquina física.    │
└──────────────────────────────┴──────────────────────────────┘
```
- **The Joss language**: The writing rules, reserved words and syntax that you will learn.
- **The CLI tool (`joss`)**: The command line executable that you use from your terminal to analyze, format, test and launch your projects.
- **The Runtime**: The engine that takes your proven program and performs the actual operations on your computer's processor and memory.

---

## 3. Step by step installation

A **terminal** (or console) is a text window where you communicate with the operating system by typing commands instead of clicking the mouse.
- On **Windows**: You can open **PowerShell** or the **Windows Terminal** application (look for it in the Start menu).
- On **Linux or macOS**: Open the application called **Terminal**.

### Option A: Official Automatic Installer (Recommended)

On **Windows** (open PowerShell as a standard user or Administrator):```powershell
iwr -useb https://raw.githubusercontent.com/josprox/Joss-language/main/install/remote-install.ps1 | iex
```
On **Linux or macOS** (open your Terminal):```bash
curl -fsSL https://raw.githubusercontent.com/josprox/Joss-language/main/install/remote-install.sh | bash
```
This command will download the compiled Joss binary, place it in a standard system folder, and register it in your `PATH`.

> [!NOTE]
> **What is the `PATH`?**
> The `PATH` is an internal list of the operating system with the paths where the programs reside that you can invoke by writing only their name. If you add Joss to the `PATH`, you can type `joss` in any folder without having to type the full path to the executable. If you have just installed it and your terminal does not recognize it, **close the terminal and open it again**.

### Option B: Manual download from GitHub Releases

1. Go to the releases section: [Joss GitHub Releases](https://github.com/josprox/Joss-language/releases).
2. Download the compressed `.zip` or `.tar.gz` package corresponding to your architecture (`windows_amd64`, `linux_amd64`, `darwin_arm64`, etc.).
3. Unzip the file and place the `joss` executable (or `joss.exe`) in an accessible folder on your disk.
4. Add this folder to your system environment variables (`PATH`).

### Option C: Compile from source with Go

If you are a developer and have [Go](https://go.dev) installed (version 1.22 or higher), you can clone this repository and build it in seconds:```bash
git clone https://github.com/josprox/Joss-language.git
cd Joss-language
go build -o joss ./cmd/joss
```
On Windows `joss.exe` will be created; on Linux/macOS the `joss` executable binary will be created.

### Verify that the installation works

Open a terminal and type:```bash
joss version
```
You should see the installed version of Joss on the screen (for example, `Joss version 3.6.7.2`). If you see that message, your environment is 100% ready to program!

---

## 4. Write and run your first program

We are going to create the classic program that every programmer writes when starting out: display a greeting on the screen.

### Step 1: Create a Portfolio

Create a clean folder for your experiments. In your terminal:```bash
mkdir mi-primer-joss
cd mi-primer-joss
```
### Step 2: Create the `hola.joss` file

Open your favorite text editor (for example, Visual Studio Code by typing `code .` in that folder) and create a new file called:

`hola.joss`

> [!CAUTION]
> Make sure the file ends exactly in `.joss`. On Windows, if known extensions are hidden, Notepad might save it as `hola.joss.txt`, which will prevent Joss from recognizing it as source code.

Write exactly the following line into the file:<!-- joss-run: ["Hola, Joss!"] -->
```joss
print("Hola, Joss!")
```
Save the file.

### Step 3: Understand each part of that line

Let's see what each character means:

1. `print`: It is the name of a **native function** built into Joss. Its exclusive purpose is to receive information and write it to the screen (standard output), adding a line break at the end so that the next instruction begins on the line below.
2. `(` and `)`: The parentheses tell Joss that you are **calling** (executing) the `print` function. Inside the parentheses you place the input data that the function needs to work. These input data are called **arguments**.
3. `"Hola, Joss!"`: It is a value of type **text** (technically called `string` or character string). The double quotes `"` are used to mark exactly where the text begins and ends. The quotes are not displayed on the screen; they only delimit the content.

### Step 4: Analyze and run the program

Go back to your terminal, make sure you are located in the `mi-primer-joss` folder and type:```bash
joss run hola.joss
```
You will immediately see the output in the terminal:```text
Hola, Joss!
```
Congratulations! You have just written, processed and executed your first program in Joss.

### Interactive mode for quick tests: `joss repl`

If you want to experiment with math operations, variables, or small functions without creating a file on disk, you can open the Joss **interactive console (REPL)** by typing in your terminal:```bash
joss repl
```
You will see a welcome cursor:```text
Joss Interactive REPL (v3.7.0)
Escribe expresiones, sentencias o 'exit' / 'quit' para salir.
>>> $x = 10
>>> $x * 5
50
>>> print("Hola desde el REPL!")
Hola desde el REPL!
>>> exit
```
The variables you create will be remembered between lines while the session is open. To exit, simply type `exit` or `quit`.

### VS Code extension and automatic formatting

If you use Visual Studio Code with the official Joss extension, you can automatically sort and align your code according to canonical style rules at any time with the standard shortcut:
- On **Windows and Linux**: `Shift + Alt + F`
- On **macOS**: `Shift + Option + F` (or right click → *Format document*)

---

## 5. The `analyze` command: Your safety net

Before running a large program or deploying it to a server, Joss allows you to statically verify it using the `analyze` command:```bash
joss analyze hola.joss
```
If the code is correct and safe, the parser will terminate successfully:```text
[Analyzer] Análisis completado sin errores.
```
What exactly does the parser do?
- **Check the syntax**: Check that you have not forgotten quotes, parentheses or braces.
- **Validate variable and function names**: Check that you are not calling things that do not exist.
- **Check data types**: If you said a variable was an integer, check that you don't try to assign it a list of users.
- **Check return paths**: Check that your functions always return a consistent value in any possible path.

The `joss run` command internally performs this analysis before running. If there is a blocking error (`Severity: error`), Joss will refuse to run it to protect your system from erratic behavior.

---

## 6. Modify the program: Use memory and variables

A program that only displays fixed text is not very interactive. Actual programs store information in the computer's memory to be retrieved or transformed later. To do this **variables** are used.

Modify your `hola.joss` file to contain:<!-- joss-run: ["Hola, Ada", "Bienvenida a Joss"] -->
```joss
$nombre = "Ada"
print("Hola, " . $nombre)
print("Bienvenida a Joss")
```
Save and run:```bash
joss run hola.joss
```
Exit:```text
Hola, Ada
Bienvenida a Joss
```
### What has changed here?

1. `$nombre = "Ada"`:
   - The `$` symbol at the beginning indicates that we are declaring or using a variable. In Joss, **all variables start with `$`**.
   - The `=` sign is called **assignment operator**. It takes the value on the right (`"Ada"`) and stores it in the memory "box" identified by the name `$nombre`.
   - Joss automatically infers that `$nombre` stores text (`string`).
2. `"Hola, " . $nombre`:
   - The dot `.` is the **concatenation operator**. It is used to join two pieces of text into one. Here it joins `"Hola, "` with the content inside `$nombre` (`"Ada"`), producing the text `"Hola, Ada"`.
3. Second call to `print`:
   - Displays the next line independently.

Try changing `"Ada"` to your own name on the first line, save the file, and run `joss run hola.joss` again. You will see how the greeting changes automatically.

---

---

## 7. Interactive Mode: Ask the user for data with `cin >>`

So far, the program only shows data that you have already written in the code. To create fun and useful programs, you need the computer to **listen** to you, wait for your response, and react to it.

The fundamental cycle of every program is:```text
┌─────────────────────────┐       ┌─────────────────────────┐       ┌─────────────────────────┐
│     1. ENTRADA          │  ──>  │     2. PROCESO          │  ──>  │     3. SALIDA           │
│ El usuario escribe con  │       │ El programa calcula,    │       │ Se muestra el resultado │
│ cin >> $variable        │       │ une o toma decisiones   │       │ en pantalla con print() │
└─────────────────────────┘       └─────────────────────────┘       └─────────────────────────┘
```
In Joss, reading data that the user types on their keyboard is as simple as using `cin >>`:<!-- joss-check: lectura interactiva de datos por teclado -->
```joss
string $nombre = ""
int $edad = 0

print("¿Cómo te llamas?")
cin >> $nombre

print("¿Cuántos años tienes?")
cin >> $edad

print("¡Mucho gusto, " . $nombre . "! El próximo año tendrás " . ($edad + 1) . " años.")
```
### How does `cin >>` work?
1. `print(...)` displays a question in the terminal so the person knows what to type.
2. `cin >> $nombre` pauses the program and waits for the user to type their name and press the **Enter** key.
3. Everything the user typed is automatically stored in the variable `$nombre`.
4. If the expected data is a number (such as age), Joss automatically converts it so you can perform direct math operations like `$edad + 1`.

---

## 8. The "Essential Beginner's Cheat-Sheet"

Keep this board nearby. These 8 patterns solve practically any program in your first weeks of learning:

| What do you want to achieve? | How do you write in Joss? | Minimal example |
|---|---|---|
| **Show a message** | `print(...)` | `print("¡Hola mundo!")` |
| **Ask user for data** | `cin >> $variable` | `cin >> $ciudad` |
| **Save information** | `$variable = valor` | `$precio = 25` |
| **Increase or add quickly** | `$variable += valor` | `$puntos += 10` |
| **Join pieces of text** | `texto1 . texto2` | `"Hola " . $nombre` |
| **Make a decision** | `(condicion) ? { si } : { no }` | `($edad >= 18) ? { print("Mayor") } : { print("Menor") }` |
| **Choose from several options** | `match ($opcion) { caso => ... }` | `match ($color) { "rojo" => "Alto", default => "Sigue" }` |
| **Save and scroll through a list** | `foreach ($lista as $item)` | `foreach (["manzana", "pera"] as $f) { print($f) }` |

---

## 9. The Three Golden Rules for Beginners

To avoid 99% of doubts and errors when taking your first steps:

1. **The sacred dollar (`$`):** Every variable in Joss necessarily begins with `$`. If you see an undefined variable error, check to see if you forgot the `$`.
2. **Quotes go in pairs (`"..."`):** All free text must begin and end with double quotes. The numbers (`42`) do not have quotes; the words (`"Hola"`) do.
3. **Everything that opens, closes:** The parentheses `()`, the curly braces `{}` and the square brackets `[]` always go in pairs. If you open a brace `{`, make sure you have a corresponding closure `}`.

---

## 10. If something doesn't work: Quick diagnosis

When you're learning to program, making mistakes is not only normal, it's the best way to understand how the computer thinks!

Here is a table with the most frequent problems and how to solve them:

| Symptom or message | Probable cause | How to fix it |
|---|---|---|
| `joss: command not found` or `el término 'joss' no se reconoce` | The terminal doesn't know where the `joss` executable is. | Close and reopen your terminal. If it persists, check that the Joss folder is included in your `PATH` environment variable. |
| `Error: No se pudo leer el archivo 'hola.joss'` (`JOSS-IO-001`) | The terminal does not find the file in the current folder. | Type `ls` (in Linux/macOS or PowerShell) or `dir` (in Windows CMD) to view the files in the folder. Check if the name is spelled correctly or if you are in the correct directory with `cd`. || `JOSS-PARSE-001: Error de sintaxis` | There is a missing closing quote `"`, a parenthesis `)`, or an unexpected character. | Read the line and column indicated by the error message. Check that all quotes and parentheses are properly matched. |
| `JOSS-SYM-001: Variable no definida` | You tried to read a variable that was never created or its name has a typo. | Remember to always include the `$`. Check case (`$nombre` and `$Nombre` are two completely different variables for Joss). |
| Dozens of errors about files that you do not recognize | You are running `joss analyze` or `joss run` inside a folder that contains other projects or previous `.joss` files. | Always work within an empty folder dedicated to your project. Joss automatically discovers all `.joss` files in the current folder and its subdirectories. |

---

## 11. Practical exercises for beginners

To consolidate what you have just learned before continuing:

1. **Your business card**: Write a program `tarjeta.joss` that declares three variables: `$miNombre`, `$miPais` and `$miProfesion`. Then, using `print` and concatenation with `.`, display a message on the screen that puts together a paragraph introducing you.
2. **Your first interactive dialog**: Create `conversacion.joss`, ask the user for their name and favorite food with `cin >>`, and respond with a cheerful culinary recommendation.
3. **Experimenting with errors (The Code Breaking Game)**: Purposely remove the trailing quote from the text in `print("Hola)` and run `joss analyze`. Notice how Joss tells you exactly the line number where he detected the problem. Put the quote back so it's clean.

---

## Next step

Now that you know what Joss is, how to create a file, how to print data and how to interact with the user, it is time to learn in depth the types of data that a computer handles, how to operate on numbers and how to decide what type of variable to use.

Continue with: [Fundamental values, variables and operations](FUNDAMENTOS.md).