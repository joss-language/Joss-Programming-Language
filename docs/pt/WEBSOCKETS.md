#WebSockets

[Índice](README.md) · Antes: [HTTP](CONTROLADORES.md), [fechamentos](FUNCIONES.md) · Depois: [concurrency](CONCURRENCIA.md)

Um WebSocket mantém uma conexão aberta para troca de mensagens em ambas as direções. Use-o para bate-papo ou atualizações contínuas; uma consulta específica pode ser resolvida com HTTP. Os exemplos são fragmentos para o servidor integrado.

As rotas aceitam parâmetros dinâmicos. O primeiro argumento do manipulador é a conexão e então os parâmetros são injetados em ordem.```joss
Router::ws("/rooms/{room}/users/{id}", "ChatController@connect")

public class ChatController {
    public func connect(WebSocket $ws, string $room, string $id) {
        $ws->onMessage(func(string $message) {
            $ws->send($message)
        })
    }
}
```
Os retornos de chamada registrados com `onMessage` capturam o ambiente léxico do
manipulador. `$ws`, parâmetros de caminho e variáveis locais seguem
disponível após o manipulador terminar. O estado capturado é preservado
entre mensagens e suas invocações são serializadas:```joss
public func connect(WebSocket $ws) {
    $count = 0
    $ws->onMessage(func(string $message) {
        $count = $count + 1
        $ws->send("Mensaje " . $count . ": " . $message)
    })
}
```
Cada fechamento retém sua própria captura. Não presuma que onClose veja realocações locais feitas por outro encerramento onMessage.

Cada conexão é executada por conta própria `Runtime.Fork()`, então o
O contexto capturado não é compartilhado com outras conexões.

A conexão expõe:

- `send($message)`: envia uma mensagem e retorna se a escrita foi bem sucedida.
- `onMessage($callback)`: registra o retorno de chamada da mensagem.
- `onClose($callback)`: registra a limpeza que sempre é executada quando finalizada.
- `subscribe($channel)` / `unsubscribe($channel)`: gerencia canais locais.
- `publish($channel, $message)`: publica para os demais membros do canal.
- `WebSocket::subscriberCount($channel)`: retorna o número de conexões
  assinantes ativos do canal.
- `close()`: na verdade fecha o soquete.

`WebSocket::publish($channel, $message)` publica para todos os membros e pode
ser invocado a partir de manipuladores HTTP. As assinaturas são excluídas automaticamente
ao fechar a conexão. `WebSocket::broadcast()` preserva o hub global
histórico.

Os canais são locais para o processo. Uma implantação com múltiplas réplicas requer
sessões fixas e um backplane externo de pub/sub, ou você deve manter um único
réplica para funções de sala.

O servidor aplica limites configuráveis ​​e de manutenção de atividade:```env
WS_MAX_MESSAGE_BYTES="8388608"
WS_IDLE_TIMEOUT_SECONDS="120"
WS_PING_INTERVAL_SECONDS="30"
```
A atualização ocorre antes do middleware HTTP normal. Para autenticação dentro do soquete, valide o JWT com `Auth::validateToken($token)`; o tempo de execução preenche novamente a sessão usada por `Auth::user()`.

Com `TLS_CERT_FILE` e `TLS_KEY_FILE`, o servidor integrado oferece `wss`. Um proxy reverso ainda é válido e deve encaminhar `Upgrade` e `Connection`.