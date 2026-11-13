# Catálogo nativo generado

Antes: [biblioteca y contratos](MODULOS_NATIVOS.md). Índice: [documentación](README.md).

Generado con `go run ./tools/docgen` desde `Runtime.RegisterNativeClasses()`.
No editar manualmente. Cada nombre es una entrada del registro real. **Retorno publicado**
no significa retorno exhaustivo observado: las discrepancias están en la biblioteca.
Los parámetros nativos no están publicados en este registro; consulta las tablas
de contratos y el handler enlazado. `mixed` no certifica aislamiento ni ausencia de fallos.

## Auth

Implementación: [executeAuthMethod](../pkg/core/auth.go#L16).

| Método | Retorno publicado al analizador |
|---|---|
| `attempt` | `mixed` |
| `challengeToken` | `string` |
| `check` | `bool` |
| `claimPasswordReset` | `mixed` |
| `complete2FA` | `mixed` |
| `create` | `mixed` |
| `databaseReady` | `bool` |
| `delete` | `mixed` |
| `enabledSocialProviders` | `array` |
| `forgotPassword` | `mixed` |
| `guest` | `bool` |
| `hasRole` | `bool` |
| `hash` | `mixed` |
| `id` | `mixed` |
| `issueToken` | `string` |
| `login` | `mixed` |
| `logout` | `mixed` |
| `prepareLogin2FA` | `map` |
| `refresh` | `mixed` |
| `resendVerification` | `mixed` |
| `resetPassword` | `mixed` |
| `socialCallback` | `mixed` |
| `socialRedirect` | `string` |
| `update` | `mixed` |
| `user` | `mixed` |
| `validateToken` | `mixed` |
| `verificationStatus` | `mixed` |
| `verify` | `bool` |
| `verify2FAChallenge` | `mixed` |
| `verifyHash` | `bool` |

## AuthLoginResult

Implementación: [executeAuthLoginResultMethod](../pkg/core/auth_fluent.go#L19).

| Método | Retorno publicado al analizador |
|---|---|
| `onChallenge` | `AuthLoginResult` |
| `onFail` | `AuthLoginResult` |
| `onSuccess` | `AuthLoginResult` |
| `response` | `mixed` |

## Blueprint

Implementación: [executeBlueprintMethod](../pkg/core/schema_blueprint.go#L41).

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
| `dropForeign` | `Blueprint` |
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

## DateInterval

| Método | Retorno publicado al analizador |
|---|---|
| `days` | `DateInterval` |
| `hours` | `DateInterval` |
| `minutes` | `DateInterval` |
| `seconds` | `DateInterval` |
| `totalSeconds` | `int` |

## DateTime

| Método | Retorno publicado al analizador |
|---|---|
| `add` | `DateTime` |
| `create` | `DateTime` |
| `day` | `int` |
| `diff` | `DateInterval` |
| `format` | `string` |
| `hour` | `int` |
| `iso` | `string` |
| `minute` | `int` |
| `month` | `int` |
| `now` | `DateTime` |
| `parse` | `DateTime` |
| `second` | `int` |
| `sub` | `DateTime` |
| `timestamp` | `int` |
| `year` | `int` |

## Exception

Implementación: [executeExceptionMethod](../pkg/core/native.go#L385).

| Método | Retorno publicado al analizador |
|---|---|
| `constructor` | `mixed` |
| `getCode` | `int` |
| `getMessage` | `string` |

## FileStream

| Método | Retorno publicado al analizador |
|---|---|
| `close` | `void` |
| `flush` | `void` |
| `open` | `FileStream` |
| `read` | `string` |
| `seek` | `int` |
| `size` | `int` |
| `write` | `int` |

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

## IndexNow

Implementación: [executeIndexNowMethod](../pkg/core/native_indexnow.go#L158).

| Método | Retorno publicado al analizador |
|---|---|
| `enabled` | `bool` |
| `key` | `string` |
| `keyLocation` | `string` |
| `submit` | `bool` |

## JSON

| Método | Retorno publicado al analizador |
|---|---|
| `decode` | `mixed` |
| `encode` | `string` |
| `parse` | `mixed` |
| `stringify` | `string` |

## Lang

| Método | Retorno publicado al analizador |
|---|---|
| `get` | `mixed` |
| `locale` | `string` |
| `locales` | `array` |
| `set` | `bool` |

## MFA

Implementación: [executeMFAMethod](../pkg/core/auth_fluent.go#L78).

| Método | Retorno publicado al analizador |
|---|---|
| `generateRecoveryCodes` | `mixed` |
| `generateTOTP` | `mixed` |
| `verifyRecoveryCode` | `bool` |
| `verifyTOTP` | `bool` |

## Markdown

| Método | Retorno publicado al analizador |
|---|---|
| `readFile` | `string` |
| `toHtml` | `string` |

## Math

| Método | Retorno publicado al analizador |
|---|---|
| `abs` | `mixed` |
| `ceil` | `float` |
| `floor` | `float` |
| `random` | `int` |

## Middleware

Clase base registrada sin métodos nativos propios.

## Migration

Clase base registrada sin métodos nativos propios.

## Model

Implementación: [executeModelMethod](../pkg/core/model.go#L433).

| Método | Retorno publicado al analizador |
|---|---|
| `all` | `mixed` |
| `attach` | `mixed` |
| `avg` | `mixed` |
| `belongsTo` | `mixed` |
| `belongsToMany` | `mixed` |
| `changeDB` | `mixed` |
| `changedb` | `mixed` |
| `chunk` | `mixed` |
| `chunkById` | `mixed` |
| `chunkbyid` | `mixed` |
| `connection` | `mixed` |
| `count` | `mixed` |
| `create` | `mixed` |
| `crossJoin` | `mixed` |
| `crossjoin` | `mixed` |
| `cursorPaginate` | `mixed` |
| `cursorpaginate` | `mixed` |
| `dd` | `mixed` |
| `decrement` | `mixed` |
| `delete` | `mixed` |
| `deleteAll` | `mixed` |
| `detach` | `mixed` |
| `distinct` | `mixed` |
| `doesntExist` | `mixed` |
| `dump` | `mixed` |
| `exists` | `mixed` |
| `explain` | `mixed` |
| `fill` | `mixed` |
| `find` | `mixed` |
| `findMany` | `mixed` |
| `findOrFail` | `mixed` |
| `findmany` | `mixed` |
| `findorfail` | `mixed` |
| `first` | `mixed` |
| `firstOrCreate` | `mixed` |
| `firstOrFail` | `mixed` |
| `firstOrNew` | `mixed` |
| `firstWhere` | `mixed` |
| `firstofail` | `mixed` |
| `firstwhere` | `mixed` |
| `forPage` | `mixed` |
| `forceDelete` | `mixed` |
| `forceFill` | `mixed` |
| `forpage` | `mixed` |
| `get` | `mixed` |
| `getBindings` | `mixed` |
| `getChanges` | `mixed` |
| `getOriginal` | `mixed` |
| `getbindings` | `mixed` |
| `groupBy` | `mixed` |
| `groupby` | `mixed` |
| `hasMany` | `mixed` |
| `hasOne` | `mixed` |
| `having` | `mixed` |
| `inRandomOrder` | `mixed` |
| `increment` | `mixed` |
| `innerJoin` | `mixed` |
| `insert` | `mixed` |
| `insertGetId` | `mixed` |
| `insertMany` | `mixed` |
| `insertgetid` | `mixed` |
| `insertmany` | `mixed` |
| `isClean` | `mixed` |
| `isDirty` | `mixed` |
| `join` | `mixed` |
| `latest` | `mixed` |
| `leftJoin` | `mixed` |
| `limit` | `mixed` |
| `load` | `mixed` |
| `loadMissing` | `mixed` |
| `max` | `mixed` |
| `min` | `mixed` |
| `offset` | `mixed` |
| `oldest` | `mixed` |
| `onlyTrashed` | `mixed` |
| `orHaving` | `mixed` |
| `orWhere` | `mixed` |
| `orWhereBetween` | `mixed` |
| `orWhereColumn` | `mixed` |
| `orWhereDate` | `mixed` |
| `orWhereDay` | `mixed` |
| `orWhereIn` | `mixed` |
| `orWhereJsonContains` | `mixed` |
| `orWhereLike` | `mixed` |
| `orWhereMonth` | `mixed` |
| `orWhereNot` | `mixed` |
| `orWhereNotBetween` | `mixed` |
| `orWhereNotIn` | `mixed` |
| `orWhereNotNull` | `mixed` |
| `orWhereNull` | `mixed` |
| `orWhereTime` | `mixed` |
| `orWhereYear` | `mixed` |
| `orderBy` | `mixed` |
| `orderByAsc` | `mixed` |
| `orderByDesc` | `mixed` |
| `orderby` | `mixed` |
| `orderbyasc` | `mixed` |
| `orderbydesc` | `mixed` |
| `orhaving` | `mixed` |
| `orwhere` | `mixed` |
| `orwherebetween` | `mixed` |
| `orwherecolumn` | `mixed` |
| `orwheredate` | `mixed` |
| `orwhereday` | `mixed` |
| `orwherein` | `mixed` |
| `orwherejsoncontains` | `mixed` |
| `orwherelike` | `mixed` |
| `orwheremonth` | `mixed` |
| `orwherenot` | `mixed` |
| `orwherenotbetween` | `mixed` |
| `orwherenotin` | `mixed` |
| `orwherenotnull` | `mixed` |
| `orwherenull` | `mixed` |
| `orwheretime` | `mixed` |
| `orwhereyear` | `mixed` |
| `paginate` | `mixed` |
| `pluck` | `mixed` |
| `query` | `mixed` |
| `refresh` | `mixed` |
| `reorder` | `mixed` |
| `restore` | `mixed` |
| `rightJoin` | `mixed` |
| `save` | `mixed` |
| `scope` | `mixed` |
| `select` | `mixed` |
| `simplePaginate` | `mixed` |
| `simplepaginate` | `mixed` |
| `skip` | `mixed` |
| `sole` | `mixed` |
| `sum` | `mixed` |
| `sync` | `mixed` |
| `table` | `mixed` |
| `take` | `mixed` |
| `toJSON` | `mixed` |
| `toMap` | `mixed` |
| `toSql` | `mixed` |
| `tosql` | `mixed` |
| `touch` | `mixed` |
| `transaction` | `mixed` |
| `truncate` | `mixed` |
| `unless` | `mixed` |
| `update` | `mixed` |
| `updateOrCreate` | `mixed` |
| `updateOrInsert` | `mixed` |
| `updateorinsert` | `mixed` |
| `upsert` | `mixed` |
| `use` | `mixed` |
| `value` | `mixed` |
| `wasChanged` | `mixed` |
| `when` | `mixed` |
| `where` | `mixed` |
| `whereBetween` | `mixed` |
| `whereColumn` | `mixed` |
| `whereDate` | `mixed` |
| `whereDay` | `mixed` |
| `whereIn` | `mixed` |
| `whereJsonContains` | `mixed` |
| `whereLike` | `mixed` |
| `whereMonth` | `mixed` |
| `whereNot` | `mixed` |
| `whereNotBetween` | `mixed` |
| `whereNotIn` | `mixed` |
| `whereNotNull` | `mixed` |
| `whereNull` | `mixed` |
| `whereTime` | `mixed` |
| `whereYear` | `mixed` |
| `wherebetween` | `mixed` |
| `wherecolumn` | `mixed` |
| `wheredate` | `mixed` |
| `whereday` | `mixed` |
| `wherein` | `mixed` |
| `wherejsoncontains` | `mixed` |
| `wherelike` | `mixed` |
| `wheremonth` | `mixed` |
| `wherenot` | `mixed` |
| `wherenotbetween` | `mixed` |
| `wherenotin` | `mixed` |
| `wherenotnull` | `mixed` |
| `wherenull` | `mixed` |
| `wheretime` | `mixed` |
| `whereyear` | `mixed` |
| `with` | `mixed` |
| `withTrashed` | `mixed` |
| `withoutGlobalScope` | `mixed` |
| `withoutGlobalScopes` | `mixed` |
| `withoutTrashed` | `mixed` |

## Mutex

| Método | Retorno publicado al analizador |
|---|---|
| `lock` | `void` |
| `tryLock` | `bool` |
| `unlock` | `void` |

## Plugin

Implementación: [executePluginMethod](../pkg/core/native_plugin.go#L17).

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

## RWMutex

| Método | Retorno publicado al analizador |
|---|---|
| `lock` | `void` |
| `rLock` | `void` |
| `rUnlock` | `void` |
| `unlock` | `void` |

## Redirect

Implementación: [executeRedirectMethod](../pkg/core/response.go#L182).

| Método | Retorno publicado al analizador |
|---|---|
| `to` | `WebResponse` |

## Redis

Implementación: [executeRedisMethod](../pkg/core/redis_stub.go#L16).

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
| `uri` | `string` |
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

Implementación: [executeSEOMethod](../pkg/core/native_seo.go#L14).

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

Implementación: [executeSQLiteMethod](../pkg/core/native_sqlite.go#L15).

| Método | Retorno publicado al analizador |
|---|---|
| `close` | `mixed` |
| `open` | `mixed` |
| `query` | `mixed` |

## Schema

Implementación: [executeSchemaMethod](../pkg/core/schema.go#L125).

| Método | Retorno publicado al analizador |
|---|---|
| `create` | `mixed` |
| `drop` | `mixed` |
| `dropIfExists` | `mixed` |
| `hasColumn` | `bool` |
| `hasForeign` | `bool` |
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

Implementación: [executeSessionMethod](../pkg/core/native_extensions.go#L78).

| Método | Retorno publicado al analizador |
|---|---|
| `all` | `map` |
| `forget` | `void` |
| `get` | `mixed` |
| `has` | `bool` |
| `put` | `void` |

## Sitemap

Implementación: [executeSitemapMethod](../pkg/core/native_seo.go#L129).

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

## Socket

| Método | Retorno publicado al analizador |
|---|---|
| `accept` | `Socket` |
| `close` | `void` |
| `connect` | `Socket` |
| `listen` | `Socket` |
| `port` | `int` |
| `receive` | `string` |
| `send` | `int` |
| `tcp` | `Socket` |

## Stack

| Método | Retorno publicado al analizador |
|---|---|
| `peek` | `mixed` |
| `pop` | `mixed` |
| `push` | `mixed` |

## Str

| Método | Retorno publicado al analizador |
|---|---|
| `contains` | `bool` |
| `endsWith` | `bool` |
| `indexOf` | `int` |
| `length` | `int` |
| `lower` | `string` |
| `random` | `string` |
| `replace` | `string` |
| `startsWith` | `bool` |
| `substr` | `string` |
| `substring` | `string` |
| `trim` | `string` |

## Stream

Implementación: [executeStreamMethod](../pkg/core/native_stream.go#L311).

| Método | Retorno publicado al analizador |
|---|---|
| `close` | `mixed` |
| `send` | `mixed` |

## StreamReader

| Método | Retorno publicado al analizador |
|---|---|
| `close` | `void` |
| `open` | `StreamReader` |
| `readLine` | `string\|null` |
| `readToEnd` | `string` |

## StreamWriter

| Método | Retorno publicado al analizador |
|---|---|
| `close` | `void` |
| `flush` | `void` |
| `open` | `StreamWriter` |
| `write` | `int` |
| `writeLine` | `int` |

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

Implementación: [executeTwoFactorMethod](../pkg/core/auth_fluent.go#L147).

| Método | Retorno publicado al analizador |
|---|---|
| `required` | `bool` |
| `verify` | `bool` |

## UUID

| Método | Retorno publicado al analizador |
|---|---|
| `generate` | `string` |
| `v4` | `string` |

## View

Implementación: [executeViewMethod](../pkg/core/view.go#L25).

| Método | Retorno publicado al analizador |
|---|---|
| `exists` | `bool` |
| `render` | `string` |
| `share` | `mixed` |

## WaitGroup

| Método | Retorno publicado al analizador |
|---|---|
| `add` | `void` |
| `done` | `void` |
| `wait` | `void` |

## WebResponse

Implementación: [executeWebResponseMethod](../pkg/core/response.go#L137).

| Método | Retorno publicado al analizador |
|---|---|
| `status` | `WebResponse` |
| `with` | `WebResponse` |
| `withCookie` | `WebResponse` |
| `withHeader` | `WebResponse` |

## WebSocket

Implementación: [executeWebSocketMethod](../pkg/core/websocket.go#L60).

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

## Window

| Método | Retorno publicado al analizador |
|---|---|
| `close` | `void` |
| `hide` | `void` |
| `on` | `void` |
| `open` | `Window` |
| `pollEvents` | `bool` |
| `render` | `void` |
| `setSize` | `void` |
| `setTitle` | `void` |
| `show` | `void` |

## Zip

| Método | Retorno publicado al analizador |
|---|---|
| `extract` | `bool` |

## Funciones globales

Contratos: [funciones globales](FUNCIONES_GLOBALES.md). Variantes que comparten implementación
se explican juntas allí; esta lista conserva todas las grafías registradas.

- `__`
- `__joss_byte_string`
- `__joss_db_execute`
- `__joss_db_get_table`
- `__joss_db_is_connected`
- `__joss_db_query`
- `__joss_db_set_table`
- `__joss_file_get_bytes`
- `__joss_get_embedded_asset`
- `__joss_http_dispatch`
- `__joss_local_utc_offset`
- `__joss_storage_delete`
- `__joss_storage_get`
- `__joss_storage_put`
- `__joss_string_byte_at`
- `__joss_timezone_data`
- `abs`
- `all`
- `any`
- `append`
- `array_column`
- `array_fill`
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
- `chr`
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
- `gmdate`
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
- `ord`
- `print`
- `printf`
- `rand`
- `rawurldecode`
- `rawurlencode`
- `recv`
- `redirect`
- `reduce`
- `request`
- `response`
- `round`
- `rsa_sign_sha256`
- `rtrim`
- `run`
- `send`
- `session`
- `sha1`
- `sha1_base64`
- `sha256`
- `sha256_base64`
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
- `urldecode`
- `urlencode`
- `usleep`
- `values`
- `view`

Total: 53 clases, 517 métodos y 147 built-ins.
