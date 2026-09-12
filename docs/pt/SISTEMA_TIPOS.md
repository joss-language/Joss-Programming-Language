# Sistema de tipos, inferência e conversões

Antes: [Coleções: Matrizes, Mapas e Texto](COLECCIONES.md). Depois: [Classes e objetos](CLASES.md).
Referência técnica: [Diagnóstico](DIAGNOSTICOS.md), [Sintaxe](SINTAXIS.md).

---

## O que você vai aprender aqui?

Um **sistema de tipos** é o conjunto de regras que governa como um computador interpreta uns e zeros na memória. Sem tipos, uma sequência de 64 bits na memória poderia ser um número, uma letra, uma imagem ou uma instrução do processador; não haveria como saber.

Em Joss, o sistema de tipos serve a um propósito duplo:
1. **Segurança e robustez**: Detecta inconsistências (como tentar multiplicar texto por uma lista) antes que o código seja executado em produção.
2. **Clareza do documento**: ajuda qualquer desenvolvedor a entender imediatamente quais dados uma função espera e qual resultado ela produzirá.

Neste guia você aprenderá:
1. A lista canônica de tipos de dados em Joss e por que certos nomes antigos (*aliases*) não são mais válidos.
2. Como funciona a **inferência de tipos** e as diferenças entre `$x = ...`, `var`, `mixed` e declarações explícitas.
3. A operação de **tipos de união** (`T|U`) e valores anuláveis ​​opcionais (`T?`).
4. Como funciona a compatibilidade e expansão de números (`int → float → decimal`).
5. Coleções digitadas com genéricos (`array<T>` e `map<K, V>`).
6. A regra de coerção de string (`typesystem.CoerceString`) e proteções de segurança aritmética.

---

## 1. Inventário de tipos de fontes canônicas

A fonte canônica da verdade do compilador (`pkg/typesystem/types.go`) reconhece os seguintes tipos válidos:

| Tipo de fonte | Significado | Representação física em tempo de execução | Exemplo de uso |
|---|---|---|---|
| `int` | Inteiro assinado de 64 bits | `int64` (−9.223.372.036.854.775.808 a 9.223.372.036.854.775.807) | `int $id = 101` |
| `float` | Ponto flutuante binário padrão IEEE-754 | `float64` de 64 bits | `float $ratio = 0.75` |
| `decimal` | Número decimal de vírgula fixa na base dez | `shopspring/decimal.Decimal` (alta precisão) | `decimal $precio = 99.99m` |
| `string` | Sequência de caracteres UTF-8 | Vá `string` com suporte a grafema | `string $email = "ada@joss.red"` |
| `bool` | Valor de verdade lógica | `bool` (`true` ou `false`) | `bool $valido = true` |
| `array` | Sequência dinâmica de elementos | `[]interface{}` (fatia Go) | `array $items = [1, 2, 3]` |
| `map` | Tabela associativa com teclas de texto | `map[string]interface{}` (Go hashmap) | `map $datos = {"rol": "admin"}` |
| `object` | Instância genérica de uma classe | Instância de classe nativa ou de usuário | `object $instancia = new Persona()` |
| `channel` | Canal de comunicação concorrente | `*core.Channel` (canal de mensagem Go) | `channel $c = make_chan(1)` |
| `mixed` | Dinamismo explícito e polimorfismo | Qualquer valor de tempo de execução válido | `mixed $dato = "dinamico"` |
| `null` / `nil` | Ausência de valor | representação `nil` | `null` || Nome da classe | Tipo nominal nativo ou definido pelo usuário | Instância de classe correspondente | `Persona $p = new Persona()` |

### Aliases antigos removidos

> [!AVISO]
> Nas versões antigas da linguagem havia nomes alternativos herdados de outros ecossistemas como `integer`, `double`, `boolean`, `dynamic`, `any` ou `list`. **Esses nomes foram completamente removidos da gramática**.
>
> Se você escrever `integer $x = 10`, o analisador não irá convertê-lo para `int`; irá procurar por uma classe de usuário chamada `integer`. Caso não seja encontrado, emitirá o erro de diagnóstico `JOSS-TYPE-009` (Unresolved Type). Sempre use os nomes canônicos: `int`, `float`, `bool`, `mixed` e `array`.

---

## 2. Inferência e formas de declarar variáveis

Joss combina a agilidade das linguagens dinâmicas com a segurança das linguagens fortemente tipadas:<!-- joss-run: ["30", "Ada", "pendiente"] -->
```joss
var $edad = 20
$edad = 30
string $nombre = "Ada"
mixed $resultado = 10
$resultado = "pendiente"
print($edad)
print($nombre)
print($resultado)
```
### Regras de declaração semântica:

