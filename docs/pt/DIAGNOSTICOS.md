# Catálogo e referência de diagnósticos da linguagem

Antes: [Tratamento de erros e exceções](ERRORES.md), [Sistema de tipos](SISTEMA_TIPOS.md). Depois: [Analisador semântico AST](ANALIZADOR.md).
Índice geral: [Documentação de Joss](README.md).

---

## O que é um diagnóstico em Joss?

Um **diagnóstico** é um relatório técnico estruturado emitido pelas ferramentas de Joss (o compilador, o analisador semântico `joss analyze`, o linter ou o runtime defensivo) quando é detectada uma violação às regras sintáticas, de tipos ou de segurança da linguagem.

Ao contrário das mensagens de erro genéricas de ferramentas antigas, cada diagnóstico de Joss é projetado seguindo estes princípios:
1. **Identificador único e estável**: Cada regra tem um código imutável (por exemplo `JOSS-TYPE-001`).
2. **Localização exata**: Arquivo, linha e coluna precisa onde se originou o conflito.
3. **Explicação pedagógica**: Descreve claramente qual regra foi descumprida e por quê.
4. **Sugestão acionável**: Propõe a correção canônica recomendada.

### Níveis de severidade

- **`error`**: Impede a execução do programa (`joss run` e o compilador param). Representa uma falha estrutural ou de segurança.
- **`warning`**: Aviso informativo (como uma variável declarada que nunca foi usada ou uma convenção de nomes desalinhada). `joss analyze` finaliza com código de saída `0` se houver apenas avisos, permitindo continuar a execução.

---

## 1. Parser, carregamento de projetos e tabela de símbolos

