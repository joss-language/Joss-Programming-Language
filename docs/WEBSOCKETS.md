# WebSockets

[Índice](README.md) · Antes: [HTTP](CONTROLADORES.md), [closures](FUNCIONES.md) · Después: [concurrencia](CONCURRENCIA.md)

Un WebSocket mantiene abierta una conexión para intercambiar mensajes en ambas direcciones. Úsalo para chat o actualizaciones continuas; una consulta puntual puede resolverse con HTTP. Los ejemplos son fragmentos para el servidor integrado.

Las rutas aceptan parámetros dinámicos. El primer argumento del handler es la conexión y después se inyectan los parámetros en orden.

```joss
Router::ws("/rooms/{room}/users/{id}", "ChatController@connect")

public class ChatController {
    public func connect(WebSocket $ws, string $room, string $id) {
        $ws->onMessage(func(string $message) {
            $ws->send($message)
        })
    }
}
```

Los callbacks registrados con `onMessage` capturan el entorno léxico del
handler. `$ws`, los parámetros de ruta y las variables locales siguen
disponibles después de que el handler termina. El estado capturado se conserva
entre mensajes y sus invocaciones se serializan:

```joss
public func connect(WebSocket $ws) {
    $count = 0
    $ws->onMessage(func(string $message) {
        $count = $count + 1
        $ws->send("Mensaje " . $count . ": " . $message)
    })
}
```

Cada closure conserva su propia captura. No asumas que onClose ve las reasignaciones locales realizadas por otra closure onMessage.

Cada conexión se ejecuta sobre su propio `Runtime.Fork()`, por lo que el
contexto capturado no se comparte con otras conexiones.

La conexión expone:

- `send($message)`: envía un mensaje y retorna si la escritura tuvo éxito.
- `onMessage($callback)`: registra el callback de mensajes.
- `onClose($callback)`: registra limpieza que siempre se ejecuta al terminar.
- `subscribe($channel)` / `unsubscribe($channel)`: administra canales locales.
- `publish($channel, $message)`: publica a los demás miembros del canal.
- `WebSocket::subscriberCount($channel)`: devuelve el número de conexiones
  activas suscritas al canal.
- `close()`: cierra realmente el socket.

`WebSocket::publish($channel, $message)` publica a todos los miembros y puede
invocarse desde handlers HTTP. Las suscripciones se eliminan automáticamente
al cerrar la conexión. `WebSocket::broadcast()` conserva el hub global
histórico.

Los canales son locales al proceso. Un despliegue con varias réplicas necesita
sticky sessions y un backplane pub/sub externo, o debe mantener una sola
réplica para funciones de sala.

El servidor integrado, incluido el servidor standalone de `std/http`, mantiene
la conexión activa con frames ping y responde los pong del cliente. Una
conexión sin actividad se cierra al alcanzar el timeout configurado. Los
valores predeterminados y límites de mensaje son configurables:

```env
WS_MAX_MESSAGE_BYTES="8388608"
WS_IDLE_TIMEOUT_SECONDS="120"
WS_PING_INTERVAL_SECONDS="30"
```

El upgrade ocurre antes del middleware HTTP normal. Para autenticación dentro del socket, valida el JWT con `Auth::validateToken($token)`; el runtime repuebla la sesión usada por `Auth::user()`.

Con `TLS_CERT_FILE` y `TLS_KEY_FILE`, el servidor integrado ofrece `wss`. Un proxy inverso sigue siendo válido y debe reenviar `Upgrade` y `Connection`.
