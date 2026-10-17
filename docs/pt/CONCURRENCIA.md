# Concorrência, operações assíncronas, Future e Canais

Antes: [Classes, objetos e herança](CLASES.md). Depois: [Projeto prático de console](PROYECTO_CONSOLA.md).
Referência técnica: [Módulos nativos](MODULOS_NATIVOS.md), [Arquitetura do runtime](ARQUITECTURA.md).

---

## O que você vai aprender aqui?

No mundo físico, os seres humanos não fazem uma única coisa estritamente depois da outra. Enquanto a máquina de lavar lava a roupa, você pode preparar a comida e ouvir música; você não fica olhando fixamente para a máquina de lavar durante 40 minutos sem fazer mais nada.

Na programação tradicional síncrona, o computador frequentemente fica "congelado" esperando:
- Espera que um servidor remoto do outro lado do mundo responda a uma consulta (500 milissegundos).
- Espera que um disco rígido leia um arquivo grande (200 milissegundos).
- Espera que uma consulta complexa ao banco de dados seja concluída.

Durante essa espera, o processador está desperdiçando milhões de ciclos de cálculo que poderiam ser aproveitados para atender outros usuários ou processar outros dados.

Neste guia você aprenderá:
1. A diferença conceitual entre operações **síncronas**, **assíncronas** e **concorrentes**.
2. O que é um **`Future`** (promessa de um resultado futuro).
3. Como delegar tarefas para segundo plano com a sintaxe **`async { ... }`**.
4. O que significa conceitualmente **`await`**, o que ele devolve e como propaga erros.
5. O que é um **canal (`channel`)**, como enviar e receber dados entre tarefas independentes e como consumi-los com `foreach` e `select`.
6. Como o Joss isola a memória entre tarefas mediante `Runtime.Fork()`.
7. Quando utilizar concorrência e quais erros clássicos evitar.

---

## 1. Os três conceitos essenciais

Para evitar confusões habituais, vamos distinguir com precisão três termos que frequentemente se misturam:

1. **Síncrono (bloqueante)**: Cada instrução espera obrigatoriamente que a anterior termine. Se a linha 1 demorar 5 segundos, a linha 2 não começa até que passem esses 5 segundos.
2. **Assíncrono (não bloqueante)**: Você inicia uma tarefa que levará tempo, mas em vez de ficar esperando de braços cruzados, o programa continua imediatamente fazendo outras coisas úteis enquanto a tarefa trabalha em segundo plano.
3. **Concorrente**: Múltiplas tarefas estão em andamento durante o mesmo intervalo de tempo, coordenando-se e compartilhando recursos de forma ordenada.

---

## 2. Lançar tarefas em segundo plano: `async` e `Future`

No Joss, quando você quer que um bloco de código seja executado em segundo plano sem interromper o fluxo principal, utiliza a construção **`async { ... }`**:

<!-- joss-run: ["Preparando resultado", "42"] -->
```joss
var $futuro = async {
    return 20 + 22
}
print("Preparando resultado")
var $resultado = await($futuro)
print($resultado)
```

Saída no terminal:

```text
Preparando resultado
42
```

### O que ocorre passo a passo neste programa?

1. `var $futuro = async { ... }`:
   - O Joss pega o bloco de código e o coloca para rodar de forma concorrente em uma thread leve gerenciada pelo sistema (uma *goroutine* do Go).
   - Imediatamente, a chamada devolve um objeto especial chamado **`Future`**. Um `Future` ainda não é o número `42`; é um "comprovante de retirada" que representa um resultado que estará pronto mais adiante.
2. `print("Preparando resultado")`:
   - Esta linha é executada imediatamente, **sem esperar** que o bloco `async` termine de calcular sua soma.
3. `var $resultado = await($futuro)`:
   - Aqui entra o `await`. Conceitual e praticamente significa:
     > **"Pause a execução desta linha até que a tarefa em segundo plano termine, abra o comprovante e deposite seu resultado em `$resultado`"**.
   - Se a tarefa já tiver terminado, o `await` entrega o resultado instantaneamente sem demora.
4. `print($resultado)`:
   - Exibe o valor final `42`.

> [!NOTE]
> No Joss, `await` é uma palavra-chave e expressão unária da linguagem: você pode escrever tanto `await($futuro)` quanto `await $futuro`. Além disso, pode ser utilizado em qualquer parte do código (no nível superior ou dentro de qualquer função) sem obrigar você a marcar suas funções como `async func`.

---

## 3. Execução paralela real: Lançar primeiro, esperar depois

Um dos erros mais comuns ao começar com assincronia é lançar uma tarefa e esperar por ela na linha imediatamente seguinte:

```joss
// INCORRECTO si buscas paralelismo (se vuelve síncrono):
var $a = await(async { return tarea1() })
var $b = await(async { return tarea2() })
```
No código acima, `tarea2` nunca começa até que `tarea1` tenha terminado completamente.

