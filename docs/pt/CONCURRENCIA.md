# Simultaneidade, operações assíncronas, futuro e canais anteriores: [Tratamento de erros e exceções](ERRORES.md). Depois: [Projeto prático de console](PROYECTO_CONSOLA.md). Referência técnica: [Módulos nativos](MODULOS_NATIVOS.md), [Arquitetura de tempo de execução](ARQUITECTURA.md). --- ## O que você vai aprender aqui? No mundo físico, os seres humanos não fazem uma coisa estritamente após a outra. Enquanto a máquina de lavar lava roupas, você pode preparar comida e ouvir música; Você não fica olhando para a máquina de lavar por 40 minutos sem fazer mais nada. Na programação síncrona tradicional, o computador muitas vezes fica “congelado” esperando: - Espera que um servidor remoto do outro lado do mundo responda a uma consulta (500 milissegundos). - Aguarde até que o disco rígido leia um arquivo grande (200 milissegundos). - Aguarda a conclusão de uma consulta complexa ao banco de dados. Durante essa espera, o processador desperdiça milhões de ciclos de cálculo que poderiam ser usados ​​para atender outros usuários ou processar outros dados. Neste guia você aprenderá: 1. A diferença conceitual entre operações **síncronas**, **assíncronas** e **concorrentes**. 2. O que é **`Future`** (promessa de resultado futuro). 3. Como delegar tarefas em segundo plano com a sintaxe **`async { ... }`**. 4. O que **`await(...)`** significa conceitualmente, o que retorna e como propaga erros. 5. O que é um **canal (`channel`)**, como enviar e receber dados entre tarefas independentes e como consumi-los com `foreach`. 6. Como Joss isola a memória entre tarefas usando `Runtime.Fork()`. 7. Quando usar simultaneidade e quais erros clássicos evitar. --- ## 1. Os três conceitos essenciais Para evitar confusões comuns, vamos distinguir com precisão três termos que muitas vezes se misturam: 1. **Síncrono (bloqueio)**: Cada instrução espera necessariamente que a anterior termine. Se a linha 1 demorar 5 segundos, a linha 2 não será iniciada até que esses 5 segundos tenham passado. 2. **Assíncrona (sem bloqueio)**: você inicia uma tarefa que levará tempo, mas em vez de esperar ociosamente, o programa continua imediatamente fazendo outras coisas úteis enquanto a tarefa funciona em segundo plano. 3. **Simultâneo**: Várias tarefas estão em andamento durante o mesmo intervalo de tempo, coordenando e compartilhando recursos de maneira ordenada. --- ## 2. Inicie tarefas em segundo plano: `async` e `*Future` No Joss, quando você deseja que um bloco de código seja executado em segundo plano sem interromper o fluxo principal, você usa a construção **`async { ... }`**:<!-- joss-run: ["Preparando resultado", "42"] -->
```joss
$futuro = async {
    return 20 + 22
}
print("Preparando resultado")
$resultado = await($futuro)
print($resultado)
```
### O que acontece passo a passo neste programa? 1. `$futuro = async { ... }`: - Joss pega o bloco de código e o lança para ser executado simultaneamente em uma thread leve gerenciada pelo sistema (uma *goroutine* do Go). - Imediatamente, a chamada retorna um objeto especial chamado **`Future`**. Um `Future` ainda não é o número `42`; é um “ticket de reclamação” que representa um resultado que estará pronto posteriormente. 2. `print("Preparando resultado")`: - Esta linha é executada imediatamente, **sem esperar** que o bloco `async` termine de calcular sua soma. 3. `$resultado = await($futuro)`: - Aqui entra em ação a função `await`. Conceitual e praticamente significa: > **"Pausar a execução desta linha até que a tarefa em segundo plano termine, abra o ticket e deposite seu resultado em `$resultado`"**. - Se a tarefa já foi concluída, `await` entrega o resultado instantaneamente sem demora. 4. `print($resultado)`: - Exibe o valor final `42`. > [!NOTE] > Em Joss, `await` é uma função nativa (`await($futuro)`), não uma palavra reservada com prefixo. Pode ser usado em qualquer lugar do código: no nível superior de um arquivo ou em qualquer função; não exige que você declare suas funções como `async func`. --- ## 3. Execução paralela real: iniciar primeiro, esperar depois Um dos erros mais comuns ao iniciar com assíncrono é iniciar uma tarefa e esperar por ela na linha imediatamente seguinte:```joss
// INCORRECTO si buscas paralelismo (se vuelve síncrono):
$a = await(async { return tarea1() })
$b = await(async { return tarea2() })
```
No código acima, `tarea2` nunca inicia até que `tarea1` seja completamente concluído. Para obter um verdadeiro benefício de desempenho quando você tem tarefas independentes (por exemplo, consultar dois serviços web diferentes ou processar duas imagens), **você deve iniciar todas as tarefas primeiro e aguardar seus resultados depois**:<!-- joss-run: ["30"] -->
```joss
$uno = async { return 10 }
$dos = async { return 20 }
$a = await($uno)
$b = await($dos)
print($a + $b)
```
Agora, ambos `$uno` e `$dos` são executados simultaneamente em núcleos de processador separados. O tempo total de espera será o da tarefa mais lenta, e não a soma de ambos. --- ## 4. O que acontece se uma tarefa assíncrona falhar O que acontece se o código dentro do bloco `async` falhar ou lançar uma exceção com `throw`? Joss não permite que seu programa trave silenciosamente: 1. O `Future` captura internamente a exceção que ocorreu. 2. No momento em que você invoca `await($futuro)`, o erro **é automaticamente relançado** no thread principal. 3. Você pode detectar e corrigir esse bug agrupando o `await` dentro de um bloco `try / catch`:```joss
$tarea = async {
    throw "Fallo al conectar con el servidor externo"
}

