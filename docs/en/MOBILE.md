# Mobile Support: Android and iOS

Joss supports execution and development on mobile platforms through two approaches:
1. **Standalone CLI and binaries on Android (Termux / ADB):** Run and compile Joss scripts directly on Android devices.
2. **Embeddable `libjoss` SDK (`pkg/mobile`) for Android and iOS:** Integrate the Joss engine into native and hybrid applications written in **Kotlin** (Android) and **Swift** (iOS). This is suitable for educational apps such as *Aprende más*, interactive playgrounds, or client-side business-rule execution.

---

## 1. Approach A: Standalone CLI on Android

Android is based on the Linux kernel and can run static ELF binaries.

### Supported architectures

The Joss native cross-compiler supports the four standard Android architectures:
- `android/arm64` (most modern phones)
- `android/arm` (older 32-bit devices)
- `android/amd64` (x86_64 Android Studio emulators)
- `android/386` (older 32-bit emulators)

### Cross-compiling from the development machine

To compile the Joss CLI or a self-contained Joss project for Android:

```bash
# Compilar un script o proyecto para Android arm64
joss build native android arm64

# O compilar el CLI completo directamente con Go
CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build ./cmd/joss
```

Transfer the resulting binary to the device and install it in **Termux**:

```bash
# En el terminal de Termux en Android:
chmod +x joss
mv joss $PREFIX/bin/
joss version
```

> [!NOTE]
> iOS does not provide a standalone command console because of Apple's security and sandboxing policies. Joss therefore supports iOS through the embeddable SDK described in Approach B.

---

## 2. Approach B: Embeddable `libjoss` SDK (`pkg/mobile`)

The `github.com/jossecurity/joss/pkg/mobile` package exposes a high-level API for mobile applications built with Java, Kotlin, Swift, Flutter, or React Native.

### API methods

| Function | Description | Return value |
| :--- | :--- | :--- |
| `Run(source string, timeoutMs int)` | Runs Joss source code, captures `stdout` and `stderr`, and applies a time limit to protect against infinite loops.

| JSON `ExecutionResult` |
| `Analyze(source string)` | Performs static and syntactic analysis without running the code; reports line-and-column diagnostics for mobile editors.

| JSON `AnalysisResult` |
| `Version()` | Returns the current Joss engine version.

| `string` |

### JSON response structure (`ExecutionResult`)

```json
{
  "success": true,
  "stdout": "Hola desde Joss en Android!\n",
  "stderr": "",
  "error": "",
  "timed_out": false,
  "duration_ms": 15,
  "diagnostics": []
}
```

If the code contains a syntax or type error, `success` is `false` and `diagnostics` contains the details needed to highlight the error in the app interface:

```json
{
  "success": false,
  "error": "JOSS-TYPE-001: tipo incompatible",
  "duration_ms": 2,
  "diagnostics": [
    {
      "code": "JOSS-TYPE-001",
      "severity": "error",
      "message": "no se puede asignar string a int",
      "line": 3,
      "column": 5,
      "suggestion": "declara el tipo compatible o usa mixed"
    }
  ]
}
```

---

## 3. Android Integration (Kotlin / Jetpack Compose)

In applications such as **Aprende más**, call the SDK from a ViewModel or execution service.

### Kotlin example

```kotlin
// Invocación del motor Joss desde Kotlin
fun ejecutarCodigoJoss(codigoFuente: String): JossResult {
    // timeout de 3000 ms para evitar que bucles infinitos cuelguen la app
    val resultadoJson: String = Mobile.run(codigoFuente, 3000)
    return Gson().fromJson(resultadoJson, JossResult::class.java)
}

data class JossResult(
    val success: Boolean,
    val stdout: String,
    val stderr: String,
    val error: String?,
    val timed_out: Boolean?,
    val duration_ms: Long,
    val diagnostics: List<JossDiagnostic>?
)

data class JossDiagnostic(
    val code: String,
    val severity: String,
    val message: String,
    val line: Int,
    val column: Int,
    val suggestion: String?
)
```

---

## 4. iOS Integration (Swift / SwiftUI)

On iOS, include the SDK in the Xcode project as an `.xcframework`.

### Swift example

```swift
import SwiftUI
import JossMobile

class JossRunnerViewModel: ObservableObject {
    @Published var salida: String = ""
    @Published var estaEjecutando: Bool = false

    func ejecutar(codigo: String) {
        estaEjecutando = true
        DispatchQueue.global(qos: .userInitiated).async {
            // timeout de 3 segundos
            let jsonString = MobileRun(codigo, 3000)
            
            if let data = jsonString.data(using: .utf8),
               let result = try? JSONDecoder().decode(JossResult.self, from: data) {
                DispatchQueue.main.async {
                    self.salida = result.success ? result.stdout : (result.error ?? "Error desconocido")
                    self.estaEjecutando = false
                }
            }
        }
    }
}
```

---

## 5. Building the SDK for Production

### Generate an Android AAR with `gomobile`

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
gomobile bind -target=android -o libjoss.aar ./pkg/mobile
```

Place the generated `libjoss.aar` file in the Android Studio `app/libs/` directory.

### Generate an iOS XCFramework with `gomobile`

```bash
gomobile bind -target=ios -o JossMobile.xcframework ./pkg/mobile
```

Drag the `JossMobile.xcframework` directory into the Xcode project's *Frameworks, Libraries, and Embedded Content* section on macOS.
