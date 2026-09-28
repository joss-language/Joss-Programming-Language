# Autenticación y segundo factor

[Índice](README.md) · Antes: [HTTP](CONTROLADORES.md), [modelos](MODELOS.md) · Después: [middleware](MIDDLEWARE.md)

**Autenticar** es comprobar quién realiza una petición. **Autorizar** es decidir
qué puede hacer esa persona. Auth integra usuarios, contraseñas y JWT, pero una
llamada como update(id,datos) no reemplaza las comprobaciones de permisos del
controlador.

Las operaciones necesitan conexión, tablas de autenticación y secretos configurados.
Un JWT es un token firmado; no debe confundirse con una contraseña ni publicarse
en registros. Esta referencia describe el código local, no certifica la seguridad
de una aplicación desplegada.

## Consultar al usuario actual

Fragmento para un controlador con sesión ya validada:

<!-- joss-check: requiere contexto autenticado -->
```joss
var $usuario = Auth::user()
$usuario != null ? {
    print($usuario->email)
} : {
    print("Sin sesion")
}
```

user() devuelve **instancia o null**, no map ni JSON. Usa `->`; para un ID
prefiere `Auth::id()`. Pasa a vistas campos escalares seleccionados en lugar
de la instancia entera.

## Contratos Auth

| Firma | Retorno y efecto |
|---|---|
| `hash(contraseña)` | Hash bcrypt como string o null al fallar. |
| `create(mapaDatos)` | Token de usuario o false según inserción; requiere datos del esquema. No es envío de correo automático. |
| `attempt(email,contraseña)` | JWT o false; valida credenciales según las reglas de tablas. No incorpora por sí mismo todo el flujo fluido MFA. |
| `login(email,contraseña)` | AuthLoginResult o null por argumentos insuficientes. |
| `check()`, `guest()` | Bool sobre contexto de sesión. |
| `user()`, `id()` | Instancia/ID actual o null. |
| `hasRole(nombre)` | Bool; no verifica propiedad de un recurso. |
| `validateToken(token)` | Bool; verifica JWT y repuebla contexto de sesión. |
| `refresh(id)` | JWT renovado o false/null; autoriza primero quién puede solicitarlo. |
| `update(id,mapa)`, `delete(id)` | Bool o null según argumentos/fallo; modifican usuarios. |
| `logout()` | Limpia contexto y retorna true. No implica revocación global de todo JWT emitido. |
| `verify(token)` | Verifica token de confirmación de correo, devuelve bool. |
| `verificationStatus(email)` | not_found, verified o unverified. |
| `resendVerification(email)` | Token de verificación o false; la entrega del mensaje corresponde a la aplicación. |
| `forgotPassword(email)` | Token de recuperación o false; no es confirmación de envío SMTP. |
| `resetPassword(token,nueva)` | true o texto de error: invalid_token, weak_password, database_error, used_token, expired_token. Compara con true, no sólo truthiness del texto. |
| `verify2FAChallenge(token,codigo)` | JWT final o false tras verificar desafío y código. |
| `complete2FA(id)` | Genera JWT tras buscar usuario; **no comprueba un código TOTP por sí misma**. No exponer como endpoint público con ID aportado por cliente. |

## Resultado fluido y callbacks

`AuthLoginResult` conserva éxito, error, usuario y respuesta. Cada método
devuelve la misma instancia salvo response():

| Método | Cuándo llama al callback | Parámetro |
|---|---|---|
| `require2FA()` | Consulta métodos MFA activos y marca requisito. | Sin callback. |
| `onSuccess(callback)` | Credenciales correctas y sin desafío requerido. | JWT. |
| `onChallenge(callback)` | Credenciales correctas y desafío requerido. | JWT temporal de desafío. |
| `onFail(callback)` | Credenciales incorrectas. | Mensaje de error. |
| `response()` | Devuelve resultado del callback ejecutado o null. | Ninguno. |

Los callbacks fuente deben tipar sus parámetros, por ejemplo
`func(mixed $token) { return Response::json({"token": $token}) }`.
Registra require2FA antes de onSuccess cuando el flujo requiera segundo factor.
No muestres un token final antes de terminar el desafío.

## MFA y TwoFactor

| Firma | Contrato |
|---|---|
| `MFA::generateTOTP()` | Map secret, qr_uri y qr_url. qr_uri es URI codificada; qr_url apunta a un servicio externo de QR e incluye el secreto. Mostrar esa URL enviaría el secreto a ese servicio. |
| `MFA::verifyTOTP(secreto,codigo)` | Bool según ventana temporal implementada. |
| `MFA::generateRecoveryCodes()` | Array de códigos. La persistencia y su asociación al usuario requieren el flujo de aplicación. |
| `MFA::verifyRecoveryCode(id,codigo)` | Consulta códigos guardados, verifica y consume el que coincide. |
| `TwoFactor::required(usuario)` | Bool sobre instancia y registros MFA. |
| `TwoFactor::verify(id,codigo)` | Bool; obtiene secreto de métodos MFA del usuario. |

El generador TOTP de esta implementación utiliza math/rand y construye una URL
externa con el secreto. Son hallazgos que necesitan revisión de seguridad;
no se presentan como garantías criptográficas del framework.

## Correo

`SmtpClient` es una clase nativa separada. `auth(usuario,contraseña)`,
`secure(bool)` y `timeout(segundos)` configuran y devuelven su instancia;
`send(destinatario,asunto,cuerpo)` devuelve bool y `lastError()` entrega
el último error textual. Requiere servidor SMTP y configuración MAIL_* según
[smtp_native.go](../pkg/core/smtp_native.go). Crear un token no envía ese correo.

Fuentes: [Auth](../pkg/core/auth.go), [JWT](../pkg/core/auth_jwt.go),
[flujo MFA](../pkg/core/auth_fluent.go), [tablas](../pkg/core/auth_tables.go).
