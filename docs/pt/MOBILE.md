# Suporte móvel: Android e iOS

Joss oferece execução e desenvolvimento em plataformas móveis por meio de duas abordagens:
1. **CLI e binários independentes no Android (Termux / ADB):** Permite executar e compilar scripts Joss diretamente em dispositivos Android.
2. **SDK incorporável `libjoss` (`pkg/mobile`) para Android e iOS:** Permite integrar o motor Joss a aplicativos nativos e híbridos escritos em **Kotlin** (Android) e **Swift** (iOS). É ideal para aplicativos educacionais como *Aprende más*, ambientes de teste interativos ou execução de regras de negócio no cliente.

---

## 1. Abordagem A: CLI independente no Android

O Android é baseado no kernel Linux e permite executar binários ELF estáticos.

### Arquiteturas compatíveis

O compilador cruzado nativo de Joss oferece suporte às quatro arquiteturas padrão do Android:
- `android/arm64` (a maioria dos telefones modernos)
- `android/arm` (dispositivos antigos de 32 bits)
- `android/amd64` (emuladores x86_64 do Android Studio)
- `android/386` (emuladores antigos de 32 bits)

### Compilação cruzada a partir da máquina de desenvolvimento

Para compilar a CLI de Joss ou um projeto Joss autocontido para Android:

```bash
# Compilar un script o proyecto para Android arm64
joss build native android arm64

# O compilar el CLI completo directamente con Go
CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build ./cmd/joss
```

Transfira o binário resultante para o dispositivo e instale-o no **Termux**:

```bash
# En el terminal de Termux en Android:
chmod +x joss
mv joss $PREFIX/bin/
joss version
```

> [!NOTE]
> O iOS não oferece um console de comandos independente por causa das políticas de segurança e sandboxing da Apple. Por isso, o suporte no iOS é fornecido pelo SDK incorporável descrito na Abordagem B.

---

## 2. Abordagem B: SDK incorporável `libjoss` (`pkg/mobile`)

O pacote `github.com/jossecurity/joss/pkg/mobile` expõe uma API de alto nível para aplicativos móveis desenvolvidos com Java, Kotlin, Swift, Flutter ou React Native.

### Métodos da API

| Função | Descrição | Retorno |
| :--- | :--- | :--- |
| `Run(source string, timeoutMs int)` | Executa código-fonte Joss, captura `stdout` e `stderr` e aplica um limite de tempo para proteger contra loops infinitos.

| JSON `ExecutionResult` |
| `Analyze(source string)` | Realiza análise estática e sintática sem executar o código; relata diagnósticos com linha e coluna para editores móveis.

| JSON `AnalysisResult` |
| `Version()` | Retorna a versão atual do motor Joss.

| `string` |

### Estrutura da resposta JSON (`ExecutionResult`)

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

Se o código contiver um erro de sintaxe ou tipo, `success` será `false` e `diagnostics` conterá os detalhes necessários para destacar o erro na interface do aplicativo:

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

## 3. Integração no Android (Kotlin / Jetpack Compose)

Em aplicativos como **Aprende más**, invoque o SDK a partir de um ViewModel ou serviço de execução.

### Exemplo em Kotlin

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

## 4. Integração no iOS (Swift / SwiftUI)

No iOS, inclua o SDK no projeto Xcode como um `.xcframework`.

### Exemplo em Swift

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

## 5. Compilação do SDK para produção

### Gerar AAR para Android com `gomobile`

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
gomobile bind -target=android -o libjoss.aar ./pkg/mobile
```

Coloque o arquivo `libjoss.aar` gerado no diretório `app/libs/` do Android Studio.

### Gerar XCFramework para iOS com `gomobile`

```bash
gomobile bind -target=ios -o JossMobile.xcframework ./pkg/mobile
```

Arraste o diretório `JossMobile.xcframework` para a seção *Frameworks, Libraries, and Embedded Content* do projeto Xcode no macOS.
