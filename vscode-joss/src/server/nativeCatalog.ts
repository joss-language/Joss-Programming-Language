import { ParameterInfo } from './languageSymbols';
import languageCatalog from './generated/languageCatalog.json';

const generatedNativeClasses = languageCatalog.nativeClasses as Record<string, string[]>;

export interface NativeCallable {
    name: string;
    owner?: string;
    parameters: ParameterInfo[];
    returnType?: string;
    documentation: string;
    static?: boolean;
}

function parameter(label: string, documentation?: string): ParameterInfo {
    const optional = label.includes('=');
    const clean = label.split('=')[0].trim();
    const pieces = clean.split(/\s+/);
    const rawName = pieces[pieces.length - 1];
    return {
        name: rawName.replace(/^\$/, ''),
        label,
        type: pieces.length > 1 ? pieces.slice(0, -1).join(' ') : undefined,
        optional,
        defaultValue: optional ? label.substring(label.indexOf('=') + 1).trim() : undefined,
        documentation
    };
}

function callable(owner: string | undefined, name: string, params: string[], returnType: string, documentation: string, isStatic = true): NativeCallable {
    return { owner, name, parameters: params.map(value => parameter(value)), returnType, documentation, static: isStatic };
}

export const nativeCallables: NativeCallable[] = [
    callable(undefined, 'print', ['$value'], 'nil', 'Escribe un valor en la salida estándar.'),
    callable(undefined, 'echo', ['$value'], 'nil', 'Alias de print.'),
    callable(undefined, 'empty', ['$value'], 'bool', 'Indica si un valor está vacío.'),
    callable(undefined, 'isset', ['$value'], 'bool', 'Indica si un valor existe y no es nil.'),
    callable(undefined, 'count', ['$value'], 'int', 'Devuelve el número de elementos.'),
    callable(undefined, 'await', ['$future'], 'any', 'Espera el resultado de una operación async.'),

    callable('Router', 'get', ['string $path', '$handler'], 'nil', 'Registra una ruta GET.'),
    callable('Router', 'post', ['string $path', '$handler'], 'nil', 'Registra una ruta POST.'),
    callable('Router', 'put', ['string $path', '$handler'], 'nil', 'Registra una ruta PUT.'),
    callable('Router', 'delete', ['string $path', '$handler'], 'nil', 'Registra una ruta DELETE.'),
    callable('Router', 'match', ['string $methods', 'string $path', '$handler'], 'nil', 'Registra varios métodos HTTP para una ruta.'),
    callable('Router', 'ws', ['string $path', '$handler'], 'nil', 'Registra una ruta WebSocket estática; los parámetros deben viajar por query o mensaje inicial.'),
    callable('Router', 'group', ['string $middleware', 'func $callback'], 'nil', 'Ejecuta un grupo de rutas bajo un middleware.'),
    callable('Router', 'middleware', ['string $name'], 'nil', 'Abre un grupo de middleware que se cierra con Router::end().'),
    callable('Router', 'registerMiddleware', ['string $name', 'func $handler'], 'nil', 'Registra un middleware personalizado.'),
    callable('Router', 'api', ['string $path', '$handler'], 'nil', 'Registra la misma ruta para GET y POST.'),
    callable('Router', 'end', [], 'nil', 'Finaliza el grupo activo.'),

    callable('Auth', 'check', [], 'bool', 'Indica si existe un usuario autenticado.'),
    callable('Auth', 'guest', [], 'bool', 'Indica si la petición es anónima.'),
    callable('Auth', 'user', [], 'User', 'Devuelve la instancia del usuario autenticado.'),
    callable('Auth', 'id', [], 'int|nil', 'Devuelve el identificador del usuario autenticado.'),
    callable('Auth', 'attempt', ['string $email', 'string $password'], 'bool|AuthLoginResult', 'Intenta autenticar credenciales.'),
    callable('Auth', 'hash', ['$password'], 'string|nil', 'Genera un hash bcrypt con el coste predeterminado.'),
    callable('Auth', 'login', ['string $email', 'string $password'], 'AuthLoginResult|nil', 'Inicia el flujo fluido de autenticación.'),
    callable('Auth', 'create', ['map $data'], 'string|bool', 'Crea un usuario y devuelve su token de verificación.'),
    callable('Auth', 'complete2FA', ['$userId'], 'string|nil', 'Genera el JWT final después del segundo factor.'),
    callable('Auth', 'verify', ['string $token'], 'bool', 'Verifica una cuenta mediante su token.'),
    callable('Auth', 'forgotPassword', ['string $email'], 'string|bool', 'Crea un token de restablecimiento.'),
    callable('Auth', 'resetPassword', ['string $token', 'string $newPassword'], 'bool|string', 'Restablece la contraseña y puede devolver un código de error.'),
    callable('Auth', 'resendVerification', ['string $email'], 'string|bool', 'Renueva el token de verificación.'),
    callable('Auth', 'hasRole', ['string $role'], 'bool', 'Comprueba el rol del usuario.'),
    callable('Auth', 'refresh', ['$userId'], 'string|bool', 'Genera un JWT nuevo para el usuario.'),
    callable('Auth', 'update', ['$userId', 'map $data'], 'bool|nil', 'Actualiza campos permitidos del usuario.'),
    callable('Auth', 'delete', ['$userId'], 'bool|nil', 'Elimina el usuario.'),
    callable('Auth', 'logout', [], 'bool', 'Limpia la autenticación del runtime.'),
    callable('Auth', 'validateToken', ['string $token'], 'bool', 'Valida un JWT y restaura la sesión.'),
    callable('AuthLoginResult', 'require2FA', [], 'AuthLoginResult', 'Marca el resultado si el usuario tiene MFA activo.', false),
    callable('AuthLoginResult', 'onSuccess', ['func $callback'], 'AuthLoginResult', 'Ejecuta el callback cuando el login termina sin reto.', false),
    callable('AuthLoginResult', 'onChallenge', ['func $callback'], 'AuthLoginResult', 'Ejecuta el callback cuando se requiere segundo factor.', false),
    callable('AuthLoginResult', 'onFail', ['func $callback'], 'AuthLoginResult', 'Ejecuta el callback cuando el login falla.', false),
    callable('AuthLoginResult', 'response', [], 'any', 'Devuelve la respuesta producida por el flujo.', false),
    callable('TwoFactor', 'required', ['$user'], 'bool', 'Comprueba si el usuario tiene un método MFA activo.'),
    callable('TwoFactor', 'verify', ['$userId', 'string $code'], 'bool', 'Verifica el código TOTP activo del usuario.'),

    callable('Request', 'input', ['string $key', '$default = nil'], 'any', 'Lee un valor de la petición.'),
    callable('Request', 'all', [], 'map', 'Devuelve todos los datos de la petición.'),
    callable('Request', 'except', ['array $keys'], 'map', 'Devuelve los datos excepto las claves indicadas.'),
    callable('Request', 'file', ['string $key'], 'map|nil', 'Obtiene un archivo subido.'),
    callable('Request', 'post', ['string $key', '$default = nil'], 'any', 'Lee un parámetro POST.'),
    callable('Request', 'cookie', ['string $key', '$default = nil'], 'string|nil', 'Lee una cookie.'),
    callable('Request', 'root', [], 'string', 'Devuelve el origen scheme://host de la petición.'),
    callable('Request', 'header', ['string $key'], 'string|nil', 'Lee un encabezado de la petición.'),

    callable('Response', 'json', ['$data', 'int $status = 200'], 'WebResponse', 'Crea una respuesta JSON.'),
    callable('Response', 'redirect', ['string $url', 'int $status = 302'], 'WebResponse', 'Crea una redirección HTTP.'),
    callable('Response', 'back', [], 'WebResponse', 'Redirige al referer de la petición o a /.'),
    callable('Response', 'error', ['string $message', 'int $status = 400'], 'WebResponse', 'Crea una respuesta de error.'),
    callable('Response', 'raw', ['$content', 'int $status = 200', 'string $mime = "text/plain"', 'map $headers = {}'], 'WebResponse', 'Crea una respuesta sin transformación.'),
    callable('Response', 'stream', ['func $callback'], 'WebResponse', 'Crea una respuesta SSE text/event-stream.'),
    callable('Redirect', 'to', ['string $url', 'int $status = 302'], 'WebResponse', 'Alias de redirección con estado explícito.'),
    callable('WebResponse', 'withCookie', ['string $name', 'string $value'], 'WebResponse', 'Añade una cookie.', false),
    callable('WebResponse', 'withHeader', ['string $name', 'string $value'], 'WebResponse', 'Añade un encabezado.', false),
    callable('WebResponse', 'status', ['int $status'], 'WebResponse', 'Cambia el código HTTP.', false),
    callable('WebResponse', 'with', ['string $key', '$value'], 'WebResponse', 'Añade datos a la respuesta.', false),

    callable('View', 'render', ['string $view', 'map $data = {}'], 'WebResponse|string', 'Renderiza una vista Joss.'),
    callable('JSON', 'parse', ['string $json'], 'any', 'Convierte JSON a un valor Joss.'),
    callable('JSON', 'decode', ['string $json'], 'any', 'Alias de parse.'),
    callable('JSON', 'encode', ['$value'], 'string', 'Codifica un valor como JSON.'),
    callable('JSON', 'stringify', ['$value'], 'string', 'Alias de encode.'),
    callable('Math', 'random', ['int $min', 'int $max'], 'int', 'Genera un entero aleatorio.'),
    callable('Math', 'floor', ['number $value'], 'number', 'Redondea hacia abajo.'),
    callable('Math', 'ceil', ['number $value'], 'number', 'Redondea hacia arriba.'),
    callable('Math', 'abs', ['number $value'], 'number', 'Devuelve el valor absoluto.'),
    callable('Str', 'length', ['string $value'], 'int', 'Devuelve la longitud.'),
    callable('Str', 'random', ['int $length = 16'], 'string', 'Genera texto aleatorio.'),
    callable('Str', 'startsWith', ['string $value', 'string $prefix'], 'bool', 'Comprueba el prefijo.'),
    callable('Str', 'substring', ['string $value', 'int $start', 'int $length = nil'], 'string', 'Extrae una sección de texto.'),
    callable('Str', 'indexOf', ['string $value', 'string $search'], 'int', 'Busca texto.'),
    callable('Str', 'contains', ['string $value', 'string $search'], 'bool', 'Comprueba si contiene texto.'),
    callable('Str', 'trim', ['string $value'], 'string', 'Elimina espacios externos.'),

    callable('System', 'env', ['string $key', '$default = nil'], 'any', 'Lee una variable de env.joss.'),
    callable('System', 'Run', ['string $command', 'array $args = []'], 'string', 'Ejecuta un proceso del sistema si ALLOW_SYSTEM_RUN está habilitado.'),
    callable('System', 'load_driver', ['string $path', 'string $name = nil'], 'bool', 'Carga una DLL, SO o dylib que implemente la ABI C v1 de Joss.'),
    callable('System', 'driver_call', ['string $name', 'string $method', 'array $args = []'], 'any', 'Invoca un driver ABI cargado y decodifica su resultado JSON.'),
    callable('System', 'log', ['$value'], 'nil', 'Escribe un diagnóstico.'),
    callable('System', 'sleep', ['int $seconds'], 'bool', 'Pausa la ejecución.'),
    callable('System', 'now', ['int $dayOffset = 0'], 'string', 'Devuelve fecha y hora local en formato YYYY-MM-DD HH:MM:SS.'),
    callable('UUID', 'v4', [], 'string', 'Genera un UUID v4.'),
    callable('UUID', 'generate', [], 'string', 'Genera un UUID.'),

    callable('Plugin', 'call', ['string $plugin', 'string $method', 'array $args = []'], 'any', 'Invoca un payload JP v2 mediante RPC o ABI C v1.'),
    callable('Plugin', 'stream', ['string $plugin', 'string $method', 'array $args', 'func $callback'], 'any', 'Invoca un payload RPC JP v2 con streaming.'),
    callable('Plugin', 'path', ['string $plugin', 'string $relativePath'], 'string', 'Materializa un recurso del plugin.'),
    callable('Plugin', 'platform', [], 'string', 'Devuelve el target os-arch actual.'),
    callable('Cron', 'schedule', ['string $name', 'string $expression', 'block $body'], 'nil', 'Registra un bloque para el planificador cron.'),
    callable('Task', 'on_request', ['string $name', 'string $interval', 'block $body'], 'nil', 'Ejecuta actualmente el bloque una vez en una goroutine; el intervalo aún no se aplica.'),
    callable('Server', 'spawn', ['string $name', 'string $command', 'int $port'], 'Process', 'Inicia un servicio administrado.'),
    callable('Server', 'start', [], 'bool', 'Inicia el servidor mediante el callback registrado.'),
    callable('Session', 'get', ['string $key'], 'any', 'Lee una variable de sesión o nil.'),
    callable('Session', 'put', ['string $key', '$value'], 'nil', 'Guarda una variable de sesión.'),
    callable('Session', 'has', ['string $key'], 'bool', 'Comprueba una variable de sesión.'),
    callable('Session', 'forget', ['string $key'], 'nil', 'Elimina una variable de sesión.'),
    callable('Session', 'all', [], 'map', 'Devuelve toda la sesión.'),
    callable('Cache', 'put', ['string $key', '$value', 'int $seconds = 60'], 'bool', 'Guarda un valor en la caché global del proceso.'),
    callable('Cache', 'get', ['string $key', '$default = nil'], 'any', 'Lee un valor de caché.'),
    callable('Cache', 'has', ['string $key'], 'bool', 'Comprueba una clave de caché.'),
    callable('Cache', 'forget', ['string $key'], 'nil', 'Elimina una clave de caché.'),
    callable('GranDB', 'table', ['string $table'], 'GranDB', 'Selecciona la tabla para la consulta.'),
    callable('GranDB', 'select', ['array $columns'], 'GranDB', 'Selecciona las columnas devueltas.'),
    callable('GranDB', 'where', ['string $column', '$operatorOrValue', '$value = nil'], 'GranDB', 'Añade una condición AND.'),
    callable('GranDB', 'orWhere', ['string $column', '$operatorOrValue', '$value = nil'], 'GranDB', 'Añade una condición OR.'),
    callable('GranDB', 'whereIn', ['string $column', 'array $values'], 'GranDB', 'Filtra por una lista de valores.'),
    callable('GranDB', 'whereNull', ['string $column'], 'GranDB', 'Filtra valores nulos.'),
    callable('GranDB', 'whereBetween', ['string $column', '$min', '$max'], 'GranDB', 'Filtra por un intervalo.'),
    callable('GranDB', 'join', ['string $table', 'string $left', 'string $operator', 'string $right'], 'GranDB', 'Añade un JOIN.'),
    callable('GranDB', 'leftJoin', ['string $table', 'string $left', 'string $operator', 'string $right'], 'GranDB', 'Añade un LEFT JOIN.'),
    callable('GranDB', 'get', [], 'array', 'Ejecuta la consulta y devuelve una lista nativa.'),
    callable('GranDB', 'first', [], 'map|nil', 'Devuelve el primer resultado.'),
    callable('GranDB', 'find', ['$id'], 'map|nil', 'Busca un registro por ID.'),
    callable('GranDB', 'value', ['string $column'], 'any', 'Devuelve el valor de una columna.'),
    callable('GranDB', 'pluck', ['string $column', 'string $key = nil'], 'array|map', 'Extrae una columna.'),
    callable('GranDB', 'exists', [], 'bool', 'Indica si la consulta tiene resultados.'),
    callable('GranDB', 'count', ['string $column = "*"'], 'int', 'Cuenta resultados.'),
    callable('GranDB', 'sum', ['string $column'], 'number', 'Suma una columna.'),
    callable('GranDB', 'avg', ['string $column'], 'number', 'Calcula el promedio.'),
    callable('GranDB', 'insert', ['map $data'], 'bool', 'Inserta un registro.'),
    callable('GranDB', 'insertGetId', ['map $data'], 'int', 'Inserta y devuelve el ID.'),
    callable('GranDB', 'update', ['map $data'], 'int', 'Actualiza los registros seleccionados.'),
    callable('GranDB', 'delete', [], 'int', 'Elimina los registros seleccionados.'),
    callable('GranDB', 'orderBy', ['string $column', 'string $direction = "asc"'], 'GranDB', 'Ordena la consulta.'),
    callable('GranDB', 'limit', ['int $count'], 'GranDB', 'Limita resultados.'),
    callable('GranDB', 'offset', ['int $count'], 'GranDB', 'Desplaza resultados.'),
    callable('Schema', 'create', ['string $table', 'func $callback'], 'bool', 'Crea una tabla con Blueprint.'),
    callable('Schema', 'table', ['string $table', 'func $callback'], 'bool', 'Modifica una tabla con Blueprint.'),
    callable('Schema', 'rename', ['string $from', 'string $to'], 'bool', 'Renombra una tabla.'),
    callable('Schema', 'drop', ['string $table'], 'bool', 'Elimina una tabla si existe.'),
    callable('Schema', 'dropIfExists', ['string $table'], 'bool', 'Alias seguro de drop en la implementación actual.'),
    callable('Schema', 'hasTable', ['string $table'], 'bool', 'Comprueba si existe una tabla.'),
    callable('Schema', 'hasColumn', ['string $table', 'string $column'], 'bool', 'Comprueba si existe una columna.'),
    callable('Blueprint', 'id', [], 'Blueprint', 'Añade id BIGINT autoincremental.', false),
    callable('Blueprint', 'increments', ['string $column'], 'Blueprint', 'Añade un entero autoincremental.', false),
    callable('Blueprint', 'integer', ['string $column'], 'Blueprint', 'Añade una columna INTEGER.', false),
    callable('Blueprint', 'tinyInteger', ['string $column'], 'Blueprint', 'Añade una columna TINYINT.', false),
    callable('Blueprint', 'smallInteger', ['string $column'], 'Blueprint', 'Añade una columna SMALLINT.', false),
    callable('Blueprint', 'mediumInteger', ['string $column'], 'Blueprint', 'Añade una columna MEDIUMINT.', false),
    callable('Blueprint', 'bigInteger', ['string $column'], 'Blueprint', 'Añade una columna BIGINT.', false),
    callable('Blueprint', 'unsignedInteger', ['string $column'], 'Blueprint', 'Añade un INTEGER unsigned.', false),
    callable('Blueprint', 'unsignedBigInteger', ['string $column'], 'Blueprint', 'Añade un BIGINT unsigned.', false),
    callable('Blueprint', 'float', ['string $column'], 'Blueprint', 'Añade una columna FLOAT.', false),
    callable('Blueprint', 'double', ['string $column'], 'Blueprint', 'Añade una columna DOUBLE.', false),
    callable('Blueprint', 'decimal', ['string $column', 'int $precision = 8', 'int $scale = 2'], 'Blueprint', 'Añade una columna DECIMAL.', false),
    callable('Blueprint', 'char', ['string $column', 'int $length = 255'], 'Blueprint', 'Añade una columna CHAR.', false),
    callable('Blueprint', 'string', ['string $column', 'int $length = 255'], 'Blueprint', 'Añade una columna VARCHAR.', false),
    callable('Blueprint', 'text', ['string $column'], 'Blueprint', 'Añade una columna TEXT.', false),
    callable('Blueprint', 'mediumText', ['string $column'], 'Blueprint', 'Añade una columna MEDIUMTEXT.', false),
    callable('Blueprint', 'longText', ['string $column'], 'Blueprint', 'Añade una columna LONGTEXT.', false),
    callable('Blueprint', 'date', ['string $column'], 'Blueprint', 'Añade una columna DATE.', false),
    callable('Blueprint', 'dateTime', ['string $column'], 'Blueprint', 'Añade una columna DATETIME.', false),
    callable('Blueprint', 'time', ['string $column'], 'Blueprint', 'Añade una columna TIME.', false),
    callable('Blueprint', 'timestamp', ['string $column'], 'Blueprint', 'Añade una columna TIMESTAMP.', false),
    callable('Blueprint', 'timestamps', [], 'Blueprint', 'Añade created_at y updated_at.', false),
    callable('Blueprint', 'softDeletes', [], 'Blueprint', 'Añade deleted_at.', false),
    callable('Blueprint', 'boolean', ['string $column'], 'Blueprint', 'Añade una columna booleana.', false),
    callable('Blueprint', 'json', ['string $column'], 'Blueprint', 'Añade una columna JSON o TEXT en SQLite.', false),
    callable('Blueprint', 'enum', ['string $column', 'array $values'], 'Blueprint', 'Añade una columna ENUM o TEXT en SQLite.', false),
    callable('Blueprint', 'nullable', [], 'Blueprint', 'Hace nullable la última columna.', false),
    callable('Blueprint', 'unsigned', [], 'Blueprint', 'Hace unsigned la última columna.', false),
    callable('Blueprint', 'unique', ['string|array $columns = nil', 'string $name = nil'], 'Blueprint', 'Sin argumentos modifica la última columna; con columnas crea un índice único.', false),
    callable('Blueprint', 'default', ['$value'], 'Blueprint', 'Define el valor predeterminado de la última columna.', false),
    callable('Blueprint', 'comment', ['string $text'], 'Blueprint', 'Guarda un comentario para la última columna.', false),
    callable('Blueprint', 'dropColumn', ['string|array $columns'], 'Blueprint', 'Elimina una o varias columnas.', false),
    callable('Blueprint', 'renameColumn', ['string $from', 'string $to'], 'Blueprint', 'Renombra una columna.', false),
    callable('Blueprint', 'index', ['string|array $columns', 'string $name = nil'], 'Blueprint', 'Crea un índice simple o compuesto.', false),
    callable('Blueprint', 'uniqueIndex', ['string|array $columns', 'string $name = nil'], 'Blueprint', 'Crea un índice único simple o compuesto.', false),
    callable('Blueprint', 'dropIndex', ['string $name'], 'Blueprint', 'Elimina un índice por nombre.', false),
    callable('Blueprint', 'foreign', ['string|array $columns', 'string $name = nil'], 'Blueprint', 'Inicia una clave foránea simple o compuesta.', false),
    callable('Blueprint', 'references', ['string|array $columns'], 'Blueprint', 'Define las columnas referenciadas de la clave foránea.', false),
    callable('Blueprint', 'on', ['string $table'], 'Blueprint', 'Define la tabla referenciada.', false),
    callable('Blueprint', 'onDelete', ['string $action'], 'Blueprint', 'Define ON DELETE.', false),
    callable('Blueprint', 'onUpdate', ['string $action'], 'Blueprint', 'Define ON UPDATE.', false),
    callable('SQLite', 'open', ['string $path'], 'bool', 'Abre una base SQLite.', false),
    callable('SQLite', 'query', ['string $sql', 'array $params = []'], 'array|nil', 'Ejecuta una consulta SQL y devuelve filas.', false),
    callable('SQLite', 'close', [], 'bool', 'Cierra la base SQLite.', false),
    callable('WebSocket', 'send', ['$content'], 'bool', 'Envía contenido al cliente.', false),
    callable('WebSocket', 'broadcast', ['$content'], 'bool', 'Difunde contenido a conexiones.', false),
    callable('WebSocket', 'onMessage', ['func $callback'], 'WebSocket', 'Registra el callback de mensajes.', false),
    callable('WebSocket', 'close', [], 'bool', 'Cierra la conexión WebSocket.', false),
    callable('Stream', 'send', ['$eventOrData', '$data = nil'], 'bool', 'Envía un frame SSE; el primer argumento puede ser el nombre de evento.', false),
    callable('Stream', 'close', [], 'bool', 'Emite el marcador [DONE].', false),
    callable('MFA', 'generateTOTP', [], 'map', 'Genera configuración TOTP.'),
    callable('MFA', 'verifyTOTP', ['string $secret', 'string $code'], 'bool', 'Verifica un código TOTP.'),
    callable('MFA', 'generateRecoveryCodes', [], 'array', 'Genera ocho códigos de recuperación.'),
    callable('MFA', 'verifyRecoveryCode', ['$userId', 'string $code'], 'bool', 'Verifica y consume un código de recuperación.'),
    callable('UserStorage', 'put', ['string $token', 'string $path', '$content'], 'bool', 'Guarda contenido en almacenamiento de usuario.'),
    callable('UserStorage', 'get', ['string $token', 'string $path'], 'any', 'Lee contenido del almacenamiento de usuario.'),
    callable('UserStorage', 'getToFile', ['string $token', 'string $path', 'string $destination'], 'bool', 'Descarga contenido directamente a un archivo local.'),
    callable('UserStorage', 'delete', ['string $token', 'string $path'], 'bool', 'Elimina contenido del almacenamiento de usuario.'),
    callable('Stack', 'push', ['$value'], 'nil', 'Añade un valor a la pila.', false),
    callable('Stack', 'pop', [], 'any', 'Extrae el último valor de la pila.', false),
    callable('Stack', 'peek', [], 'any', 'Lee el último valor sin extraerlo.', false),
    callable('Queue', 'enqueue', ['$value'], 'nil', 'Añade un valor a la cola.', false),
    callable('Queue', 'dequeue', [], 'any', 'Extrae el primer valor de la cola.', false),
    callable('Queue', 'peek', [], 'any', 'Lee el primer valor sin extraerlo.', false),
    callable('Zip', 'extract', ['string $zipPath', 'string $destination'], 'bool', 'Extrae un ZIP.'),
    callable('Markdown', 'toHtml', ['string $markdown'], 'string', 'Convierte Markdown a HTML.'),
    callable('Markdown', 'readFile', ['string $path'], 'string', 'Lee Markdown desde un archivo.'),
    callable('Lang', 'get', ['string $key', 'map $replace = {}'], 'string', 'Obtiene una traducción.'),
    callable('Lang', 'set', ['string $locale'], 'bool|nil', 'Cambia el idioma activo.'),
    callable('Lang', 'locale', [], 'string', 'Devuelve el idioma activo.'),
    callable('Lang', 'locales', [], 'array', 'Lista los idiomas disponibles.'),
    callable('Redis', 'connect', ['string $address', 'string $password = ""', 'int $database = 0'], 'bool', 'Inicializa el cliente Redis global.'),
    callable('Redis', 'set', ['string $key', '$value', 'int $ttlSeconds = 0'], 'bool|nil', 'Guarda un valor en Redis.'),
    callable('Redis', 'get', ['string $key'], 'string|nil', 'Lee un valor de Redis.'),
    callable('Redis', 'del', ['string $key'], 'bool|nil', 'Elimina una clave de Redis.'),
    callable('Process', 'constructor', ['string $command', 'array $args = []'], 'Process', 'Prepara un proceso externo; requiere ALLOW_SYSTEM_RUN.'),
    callable('Process', 'start', [], 'bool', 'Inicia el proceso preparado.', false),
    callable('Process', 'wait', [], 'int', 'Espera el proceso y devuelve su código de salida.', false),
    callable('Process', 'kill', [], 'bool', 'Termina el proceso.', false),
    callable('Process', 'pid', [], 'int', 'Devuelve el PID o cero.', false),
    callable('Process', 'stdin', ['$value'], 'Process', 'Escribe una línea en stdin.', false),
    callable('Process', 'stdout_chan', [], 'Channel', 'Devuelve el canal de stdout.', false),
    callable('Process', 'stderr_chan', [], 'Channel', 'Devuelve el canal de stderr.', false),

    callable(undefined, 'map', ['$collection', 'func $callback'], 'array', 'Transforma cada elemento de la colección aplicando la función.'),
    callable(undefined, 'filter', ['$collection', 'func $predicate'], 'array', 'Filtra los elementos que cumplen la condición booleana.'),
    callable(undefined, 'reduce', ['$collection', 'func $callback', '$initial = nil'], 'any', 'Reduce la colección a un único valor acumulado.'),
    callable(undefined, 'find', ['$collection', 'func $predicate'], 'any', 'Devuelve el primer elemento que cumple la condición o nil.'),
    callable(undefined, 'any', ['$collection', 'func $predicate'], 'bool', 'Comprueba si al menos un elemento cumple la condición.'),
    callable(undefined, 'all', ['$collection', 'func $predicate'], 'bool', 'Comprueba si todos los elementos cumplen la condición.'),
    callable(undefined, 'sum', ['array $collection'], 'int|float', 'Calcula la suma numérica total de los elementos de la colección.'),
    callable(undefined, 'json_encode', ['$data', 'bool $pretty = false'], 'string', 'Serializa un array o mapa asociativo a formato JSON.'),
    callable(undefined, 'json_decode', ['string $json'], 'any', 'Deserializa una cadena JSON en mapas asociativos o arrays de Joss.'),
    callable(undefined, 'json_verify', ['string $json'], 'bool', 'Comprueba si una cadena contiene una estructura JSON válida.'),

    callable('Console', 'green', ['string $text'], 'string', 'Colorea el texto en verde.'),
    callable('Console', 'red', ['string $text'], 'string', 'Colorea el texto en rojo.'),
    callable('Console', 'yellow', ['string $text'], 'string', 'Colorea el texto en amarillo.'),
    callable('Console', 'blue', ['string $text'], 'string', 'Colorea el texto en azul.'),
    callable('Console', 'cyan', ['string $text'], 'string', 'Colorea el texto en cian.'),
    callable('Console', 'magenta', ['string $text'], 'string', 'Colorea el texto en magenta.'),
    callable('Console', 'gray', ['string $text'], 'string', 'Colorea el texto en gris.'),
    callable('Console', 'bold', ['string $text'], 'string', 'Aplica estilo negrita al texto.'),
    callable('Console', 'clear', [], 'nil', 'Limpia la pantalla de la terminal.'),
    callable('Console', 'log', ['$value'], 'nil', 'Imprime un valor en la terminal.')
];