try {
    $resultado = await($tarea)
} catch ($e) {
    print("Error recuperado con éxito: " . $e)
}
```
--- ## 5. Canais (`channel`): Comunicação segura entre tarefas Quando duas tarefas simultâneas precisam passar mensagens continuamente uma para a outra (como uma linha de montagem onde um processo baixa dados e outro os processa), o compartilhamento de variáveis ​​globais mutáveis ​​é muito perigoso porque elas podem se sobrescrever e gerar condições de corrida (*condições de corrida*). A solução Joss canônica e segura é **canais (`channel`)**. Um canal é um canal unidirecional: uma extremidade traz dados e a outra extremidade os extrai estritamente no estilo FIFO (primeiro a chegar, primeiro a ser servido).<!-- joss-run: ["hola"] -->
```joss
$canal = make_chan(1)
send($canal, "hola")
print(recv($canal))
close($canal)
```
### Operações essenciais com canais: 1. `make_chan($capacidad)`: - Crie um novo canal. O argumento define o tamanho do **buffer** (quantas mensagens podem ser armazenadas no pipeline antes que o remetente tenha que parar e esperar que alguém leia). - Se você criar `make_chan(1)`, poderá depositar uma mensagem sem esperar que um receptor ouça naquele exato milissegundo. - Se você criar `make_chan()` (sem argumentos ou com `0`), é um canal sem buffer: o remetente será bloqueado até que o destinatário esteja pronto para receber os dados corpo a corpo. 2. `send($canal, $valor)` (ou o operador `$canal << $valor`): - Envia dados pelo pipe. 3. `recv($canal)`: - Aguarda a chegada de uma mensagem pelo canal e a extrai. 4. `close($canal)`: - Fecha o canal, notificando todos os receptores que não serão enviados mais dados. --- ## 6. Padrão Produtor-Consumidor com `foreach` Um dos recursos mais elegantes da linguagem é que você pode usar um loop `foreach` comum para consumir todas as mensagens em um canal até que ele seja fechado:<!-- joss-run: ["10", "20"] -->
```joss
$canal = make_chan()
$productor = async {
    send($canal, 10)
    send($canal, 20)
    close($canal)
}
foreach ($canal as $valor) {
    print($valor)
}
await($productor)
```
### Por que isso funciona de forma tão limpa? 1. O bloco `async` atua como **produtor**: envia `10`, depois `20` e por fim avisa que acabou fechando o canal com `close($canal)`. 2. O loop `foreach` atua como um **consumidor**: espera pacientemente por cada número, imprime-o e, assim que detecta que o canal foi fechado e está vazio, o loop termina de forma limpa e automática. --- ## 7. Multiplexação de canal com `select` A instrução **`select`** permite esperar e reagir a múltiplas operações de canal simultaneamente, executando o primeiro caso que está pronto para ser concluído sem bloquear o thread se uma cláusula `default:` for fornecida: - `case send($ch, $valor):` Tente enviar um valor para um canal. - `case recv($ch):` Espera receber de um canal descartando o valor. - `case $msg = recv($ch):` Recebe de um canal e atribui o valor recebido a uma variável. - `default:` Executa imediatamente se nenhum dos canais tiver operações prontas (sem bloqueio).<!-- joss-run: ["recibido: listo"] -->
```joss
$ch = make_chan(1)
send($ch, "listo")