Para alcançar um verdadeiro benefício de desempenho quando você tem tarefas independentes (por exemplo, consultar dois serviços web diferentes ou processar duas imagens), **você deve lançar todas as tarefas primeiro e aguardar seus resultados depois**:

<!-- joss-run: ["30"] -->
```joss
var $uno = async { return 10 }
var $dos = async { return 20 }
var $a = await($uno)
var $b = await($dos)
print($a + $b)
```

Saída no terminal:

```text
30
```

Agora, tanto `$uno` quanto `$dos` são executados ao mesmo tempo em núcleos de processador separados. O tempo total de espera será o da tarefa mais lenta, e não a soma de ambas.

---

## 4. O que acontece se uma tarefa assíncrona falhar

O que ocorre se o código dentro do bloco `async` sofrer um erro ou lançar uma exceção com `throw`?

O Joss não permite que seu programa entre em colapso silenciosamente:
1. O `Future` captura internamente a exceção ocorrida.
2. No momento em que você invoca `await($futuro)`, o erro **é relançado automaticamente** na thread principal.
3. Você pode capturar e solucionar essa falha envolvendo o `await` dentro de um bloco `try / catch`:

<!-- joss-run: ["[ASYNC PANIC] Fallo al conectar con el servidor externo", "Error recuperado con éxito: Fallo al conectar con el servidor externo"] -->
```joss
var $tarea = async {
    throw "Fallo al conectar con el servidor externo"
}

try {
    var $resultado = await($tarea)
} catch ($e) {
    print("Error recuperado con éxito: " . $e)
}
```

Saída no terminal:

```text
[ASYNC PANIC] Fallo al conectar con el servidor externo
Error recuperado con éxito: Fallo al conectar con el servidor externo
```

---

## 5. Canais (`channel`): Comunicação segura entre tarefas

Quando duas tarefas concorrentes precisam passar mensagens continuamente (como uma linha de montagem onde um processo baixa dados e outro os processa), compartilhar variáveis globais mutáveis é muito perigoso porque elas podem ser sobrescritas e gerar condições de corrida (*race conditions*).

A solução canônica e segura do Joss são os **canais (`channel`)**. Um canal é uma tubulação unidirecional: uma extremidade introduz dados e a outra extremidade os extrai em estrita ordem de chegada (FIFO).

<!-- joss-run: ["hola"] -->
```joss
var $canal = make_chan(1)
send($canal, "hola")
print(recv($canal))
close($canal)
```

Saída no terminal:

```text
hola
```

### Operações essenciais com canais:

1. `make_chan($capacidad)`:
   - Cria um novo canal. O argumento define o tamanho do **buffer** (quantas mensagens podem ser guardadas na tubulação antes que quem envia tenha que parar e esperar que alguém leia).
   - Se você criar `make_chan(1)`, pode depositar uma mensagem sem esperar que haja um receptor escutando naquele exato milissegundo.
   - Se você criar `make_chan()` (sem argumentos ou com `0`), é um canal sem buffer: o emissor ficará bloqueado até que o receptor esteja pronto para receber o dado em mãos.
2. `send($canal, $valor)` (ou o operador `$canal << $valor`):
   - Envia um dado através da tubulação.
3. `recv($canal)`:
   - Espera que chegue uma mensagem pelo canal e a extrai.
4. `close($canal)`:
   - Fecha o canal, avisando a todos os receptores que não serão enviados mais dados.

---

## 6. Padrão Produtor-Consumidor com `foreach`

Uma das características mais elegantes da linguagem é que você pode utilizar um laço `foreach` comum para consumir todas as mensagens de um canal até que ele seja fechado:

<!-- joss-run: ["10", "20"] -->
```joss
var $canal = make_chan()
var $productor = async {
    send($canal, 10)
    send($canal, 20)
    close($canal)
}
foreach ($canal as $valor) {
    print($valor)
}
await($productor)
```

Saída no terminal:

```text
10
20
```

### Por que isso funciona de forma tão limpa?
1. O bloco `async` atua como **produtor**: envia `10`, depois `20` e finalmente avisa que terminou fechando o canal com `close($canal)`.
2. O laço `foreach` atua como **consumidor**: espera pacientemente cada número, o imprime e, assim que detecta que o canal foi fechado e está vazio, o laço termina de forma limpa e automática.

---

## 7. Multiplexação de canais com `select`

A instrução **`select`** permite esperar e reagir a múltiplas operações de canais simultaneamente, executando o primeiro caso que estiver pronto para ser concluído sem bloquear a thread caso seja fornecida uma cláusula `default:`:

- `case send($ch, $valor):` Tenta enviar um valor para um canal.
- `case recv($ch):` Espera receber de um canal descartando o valor.
- `case $msg = recv($ch):` Recebe de um canal e atribui o valor recebido a uma variável.
- `default:` É executado imediatamente se nenhum dos canais tiver operações prontas (não bloqueante).

