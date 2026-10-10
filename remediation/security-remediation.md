# Remediación de Seguridad: Autenticación, Middleware y WebSockets

**Proyecto:** Joss Programming Language  
**Componentes:** `pkg/core/auth.go`, `pkg/core/auth_fluent.go`, `pkg/server/handler_routing.go`, `pkg/server/websocket.go`  
**Fecha:** 9 de octubre de 2026  

---

## 1. Generación Segura de Desafíos OTP y Secretos MFA

### 1.1 Vulnerabilidad Previa
El método `sendEmailOTPChallenge` en `pkg/core/auth.go` utilizaba `math/rand.Intn(1000000)`, un generador determinista no apto para fines criptográficos. De igual forma, `generateRandomBase32Secret` y `generateRandomRecoveryCode` en `pkg/core/auth_fluent.go` seleccionaban caracteres aleatorios empleando `math/rand`.

### 1.2 Remediación Aplicada
- Se reemplazó el import de `math/rand` por `crypto/rand` y `math/big`.
- En `auth.go`:
  ```go
  n, err := crand.Int(crand.Reader, big.NewInt(1000000))
  if err != nil {
      return false
  }
  code := fmt.Sprintf("%06d", n.Int64())
  ```
- En `auth_fluent.go`:
  ```go
  idx, err := crand.Int(crand.Reader, big.NewInt(int64(len(alphabet))))
  if err != nil {
      panic(fmt.Sprintf("crypto/rand error: %v", err))
  }
  sb.WriteByte(alphabet[idx.Int64()])
  ```

---

## 2. Protección Contra Fuga de Secretos en Middleware CSRF

### 2.1 Vulnerabilidad Previa
En `pkg/server/handler_routing.go`, cada validación de solicitud HTTP POST/PUT imprimía a `stdout` el ID de sesión del usuario junto con el token CSRF esperado y el recibido. Adicionalmente, ante un desajuste, la respuesta 419 incluía un comentario HTML exponiendo ambos tokens.

### 2.2 Remediación Aplicada
- Se eliminó la instrucción de depuración `fmt.Printf("[CSRF DEBUG]...")`.
- Se eliminó el comentario HTML en la respuesta HTTP 419.
- Se implementó comparación en tiempo constante:
  ```go
  if csrfToken == "" || reqToken == "" || subtle.ConstantTimeCompare([]byte(reqToken), []byte(csrfToken)) != 1 {
      w.WriteHeader(http.StatusForbidden)
      fmt.Fprintf(w, "<h1>419 Page Expired</h1><p>CSRF token mismatch.</p>")
      return false
  }
  ```

---

## 3. Política Estricta de Origen en Conexiones WebSocket

### 3.1 Vulnerabilidad Previa
En `pkg/server/websocket.go`, el upgrader de WebSocket utilizaba `CheckOrigin: func(r *http.Request) bool { return true }`, permitiendo que cualquier sitio web malicioso de terceros pudiera secuestrar la conexión WebSocket del usuario autenticado (CSWSH).

### 3.2 Remediación Aplicada
Se implementó la función `checkWebSocketOrigin(r *http.Request)` que:
1. Permite conexiones de clientes sin encabezado `Origin` (clientes móviles o directos de backend).
2. Valida que el host de `Origin` coincida con el encabezado `Host` de la solicitud entrante.
3. Permite orígenes en loopback (`localhost`, `127.0.0.1`, `::1`) para desarrollo local.
4. Permite dominios adicionales explícitamente configurados a través de `APP_ALLOWED_ORIGINS` o `APP_URL`.
5. Rechaza cualquier conexión que no satisfaga estas condiciones antes de completar el handshake de WebSocket.
