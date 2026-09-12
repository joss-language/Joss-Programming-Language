# Catálogo nativo gerado

Antes: [biblioteca e contratos](MODULOS_NATIVOS.md). Índice: [documentação](README.md).

Gerado com `go run ./tools/docgen` de `Runtime.RegisterNativeClasses()`.
Não edite manualmente. Cada nome é uma entrada no registro real. **Retorno publicado**
não significa retorno exaustivo observado: as discrepâncias estão na biblioteca.
Os parâmetros nativos não são publicados neste registro; consulte as tabelas
de contratos e o manipulador vinculado. `mixed` não certifica isolamento ou ausência de falhas.

##Autorização

Implementação: [executeAuthMethod](../../pkg/core/auth.go#L14).

| Método | Publicado retorno ao analisador |
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

Implementação: [executeAuthLoginResultMethod](../../pkg/core/auth_fluent.go#L18).

| Método | Publicado retorno ao analisador |
|---|---|
| `onChallenge` | `AuthLoginResult` |
| `onFail` | `AuthLoginResult` |
| `onSuccess` | `AuthLoginResult` |
| `require2FA` | `AuthLoginResult` |
| `response` | `mixed` |

##Projeto

Implementação: [executeBlueprintMethod](../../pkg/core/schema_blueprint.go#L34).

| Método | Publicado retorno ao analisador |
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

Implementação: [executeCacheMethod](../../pkg/core/native_cache.go#L18).

| Método | Publicado retorno ao analisador |
|---|---|
| `forget` | `mixed` |
| `get` | `mixed` |
| `has` | `bool` |
| `put` | `mixed` |

##Console

| Método | Publicado retorno ao analisador |
|---|---|
| `blue` | `string` || `bold` | `string` |
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

Implementação: [executeCronMethod](../../pkg/core/cron.go#L11).

| Método | Publicado retorno ao analisador |
|---|---|
| `schedule` | `mixed` |

##Exceção

Implementação: [executeExceptionMethod](../../pkg/core/native.go#L367).

| Método | Publicado retorno ao analisador |
|---|---|
| `constructor` | `mixed` |
| `getCode` | `int` |
| `getMessage` | `string` |

## BigDB

Implementação: [executeGranDBMethod](../../pkg/core/database.go#L11).

| Método | Publicado retorno ao analisador |
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
| `rightJoin` | `GranDB` || `select` | `GranDB` |
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

##http

Implementação: [executeHttpMethod](../../pkg/core/http_client.go#L14).

| Método | Publicado retorno ao analisador |
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

| Método | Publicado retorno ao analisador |
|---|---|
| `decode` | `mixed` |
| `encode` | `string` |
| `parse` | `mixed` |
| `stringify` | `string` |

##Lang

| Método | Publicado retorno ao analisador |
|---|---|
| `get` | `mixed` |
| `locale` | `string` |
| `locales` | `array` |
| `set` | `mixed` |

## MFA

Implementação: [executeMFAMethod](../../pkg/core/auth_fluent.go#L99).

| Método | Publicado retorno ao analisador |
|---|---|
| `generateRecoveryCodes` | `mixed` |
| `generateTOTP` | `mixed` |
| `verifyRecoveryCode` | `bool` |
| `verifyTOTP` | `bool` |

##Redução

| Método | Publicado retorno ao analisador |
|---|---|
| `readFile` | `string` |
| `toHtml` | `string` |

## Matemática

| Método | Publicado retorno ao analisador |
|---|---|
| `abs` | `float` |
| `ceil` | `float` |
| `floor` | `float` |
| `random` | `int` |

##Middleware

Classe base registrada sem seus próprios métodos nativos.

## Migração

Classe base registrada sem seus próprios métodos nativos.

##Plug-in

Implementação: [executePluginMethod](../../pkg/core/native_plugin.go#L47).

| Método | Publicado retorno ao analisador |
|---|---|
| `call` | `mixed` |
| `path` | `mixed` |
| `platform` | `mixed` |
| `stream` | `mixed` |

## Processo

Implementação: [executeProcessMethod](../../pkg/core/native_process.go#L13).

| Método | Publicado retorno ao analisador |
|---|---|
| `constructor` | `mixed` |
| `kill` | `mixed` |
| `pid` | `mixed` |
| `start` | `mixed` |
| `stderr_chan` | `mixed` |
| `stdin` | `mixed` |
| `stdout_chan` | `mixed` |
| `wait` | `mixed` |##Fila

| Método | Publicado retorno ao analisador |
|---|---|
| `dequeue` | `mixed` |
| `enqueue` | `mixed` |
| `peek` | `mixed` |

##Redirecionar

Implementação: [executeRedirectMethod](../../pkg/core/response.go#L177).

| Método | Publicado retorno ao analisador |
|---|---|
| `to` | `WebResponse` |

##Redis

Implementação: [executeRedisMethod](../../pkg/core/redis.go#L96).

| Método | Publicado retorno ao analisador |
|---|---|
| `connect` | `mixed` |
| `del` | `mixed` |
| `flush` | `mixed` |
| `forget` | `mixed` |
| `get` | `mixed` |
| `has` | `mixed` |
| `set` | `mixed` |
| `ttl` | `mixed` |

##Solicitar

Implementação: [executeRequestMethod](../../pkg/core/request.go#L9).

| Método | Publicado retorno ao analisador |
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

## Resposta

Implementação: [executeResponseMethod](../../pkg/core/response.go#L10).

| Método | Publicado retorno ao analisador |
|---|---|
| `back` | `WebResponse` |
| `download` | `WebResponse` |
| `error` | `WebResponse` |
| `json` | `WebResponse` |
| `raw` | `WebResponse` |
| `redirect` | `WebResponse` |
| `stream` | `WebResponse` |

## Roteador

Implementação: [executeRouterMethod](../../pkg/core/router.go#L12).

| Método | Publicado retorno ao analisador |
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

Implementação: [executeSEOMethod](../../pkg/core/native_seo.go#L12).

| Método | Publicado retorno ao analisador |
|---|---|
| `canonical` | `SEO` |
| `description` | `SEO` |
| `keywords` | `SEO` |
| `meta` | `SEO` |
| `og` | `SEO` |
| `render` | `string` |
| `title` | `SEO` |

##SQLite

Implementação: [executeSQLiteMethod](../../pkg/core/native_sqlite.go#L16).

| Método | Publicado retorno ao analisador |
|---|---|
| `close` | `mixed` |
| `open` | `mixed` |
| `query` | `mixed` |

## Esquema

Implementação: [executeSchemaMethod](../../pkg/core/schema.go#L119).

| Método | Publicado retorno ao analisador |
|---|---|
| `create` | `mixed` |
| `drop` | `mixed` |
| `dropIfExists` | `mixed` |
| `hasColumn` | `bool` |
| `hasTable` | `bool` |
| `rename` | `mixed` |
| `table` | `mixed` |

## Servidor

Implementação: [executeServerControlMethod](../../pkg/core/native_server_control.go#L15).

| Método | Publicado retorno ao analisador |
|---|---|
| `spawn` | `mixed` |
| `start` | `mixed` |

##Sessão

Implementação: [executeSessionMethod](../../pkg/core/native_extensions.go#L75).

| Método | Publicado retorno ao analisador ||---|---|
| `all` | `map` |
| `forget` | `mixed` |
| `get` | `mixed` |
| `has` | `bool` |
| `put` | `mixed` |

## Mapa do site

Implementação: [executeSitemapMethod](../../pkg/core/native_seo.go#L127).

| Método | Publicado retorno ao analisador |
|---|---|
| `add` | `Sitemap` |
| `exclude` | `Sitemap` |
| `generate` | `string` |
| `provider` | `Sitemap` |
| `xsl` | `Sitemap` |

##SmtpCliente

Implementação: [executeSmtpClientMethod](../../pkg/core/smtp_native.go#L12).

| Método | Publicado retorno ao analisador |
|---|---|
| `auth` | `SmtpClient` |
| `lastError` | `string\|null` |
| `secure` | `SmtpClient` |
| `send` | `bool` |
| `timeout` | `SmtpClient` |

##Pilha

| Método | Publicado retorno ao analisador |
|---|---|
| `peek` | `mixed` |
| `pop` | `mixed` |
| `push` | `mixed` |

##Str

| Método | Publicado retorno ao analisador |
|---|---|
| `contains` | `bool` |
| `indexOf` | `int` |
| `length` | `int` |
| `random` | `string` |
| `replace` | `string` |
| `startsWith` | `bool` |
| `substring` | `string` |
| `trim` | `string` |

##Transmitir

Implementação: [executeStreamMethod](../../pkg/core/native_stream.go#L10).

| Método | Publicado retorno ao analisador |
|---|---|
| `close` | `mixed` |
| `send` | `mixed` |

##Sistema

Implementação: [executeSystemMethod](../../pkg/core/system.go#L14).

| Método | Publicado retorno ao analisador |
|---|---|
| `Run` | `mixed` |
| `driver_call` | `mixed` |
| `env` | `mixed` |
| `load_driver` | `mixed` |
| `log` | `mixed` |
| `now` | `int` |
| `sleep` | `mixed` |

##Tarefa

Implementação: [executeTaskMethod](../../pkg/core/task.go#L10).

| Método | Publicado retorno ao analisador |
|---|---|
| `on_request` | `mixed` |

##DoisFatores

Implementação: [executeTwoFactorMethod](../../pkg/core/auth_fluent.go#L168).

| Método | Publicado retorno ao analisador |
|---|---|
| `required` | `bool` |
| `verify` | `bool` |

##UUID

| Método | Publicado retorno ao analisador |
|---|---|
| `generate` | `string` |
| `v4` | `string` |

##Armazenamento do usuário

Implementação: [executeUserStorageMethod](../../pkg/core/lib_storage.go#L20).

| Método | Publicado retorno ao analisador |
|---|---|
| `delete` | `mixed` |
| `get` | `mixed` |
| `getToFile` | `mixed` |
| `path` | `mixed` |
| `put` | `mixed` |

##Ver

Implementação: [executeViewMethod](../../pkg/core/view.go#L61).

| Método | Publicado retorno ao analisador |
|---|---|
| `exists` | `bool` |
| `render` | `string` |
| `share` | `mixed` |

## WebResponse

Implementação: [executeWebResponseMethod](../../pkg/core/response.go#L132).

| Método | Publicado retorno ao analisador |
|---|---|
| `status` | `WebResponse` |
| `with` | `WebResponse` |
| `withCookie` | `WebResponse` |
| `withHeader` | `WebResponse` |

##WebSocket

Implementação: [executeWebSocketMethod](../../pkg/core/websocket.go#L60).

| Método | Publicado retorno ao analisador |
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

## CEP

| Método | Publicado retorno ao analisador |
|---|---|
| `extract` | `bool` |

## Funções globaisContratos: [funções globais](FUNCIONES_GLOBALES.md). Variantes que compartilham implementação
eles são explicados juntos lá; Esta lista preserva todas as grafias registradas.

-`__`
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

Total: 43 classes, 404 métodos e 126 built-ins.