# Tratamento de erros, exceções e diagnósticos

Antes: [Classes, objetos e herança](CLASES.md). Depois: [Simultaneidade, assincronia e canais](CONCURRENCIA.md).
Referência técnica: [Diagnóstico estruturado](DIAGNOSTICOS.md), [Analisador semântico](ANALIZADOR.md).

---

## O que você vai aprender aqui?

Num mundo ideal, os programas receberiam sempre os dados corretos, os discos rígidos nunca ficariam cheios e as ligações de rede nunca falhariam. No mundo real, os erros são inevitáveis:
- Um usuário digita letras onde é esperado um número de cartão.
- Um arquivo de configuração necessário ao programa foi excluído acidentalmente.
- O servidor de banco de dados é reiniciado no meio de uma transação.

Um programa profissional não é aquele que nunca encontra problemas, mas sim aquele que **sabe como antecipá-los, contê-los e se recuperar sem entrar em colapso**.

Neste guia você aprenderá:
1. As três fases de tempo onde os erros são detectados: sintaxe, análise estática e tempo de execução.
2. Como ler e interpretar uma mensagem de diagnóstico estruturada de Joss.
3. O mecanismo de tratamento de exceções com **`try`**, **`catch`** e **`throw`**.
4. Quais informações estão contidas na variável de erro capturada (o mapa `$error`).
5. Por que as instruções `return`, `break` e `continue` dentro de um bloco `try` não sofrem interferência de `catch`.
6. Quando tratar erros usando exceções versus quando verificar códigos ou valores de retorno (`null`).

---

## 1. As três fases de um erro

Para resolver um problema rapidamente, a primeira coisa é identificar em qual fase do ciclo de vida do programa ele ocorreu:```text
Código fuente (.joss)
         │
         ▼
[ 1. PARSER ] ────────► ¿Error de Sintaxis?
         │              (Falta cerrar paréntesis, comilla o llave)
         ▼
[ 2. ANALYZER ] ──────► ¿Error Semántico o de Tipo?
         │              (Variable no definida, tipo incompatible, retorno ausente)
         ▼
[ 3. RUNTIME ] ───────► ¿Excepción en Ejecución?
                        (Archivo no encontrado, división por cero, throw explícito)
```
| Fase | Quando isso ocorre | Exemplo | Como é resolvido |
|---|---|---|---|
| **Sintaxe** | Durante a leitura inicial do texto | `print("Hola)` (aspas abertas) | Corrige a pontuação indicada pelo analisador. |
| **Análise** | Durante a pré-verificação estática | `int $x = "texto"` (`JOSS-TYPE-001`) | Corrija a incompatibilidade de tipo ou nome antes de executar. |
| **Tempo de execução** | Enquanto o programa está sendo executado na memória | `$n / 0` ou falha de rede | Ele é interceptado e gerenciado com blocos `try / catch`. |

---

## 2. Anatomia de uma mensagem de diagnóstico

Quando Joss detecta um problema antes de executar, ele imprime um diagnóstico estruturado. Por exemplo:```text
error[JOSS-TYPE-001] app.joss:15:5: Asignación incompatible: se esperaba 'int', se obtuvo 'string'.
  --> app.joss:15:5
   |
15 |     $edad = "veinte"
   |     ^^^^^
Sugerencia: Modifica el valor asignado o declara la variable como 'mixed'.
```
Cada parte da mensagem tem um propósito:
1. **Gravidade (`error` ou `warning`)**:
   - `error`: Problema crítico. Joss se recusará a executar o programa para evitar comportamentos imprevisíveis.
   - `warning`: Aviso informativo (como uma variável declarada que nunca foi usada). O programa pode ser executado, mas é recomendável limpá-lo.
2. **Código estável (`JOSS-TYPE-001`)**: Um identificador exclusivo que permite pesquisar a causa exata e exemplos na [Referência de diagnóstico](DIAGNOSTICOS.md).
3. **Local (`app.joss:15:5`)**: O arquivo exato, número de linha (`15`) e coluna (`5`) onde a discrepância foi detectada.
4. **Explicação e sugestão**: uma descrição em linguagem natural de qual regra foi quebrada e como corrigi-la.

---

## 3. Tratamento de exceções: `try`, `catch` e `throw`

Uma **exceção** é um sinal de alarme que interrompe o fluxo normal do programa quando ocorre uma situação imprevista que o código atual não consegue resolver sozinho.

- **`throw`**: Lança o alarme (a exceção).
- **`try`**: Delimita uma zona protegida de código onde suspeitamos que algo possa falhar.
- **`catch ($error)`**: Esta é a brigada de emergência. Se algo explodir dentro do bloco `try`, a execução salta imediatamente para o bloco `catch` para mitigar o problema:<!-- joss-run: ["No se pudo continuar: faltan datos"] -->
```joss
try {
    throw "faltan datos"
} catch ($error) {
    print("No se pudo continuar: " . $error)
}
```
### O que acontece neste exemplo?
1. O computador entra no bloco `try`.
2. Execute `throw "faltan datos"`. Nesse momento, a execução normal é interrompida.
3. O controle salta diretamente para o bloco `catch`.
4. A mensagem `"faltan datos"` é armazenada na variável `$error`.
5. O bloco `catch` imprime o aviso.
6. O programa não trava; continua executando as linhas após o `catch`.