| Código | Categoria | Significado e Causa | Exemplo incorreto | Solução e Caso correto |
|---|---|---|---|---|
| `JOSS-IO-001` | Entrada / Saída | Não é possível ler o arquivo-fonte no disco (permissões ou caminho inexistente). | `joss run fantasma.joss` | Verificar se o arquivo existe no caminho indicado com permissões de leitura. |
| `JOSS-PARSE-001` | Sintaxe | Token ou estrutura gramatical inválida: falta uma aspa, chave ou visibilidade obrigatória. | `class MiClase {}`<br>`func prueba() {}` | Usar sintaxe canônica:<br>`public class MiClase {}`<br>`public func prueba() {}` |
| `JOSS-SYM-001` | Símbolos | Variável não declarada: tenta-se ler ou atribuir a uma variável que não foi declarada previamente. | `print($variable)`<br>`$nueva = 10` | Declarar a variável antes de usá-la:<br>`var $variable = "valor"`<br>`var $nueva = 10` |
| `JOSS-SYM-002` | Símbolos | Redeclaração de variável no mesmo escopo léxico. | `int $x = 1`<br>`int $x = 2` | Reatribuir sem declarar novamente:<br>`$x = 2` |
| `JOSS-SYM-003` | Símbolos | Função não resolvida: invoca-se uma função que não existe no projeto nem nos built-ins. | `calcularTotal()` | Definir a função com `public func` ou verificar a ortografia do nome. |
| `JOSS-SYM-004` | Símbolos | Classe não resolvida: tentativa de instanciar (`new`) uma classe não declarada nem registrada. | `var $p = new Persona()` | Declarar `public class Persona {}` ou carregar o plugin que a exponha. |
| `JOSS-SYM-005` | Símbolos | Herança inválida: a classe especificada em `extends` não existe ou cria herança circular. | `public class A extends B {}` | Certificar-se de que a superclasse `B` esteja declarada no projeto sem ciclos. |
| `JOSS-SYM-006` | Símbolos | Tentativa de reatribuir uma constante imutável. | `const int $MAX = 5`<br>`$MAX = 10` | Se o valor deve mudar, declará-la como variável mutável: `var $MAX = 5`. |
| `JOSS-SYM-007` | Símbolos | Interface não resolvida: a interface especificada em `implements` ou `extends` não existe. | `public class A implements IDesconocida {}` | Certificar-se de declarar a interface ou verificar sua ortografia. |
| `JOSS-SYM-008` | Símbolos | Herança cíclica em interfaces: uma interface estende a si mesma direta ou indiretamente. | `public interface A extends B {}`<br>`public interface B extends A {}` | Quebrar o ciclo de herança entre as interfaces. |
| `JOSS-DECL-001` | Declarações | Conflito de nomes: duas funções globais têm exatamente o mesmo identificador. | Dois arquivos com:<br>`public func procesar() {}` | Renomear uma das duas funções. Em Joss não há namespaces de origem por pasta. |
| `JOSS-DECL-002` | Declarações | Conflito de classes: duas classes globais têm o mesmo nome no projeto. | Dois arquivos com:<br>`public class Usuario {}` | Manter uma única declaração canônica da classe em todo o projeto. |
| `JOSS-DECL-003` | Declarações | Métodos duplicados em uma mesma classe/interface, ou interface desconhecida/duplicada em `implements`. | `public func id() {}`<br>`public func id(int $x) {}` | Joss não admite sobrecarga de métodos por assinatura; use nomes descritivos distintos. |
| `JOSS-DECL-004` | Declarações | Conflito de nomes entre tipos nominais (classe/interface/enum) ou descumprimento de contrato de interface (`implements`). | `public class MiClase implements IFigura {}` (sem `calcularArea()`) | Usar nomes únicos para cada tipo e implementar todos os métodos da interface com assinaturas compatíveis. |
| `JOSS-DECL-005` | Declarações | Classe concreta sem implementar um método `abstract` herdado, ou método `abstract` declarado em classe não abstrata. | `public class Perro extends Animal {}` (faltando `hablar()`) | Implementar todos os métodos abstratos na subclasse concreta ou marcar a classe como `abstract`. |
| `JOSS-DECL-006` | Declarações / Migração | Tentativa de instanciar uma classe `abstract` com `new` (`error`), ou uso da palavra histórica `let` (`warning` em modo migração). | `new AnimalAbstracto()`<br>`let $valor = 1` | Instancie uma subclasse concreta; para variáveis use `var`, tipo explícito ou `mixed`. |
| `JOSS-DECL-007` | Migração | Uma atribuição criou uma variável implicitamente sem declaração prévia. | `$valor = 1` | Declare antes com `var`, tipo explícito, `mixed` ou `const`. |
| `JOSS-DECL-008` | Migração | Usou-se a forma histórica `Init constructor(...)`. | `Init constructor() {}` | Migre para `public func constructor()` ou `Init()`. |
| `JOSS-DECL-010` | Migração | Usou-se `nil`, alias histórico de `null`. | `var $valor = nil` | Use `null`; `joss fix` aplica a alteração sem tocar em strings nem comentários. |
| `JOSS-SEM-001` | Semântica | Método de interface implementado com visibilidade não pública. | `private func calcularArea(): int` | Todos os métodos que implementam uma interface devem ser `public`. |
| `JOSS-CONST-001` | Constantes | Tentativa de modificar um campo imutável de um `record` ou propriedade `const`. | `$coordenada->x = 5` | Os campos de um `record` e propriedades `const` são somente leitura após a inicialização. |

---

## 2. Sistema de tipos, chamadas e acessibilidade