select {
    case $msg = recv($ch):
        print("recibido: " . $msg)
    default:
        print("sin mensajes")
}
```
--- ## 8. Gerando funções e `yield` Uma **função geradora** permite que uma sequência de valores seja produzida preguiçosamente (*avaliação preguiçosa*) sob demanda, suspendendo sua execução após cada `yield` e retomando-a exatamente naquele ponto quando o próximo elemento for solicitado. Joss suporta: - `yield $valor`: Emite um valor. - `yield $clave => $valor`: Emite um par chave-valor. - Consumo direto através de loop `foreach`. - Inspeção manual utilizando métodos da instância retornada: `->current()`, `->next()`, `->key()`, `->valid()`.<!-- joss-run: ["0: 10", "1: 20", "2: 30"] -->
```joss
public func contar(): mixed {
    yield 10
    yield 20
    yield 30
}

$gen = contar()
foreach ($gen as $k => $v) {
    print($k . ": " . $v)
}
```
--- ## 9. O modelo de isolamento de memória: `Runtime.Fork()` Muitas linguagens sofrem de bugs obscuros de simultaneidade quando duas tarefas modificam as mesmas variáveis ​​ao mesmo tempo. Joss evita isso em sua arquitetura interna: - Cada vez que você executa `async { ... }`, o mecanismo executa uma operação de ramificação controlada (`Runtime.Fork()`). - Isto **copia as variáveis ​​locais, tipos e constantes** para a nova tarefa, garantindo que a tarefa em segundo plano não corrompa os nomes de quem a iniciou. - Recursos que deveriam ser legitimamente compartilhados (como conexões ativas de banco de dados e canais `channel`) são mantidos acessíveis para coordenação. --- ## 10. Tarefas periódicas: Cron Para operações que devem ser repetidas periodicamente ao longo do tempo (como limpar sessões inativas toda meia-noite ou gerar relatórios a cada hora), Joss inclui a classe nativa `Cron`:```joss
Cron::schedule("limpieza_diaria", "0 0 * * *", {
    print("Ejecutando limpieza programada del sistema...")
})
```
`Cron::schedule` aceita expressões cron padrão de 5 campos ou atalhos comuns, como `hourly`, `daily`, `weekly` ou `monthly`. --- ## 11. Boas práticas e erros comuns | Situação | O que você deve fazer | O que você deve evitar | |---|---|---| | Múltiplas tarefas independentes | Inicie todos eles com `async` primeiro e faça `await` por último. | Faça `await` imediatamente após cada `async`. | | Comunicação entre tarefas | Use canais (`make_chan`, `send`, `recv`) ou o valor retornado por `return`. | Modifique variáveis ​​globais compartilhadas de diferentes threads. | | Canais sem buffer em um único thread | Se você não usar `async`, forneça pelo menos tamanho 1 (`make_chan(1)`). | Use `make_chan()` e chame `send` antes de `recv` no mesmo thread (causará um *deadlock*). | | Fim da transmissão nos canais | O produtor deve sempre ligar para `close($canal)` ao finalizar a emissão dos dados. | Deixe um canal aberto indefinidamente se `foreach` estiver esperando por ele. | --- ## 12. Exercício prático 1. **Simulador de download paralelo**: - Crie uma função que simule o download de três arquivos:     ```joss
     $f1 = async { return "archivo1.png descargado" }
     $f2 = async { return "archivo2.pdf descargado" }
     $f3 = async { return "archivo3.zip descargado" }
     ```
- Aguarde os três resultados com `await` e imprima cada um. 2. **Fila de tarefas com canal**: - Crie um canal com buffer para 3 itens: `$cola = make_chan(3)`. - Envie três tarefas: `"enviar_correo"`, `"generar_pdf"`, `"actualizar_stock"`. - Feche o canal. - Navegue no canal com `foreach` e imprima `"Procesando: " . $tarea`. --- ## Próximo passo Parabéns! Você concluiu o aprendizado de todos os fundamentos da linguagem Joss: tipos, estruturas de controle, funções, coleções, programação orientada a objetos, exceções e simultaneidade. Agora vamos colocar todo esse conhecimento em prática construindo passo a passo projetos reais: Continue com: [Construa um projeto de console completo](PROYECTO_CONSOLA.md).