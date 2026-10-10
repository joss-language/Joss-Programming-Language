# Authentication and second factor

[Index](README.md) · Before: [HTTP](CONTROLADORES.md) , [models](MODELOS.md) · After:[middleware](MIDDLEWARE.md)

**Authenticate** is checking who makes a request.**Authorize** is deciding
what that person can do.Auth integrates users, passwords, and JWT, but a
call like update(id,data) does not replace the permissions checks of the
driver.

Operations require connection, authentication tables, and secrets configured.
A JWT is a signed token;should not be confused with a password or published
in logs.This reference describes local code, it does not certify the security
of a deployed application.

## Query the current user

Fragment for a controller with already validated session:

<!-- joss-check: requiere contexto autenticado -->
```joss
var $usuario = Auth::user()
$usuario != null ? {
    print($usuario->email)
} : {
    print("Sin sesion")
}
```

user() returns **instance or null**, not map or JSON.Use `->` ;for an ID
prefers `Auth::id()` .Passes selected scalar fields to views instead of
in the entire instance.

## Auth Contracts

|Signature |Return and effect |
|---|---|
|`hash(contraseña)` |Hash bcrypt as string or null on failure.|
|`create(mapaDatos)` |User token or false depending on insertion;requires schema data.It is not automatic mailing.|
|`attempt(email,contraseña)` |JWT or false;validates credentials according to table rules.It does not itself incorporate the entire MFA fluid flow.|
|`login(email,contraseña)` |AuthLoginResult or null for insufficient arguments.|
|`check()` , `guest()` |Bool on session context.|
|`user()` , `id()` |Current instance/ID or null.|
|`hasRole(nombre)` |Bool;does not verify ownership of a resource.|
|`validateToken(token)` |Bool;verifies JWT and repopulates session context.|
|`refresh(id)` |JWT renewed or false/null;authorizes first who can request it.|
|`update(id,mapa)` , `delete(id)` |Bool or null depending on arguments/fault;modify users.|
|`logout()` |Clear context and return true.It does not imply global revocation of all JWT issued.|
|`verify(token)` |Check mail confirmation token, return bool.|
|`verificationStatus(email)` |not_found, verified or unverified.|
|`resendVerification(email)` |verification token or false;message delivery is up to the application.|
|`forgotPassword(email)` |Recovery token or false;It is not SMTP sending confirmation.|
|`resetPassword(token,nueva)` |true or error text: invalid_token, weak_password, database_error, used_token, expired_token.Compare with true, not just truthiness of the text.|
|`verify2FAChallenge(token,codigo)` |JWT final or false after verifying challenge and code (supports App TOTP and Email OTP).|
|`complete2FA(id)` |Generates JWT after searching for user;**does not check a TOTP code itself**.Do not expose as a public endpoint with an ID provided by the client.|
|`enabledSocialProviders()` |Array of OAuth providers enabled in the environment ( `google` , `apple` , `facebook` , `x` , `github` , `twitch` , `yahoo` ,`microsoft` ).|
|`socialRedirect(proveedor,urlRetorno)` |OAuth redirect URL for the indicated provider.|
|`socialCallback(proveedor,codigo,urlRetorno)` |AuthLoginResult processed after exchanging the code and registering/binding the user.|

## Smooth output and callbacks

`AuthLoginResult` preserves success, error, user and response.Two-factor authentication (2FA) is performed automatically in `Auth::login` (detecting both TOTP authentication apps and email codes):

|Method |When to call callback |Parameter |
|---|---|---|
|`onSuccess(callback)` |Correct credentials and no challenge required.|JWT.|
|`onChallenge(callback)` |Correct credentials and challenge required (TOTP App or OTP Email).|Challenge temporary JWT.|
|`onFail(callback)` |Incorrect credentials.|Error message.|
|`response()` |Returns result of the executed callback or null.|None.|

Source callbacks must type their parameters, for example
`func(mixed $token) { return Response::json({"token": $token}) }` .
Don't show a final token before finishing the challenge.

## MFA and TwoFactor

|Signature |Contract |
|---|---|
|`MFA::generateTOTP()` |Map secret, qr_uri and qr_url.qr_uri is encoded URI;qr_url points to an external QR service and includes the secret.Showing that URL would send the secret to that service.|
|`MFA::verifyTOTP(secreto,codigo)` |Bool according to the implemented time window.|
|`MFA::generateRecoveryCodes()` |Code array.Persistence and its association to the user require the application flow.|
|`MFA::verifyRecoveryCode(id,codigo)` |Consult saved codes, verify and consume the one that matches.|
|`TwoFactor::required(usuario)` |Bool about MFA instance and records.|
|`TwoFactor::verify(id,codigo)` |Bool;Gets secret from user's MFA methods.|

The TOTP generator in this implementation uses math/rand and constructs an external
URL with the secret.These are findings that need security review;
are not presented as cryptographic guarantees of the framework.

## Mail

`SmtpClient` is a separate native class.`auth(usuario,contraseña)` ,
`secure(bool)` and `timeout(segundos)` configure and return your instance;
`send(destinatario,asunto,cuerpo)` returns bool and `lastError()` returns
the last textual error.Requires SMTP server and MAIL_* configuration as per
[smtp_native.go](../../pkg/core/smtp_native.go) .Creating a token does not send that email.

## Social Authentication (OAuth 2.0)

Joss natively supports login and account linking with 8 OAuth providers.Each provider is enabled **automatically** if you define its two environment variables in the `.env` or `env.joss` file:

|Supplier |Client ID variable |Variable Client Secret |
|---|---|---|
|Google |`GOOGLE_CLIENT_ID` |`GOOGLE_CLIENT_SECRET` |
|GitHub |`GITHUB_CLIENT_ID` |`GITHUB_CLIENT_SECRET` |
|Microsoft |`MICROSOFT_CLIENT_ID` |`MICROSOFT_CLIENT_SECRET` |
|X (Twitter) |`X_CLIENT_ID` |`X_CLIENT_SECRET` |
|Facebook |`FACEBOOK_CLIENT_ID` |`FACEBOOK_CLIENT_SECRET` |
|Apple |`APPLE_CLIENT_ID` |`APPLE_CLIENT_SECRET` |
|Twitch |`TWITCH_CLIENT_ID` |`TWITCH_CLIENT_SECRET` |
|Yahoo |`YAHOO_CLIENT_ID` |`YAHOO_CLIENT_SECRET` |

The callback URL registered in each developer console should point to:
```text
https://tu-dominio.com/auth/{proveedor}/callback
```

- `Auth::enabledSocialProviders()` returns only the list of providers whose credentials are configured.
- `Auth::socialRedirect(proveedor, urlRetorno)` generates the secure URL to the OAuth gateway with `state` anti-CSRF validation.
- `Auth::socialCallback(proveedor, codigo, urlRetorno)` swaps the code, gets the profile, binds in `user_social_accounts` and returns a `AuthLoginResult` .

Sources: [Auth](../../pkg/core/auth.go) , [OAuth Social](../../pkg/core/auth_social.go) , [JWT](../../pkg/core/auth_jwt.go) ,
[MFA flow](../../pkg/core/auth_fluent.go) , [tables](../../pkg/core/auth_tables.go) .