| Código | Categoria | Significado e Causa | Exemplo incorreto | Solução e Caso correto |
|---|---|---|---|---|
| `JOSS-TYPE-001` | Tipos | Reatribuição incompatível: atribui-se um dado de tipo distinto a uma variável tipada ou inferida. | `var $edad = 20`<br>`$edad = "veinte"` | Manter o tipo homogêneo, ou usar dinamismo voluntário:<br>`mixed $edad = 20`<br>`$edad = "veinte"` |
| `JOSS-TYPE-002` | Tipos | Valor inicial incompatível com a anotação de tipo explícita. | `int $x = "texto"` | Fornecer um valor do tipo esperado ou uma cadeia conversível segundo `CoerceString`. |
| `JOSS-TYPE-003` | Tipos | Argumento incompatível com o parâmetro tipado de uma função ou método. | `public func f(int $n) {}`<br>`f("hola")` | Passar o tipo correto ou converter o argumento antes da chamada. |
| `JOSS-TYPE-004` | Tipos | Operador aplicado a operandos não suportados (ex. somar texto com `+`). | `"hola" + " mundo"` | Para aritmética use números; para unir texto use o operador ponto (`.`): `"hola" . " mundo"`. |
| `JOSS-TYPE-005` | Tipos | Tipo de chave não suportado em um mapa associativo. | `{[1, 2]: "valor"}` | As chaves de um `map` devem ser do tipo `string`. |
| `JOSS-TYPE-006` | Tipos | Tipo de índice incorreto para uma coleção conhecida. | `$arr["clave"]` (em um array)<br>`$map[true]` (em um map) | Os arrays são indexados com inteiros (`$arr[0]`); os maps com cadeias (`$map["clave"]`). |
| `JOSS-TYPE-007` | Tipos | Tentativa de indexar com `[...]` um valor que não é indexável (ex. um inteiro ou booleano). | `var $n = 42`<br>`print($n[0])` | Indexar unicamente arrays, maps, cadeias ou instâncias compatíveis. |
| `JOSS-TYPE-008` | Tipos | O valor retornado por `return` não coincide com o tipo prometido na assinatura `: Tipo`. | `public func f(): int {`<br>`    return "no es int"`<br>`}` | Devolver um valor compatível com a assinatura declarada. |
| `JOSS-TYPE-009` | Tipos | Tipo de dado ou classe inexistente (inclui aliases eliminados como `integer`, `double`, `boolean`, `any`, `list`). | `integer $x = 10`<br>`boolean $flag = true` | Usar os tipos canônicos de Joss:<br>`int $x = 10`<br>`bool $flag = true` |
| `JOSS-TYPE-010` | Tipos | Função com tipo de retorno anotado pode terminar sem executar um `return` ou `throw`. | `public func f(int $n): string {`<br>`    $n > 0 ? { return "si" } : {}`<br>`}` | Garantir que todas as rotas possíveis retornem um valor do tipo prometido. |
| `JOSS-TYPE-011` | Tipos | Declarou-se um parâmetro sem tipo explícito. | `public func f($x) {}` | Em Joss todos os parâmetros devem declarar seu tipo:<br>`public func f(int $x) {}` ou `public func f(mixed $x) {}` |
| `JOSS-TYPE-012` | Warning | Uma coleção mutável sem parâmetros é usada como coleção tipada. O alias original poderia inserir valores incompatíveis. | `array $origen = [1]`<br>`array<int> $numeros = $origen` | Manter o tipo desde a declaração ou validar e copiar no limite: `array<int> $origen = [1]`. |
| `JOSS-TYPE-014` | Migração | Uma callable com nome omitiu seu contrato de retorno. | `public func guardar() {}` | Declare `: Tipo` ou `: void` quando não retornar um valor. |
| `JOSS-ARITH-003` | Aritmética | Uma operação exige uma conversão numérica implícita que pode perder precisão. | `9007199254740993 / 1`<br>`1 + 0.5`<br>`0.1 + 1.00m` | Usar operandos do mesmo tipo e converter explicitamente; para dinheiro, conservar toda a operação em `decimal`. |
| `JOSS-CALL-001` | Chamadas | Quantidade incorreta de argumentos em relação aos parâmetros da assinatura conhecida. | `public func f(int $a, int $b) {}`<br>`f(1)` | Fornecer todos os argumentos obrigatórios exigidos pela função. |
| `JOSS-MEMBER-001` | Membros | Tenta-se invocar um método que não existe na classe receptora resolvida. | `$usuario->metodoInexistente()` | Verificar o nome do método na definição da classe ou no catálogo nativo. |
| `JOSS-ACCESS-001` | Visibilidade | Tenta-se usar uma classe ou função declarada como `private` a partir de outro arquivo. | Chamar uma função privada de outro arquivo. | Declarar a função ou classe como `public` se deve ser compartilhada no projeto. |
| `JOSS-ACCESS-002` | Visibilidade | Tentativa de acessar uma propriedade ou método `private` ou `protected` fora de seu escopo autorizado. | `$cuenta->saldo` (sendo privado) | Acessar através de métodos públicos autorizados (getters/setters). |