<!-- joss-run: ["recibido: listo"] -->
```joss
var $ch = make_chan(1)
send($ch, "listo")

select {
    case $msg = recv($ch):
        print("recibido: " . $msg)
    default:
        print("sin mensajes")
}
```

Saída no terminal:

```text
recibido: listo
```

---

## 8. Funções geradoras e `yield`

Uma **função geradora** permite produzir uma sequência de valores preguiçosamente (*lazy evaluation*) sob demanda, suspendendo sua execução após cada `yield` e retomando-a exatamente nesse ponto quando o próximo elemento for solicitado.

O Joss suporta:
- `yield $valor`: Emite um valor.
- `yield $clave => $valor`: Emite um par chave-valor.
- Consumo direto mediante um laço `foreach`.
- Inspeção manual mediante métodos da instância devolvida: `->current()`, `->next()`, `->key()`, `->valid()`.

<!-- joss-run: ["0: 10", "1: 20", "2: 30"] -->
```joss
public func contar(): mixed {
    yield 10
    yield 20
    yield 30
}

var $gen = contar()
foreach ($gen as $k => $v) {
    print($k . ": " . $v)
}
```

Saída no terminal:

```text
0: 10
1: 20
2: 30
```

---

## 9. O modelo de isolamento de memória: `Runtime.Fork()`

Muitas linguagens sofrem com erros obscuros de concorrência quando duas tarefas modificam as mesmas variáveis ao mesmo tempo.

O Joss previne isso em sua arquitetura interna:
- Cada vez que você executa `async { ... }`, o motor realiza uma operação de bifurcação controlada (`Runtime.Fork()`).
- Isso **copia as variáveis locais, tipos e constantes** para a nova tarefa, garantindo que a tarefa em segundo plano não corrompa os nomes de quem a lançou.
- Os recursos que legitimamente devem ser compartilhados (como as conexões ativas a bancos de dados e os canais `channel`) mantêm-se acessíveis para a coordenação.

---

## 10. Tarefas periódicas: Cron

Para operações que devem se repetir periodicamente no tempo (como limpar sessões inativas a cada meia-noite ou gerar relatórios a cada hora), o Joss inclui a classe nativa `Cron`:

```joss
Cron::schedule("limpieza_diaria", "0 0 * * *", {
    print("Ejecutando limpieza programada del sistema...")
})
```

`Cron::schedule` aceita expressões padrão de 5 campos cron ou atalhos comuns como `hourly`, `daily`, `weekly` ou `monthly`.

---

## 11. Boas práticas e erros comuns

| Situação | O que você deve fazer | O que você deve evitar |
|---|---|---|
| Múltiplas tarefas independentes | Lance todas com `async` primeiro e faça `await` no final. | Fazer `await` imediatamente após cada `async`. |
| Comunicação entre tarefas | Use canais (`make_chan`, `send`, `recv`) ou o valor devolvido por `return`. | Modificar variáveis globais compartilhadas a partir de diferentes threads. |
| Canais sem buffer em uma única thread | Se não usar `async`, dê pelo menos tamanho 1 (`make_chan(1)`). | Usar `make_chan()` e chamar `send` antes de `recv` na mesma thread (provocará um bloqueio permanente ou *deadlock*). |
| Fim de transmissão em canais | O produtor deve sempre chamar `close($canal)` ao terminar de emitir dados. | Deixar um canal aberto indefinidamente se um `foreach` estiver esperando por ele. |

---

## 12. Exercícios práticos

### Nível 1 — Teste rápido
Lance uma tarefa com `var $f = async { return "Tarea completada" }`, imprima primeiro `"Esperando..."` e depois imprima o resultado de `await($f)`.

### Nível 2 — Exercício guiado
1. Simule baixar três arquivos lançando três blocos `async` em paralelo:
   ```joss
   var $f1 = async { return "archivo1.png descargado" }
   var $f2 = async { return "archivo2.pdf descargado" }
   var $f3 = async { return "archivo3.zip descargado" }
   ```
2. Aguarde os três resultados com `await` e imprima cada um no console.

### Nível 3 — Desafio
1. Crie um canal com buffer para 3 elementos: `var $cola = make_chan(3)`.
2. Lance um bloco `var $productor = async { ... }` que envie três tarefas (`"enviar_correo"`, `"generar_pdf"`, `"actualizar_stock"`) e feche o canal com `close($cola)`.
3. No fluxo principal, percorra `$cola` com `foreach ($cola as $tarea)` e imprima `"Procesando: " . $tarea`.
4. Ao terminar o laço, aguarde o produtor com `await($productor)`.

---

## Próximo passo

Parabéns! Você concluiu o aprendizado de todos os fundamentos da linguagem Joss: tipos, estruturas de controle, coleções, funções, exceções, programação orientada a objetos e concorrência.

Agora colocaremos todo esse conhecimento em prática construindo projetos reais passo a passo:

Continue com: [Construir um projeto de console completo](PROYECTO_CONSOLA.md).