# 05. Seguridad, Análisis Defensivo y Gestión de Recursos

**Proyecto:** Joss Programming Language  
**Áreas Auditadas:** `pkg/server`, `pkg/core/auth.go`, `pkg/core/builtins_io.go`, `pkg/pluginpkg`  
**Clasificación de Riesgos:** Estándares OWASP / CVSS  

---

## 1. Resumen de Seguridad

Joss está concebido para ejecutar aplicaciones web de backend de confianza. La documentación oficial del proyecto (`docs/PLUGINS.md:115`) aclara expresamente que **no constituye un sandbox general ni aislamiento de procesos a nivel de sistema operativo**. Cualquier código fuente escrito en Joss tiene acceso a las capacidades del sistema anfitrión expuestas por sus built-ins.

---

## 2. Hallazgos de Seguridad Críticos y de Alta Prioridad

### 2.1 [HAL-SEC-01] Generación de Códigos OTP/MFA con Generador Pseudoaleatorio No Criptográfico (Severidad: P1)
- **Ubicación:** `pkg/core/auth.go:783-785` y `pkg/core/auth_fluent.go:217-226`.
- **Código Fuente:**
  ```go
  // pkg/core/auth.go:783
  code := fmt.Sprintf("%06d", rand.Intn(1000000))
  hashedBytes, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
  ```
- **Descripción:** El paquete utilizado para generar códigos de autenticación de dos factores (2FA / OTP) de 6 dígitos es `math/rand` (importado en la línea 6), el cual es un generador determinista no seguro para propósitos criptográficos. Aunque el código generado es posteriormente hasheado con `bcrypt`, el espacio de semillas de `math/rand` permite predecir los tokens generados conociendo valores previos o el tiempo del servidor.
- **Impacto:** Posible evasión de autenticación de dos factores (MFA Bypass).
- **Remediación:** Sustituir inmediatamente por `crypto/rand` mediante `crand.Int(crand.Reader, big.NewInt(1000000))`.

---

### 2.2 [HAL-SEC-02] Fuga de Tokens CSRF e ID de Sesión a stdout y Respuestas HTTP (Severidad: P1)
- **Ubicación:** `pkg/server/handler_routing.go:344` y `348-350`.
- **Código Fuente:**
  ```go
  // Línea 344:
  fmt.Printf("[CSRF DEBUG] Session: %s | Stored: %s | Received: %s\n", sessionID, csrfToken, reqToken)
  // Línea 346:
  if reqToken == "" || reqToken != csrfToken {
      fmt.Fprintf(w, "<h1>419 Page Expired</h1><p>CSRF token mismatch.</p>")
  ```
- **Descripción:**  
  1. Cada validación de CSRF imprime a la salida estándar (`stdout`) el identificador completo de la sesión del usuario junto con el token esperado y el recibido. En entornos de producción con agregadores de logs (como Datadog o CloudWatch), esto expone las sesiones de los usuarios a cualquier operador o servicio con acceso a logs.
  2. La comparación entre `reqToken` y `csrfToken` utiliza el operador estándar `!=` de Go en lugar de comparación en tiempo constante (`subtle.ConstantTimeCompare`), lo que introduce susceptibilidad teórica a ataques de temporización.

---

### 2.3 [HAL-SEC-03] Ausencia de Validación de Origen en WebSockets (CSWSH) (Severidad: P1)
- **Ubicación:** `pkg/server/websocket.go:16-18` y `pkg/server/hotreload.go:28`.
- **Código Fuente:**
  ```go
  CheckOrigin: func(r *http.Request) bool {
      return true // Allow all for now
  },
  ```
- **Descripción:** El multiplexor de WebSockets acepta conexiones entrantes desde cualquier origen (`CheckOrigin` siempre retorna `true`). Si una aplicación Joss maneja sesiones autenticadas por cookies a través del protocolo WebSocket, un sitio web malicioso de terceros puede secuestrar la conexión del usuario mediante *Cross-Site WebSocket Hijacking* (CSWSH).
- **Remediación:** Validar que el encabezado `Origin` coincida con el dominio del host configurado en `APP_URL`.

---

### 2.4 [HAL-SEC-04] Carga de Drivers Nativos sin Verificación de Firma ni Integridad (Severidad: P2)
- **Ubicación:** `pkg/core/native_driver_unix.go:21` y `native_driver_windows.go`.
- **Descripción:** El sistema permite cargar bibliotecas compartidas binarias (`.so`, `.dll`) mediante llamadas dinámicas de `purego.Dlopen`. No existe verificación de sumas de verificación (SHA-256), firmas digitales ni listas blancas de rutas autorizadas. Si un atacante logra escribir en una ruta de plugins, puede ejecutar código arbitrario a nivel de sistema operativo.

---

## 3. Manejo de Recursos y DoS

1. **Límite de Recursión:** Protegido eficazmente con `MaxCallDepth = 1024` en `pkg/core/call_method.go:55`.
2. **Bucles Infinitos:** No existe un presupuesto máximo de instrucciones (fuel/quota) en el evaluador AST estándar. Un script con `while(true)` consume el 100% de un núcleo de CPU hasta ser terminado externamente.
3. **Carga de Archivos Multipart:** Protegido con límite de memoria de 10 MB (`ParseMultipartForm(10 << 20)` en `request_data.go`). Sin embargo, no se implementa `http.MaxBytesReader` global en el middleware de entrada para el cuerpo de solicitudes HTTP arbitrarias.