---

## 3. Referências temporárias (`ref`)

| Código | Categoria | Significado e Causa | Solução |
|---|---|---|---|
| `JOSS-REF-001` | Referências | Falta o marcador bilateral `ref` na chamada ou na definição, ou cruzamento ilegal para nativo/async. | Se a função espera `ref int $x`, a chamada deve ser `f(ref $x)`. |
| `JOSS-REF-002` | Referências | Passa-se uma expressão, um literal, um campo de objeto ou um índice de array como referência. | Passar unicamente variáveis locais mutáveis diretas (`ref $miVariable`). |
| `JOSS-REF-003` | Referências | A variável passada como referência é uma constante imutável (`const`). | Uma referência muta o valor original; você não pode passar constantes para um parâmetro `ref`. |
| `JOSS-REF-004` | Referências | Discrepância de tipo na referência (o tipo da variável não coincide exatamente com o parâmetro). | As referências são estritamente invariantes: um `ref float` exige exatamente uma variável `float`. |
| `JOSS-REF-005` | Referências | Tentativa de armazenar, capturar em closure, retornar ou fazer escapar uma referência. | Uma referência só vive durante a chamada; para devolver dados use `return`. |
| `JOSS-REF-006` | Referências | Parâmetro `ref` declarado com um valor padrão. | Os parâmetros por referência não admitem valores padrão; remova o `= valor`. |

---

## 4. Controle de fluxo, linter e templates

| Código | Severidade | Significado e Causa | Solução |
|---|---|---|---|
| `JOSS-FLOW-001` | Warning | Código inalcançável (*dead code*): instruções escritas após um `return` incondicional. | Mover as instruções antes do `return` ou removê-las. |
| `JOSS-FLOW-002` | Warning / Error | Match não exaustivo em enum (sem cobrir todos os casos nem `default`), ou `break`/`continue` fora de um laço. | Adicionar os casos de enum faltantes ou incluir `default => ...`; usar `break`/`continue` apenas dentro de laços. |
| `JOSS-FLOW-003` | Warning | Acesso a membro sobre um valor cujo tipo ainda inclui `null`. | Verificar o valor contra `null` ou usar o operador seguro `?->`. |
| `JOSS-FLOW-005` | Error | O bloco `: { ... }` de uma cláusula `guard` não termina o fluxo do callable com `return` ou `throw`. | Certificar-se de que o bloco `guard` finalize sempre com `return` ou `throw`. |
| `JOSS-LINT-001` | Warning | Variável local declarada mas nunca lida no corpo. | Utilizar a variável ou retirá-la para manter o código limpo. |
| `JOSS-SYNTAX-001` | Error | Erro sintático capturado durante a fase de análise do linter. | Corrigir a pontuação ou estrutura apontada pelo parser. |
| `JOSS-LINT-002` | Error | Parâmetro sem tipo explícito reportado pelo linter. | Adicionar a anotação de tipo correspondente (`int`, `string`, `mixed`). |
| `JOSS-LINT-007` | Warning | Desvio das convenções de nomes do projeto (classes em PascalCase, funções em camelCase). | Ajustar o nome ao padrão canônico. |
| `JOSS-SEC-001` | Warning | Detecção heurística de um possível segredo sensível escrito em texto puro (chaves API, tokens). | Mover o segredo para o arquivo de configuração de ambiente (`env.joss`). |
| `JOSS-VIEW-001` | Error | Falha crítica ao compilar ou analisar um template de view HTML. | Verificar o fechamento de diretivas (`@foreach`, `@endforeach`) e arquivos incluídos. |
| `JOSS-VIEW-SYNTAX` | Error | Expressão de Joss mal formada dentro de uma tag de view `{{ ... }}`. | Corrigir a sintaxe da expressão embutida. |
| `JOSS-VIEW-UNDEF` | Warning | Variável de view acessada sem proteção contra valores indefinidos ou nulos. | Proteger com o operador de coalescência nula: `{{ $variable ?? 'default' }}`. |

---

