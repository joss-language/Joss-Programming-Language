# Catálogo nativo generado

Antes: [biblioteca y contratos](MODULOS_NATIVOS.md). Índice: [documentación](README.md).

Generado con `go run ./tools/docgen` desde `Runtime.RegisterNativeClasses()`.
No editar manualmente. Cada nombre es una entrada del registro real. **Retorno publicado**
no significa retorno exhaustivo observado: las discrepancias están en la biblioteca.
Los parámetros nativos no están publicados en este registro; consulta las tablas
de contratos y el handler enlazado. `mixed` no certifica aislamiento ni ausencia de fallos.

## Auth

Implementación: [executeAuthMethod](../pkg/core/auth.go#L14).

| Método | Retorno publicado al analizador |
|---|---|
| `attempt` | `mixed` |
| `check` | `bool` |
| `complete2FA` | `mixed` |
| `create` | `mixed` |
| `delete` | `mixed` |
| `forgotPassword` | `mixed` |
| `guest` | `bool` |
| `hasRole` | `bool` |
| `hash` | `mixed` |
| `id` | `mixed` |
| `login` | `mixed` |
| `logout` | `mixed` |
| `refresh` | `mixed` |
| `resendVerification` | `mixed` |
| `resetPassword` | `mixed` |
| `update` | `mixed` |
| `user` | `mixed` |
| `validateToken` | `mixed` |
| `verificationStatus` | `mixed` |
| `verify` | `bool` |
| `verify2FAChallenge` | `mixed` |

## AuthLoginResult

Implementación: [executeAuthLoginResultMethod](../pkg/core/auth_fluent.go#L18).

| Método | Retorno publicado al analizador |
|---|---|
| `onChallenge` | `AuthLoginResult` |
| `onFail` | `AuthLoginResult` |
| `onSuccess` | `AuthLoginResult` |
| `require2FA` | `AuthLoginResult` |
| `response` | `mixed` |

## Blueprint

Implementación: [executeBlueprintMethod](../pkg/core/schema_blueprint.go#L34).

| Método | Retorno publicado al analizador |
|---|---|
| `bigInteger` | `Blueprint` |
| `boolean` | `Blueprint` |
| `char` | `Blueprint` |
| `comment` | `Blueprint` |
| `date` | `Blueprint` |
| `dateTime` | `Blueprint` |
| `decimal` | `Blueprint` |
| `default` | `Blueprint` |
| `double` | `Blueprint` |
| `dropColumn` | `Blueprint` |
| `dropIndex` | `Blueprint` |
| `enum` | `Blueprint` |
| `float` | `Blueprint` |
| `foreign` | `Blueprint` |
| `id` | `Blueprint` |
| `increments` | `Blueprint` |
| `index` | `Blueprint` |
| `integer` | `Blueprint` |
| `json` | `Blueprint` |
| `longText` | `Blueprint` |
| `mediumInteger` | `Blueprint` |
| `mediumText` | `Blueprint` |
| `nullable` | `Blueprint` |
| `on` | `Blueprint` |
| `onDelete` | `Blueprint` |
| `onUpdate` | `Blueprint` |
| `references` | `Blueprint` |
| `renameColumn` | `Blueprint` |
| `smallInteger` | `Blueprint` |
| `softDeletes` | `Blueprint` |
| `string` | `Blueprint` |
| `text` | `Blueprint` |
| `time` | `Blueprint` |
| `timestamp` | `Blueprint` |
| `timestamps` | `Blueprint` |
| `tinyInteger` | `Blueprint` |
| `unique` | `Blueprint` |
| `uniqueIndex` | `Blueprint` |
| `unsigned` | `Blueprint` |
| `unsignedBigInteger` | `Blueprint` |
| `unsignedInteger` | `Blueprint` |

## Cache

Implementación: [executeCacheMethod](../pkg/core/native_cache.go#L18).

| Método | Retorno publicado al analizador |
|---|---|
| `forget` | `mixed` |
| `get` | `mixed` |
| `has` | `bool` |
| `put` | `mixed` |

## Console

Implementación: [executeConsoleMethod](../pkg/core/native.go#L307).

| Método | Retorno publicado al analizador |
|---|---|
| `blue` | `string` |
| `bold` | `string` |
| `clear` | `string` |
| `color` | `string` |
| `cyan` | `string` |
| `gray` | `string` |
| `green` | `string` |
| `log` | `void` |
| `magenta` | `string` |
| `red` | `string` |
| `yellow` | `string` |

## Cron

Implementación: [executeCronMethod](../pkg/core/cron.go#L11).

| Método | Retorno publicado al analizador |
|---|---|
| `schedule` | `mixed` |

## Exception

Implementación: [executeExceptionMethod](../pkg/core/native.go#L368).

| Método | Retorno publicado al analizador |
|---|---|
| `constructor` | `mixed` |
| `getCode` | `int` |
| `getMessage` | `string` |

## GranDB

Implementación: [executeGranDBMethod](../pkg/core/database.go#L11).

| Método | Retorno publicado al analizador |
|---|---|
| `avg` | `float\|null` |
| `changeDB` | `GranDB` |
| `changedb` | `GranDB` |
| `chunk` | `mixed` |
| `connection` | `GranDB` |
| `count` | `int` |
| `crossJoin` | `GranDB` |
| `crossjoin` | `GranDB` |
| `dd` | `mixed` |
| `decrement` | `mixed` |
| `delete` | `mixed` |
| `deleteAll` | `mixed` |
| `distinct` | `GranDB` |
| `doesntExist` | `bool` |
| `dump` | `mixed` |
| `exists` | `bool` |
| `find` | `mixed` |
| `findMany` | `mixed` |
| `findOrFail` | `mixed` |
| `findmany` | `mixed` |
| `findorfail` | `mixed` |
| `first` | `mixed` |
| `firstOrFail` | `mixed` |
| `firstWhere` | `mixed` |
| `firstofail` | `mixed` |
| `firstwhere` | `mixed` |
| `forPage` | `GranDB` |
| `forpage` | `GranDB` |
| `get` | `array` |
| `getBindings` | `array` |
| `getbindings` | `array` |
| `groupBy` | `GranDB` |
| `groupby` | `GranDB` |
| `having` | `GranDB` |
| `inRandomOrder` | `GranDB` |
| `increment` | `mixed` |
| `innerJoin` | `GranDB` |
| `insert` | `mixed` |
| `insertGetId` | `mixed` |
| `insertgetid` | `mixed` |
| `join` | `GranDB` |
| `latest` | `GranDB` |
| `leftJoin` | `GranDB` |
| `limit` | `GranDB` |
| `max` | `mixed` |
| `min` | `mixed` |
| `offset` | `GranDB` |
| `oldest` | `GranDB` |
| `orHaving` | `GranDB` |
| `orWhere` | `GranDB` |
| `orWhereBetween` | `GranDB` |
| `orWhereColumn` | `GranDB` |
| `orWhereDate` | `mixed` |
| `orWhereDay` | `mixed` |
| `orWhereIn` | `GranDB` |
| `orWhereJsonContains` | `mixed` |
| `orWhereLike` | `GranDB` |
| `orWhereMonth` | `mixed` |
| `orWhereNot` | `GranDB` |
| `orWhereNotBetween` | `GranDB` |
| `orWhereNotIn` | `GranDB` |
| `orWhereNotNull` | `GranDB` |
| `orWhereNull` | `GranDB` |
| `orWhereTime` | `mixed` |
| `orWhereYear` | `mixed` |
| `orderBy` | `GranDB` |
| `orderByAsc` | `GranDB` |
| `orderByDesc` | `GranDB` |
| `orderby` | `GranDB` |
| `orderbyasc` | `GranDB` |
| `orderbydesc` | `GranDB` |
| `orhaving` | `GranDB` |
| `orwhere` | `GranDB` |
| `orwherebetween` | `GranDB` |
| `orwherecolumn` | `GranDB` |
| `orwheredate` | `mixed` |
| `orwhereday` | `mixed` |
| `orwherein` | `GranDB` |
| `orwherejsoncontains` | `mixed` |
| `orwherelike` | `GranDB` |
| `orwheremonth` | `mixed` |
| `orwherenot` | `GranDB` |
| `orwherenotbetween` | `GranDB` |
| `orwherenotin` | `GranDB` |
| `orwherenotnull` | `GranDB` |
| `orwherenull` | `GranDB` |
| `orwheretime` | `mixed` |
| `orwhereyear` | `mixed` |
| `paginate` | `mixed` |
| `pluck` | `array` |
| `reorder` | `GranDB` |
| `rightJoin` | `GranDB` |
| `select` | `GranDB` |
| `skip` | `GranDB` |
| `sole` | `mixed` |
| `sum` | `float` |
| `table` | `GranDB` |
| `take` | `GranDB` |
| `toSql` | `string` |
| `tosql` | `string` |
| `touch` | `mixed` |
| `transaction` | `mixed` |
| `truncate` | `mixed` |
| `unless` | `GranDB` |
| `update` | `mixed` |
| `updateOrInsert` | `mixed` |
| `updateorinsert` | `mixed` |
| `upsert` | `mixed` |
| `use` | `GranDB` |
| `value` | `mixed` |
| `when` | `GranDB` |
| `where` | `GranDB` |
| `whereBetween` | `GranDB` |
| `whereColumn` | `GranDB` |
| `whereDate` | `mixed` |
| `whereDay` | `mixed` |
| `whereIn` | `GranDB` |
| `whereJsonContains` | `mixed` |
| `whereLike` | `GranDB` |
| `whereMonth` | `mixed` |
| `whereNot` | `GranDB` |
| `whereNotBetween` | `GranDB` |
| `whereNotIn` | `GranDB` |
| `whereNotNull` | `GranDB` |
| `whereNull` | `GranDB` |
| `whereTime` | `mixed` |
| `whereYear` | `mixed` |
| `wherebetween` | `GranDB` |
| `wherecolumn` | `GranDB` |
| `wheredate` | `mixed` |
| `whereday` | `mixed` |
| `wherein` | `GranDB` |
| `wherejsoncontains` | `mixed` |
| `wherelike` | `GranDB` |
| `wheremonth` | `mixed` |
| `wherenot` | `GranDB` |
| `wherenotbetween` | `GranDB` |
| `wherenotin` | `GranDB` |
| `wherenotnull` | `GranDB` |
| `wherenull` | `GranDB` |
| `wheretime` | `mixed` |
| `whereyear` | `mixed` |

## Http

Implementación: [executeHttpMethod](../pkg/core/http_client.go#L14).

| Método | Retorno publicado al analizador |
|---|---|
| `delete` | `mixed` |
| `get` | `mixed` |
| `head` | `mixed` |
| `json` | `mixed` |
| `options` | `mixed` |
| `patch` | `mixed` |
| `post` | `mixed` |
| `put` | `mixed` |
| `request` | `mixed` |

## JSON

Implementación: [executeJSONMethod](../pkg/core/json.go#L8).

| Método | Retorno publicado al analizador |
|---|---|
| `decode` | `mixed` |
| `encode` | `string` |
| `parse` | `mixed` |
| `stringify` | `string` |

## Lang

Implementación: [executeLangMethod](../pkg/core/native_lang.go#L12).

| Método | Retorno publicado al analizador |
|---|---|
| `get` | `mixed` |
| `locale` | `string` |
| `locales` | `array` |
| `set` | `mixed` |

## MFA

Implementación: [executeMFAMethod](../pkg/core/auth_fluent.go#L99).

| Método | Retorno publicado al analizador |
|---|---|
| `generateRecoveryCodes` | `mixed` |
| `generateTOTP` | `mixed` |
| `verifyRecoveryCode` | `bool` |
| `verifyTOTP` | `bool` |

## Markdown

Implementación: [executeMarkdownMethod](../pkg/core/markdown.go#L15).

| Método | Retorno publicado al analizador |
|---|---|
| `readFile` | `string` |
| `toHtml` | `string` |

## Math

| Método | Retorno publicado al analizador |
|---|---|
| `abs` | `float` |
| `ceil` | `float` |
| `floor` | `float` |
| `random` | `int` |

## Middleware

Clase base registrada sin métodos nativos propios.

## Migration

Clase base registrada sin métodos nativos propios.

## Plugin

Implementación: [executePluginMethod](../pkg/core/native_plugin.go#L47).

| Método | Retorno publicado al analizador |
|---|---|
| `call` | `mixed` |
| `path` | `mixed` |
| `platform` | `mixed` |
| `stream` | `mixed` |

## Process

Implementación: [executeProcessMethod](../pkg/core/native_process.go#L13).

| Método | Retorno publicado al analizador |
|---|---|
| `constructor` | `mixed` |
| `kill` | `mixed` |
| `pid` | `mixed` |
| `start` | `mixed` |
| `stderr_chan` | `mixed` |
| `stdin` | `mixed` |
| `stdout_chan` | `mixed` |
| `wait` | `mixed` |

## Queue

| Método | Retorno publicado al analizador |
|---|---|
| `dequeue` | `mixed` |
| `enqueue` | `mixed` |
| `peek` | `mixed` |

## Redirect

Implementación: [executeRedirectMethod](../pkg/core/response.go#L177).

| Método | Retorno publicado al analizador |
|---|---|
| `to` | `WebResponse` |

## Redis

Implementación: [executeRedisMethod](../pkg/core/redis.go#L96).

| Método | Retorno publicado al analizador |
|---|---|
| `connect` | `mixed` |
| `del` | `mixed` |
| `flush` | `mixed` |
| `forget` | `mixed` |
| `get` | `mixed` |
| `has` | `mixed` |
| `set` | `mixed` |
| `ttl` | `mixed` |

## Request

Implementación: [executeRequestMethod](../pkg/core/request.go#L9).

| Método | Retorno publicado al analizador |
|---|---|
| `all` | `map` |
| `bearerToken` | `mixed` |
| `bearertoken` | `mixed` |
| `cookie` | `mixed` |
| `except` | `map` |
| `file` | `mixed` |
| `has` | `bool` |
| `hasFile` | `bool` |
| `hasfile` | `bool` |
| `header` | `mixed` |
| `input` | `mixed` |
| `ip` | `mixed` |
| `isMethod` | `bool` |
| `ismethod` | `bool` |
| `method` | `string` |
| `path` | `string` |
| `post` | `mixed` |
| `root` | `string` |
| `url` | `string` |
| `userAgent` | `mixed` |
| `useragent` | `mixed` |

## Response

Implementación: [executeResponseMethod](../pkg/core/response.go#L10).

| Método | Retorno publicado al analizador |
|---|---|
| `back` | `WebResponse` |
| `download` | `WebResponse` |
| `error` | `WebResponse` |
| `json` | `WebResponse` |
| `raw` | `WebResponse` |
| `redirect` | `WebResponse` |
| `stream` | `WebResponse` |

## Router

Implementación: [executeRouterMethod](../pkg/core/router.go#L12).

| Método | Retorno publicado al analizador |
|---|---|
| `any` | `mixed` |
| `api` | `mixed` |
| `delete` | `mixed` |
| `end` | `mixed` |
| `get` | `mixed` |
| `group` | `mixed` |
| `head` | `mixed` |
| `match` | `mixed` |
| `middleware` | `mixed` |
| `options` | `mixed` |
| `patch` | `mixed` |
| `post` | `mixed` |
| `put` | `mixed` |
| `query` | `mixed` |
| `registerMiddleware` | `mixed` |
| `ws` | `mixed` |

## SEO

Implementación: [executeSEOMethod](../pkg/core/native_seo.go#L12).

| Método | Retorno publicado al analizador |
|---|---|
| `canonical` | `SEO` |
| `description` | `SEO` |
| `keywords` | `SEO` |
| `meta` | `SEO` |
| `og` | `SEO` |
| `render` | `string` |
| `title` | `SEO` |

## SQLite

Implementación: [executeSQLiteMethod](../pkg/core/native_sqlite.go#L16).

| Método | Retorno publicado al analizador |
|---|---|
| `close` | `mixed` |
| `open` | `mixed` |
| `query` | `mixed` |

## Schema

Implementación: [executeSchemaMethod](../pkg/core/schema.go#L119).

| Método | Retorno publicado al analizador |
|---|---|
| `create` | `mixed` |
| `drop` | `mixed` |
| `dropIfExists` | `mixed` |
| `hasColumn` | `bool` |
| `hasTable` | `bool` |
| `rename` | `mixed` |
| `table` | `mixed` |

## Server

Implementación: [executeServerControlMethod](../pkg/core/native_server_control.go#L15).

| Método | Retorno publicado al analizador |
|---|---|
| `spawn` | `mixed` |
| `start` | `mixed` |

## Session

Implementación: [executeSessionMethod](../pkg/core/native_extensions.go#L75).

| Método | Retorno publicado al analizador |
|---|---|
| `all` | `map` |
| `forget` | `mixed` |
| `get` | `mixed` |
| `has` | `bool` |
| `put` | `mixed` |

## Sitemap

Implementación: [executeSitemapMethod](../pkg/core/native_seo.go#L127).

| Método | Retorno publicado al analizador |
|---|---|
| `add` | `Sitemap` |
| `exclude` | `Sitemap` |
| `generate` | `string` |
| `provider` | `Sitemap` |
| `xsl` | `Sitemap` |

## SmtpClient

Implementación: [executeSmtpClientMethod](../pkg/core/smtp_native.go#L12).

| Método | Retorno publicado al analizador |
|---|---|
| `auth` | `SmtpClient` |
| `lastError` | `string\|null` |
| `secure` | `SmtpClient` |
| `send` | `bool` |
| `timeout` | `SmtpClient` |

## Stack

| Método | Retorno publicado al analizador |
|---|---|
| `peek` | `mixed` |
| `pop` | `mixed` |
| `push` | `mixed` |

## Str

Implementación: [executeStrMethod](../pkg/core/native_extensions.go#L145).

| Método | Retorno publicado al analizador |
|---|---|
| `contains` | `bool` |
| `indexOf` | `int` |
| `length` | `int` |
| `random` | `string` |
| `replace` | `string` |
| `startsWith` | `bool` |
| `substring` | `string` |
| `trim` | `string` |

## Stream

Implementación: [executeStreamMethod](../pkg/core/native_stream.go#L10).

| Método | Retorno publicado al analizador |
|---|---|
| `close` | `mixed` |
| `send` | `mixed` |

## System

Implementación: [executeSystemMethod](../pkg/core/system.go#L14).

| Método | Retorno publicado al analizador |
|---|---|
| `Run` | `mixed` |
| `driver_call` | `mixed` |
| `env` | `mixed` |
| `load_driver` | `mixed` |
| `log` | `mixed` |
| `now` | `int` |
| `sleep` | `mixed` |

## Task

Implementación: [executeTaskMethod](../pkg/core/task.go#L10).

| Método | Retorno publicado al analizador |
|---|---|
| `on_request` | `mixed` |

## TwoFactor

Implementación: [executeTwoFactorMethod](../pkg/core/auth_fluent.go#L168).

| Método | Retorno publicado al analizador |
|---|---|
| `required` | `bool` |
| `verify` | `bool` |

## UUID

Implementación: [executeUUIDMethod](../pkg/core/lib_uuid.go#L8).

| Método | Retorno publicado al analizador |
|---|---|
| `generate` | `string` |
| `v4` | `string` |

## UserStorage

Implementación: [executeUserStorageMethod](../pkg/core/lib_storage.go#L20).

| Método | Retorno publicado al analizador |
|---|---|
| `delete` | `mixed` |
| `get` | `mixed` |
| `getToFile` | `mixed` |
| `path` | `mixed` |
| `put` | `mixed` |

## View

Implementación: [executeViewMethod](../pkg/core/view.go#L61).

| Método | Retorno publicado al analizador |
|---|---|
| `exists` | `bool` |
| `render` | `string` |
| `share` | `mixed` |

## WebResponse

Implementación: [executeWebResponseMethod](../pkg/core/response.go#L132).

| Método | Retorno publicado al analizador |
|---|---|
| `status` | `WebResponse` |
| `with` | `WebResponse` |
| `withCookie` | `WebResponse` |
| `withHeader` | `WebResponse` |

## WebSocket

Implementación: [executeWebSocketMethod](../pkg/core/websocket.go#L19).

| Método | Retorno publicado al analizador |
|---|---|
| `broadcast` | `mixed` |
| `close` | `mixed` |
| `onClose` | `mixed` |
| `onMessage` | `mixed` |
| `publish` | `mixed` |
| `send` | `mixed` |
| `subscribe` | `mixed` |
| `subscriberCount` | `int` |
| `unsubscribe` | `mixed` |

## Zip

Implementación: [executeZipMethod](../pkg/core/native_zip.go#L17).

| Método | Retorno publicado al analizador |
|---|---|
| `extract` | `mixed` |

## Funciones globales

Contratos: [funciones globales](FUNCIONES_GLOBALES.md). Variantes que comparten implementación
se explican juntas allí; esta lista conserva todas las grafías registradas.

- `__`
- `abs`
- `all`
- `any`
- `append`
- `array_column`
- `array_key_exists`
- `array_keys`
- `array_merge`
- `array_pop`
- `array_push`
- `array_reverse`
- `array_shift`
- `array_slice`
- `array_unique`
- `array_values`
- `async`
- `await`
- `back`
- `base64_decode`
- `base64_encode`
- `boolval`
- `ceil`
- `cerr`
- `close`
- `config`
- `contains`
- `count`
- `cout`
- `csrf_field`
- `date`
- `decimal`
- `doubleval`
- `echo`
- `empty`
- `end`
- `ends_with`
- `env`
- `explode`
- `file_delete`
- `file_exists`
- `file_get_contents`
- `file_put_contents`
- `filter`
- `find`
- `floatval`
- `floor`
- `hive_read_box`
- `html_escape`
- `implode`
- `in_array`
- `intval`
- `is_array`
- `is_decimal`
- `is_dir`
- `is_double`
- `is_file`
- `is_float`
- `is_int`
- `is_integer`
- `is_null`
- `is_numeric`
- `is_string`
- `isset`
- `join`
- `json`
- `json_decode`
- `json_encode`
- `json_verify`
- `keys`
- `lcfirst`
- `len`
- `ltrim`
- `make_chan`
- `map`
- `max`
- `md5`
- `merge`
- `microtime`
- `min`
- `mkdir`
- `now`
- `print`
- `printf`
- `rand`
- `recv`
- `redirect`
- `reduce`
- `request`
- `response`
- `round`
- `rtrim`
- `run`
- `send`
- `session`
- `sha1`
- `sha256`
- `sleep`
- `starts_with`
- `str_contains`
- `str_ends_with`
- `str_pad`
- `str_repeat`
- `str_replace`
- `str_starts_with`
- `strlen`
- `strpos`
- `strtolower`
- `strtotime`
- `strtoupper`
- `strval`
- `substr`
- `sum`
- `time`
- `to_lower`
- `to_upper`
- `toon_decode`
- `toon_encode`
- `toon_verify`
- `trim`
- `ucfirst`
- `ucwords`
- `unlink`
- `usleep`
- `values`
- `view`

Total: 43 clases, 404 métodos y 126 built-ins.