---

## 4. Inspeção avançada de objetos de erro em `catch`

Quando o próprio tempo de execução do Joss gera um erro interno (chamado `JossError`, como um estouro aritmético ou um índice fora do intervalo), a variável `$error` do `catch` se torna um **mapa associativo** com informações técnicas detalhadas:```joss
try {
    $arr = [1, 2]
    print($arr[99]) // Índice fuera de rango
} catch ($e) {
    print("Mensaje: " . $e["message"])
    print("Archivo: " . $e["file"])
    print("Línea: " . $e["line"])
}
```
### Campos disponíveis no mapa de erros interno:
- `$e["message"]`: A mensagem descritiva da falha.
- `$e["type"]`: A categoria interna do erro (por exemplo, `"IndexOutOfRange"` ou `"ArithmeticFault"`).
- `$e["file"]`: O caminho para o arquivo de origem onde a falha se originou.
- `$e["line"]`: O número exato da linha.
- `$e["error"]`: A representação textual completa do erro.

### Exceções personalizadas com instâncias de classe

Se sua aplicação lança uma instância de uma classe (`throw new MiExcepcion(...)`), o bloco `catch ($e)` preserva a **instância ativa do objeto**, permitindo acesso direto aos seus métodos e propriedades especializados:<!-- joss-run: ["Campo: email", "Motivo: Formato inválido"] -->
```joss
public class ErrorValidacion {
    Init(public string $campo, public string $motivo) {}
}

try {
    throw new ErrorValidacion("email", "Formato inválido")
} catch ($e) {
    print("Campo: " . $e->campo)
    print("Motivo: " . $e->motivo)
}
```
---

## 5. Garantia de fluxo: `return` e loops dentro de `try`

Em muitas linguagens, colocar instruções de controle dentro de um bloco protegido pode causar um comportamento inesperado. Em Joss:

- Se você executar `return $valor` dentro de um bloco `try`, a função retornará imediatamente e o bloco `catch` **não intervirá**.
- Se você executar `break` ou `continue` dentro de um `try` que esteja em um loop, o loop irá quebrar ou prosseguir normalmente sem que `catch` confunda o salto com uma exceção.

Joss distingue internamente os sinais de controle de fluxo dos erros reais do usuário, garantindo que suas cláusulas de proteção funcionem de forma 100% previsível.

---

## 6. Exceções versus verificação de retorno

Nem todos os problemas devem ser resolvidos com `try / catch`. Na biblioteca padrão Joss, muitas operações retornam um valor especial (`null` ou `false`) quando uma consulta simplesmente não encontra resultados.

Por exemplo, lendo um arquivo que não existe:<!-- joss-run: ["No se pudo leer el archivo"] -->
```joss
$contenido = file_get_contents("archivo-que-no-existe.txt")
($contenido == null) ? {
    print("No se pudo leer el archivo")
} : {
    print($contenido)
}
```
- `file_get_contents(...)`: Retorna o texto do arquivo se existir, ou `null` se não puder ser lido. Não lança uma exceção destrutiva; permite verificar o resultado com um ternário simples.
- `file_put_contents(...)`: Retorna `true` se o arquivo foi gravado no disco ou `false` se houve falha de permissão.

### Quando usar cada abordagem?
- **Usar verificação de retorno (`$res == null`)**: Quando a ausência dos dados é uma possibilidade normal da aplicação (usuário buscando um produto que não existe no catálogo).
- **Use `throw` e `try / catch`**: Quando a falha representa uma condição crítica ou anormal da qual o código local não consegue se recuperar (a conexão com o servidor de pagamento foi perdida no meio do pagamento ou faltam variáveis ​​de ambiente essenciais para a inicialização).

---

## 7. Antipadrões: O que NÃO fazer

> [!CUIDADO]
> **Nunca silencie erros com um `catch`** vazio:```joss
// MALO: Esconde bugs catastróficos
try {
    iniciarBaseDeDatos()
} catch ($e) {}
```
> Se o banco de dados falhar ao iniciar, o programa continuará rodando às cegas e falhará incompreensivelmente dez linhas depois. No mínimo, registre o erro no console com `print($e["error"])` ou cancele a execução.

---

## 8. Exercício prático

1. **Validador de usuário com exceções**:
   - Escreva uma função `public func registrarEdad(int $edad): string`.
   - Se `$edad < 0`, lança uma exceção: `throw "La edad no puede ser negativa"`.
   - Se `$edad < 18`, throw: `throw "Debe ser mayor de edad para registrarse"`.
   - Se válido, retorna `"Registro exitoso"`.
   - Chame a função dentro de um bloco `try / catch`, tentando `-5`, `15` e `25`, e imprima o resultado ou a mensagem de erro capturada.

---

## Próxima etapa

Agora que seu código sabe como se defender contra falhas e se recuperar normalmente, é hora de aprender um dos recursos mais poderosos do Joss: como executar tarefas em paralelo, delegar operações em segundo plano e comunicar processos sem bloqueio.

Continue com: [Simultaneidade, assincronia, Futuro e canais](CONCURRENCIA.md).