## 5. Erros aritméticos e indexação em tempo de execução

| Código | Categoria | Operação inválida em Runtime | Como prevenir |
|---|---|---|---|
| `JOSS-ARITH-001` | Runtime | Estouro de inteiro com sinal de 64 bits (−2⁶³ a 2⁶³−1). | Validar os limites antes de operar ou utilizar o tipo `decimal` para cálculos de grande escala. |
| `JOSS-ARITH-002` | Runtime | Divisão ou módulo por zero (`$x / 0` ou `$x % 0`). | Verificar se o divisor é diferente de zero antes de executar a divisão: `($divisor != 0) ? ($x / $divisor) : 0.0`. |
| `JOSS-INDEX-001` | Runtime | Índice fora do intervalo: acesso a posição negativa ou maior/igual ao comprimento. | Verificar `count($arr)` antes de indexar ou comprovar a existência com `array_key_exists`. |
| `JOSS-INDEX-002` | Runtime | Tipo de índice não compatível com a estrutura de dados. | Indexar arrays com números inteiros e maps com cadeias de texto. |

### Armazenamento (`joss doctor storage`)

Diagnósticos de configuração emitidos por `joss doctor storage`. Não são erros de linguagem, mas compartilham o mesmo modelo de diagnóstico.

| Código | O que indica | Como resolver |
|---|---|---|
| `JOSS-STORAGE-000` | Verificação aprovada (severidade `info`). | Nada a fazer. |
| `JOSS-STORAGE-001` | `STORAGE` possui um valor desconhecido. | Usar `STORAGE=local` ou `STORAGE=OCI`. |
| `JOSS-STORAGE-002` | Chave privada ausente, não é PEM, não é RSA ou está criptografada sem `OCI_PASSPHRASE`. | Corrigir `OCI_PRIVATE_KEY_PATH` ou gerar nova API key. |
| `JOSS-STORAGE-003` | Fingerprint da chave PEM local não coincide com `OCI_FINGERPRINT`. | Usar a chave PEM que corresponde ao fingerprint registrado na OCI, ou atualizar `OCI_FINGERPRINT`. |
| `JOSS-STORAGE-004` | Variável `OCI_*` ausente ou formato inválido. | Copiar os valores exatos da console OCI. |
| `JOSS-STORAGE-005` | OCI rejeitou a autenticação (401) ou ficou inalcançável. | Registrar a chave pública no usuário; se recente, aguardar alguns minutos. |
| `JOSS-STORAGE-006` | `OCI_NAMESPACE` não pertence à tenancy; a sugestão inclui o real. | Usar o namespace sugerido. |
| `JOSS-STORAGE-007` | O bucket não existe no namespace ou faltam permissões. | Criar o bucket, corrigir `OCI_BUCKET_NAME` ou revisar políticas IAM. |
| `JOSS-STORAGE-008` | `./storage` não é gravável com `STORAGE=local`. | Revisar permissões e espaço em disco. |
| `JOSS-STORAGE-009` | O provedor (`s3`, `aws`, `azure`) ainda não possui driver. | Usar `local` ou `OCI`. |

Exemplo inválido: `OCI_NAMESPACE=axghxqmjl3jf` com credenciais de outra tenancy produz `JOSS-STORAGE-006`. Vizinho válido: mesmo arquivo com o namespace indicado na sugestão.

---

## 6. Casos completos de teste verificados

As operações de canal também possuem defesas runtime estáveis. `JOSS-CHANNEL-001`
ocorre ao fechar duas vezes um canal ou enviar após o fechamento; o vizinho
válido é enviar enquanto está aberto e fechá-lo uma única vez. `JOSS-CHANNEL-002`
ocorre com `make_chan(-1)`; `make_chan(0)` é uma capacidade válida para um
canal sem buffer. Esses estados podem depender de outra tarefa concorrente, portanto
o analyzer não pode substituir a defesa runtime.

A seguir apresentam-se exemplos executáveis que validam formalmente a emissão dos diagnósticos e sua contraparte correta:

### Tipo inexistente ou alias eliminado (`JOSS-TYPE-009`)

