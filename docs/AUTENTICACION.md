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
| `verify2FAChallenge(token,codigo)` | JWT final o false tras verificar desafío y código (soporta App TOTP y Email OTP). |
| `complete2FA(id)` | Genera JWT tras buscar usuario; **no comprueba un código TOTP por sí misma**. No exponer como endpoint público con ID aportado por cliente. |
| `enabledSocialProviders()` | Array de proveedores OAuth habilitados en el entorno (`google`, `apple`, `facebook`, `x`, `github`, `twitch`, `yahoo`, `microsoft`). |
| `socialRedirect(proveedor,urlRetorno)` | URL de redirección OAuth para el proveedor indicado. |
| `socialCallback(proveedor,codigo,urlRetorno)` | AuthLoginResult procesado tras intercambiar el código y registrar/enlazar al usuario. |

## Resultado fluido y callbacks

`AuthLoginResult` conserva éxito, error, usuario y respuesta. La evaluación de doble factor (2FA) se realiza de forma automática en `Auth::login` (detectando tanto apps de autenticación TOTP como códigos por correo electrónico):

| Método | Cuándo llama al callback | Parámetro |
|---|---|---|
| `onSuccess(callback)` | Credenciales correctas y sin desafío requerido. | JWT. |
| `onChallenge(callback)` | Credenciales correctas y desafío requerido (App TOTP o Email OTP). | JWT temporal de desafío. |
| `onFail(callback)` | Credenciales incorrectas. | Mensaje de error. |
| `response()` | Devuelve resultado del callback ejecutado o null. | Ninguno. |

Los callbacks fuente deben tipar sus parámetros, por ejemplo
`func(mixed $token) { return Response::json({"token": $token}) }`.
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

## Autenticación Social (OAuth 2.0)

Joss soporta de forma nativa inicio de sesión y vinculación de cuentas con 8 proveedores OAuth. Cada proveedor se habilita **automáticamente** si defines sus dos variables de entorno en el archivo `.env` o `env.joss`:

| Proveedor | Variable Client ID | Variable Client Secret |
|---|---|---|
| Google | `GOOGLE_CLIENT_ID` | `GOOGLE_CLIENT_SECRET` |
| GitHub | `GITHUB_CLIENT_ID` | `GITHUB_CLIENT_SECRET` |
| Microsoft | `MICROSOFT_CLIENT_ID` | `MICROSOFT_CLIENT_SECRET` |
| X (Twitter) | `X_CLIENT_ID` | `X_CLIENT_SECRET` |
| Facebook | `FACEBOOK_CLIENT_ID` | `FACEBOOK_CLIENT_SECRET` |
| Apple | `APPLE_CLIENT_ID` | `APPLE_CLIENT_SECRET` |
| Twitch | `TWITCH_CLIENT_ID` | `TWITCH_CLIENT_SECRET` |
| Yahoo | `YAHOO_CLIENT_ID` | `YAHOO_CLIENT_SECRET` |

La URL de redirección (callback) registrada en cada consola de desarrollador debe apuntar a:
```text
https://tu-dominio.com/auth/{proveedor}/callback
```

- `Auth::enabledSocialProviders()` devuelve únicamente la lista de proveedores cuyas credenciales están configuradas.
- `Auth::socialRedirect(proveedor, urlRetorno)` genera la URL segura hacia la pasarela OAuth con validación `state` anti-CSRF.
- `Auth::socialCallback(proveedor, codigo, urlRetorno)` intercambia el código, obtiene el perfil, vincula en `user_social_accounts` y retorna un `AuthLoginResult`.

Fuentes: [Auth](../pkg/core/auth.go), [OAuth Social](../pkg/core/auth_social.go), [JWT](../pkg/core/auth_jwt.go),
[flujo MFA](../pkg/core/auth_fluent.go), [tablas](../pkg/core/auth_tables.go).