// Signatures enrich the editor, but symbol existence is governed exclusively
// by the generated runtime catalog. Stale metadata must never expose a method
// that the current runtime does not register.
const registeredMetadata = nativeCallables.filter(item => {
    if (!item.owner) {
        return languageCatalog.builtins.includes(item.name);
    }
    return generatedNativeClasses[item.owner]?.includes(item.name) ?? false;
});
nativeCallables.splice(0, nativeCallables.length, ...registeredMetadata);

for (const [owner, methods] of Object.entries(generatedNativeClasses)) {
    for (const name of methods) {
        if (!nativeCallables.some(item => item.owner === owner && item.name === name)) {
            nativeCallables.push(callable(owner, name, [], 'any', `Método nativo ${owner}::${name}.`));
        }
    }
}

export const nativeClasses = Object.keys(generatedNativeClasses).sort();

export function nativeSignature(item: NativeCallable): string {
    const owner = item.owner ? `${item.owner}${item.static === false ? '->' : '::'}` : '';
    const result = `${owner}${item.name}(${item.parameters.map(param => param.label).join(', ')})`;
    return item.returnType ? `${result}: ${item.returnType}` : result;
}

export function findNativeCallable(reference: string): NativeCallable | undefined {
    const normalized = reference.replace('->', '::');
    const separator = normalized.lastIndexOf('::');
    if (separator >= 0) {
        const owner = normalized.substring(0, separator).replace(/^\$/, '');
        const name = normalized.substring(separator + 2);
        return nativeCallables.find(item => item.owner === owner && item.name === name);
    }
    return nativeCallables.find(item => !item.owner && item.name === reference.replace(/^\$/, ''));
}