<!-- joss-error: JOSS-TYPE-009 -->
```joss-invalid
integer $edad = 20
```

Caso corrigido com o tipo canônico `int`:

<!-- joss-run: ["20"] -->
```joss
int $edad = 20
print($edad)
```

---

### Parâmetro sem tipo explícito (`JOSS-TYPE-011`)

<!-- joss-error: JOSS-TYPE-011 -->
```joss-invalid
public func duplicar($valor) { return $valor * 2 }
```

Caso corrigido tipando o parâmetro e o retorno:

<!-- joss-run: ["6"] -->
```joss
public func duplicar(int $valor): int { return $valor * 2 }
print(duplicar(3))
```

---

### Reatribuição com tipo incompatível (`JOSS-TYPE-001`)

<!-- joss-error: JOSS-TYPE-001 -->
```joss-invalid
var $edad = 20
$edad = "veinte"
```

Caso corrigido usando dinamismo voluntário (`mixed`):

<!-- joss-run: ["veinte"] -->
```joss
mixed $dato = 20
$dato = "veinte"
print($dato)
```

---

### Estreitamento inseguro de coleção mutável (`JOSS-TYPE-012`)

A compatibilidade herdada permite a operação, mas o analyzer avisa porque ambos os nomes podem compartilhar a mesma coleção mutável.

<!-- joss-error: JOSS-TYPE-012 -->
```joss-invalid
array $origen = [1]
array<int> $numeros = $origen
```

Caso seguro mantendo o tipo desde a origem:

<!-- joss-run: ["1"] -->
```joss
array<int> $origen = [1]
array<int> $numeros = $origen
print($numeros[0])
```

---

### Retorno ausente em rotas de fluxo (`JOSS-TYPE-010`)

<!-- joss-error: JOSS-TYPE-010 -->
```joss-invalid
public func signo(int $n): string {
    $n > 0 ? { return "positivo" } : {}
}
```

Caso corrigido garantindo retorno em todas as rotas possíveis:

<!-- joss-run: ["no positivo"] -->
```joss
public func signo(int $n): string {
    return $n > 0 ? "positivo" : "no positivo"
}
print(signo(0))
```

---

### Conversão numérica com perda (`JOSS-ARITH-003`)

<!-- joss-error: JOSS-ARITH-003 -->
```joss-invalid
var $resultado = 9007199254740993 / 1
```

Caso corrigido conservando precisão decimal:

<!-- joss-run: ["9007199254740993"] -->
```joss
decimal $resultado = 9007199254740993m / 1m
print($resultado)
```

---

### Match não exaustivo em enum (`JOSS-FLOW-002`)

<!-- joss-error: JOSS-FLOW-002 -->
```joss-invalid
public enum Estado {
    case Activo;
    case Inactivo;
}

public func describir(Estado $e): string {
    return match ($e) {
        Estado::Activo => "activo",
    }
}
```

Caso corrigido cobrindo exaustivamente todos os casos do enum:

<!-- joss-run: ["activo"] -->
```joss
public enum Estado {
    case Activo;
    case Inactivo;
}

public func describir(Estado $e): string {
    return match ($e) {
        Estado::Activo => "activo",
        Estado::Inactivo => "inactivo",
    }
}
print(describir(Estado::Activo))
```

---

### Acesso potencialmente nulo (`JOSS-FLOW-003`)

<!-- joss-error: JOSS-FLOW-003 -->
```joss-invalid
public class Usuario { public string $nombre = "Ada" }
public func nombre(Usuario|null $usuario): string {
    return $usuario->nombre
}
```

Caso corrigido mediante acesso seguro:

<!-- joss-run: ["sin usuario"] -->
```joss
public class Usuario { public string $nombre = "Ada" }
public func nombre(Usuario|null $usuario): string {
	return $usuario == null ? "sin usuario" : $usuario->nombre
}
print(nombre(null))
```

---

## Próximo passo

Agora que você conhece todos os diagnósticos e como resolvê-los, pode aprofundar-se em como o analisador semântico examina a árvore de sintaxe abstrata para emitir esses códigos:

Continue com: [Analisador estático AST](ANALIZADOR.md).
