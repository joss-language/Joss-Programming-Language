#WebSockets

[Index](README.md) · Before: [HTTP](CONTROLADORES.md), [closures](FUNCIONES.md) · After: [concurrency](CONCURRENCIA.md)

A WebSocket keeps a connection open to exchange messages in both directions. Use it for chat or continuous updates; a specific query can be resolved with HTTP. The examples are snippets for the integrated server.

Routes accept dynamic parameters. The first argument of the handler is the connection and then the parameters are injected in order.```joss
Router::ws("/rooms/{room}/users/{id}", "ChatController@connect")

public class ChatController {
    public func connect(WebSocket $ws, string $room, string $id) {
        $ws->onMessage(func(string $message) {
            $ws->send($message)
        })
    }
}
```
Callbacks registered with `onMessage` capture the lexical environment of the
handler. `$ws`, path parameters and local variables follow
available after the handler finishes. Captured state is preserved
between messages and their invocations are serialized:```joss
public func connect(WebSocket $ws) {
    $count = 0
    $ws->onMessage(func(string $message) {
        $count = $count + 1
        $ws->send("Mensaje " . $count . ": " . $message)
    })
}
```
Each closure retains its own catch. Don't assume that onClose sees local reallocations made by another onMessage closure.

Each connection runs on its own `Runtime.Fork()`, so the
Captured context is not shared with other connections.

The connection exposes:

- `send($message)`: sends a message and returns if the write was successful.
- `onMessage($callback)`: register message callback.
- `onClose($callback)`: registers cleanup that is always executed when finished.
- `subscribe($channel)` / `unsubscribe($channel)`: manage local channels.
- `publish($channel, $message)`: publishes to the other members of the channel.
- `WebSocket::subscriberCount($channel)`: returns the number of connections
  active subscribers to the channel.
- `close()`: actually closes the socket.

`WebSocket::publish($channel, $message)` publishes to all members and can
be invoked from HTTP handlers. Subscriptions are automatically deleted
when closing the connection. `WebSocket::broadcast()` preserves the global hub
historical.

The channels are local to the process. A deployment with multiple replicas requires
sticky sessions and an external pub/sub backplane, or should you maintain a single
replica for room functions.

The server applies keepalive and configurable limits:```env
WS_MAX_MESSAGE_BYTES="8388608"
WS_IDLE_TIMEOUT_SECONDS="120"
WS_PING_INTERVAL_SECONDS="30"
```
The upgrade occurs before the normal HTTP middleware. For authentication within the socket, validate the JWT with `Auth::validateToken($token)`; the runtime repopulates the session used by `Auth::user()`.

With `TLS_CERT_FILE` and `TLS_KEY_FILE`, the integrated server offers `wss`. A reverse proxy is still valid and must forward `Upgrade` and `Connection`.