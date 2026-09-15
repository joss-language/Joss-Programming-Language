# Soporte Móvil: Android e iOS

Joss soporta ejecución y desarrollo en plataformas móviles mediante dos enfoques:
1. **CLI y binarios independientes en Android (Termux / ADB):** Permite ejecutar y compilar scripts Joss directamente en dispositivos Android.
2. **SDK Embebible `libjoss` (`pkg/mobile`) para Android e iOS:** Permite integrar el motor de Joss dentro de aplicaciones nativas e híbridas en **Kotlin** (Android) y **Swift** (iOS), ideal para apps educativas como *Aprende más*, entornos de prueba interactivos o ejecución de reglas de negocio en el cliente.

---

## 1. Enfoque A: CLI Independiente en Android

Android está basado en el kernel Linux y permite la ejecución de binarios ELF estáticos.

### Arquitecturas soportadas

El compilador cruzado nativo de Joss soporta las 4 arquitecturas estándar de Android:
- `android/arm64` (la mayoría de teléfonos modernos)
- `android/arm` (dispositivos antiguos de 32 bits)
- `android/amd64` (emuladores de Android Studio en x86_64)
- `android/386` (emuladores antiguos de 32 bits)

### Compilación cruzada desde la máquina de desarrollo

Para compilar el CLI de Joss o un proyecto Joss autocontenido para Android:

```bash
# Compilar un script o proyecto para Android arm64
joss build native android arm64

# O compilar el CLI completo directamente con Go
CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build ./cmd/joss
```

El binario resultante puede transferirse al dispositivo e instalarse en **Termux**:

```bash
# En el terminal de Termux en Android:
chmod +x joss
mv joss $PREFIX/bin/
joss version
```

> [!NOTE]
> En iOS no existe una consola de comandos independiente debido a las directivas de seguridad y sandboxing de Apple. Por ello, el soporte en iOS se implementa a través del SDK embebible (Enfoque B).

---

## 2. Enfoque B: SDK Embebible `libjoss` (`pkg/mobile`)

El paquete `github.com/jossecurity/joss/pkg/mobile` expone una API de alto nivel lista para ser consumida desde aplicaciones móviles (Java, Kotlin, Swift, Flutter o React Native).

### Métodos de la API

| Función | Descripción | Retorno |
| :--- | :--- | :--- |
| `Run(source string, timeoutMs int)` | Ejecuta código fuente Joss capturando `stdout`, `stderr` y controlando el tiempo límite contra bucles infinitos. | JSON `ExecutionResult` |
| `Analyze(source string)` | Análisis estático y sintáctico sin ejecutar el código; reporta diagnósticos con número de línea y columna para editores móviles. | JSON `AnalysisResult` |
| `Version()` | Retorna la versión actual del motor Joss. | `string` |

### Estructura de Respuesta JSON (`ExecutionResult`)

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

Si el código contiene un error de sintaxis o tipo, `success` es `false` y `diagnostics` lista los detalles para resaltar el error en la interfaz de la app:

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

## 3. Integración en Android (Kotlin / Jetpack Compose)

En aplicaciones como **Aprende más**, puedes invocar el SDK desde un ViewModel o servicio de ejecución.

### Ejemplo en Kotlin

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

## 4. Integración en iOS (Swift / SwiftUI)

En iOS, el SDK se incluye como un `.xcframework` en el proyecto Xcode.

### Ejemplo en Swift

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

## 5. Compilación del SDK para Producción

### Generar AAR para Android con `gomobile`

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
gomobile bind -target=android -o libjoss.aar ./pkg/mobile
```

El archivo `libjoss.aar` generado se coloca en la carpeta `app/libs/` de Android Studio.

### Generar XCFramework para iOS con `gomobile`

```bash
gomobile bind -target=ios -o JossMobile.xcframework ./pkg/mobile
```

El directorio `JossMobile.xcframework` se arrastra a la sección *Frameworks, Libraries, and Embedded Content* del proyecto Xcode en macOS.
