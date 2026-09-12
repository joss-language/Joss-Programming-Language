# Native catalog generated

Before: [library and contracts](MODULOS_NATIVOS.md). Index: [documentation](README.md).

Generated with `go run ./tools/docgen` from `Runtime.RegisterNativeClasses()`.
Do not edit manually. Each name is an entry in the actual registry. **Published return**
does not mean exhaustive return observed: the discrepancies are in the library.
Native parameters are not published in this registry; consult the tables
of contracts and the linked handler. `mixed` does not certify isolation or absence of faults.

##Auth

Implementation: [executeAuthMethod](../../pkg/core/auth.go#L14).

| Method | Posted return to parser |
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

##AuthLoginResult

Implementation: [executeAuthLoginResultMethod](../../pkg/core/auth_fluent.go#L18).

| Method | Posted return to parser |
|---|---|
| `onChallenge` | `AuthLoginResult` |
| `onFail` | `AuthLoginResult` |
| `onSuccess` | `AuthLoginResult` |
| `require2FA` | `AuthLoginResult` |
| `response` | `mixed` |

##Blueprint

Implementation: [executeBlueprintMethod](../../pkg/core/schema_blueprint.go#L34).

| Method | Posted return to parser |
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

Implementation: [executeCacheMethod](../../pkg/core/native_cache.go#L18).

| Method | Posted return to parser |
|---|---|
| `forget` | `mixed` |
| `get` | `mixed` |
| `has` | `bool` |
| `put` | `mixed` |

##Console

| Method | Posted return to parser |
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

##Cron

Implementation: [executeCronMethod](../../pkg/core/cron.go#L11).

| Method | Posted return to parser |
|---|---|
| `schedule` | `mixed` |

##Exception

Implementation: [executeExceptionMethod](../../pkg/core/native.go#L367).

| Method | Posted return to parser |
|---|---|
| `constructor` | `mixed` |
| `getCode` | `int` |
| `getMessage` | `string` |

## BigDB

Implementation: [executeGranDBMethod](../../pkg/core/database.go#L11).

| Method | Posted return to parser |
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

##Http

Implementation: [executeHttpMethod](../../pkg/core/http_client.go#L14).

| Method | Posted return to parser |
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

##JSON

| Method | Posted return to parser |
|---|---|
| `decode` | `mixed` |
| `encode` | `string` |
| `parse` | `mixed` |
| `stringify` | `string` |

##Lang

| Method | Posted return to parser |
|---|---|
| `get` | `mixed` |
| `locale` | `string` |
| `locales` | `array` |
| `set` | `mixed` |

## M.F.A.

Implementation: [executeMFAMethod](../../pkg/core/auth_fluent.go#L99).

| Method | Posted return to parser |
|---|---|
| `generateRecoveryCodes` | `mixed` |
| `generateTOTP` | `mixed` |
| `verifyRecoveryCode` | `bool` |
| `verifyTOTP` | `bool` |

##Markdown

| Method | Posted return to parser |
|---|---|
| `readFile` | `string` |
| `toHtml` | `string` |

## Math

| Method | Posted return to parser |
|---|---|
| `abs` | `float` |
| `ceil` | `float` |
| `floor` | `float` |
| `random` | `int` |

##Middleware

Registered base class without its own native methods.

## Migration

Registered base class without its own native methods.

##Plugin

Implementation: [executePluginMethod](../../pkg/core/native_plugin.go#L47).

| Method | Posted return to parser |
|---|---|
| `call` | `mixed` |
| `path` | `mixed` |
| `platform` | `mixed` |
| `stream` | `mixed` |

## Process

Implementation: [executeProcessMethod](../../pkg/core/native_process.go#L13).

| Method | Posted return to parser |
|---|---|
| `constructor` | `mixed` |
| `kill` | `mixed` |
| `pid` | `mixed` |
| `start` | `mixed` |
| `stderr_chan` | `mixed` |
| `stdin` | `mixed` |
| `stdout_chan` | `mixed` |
| `wait` | `mixed` |

##Queue

| Method | Posted return to parser |
|---|---|
| `dequeue` | `mixed` |
| `enqueue` | `mixed` |
| `peek` | `mixed` |

##Redirect

Implementation: [executeRedirectMethod](../../pkg/core/response.go#L177).

| Method | Posted return to parser |
|---|---|
| `to` | `WebResponse` |

##Redis

Implementation: [executeRedisMethod](../../pkg/core/redis.go#L96).

| Method | Posted return to parser |
|---|---|
| `connect` | `mixed` |
| `del` | `mixed` |
| `flush` | `mixed` |
| `forget` | `mixed` |
| `get` | `mixed` |
| `has` | `mixed` |
| `set` | `mixed` |
| `ttl` | `mixed` |

##Request

Implementation: [executeRequestMethod](../../pkg/core/request.go#L9).

| Method | Posted return to parser |
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

Implementation: [executeResponseMethod](../../pkg/core/response.go#L10).

| Method | Posted return to parser |
|---|---|
| `back` | `WebResponse` |
| `download` | `WebResponse` |
| `error` | `WebResponse` |
| `json` | `WebResponse` |
| `raw` | `WebResponse` |
| `redirect` | `WebResponse` |
| `stream` | `WebResponse` |

## Router

Implementation: [executeRouterMethod](../../pkg/core/router.go#L12).

| Method | Posted return to parser |
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

##SEO

Implementation: [executeSEOMethod](../../pkg/core/native_seo.go#L12).

| Method | Posted return to parser |
|---|---|
| `canonical` | `SEO` |
| `description` | `SEO` |
| `keywords` | `SEO` |
| `meta` | `SEO` |
| `og` | `SEO` |
| `render` | `string` |
| `title` | `SEO` |

## SQLite

Implementation: [executeSQLiteMethod](../../pkg/core/native_sqlite.go#L16).

| Method | Posted return to parser |
|---|---|
| `close` | `mixed` |
| `open` | `mixed` |
| `query` | `mixed` |

## Schema

Implementation: [executeSchemaMethod](../../pkg/core/schema.go#L119).

| Method | Posted return to parser |
|---|---|
| `create` | `mixed` |
| `drop` | `mixed` |
| `dropIfExists` | `mixed` |
| `hasColumn` | `bool` |
| `hasTable` | `bool` |
| `rename` | `mixed` |
| `table` | `mixed` |

## Server

Implementation: [executeServerControlMethod](../../pkg/core/native_server_control.go#L15).

| Method | Posted return to parser |
|---|---|
| `spawn` | `mixed` |
| `start` | `mixed` |

##Session

Implementation: [executeSessionMethod](../../pkg/core/native_extensions.go#L75).

| Method | Posted return to parser |
|---|---|
| `all` | `map` |
| `forget` | `mixed` |
| `get` | `mixed` |
| `has` | `bool` |
| `put` | `mixed` |

## Sitemap

Implementation: [executeSitemapMethod](../../pkg/core/native_seo.go#L127).

| Method | Posted return to parser |
|---|---|
| `add` | `Sitemap` |
| `exclude` | `Sitemap` |
| `generate` | `string` |
| `provider` | `Sitemap` |
| `xsl` | `Sitemap` |

##SmtpClient

Implementation: [executeSmtpClientMethod](../../pkg/core/smtp_native.go#L12).

| Method | Posted return to parser |
|---|---|
| `auth` | `SmtpClient` |
| `lastError` | `string\|null` |
| `secure` | `SmtpClient` |
| `send` | `bool` |
| `timeout` | `SmtpClient` |

##Stack

| Method | Posted return to parser |
|---|---|
| `peek` | `mixed` |
| `pop` | `mixed` |
| `push` | `mixed` |

##Str

| Method | Posted return to parser |
|---|---|
| `contains` | `bool` |
| `indexOf` | `int` |
| `length` | `int` |
| `random` | `string` |
| `replace` | `string` |
| `startsWith` | `bool` |
| `substring` | `string` |
| `trim` | `string` |

##Stream

Implementation: [executeStreamMethod](../../pkg/core/native_stream.go#L10).

| Method | Posted return to parser |
|---|---|
| `close` | `mixed` |
| `send` | `mixed` |

##System

Implementation: [executeSystemMethod](../../pkg/core/system.go#L14).

| Method | Posted return to parser |
|---|---|
| `Run` | `mixed` |
| `driver_call` | `mixed` |
| `env` | `mixed` |
| `load_driver` | `mixed` |
| `log` | `mixed` |
| `now` | `int` |
| `sleep` | `mixed` |

##Task

Implementation: [executeTaskMethod](../../pkg/core/task.go#L10).

| Method | Posted return to parser |
|---|---|
| `on_request` | `mixed` |

##TwoFactor

Implementation: [executeTwoFactorMethod](../../pkg/core/auth_fluent.go#L168).

| Method | Posted return to parser |
|---|---|
| `required` | `bool` |
| `verify` | `bool` |

## UUID

| Method | Posted return to parser |
|---|---|
| `generate` | `string` |
| `v4` | `string` |

##UserStorage

Implementation: [executeUserStorageMethod](../../pkg/core/lib_storage.go#L20).

| Method | Posted return to parser |
|---|---|
| `delete` | `mixed` |
| `get` | `mixed` |
| `getToFile` | `mixed` |
| `path` | `mixed` |
| `put` | `mixed` |

##View

Implementation: [executeViewMethod](../../pkg/core/view.go#L61).

| Method | Posted return to parser |
|---|---|
| `exists` | `bool` |
| `render` | `string` |
| `share` | `mixed` |

## WebResponse

Implementation: [executeWebResponseMethod](../../pkg/core/response.go#L132).

| Method | Posted return to parser |
|---|---|
| `status` | `WebResponse` |
| `with` | `WebResponse` |
| `withCookie` | `WebResponse` |
| `withHeader` | `WebResponse` |

##WebSocket

Implementation: [executeWebSocketMethod](../../pkg/core/websocket.go#L60).

| Method | Posted return to parser |
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

| Method | Posted return to parser |
|---|---|
| `extract` | `bool` |

## Global functions

Contracts: [global functions](FUNCIONES_GLOBALES.md). Variants that share implementation
they are explained together there; This list preserves all registered spellings.

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

Total: 43 classes, 404 methods and 126 built-ins.