1. **Inferência por atribuição simples (`$x = 20`) ou com `var` (`var $x = 20`)**:
   - Na primeira tarefa, Joss inspeciona o valor e define o tipo concreto (neste caso `int`).
   - As atribuições subsequentes **devem ser compatíveis** com esse tipo. Se você tentar inserir texto, o analisador rejeitará o programa com `JOSS-TYPE-001`.
2. **Declaração explícita (`string $nombre = "Ada"`)**:
   - Definir o tipo de forma visível e documentada.
3. **Dinamismo voluntário (`mixed $resultado = 10`)**:
   - Indica ao analisador que esta variável mudará de natureza ao longo do tempo. Você pode reatribuir um texto, um mapa ou uma classe sem erros.
   - `let $resultado = 10` é um atalho sintático que produz exatamente uma variável `mixed`.
4. **Inicialização com `null`**:
   - Se você escrever `$x = null` sem um tipo, a inferência será **adiada** até a primeira atribuição que contenha um valor específico.

---

## 3. Tipos de união (`T|U`) e tipos anuláveis (`T?`)

No desenvolvimento real é muito comum que uma operação retorne um dado específico ou retorne `null` se nada for encontrado (por exemplo, procurando um usuário no banco de dados).

Para esses casos, Joss oferece **tipos de união**:<!-- joss-run: ["10", "A-10", "sin dato"] -->
```joss
int|string $id = 10
print($id)
$id = "A-10"
print($id)
int? $cantidad = null
print($cantidad ?? "sin dato")
```
### Regras do tipo união:

1. **Sintaxe com barra vertical (`|`)**: `int|string` significa que a variável aceitará apenas números inteiros ou textos, mas rejeitará booleanos ou listas.
2. **O atalho do ponto de interrogação (`?`)**: Digitar `int?` é exatamente equivalente a digitar `int|null`. O AST do compilador normaliza-o automaticamente para uma união com `null`.
3. **Refinamento de tipo (estreitamento) em ternários**:
   Se você tiver uma variável `string? $nombre` e perguntas `($nombre != null)`, dentro do branch verdadeiro o analisador sabe que `$nombre` não pode mais ser nulo, permitindo que você acesse suas operações de texto com segurança.

---

## 4. Compatibilidade e regras de atribuição

Quando um valor do tipo origem pode ser atribuído a uma variável do tipo destino?```text
       int ──────────► float ──────────► decimal
(Exacto 64 bits)    (Binario IEEE)     (Base 10 exacta)
```
1. **Mesmo tipo**: Sempre permitido.
2. **`int → float`**: Permitido automaticamente. Um número inteiro pode ser promovido para flutuante.
3. **`int → decimal` ou `float → decimal`**: Permitido automaticamente. Joss converte o valor para a representação decimal exata.
4. **`Clase → object`**: Qualquer instância de classe é compatível com o tipo universal `object`.
5. **`Subclase → Superclase`**: Uma classe derivada que estende uma classe base é aceita onde quer que a classe base seja esperada.
6. **`Clase → Interfaz`**: Uma classe que implementa uma interface (`implements`) é compatível onde a referida interface é declarada como um tipo.
7. **`mixed`**: É universalmente compatível em ambas as direções.

Qualquer outra mixagem (como tentar colocar uma `string` em um `int` ou um `bool` em um `array`) será bloqueada pelo analisador com `JOSS-TYPE-001` (Type Mismatch).

---

## 5. Coleções digitadas (genéricos de primeiro nível)

Embora Joss não possua modelos genéricos complexos em funções de usuário, ele permite a parametrização das duas principais estruturas de dados:<!-- joss-run: ["6", "2"] -->
```joss
array<int> $cantidades = [2, 4, 6]
map<string, int> $inventario = {"pan": 2}
print($cantidades[2])
print($inventario["pan"])
```
- `array<T>`: Um array onde todos os elementos devem ser do tipo `T`.
- `map<K, V>`: Um mapa com chaves do tipo `K` (deve ser `string`) e valores do tipo `V`.

Ao indexar uma coleção parametrizada (por exemplo `$cantidades[0]`), o analisador infere imediatamente que o resultado é do tipo `int`, garantindo segurança no restante do código.

---

## 6. Enums (`enum`)

**enumerações** permitem definir um tipo fechado com um conjunto finito de casos possíveis, evitando o uso de constantes esparsas ou strings mágicas.

Em Joss existem dois tipos de enumerações:

### Enums Puros (Enums de Unidade)
Cada caso representa um valor simbólico único com a propriedade `->name`:<!-- joss-run: ["Pendiente", "Aprobado"] -->
```joss
public enum Estado {
    case Pendiente
    case Aprobado
    case Rechazado
}

$e = Estado::Pendiente
print($e->name)
$e2 = Estado::Aprobado
print($e2->name)
```
### Enums apoiados
Eles associam cada caso a um valor escalar primitivo (`string` ou `int`):<!-- joss-run: ["admin", "admin", "Admin", "3"] -->
```joss
public enum Rol: string {
    case Admin = "admin"
    case Editor = "editor"
    case Lector = "lector"
}

$r = Rol::Admin
print($r->value)

// Instanciar desde valor escalar con from() o tryFrom()
$desdeValor = Rol::from("admin")
print($desdeValor->value)
print($desdeValor->name)

// Obtener todos los casos con cases()
$todos = Rol::cases()
print(count($todos))
```
---

## 7. Operadores de verificação de tipo: `is` e `instanceof`

O operador **`is`** (e seu alias **`instanceof`**) permite consultar em tempo de execução se um valor pertence a um tipo primitivo (`int`, `string`, `bool`, etc.), uma classe ou uma interface:<!-- joss-run: ["true", "true", "true"] -->
```joss
$numero = 42
$texto = "hola"
print($numero is int)
print($texto is string)
print($numero instanceof int)
```
---

## 8. Coerção Textual Digitada (`typesystem.CoerceString`)

Em aplicações web, os dados que chegam de formulários HTTP ou solicitações JSON são strings de texto bruto (por exemplo, `"8080"` ou `"true"`).

Joss implementa uma política de coerção textual compartilhada (`CoerceString`) tanto no analisador estático quanto no tempo de execução:<!-- joss-run: ["9000", "49.99", "true"] -->
```joss
int $puerto = "9000"
decimal $precio = "49.99"
bool $activo = "yes"
print($puerto)
print($precio)
print($activo)
```
### Tabela de entradas textuais aceitas:

| Tipo de destino | Strings de texto que Joss converte automaticamente |
|---|---|
| `int` | Textos com dígitos inteiros (`"9000"`, `"-42"`) ou números flutuantes sem parte fracionária (`"100.0"`). |
| `float` | Qualquer texto com notação decimal válida (`"3.1416"`, `"-0.05"`). |
| `decimal` | Textos numéricos limpos (`"49.99"`, `"120.50m"`). |
| `bool` | Aceita sem distinção entre maiúsculas e minúsculas: `"true"`, `"1"`, `"yes"` (como `true`); e `"false"`, `"0"`, `"no"`, `""` (como `false`). |

Se o texto não puder ser convertido (por exemplo `int $x = "manzana"`), o analisador emite um erro estático `JOSS-TYPE-002` ou o tempo de execução o rejeita defensivamente.

---

## 9. Precisão numérica e defesas de tempo de execução

| Regra de segurança | Comportamento em Joss | Diagnóstico |
|---|---|---|
| Estouro de inteiro | As operações em números inteiros de 64 bits (`+`, `-`, `*`) são verificadas em relação ao estouro. Se excederem os limites de 64 bits, a execução será interrompida imediatamente. | `JOSS-ARITH-001` |
| Divisão por zero | Dividir ou calcular o restante de um número por zero (`$n / 0` ou `$n % 0`) produz um erro controlado em vez de valores inesperados `NaN` ou `Infinity`. | `JOSS-ARITH-002` |
| Índice fora do intervalo | Acessar um índice negativo ou maior que o comprimento de uma lista ou string interrompe o programa de maneira estruturada. | `JOSS-INDEX-001` |

---

## 10. Diagnóstico de sistema de tipo comum

| Código | Significado | Solução habitual |
|---|---|---|
| `JOSS-TYPE-001` | Remapeamento com tipo incompatível (por exemplo, `$x = 1; $x = "hola"`). | Mantenha o tipo homogêneo ou declare a variável explicitamente como `mixed $x`. |
| `JOSS-TYPE-002` | Valor inicial incompatível com anotação de tipo. | Corrija o valor inicial para corresponder ao tipo declarado. |
| `JOSS-TYPE-008` | O valor retornado por uma função não corresponde ao tipo prometido em `: Tipo`. | Verifique a expressão `return` para que ela retorne o tipo prometido. |
| `JOSS-TYPE-009` | Tipo ou nome de classe inexistente (inclui aliases retirados, como `integer`). | Substitua `integer`, `double`, `boolean`, `any` por seus nomes canônicos: `int`, `float`, `bool`, `mixed`. |
| `JOSS-TYPE-010` | Uma função digitada pode terminar sem executar um `return` ou `throw`. | Certifique-se de que todas as ramificações ternárias concluam com um valor de retorno. |
| `JOSS-TYPE-011` | Um parâmetro não digitado (`func($x)`) foi declarado. | Insira o tipo do parâmetro: `func(int $x)` ou `func(mixed $x)`. |

---

## Próxima etapa

Agora que você entende o sistema de tipos, as uniões e as conversões seguras, daremos o próximo passo na modelagem de domínio e na programação orientada a objetos:

Continue com: [Classes, objetos, métodos e herança](CLASES.md).