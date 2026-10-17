# Sistema de tipos, inferência e conversões

Antes: [Projeto web completo](PROYECTO_WEB.md), [Coleções](COLECCIONES.md). Depois: [Referência de sintaxe](SINTAXIS.md), [Diagnósticos](DIAGNOSTICOS.md).
Referência técnica: [Diagnósticos](DIAGNOSTICOS.md), [Sintaxe](SINTAXIS.md).

---

## O que você vai aprender aqui?

Um **sistema de tipos** é o conjunto de regras que governa como um computador interpreta os uns e zeros na memória. Sem tipos, uma sequência de 64 bits na memória poderia ser um número, uma letra, uma imagem ou uma instrução de processador; não haveria como saber.

Em Joss, o sistema de tipos cumpre um duplo propósito:
1. **Segurança e robustez**: Detecta incoerências (como tentar multiplicar um texto por uma lista) antes que o código seja executado em produção.
2. **Clareza documental**: Ajuda qualquer desenvolvedor a compreender imediatamente quais dados uma função espera e que resultado vai produzir.

Neste guia você aprenderá:
1. A lista canônica de tipos de dados em Joss e por que certos nomes antigos (*aliases*) já não são válidos.
2. Como funciona a **inferência de tipos** e as diferenças entre `var`, tipos explícitos, `mixed` e `const`.
3. O funcionamento dos **tipos união** (`T|U`), valores opcionais anuláveis (`T?`) e o tipo de retorno `: void`.
4. Como funciona a compatibilidade e a promoção numérica exata (`int → decimal`) e por que misturar com `float` exige conversão explícita (`JOSS-ARITH-003`).
5. Coleções tipadas com genéricos (`array<T>`, `map<K, V>`, `channel<T>`, `Result<T, E>`).
6. A regra de coerção de cadeias (`typesystem.CoerceString`) e as proteções aritméticas de segurança.

---

## 1. Inventário de tipos-fonte canônicos

A fonte da verdade canônica do compilador (`pkg/typesystem/types.go`) reconhece os seguintes tipos válidos:

| Tipo-fonte | Significado | Representação física no Runtime | Exemplo de uso |
|---|---|---|---|
| `int` | Inteiro com sinal de 64 bits | `int64` (−9,223,372,036,854,775,808 a 9,223,372,036,854,775,807) | `int $id = 101` |
| `float` | Ponto flutuante binário padrão IEEE-754 | `float64` de 64 bits | `float $ratio = 0.75` |
| `decimal` | Número decimal de ponto fixo na base dez | `shopspring/decimal.Decimal` (alta precisão) | `decimal $precio = 99.99m` |
| `string` | Sequência de caracteres UTF-8 | `string` de Go com suporte a grafemas | `string $email = "ada@joss.red"` |
| `bool` | Valor lógico de verdade | `bool` (`true` ou `false`) | `bool $valido = true` |
| `array` | Sequência dinâmica de elementos | `[]interface{}` (slice de Go) | `array $items = [1, 2, 3]` |
| `map` | Tabela associativa com chaves de texto | `map[string]interface{}` (hashmap de Go) | `map $datos = {"rol": "admin"}` |
| `object` | Instância genérica de uma classe | Instância de classe de usuário ou nativa | `object $instancia = new Persona()` |
| `channel` | Canal de comunicação concorrente | `*core.Channel` (canal Go de mensagens) | `channel $c = make_chan(1)` |
| `mixed` | Dinamismo explícito e polimorfismo | Qualquer valor válido do runtime | `mixed $dato = "dinamico"` |
| `void` | Terminação sem valor (apenas retorno de callable) | Sem valor de retorno (`nil`) | `public func log(): void` |
| `null` | Ausência de valor | Representação `nil` | `null` |
| Nome de classe / interface / enum | Tipo nominal definido pelo usuário ou nativo | Instância ou caso correspondente | `Persona $p = new Persona()` |

### Os antigos aliases eliminados

> [!WARNING]
> Em versões antigas da linguagem existiam nomes alternativos herdados de outros ecossistemas como `integer`, `double`, `boolean`, `dynamic`, `any` ou `list`. **Esses nomes foram completamente eliminados da gramática**.
>
> Se você escrever `integer $x = 10`, o analisador não o converterá para `int`; procurará uma classe de usuário chamada `integer`. Ao não encontrá-la, emitirá o erro de diagnóstico `JOSS-TYPE-009` (Tipo não resolvido). Utilize sempre os nomes canônicos: `int`, `float`, `bool`, `mixed` e `array`.

---

## 2. Inferência e formas de declarar variáveis

Joss combina a agilidade da inferência moderna com a segurança das linguagens fortemente tipadas:

<!-- joss-run: ["30", "Ada", "pendiente"] -->
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

### Regras semânticas de declaração:

1. **Inferência canônica com `var` (`var $x = 20`)**:
   - Na declaração, Joss inspeciona o valor inicial e fixa o tipo concreto (neste caso `int`).
   - As reatribuições posteriores (`$x = 30`) **devem ser compatíveis** com esse tipo. Se você tentar atribuir um texto, o analisador rejeitará o programa com `JOSS-TYPE-001`.
2. **Declaração explícita (`string $nombre = "Ada"`)**:
   - Fixa o tipo de forma visível e documentada.
3. **Dinamismo voluntário (`mixed $resultado = 10`)**:
   - Indica ao analisador que esta variável mudará de natureza ao longo do tempo. Você pode reatribuir um texto, um mapa ou uma classe sem erros.
   - `let $resultado = 10` é uma sintaxe histórica em transição (`JOSS-DECL-006`); prefira sempre `mixed $resultado = 10`.
4. **Proibição de variáveis não declaradas (`$x = 20` direto)**:
   - Atribuir pela primeira vez a uma variável sem tê-la declarado previamente com `var`, um tipo explícito, `mixed` ou `const` emite `JOSS-SYM-001` (e o aviso de transição `JOSS-DECL-007`).
5. **Inicialização com `null`**:
   - Se você escrever `var $x = null`, a inferência é **adiada** até a primeira atribuição que contenha um valor concreto.

---

## 3. Tipos União (`T|U`) e tipos anuláveis (`T?`)

No desenvolvimento real é muito comum que uma operação possa devolver um dado concreto ou `null` se nada for encontrado (por exemplo, buscar um usuário no banco de dados).

Para esses casos, Joss oferece **tipos união**:

<!-- joss-run: ["10", "A-10", "sin dato"] -->
```joss
int|string $id = 10
print($id)
$id = "A-10"
print($id)
int? $cantidad = null
print($cantidad ?? "sin dato")
```

### Regras dos tipos união:

1. **Sintaxe com barra vertical (`|`)**: `int|string` significa que a variável só aceitará inteiros ou textos, mas rejeitará booleanos ou listas.
2. **O atalho de interrogação (`?`)**: Escrever `int?` é exatamente equivalente a escrever `int|null`. O AST do compilador o normaliza automaticamente para uma união com `null`.
3. **Refinamento de tipos (Smart Cast Narrowing)**:
   Se você tiver uma variável `string? $nombre` e a filtrar com `guard ($nombre != null) : { return "" }` ou com um ternário `($nombre != null) ? { ... }`, o analisador estreita automaticamente seu tipo para `string` não nulo na rota validada.

---

## 4. Regras de compatibilidade e promoção numérica

Quando um valor de tipo origem pode ser atribuído a uma variável de tipo destino?

```text
int ──────────► decimal
    promoción exacta (única implícita)
```

1. **Mesmo tipo**: Sempre permitido.
2. **`int → decimal`**: É a **única promoção numérica implícita** entre tipos distintos porque conserva o valor exato na base dez sem perda de precisão.
3. **`int → float` e `float → decimal`**: **Exigem conversão explícita** (`floatval($entero)` ou `decimal($flotante)`). A primeira pode perder precisão em inteiros grandes ($> 2^{53}$) e a segunda só conserva a aproximação binária prévia. Tentar misturá-los implicitamente emite `JOSS-ARITH-003` ou `JOSS-TYPE-001`/`002`.
4. **`Clase → object`**: Qualquer instância de classe é compatível com o tipo universal `object`.
5. **`Subclase → Superclase`**: Uma classe derivada que estende uma classe base é aceita onde se espera a classe base.
6. **`Clase → Interfaz`**: Uma classe que implementa uma interface (`implements`) é compatível onde tal interface for declarada como tipo.
7. **`mixed`**: É universalmente compatível em ambas as direções.

As operações que misturam `int` com `float`, ou `float` com `decimal`, também
exigem converter deliberadamente um dos operandos. A divisão de dois
inteiros produz `float`; se um operando não puder ser representado exatamente em IEEE-754, o
analyzer emite `JOSS-ARITH-003` quando o conhece e o runtime mantém a mesma
defesa. Para valores monetários, percentuais e impostos use literais `decimal` (`19.99m`) e evite
passar primeiro por `float`.

---

## 5. Coleções tipadas (Genéricos de primeiro nível)

Joss permite parametrizar coleções e contêineres:

<!-- joss-run: ["6", "2"] -->
```joss
array<int> $cantidades = [2, 4, 6]
map<string, int> $inventario = {"pan": 2}
print($cantidades[2])
print($inventario["pan"])
```

- `array<T>`: Um array onde todos os elementos devem ser do tipo `T`.
- `map<K, V>`: Um mapa com chaves do tipo `K` (devem ser `string`) e valores do tipo `V`.
- `channel<T>`: Um canal de concorrência tipado onde as mensagens transmitidas devem ser do tipo `T` (ex. `channel<string>`).
- `Result<T, E>`: Tipo canônico para resultados de operações que podem ter sucesso com valor do tipo `T` ou falhar com erro do tipo `E`.
- `Clase<T>`: Classes com parâmetros formais de tipo genérico (ex. `Caja<T>`).

`array<T>`, `map<K, V>` e `channel<T>` são mutáveis e por isso seus parâmetros são **invariantes**. Por exemplo, `array<int>` não pode ser reutilizado como `array<decimal>`, embora um `int` individual possa ser promovido a `decimal`: permitir o alias deixaria inserir depois um `decimal` fracionário visível a partir de uma referência que promete conter apenas `int`. As coleções sem parâmetro conservam compatibilidade por transição, mas ao cruzar para uma coleção tipada emitem `JOSS-TYPE-012` porque outro alias ainda poderia mutá-las sem respeitar o tipo.

Ao indexar uma coleção parametrizada (por exemplo `$cantidades[0]`), o analisador infere imediatamente que o resultado é do tipo `int`, garantindo a segurança no restante do código.

---

## 6. Enumerações (`enum`)

As **enumerações** permitem definir um tipo fechado com um conjunto finito de casos possíveis, evitando o uso de constantes dispersas ou strings mágicas.

Em Joss existem dois tipos de enumerações:

### Enums puros (Unit Enums)
Cada caso representa um valor simbólico único com a propriedade `->name`:

<!-- joss-run: ["Pendiente", "Aprobado"] -->
```joss
public enum Estado {
    case Pendiente
    case Aprobado
    case Rechazado
}

var $e = Estado::Pendiente
print($e->name)
var $e2 = Estado::Aprobado
print($e2->name)
```

### Enums apoiados (Backed Enums)
Associam cada caso a um valor escalar primitivo (`string` ou `int`):

<!-- joss-run: ["admin", "admin", "Admin", "3"] -->
```joss
public enum Rol: string {
    case Admin = "admin"
    case Editor = "editor"
    case Lector = "lector"
}

var $r = Rol::Admin
print($r->value)

// Instanciar desde valor escalar con from() o tryFrom()
var $desdeValor = Rol::from("admin")
print($desdeValor->value)
print($desdeValor->name)

// Obtener todos los casos con cases()
var $todos = Rol::cases()
print(count($todos))
```

---

## 7. Operadores de verificação de tipo: `is` e `instanceof`

O operador **`is`** (e seu alias **`instanceof`**) permite consultar em tempo de execução se um valor pertence a um tipo primitivo (`int`, `string`, `bool`, etc.), uma classe ou uma interface:

<!-- joss-run: ["true", "true", "true"] -->
```joss
var $numero = 42
var $texto = "hola"
print($numero is int)
print($texto is string)
print($numero instanceof int)
```

---

## 8. Coerção textual tipada (`typesystem.CoerceString`)

Em aplicações web, os dados que chegam de formulários HTTP ou requisições JSON são cadeias de texto brutas (por exemplo, `"8080"` ou `"true"`).

Joss implementa uma política compartilhada de coerção textual (`CoerceString`) tanto no analisador estático quanto no runtime quando você atribui um literal ou cadeia a uma variável com tipo explícito:

<!-- joss-run: ["9000", "49.99", "true"] -->
```joss
int $puerto = "9000"
decimal $precio = "49.99"
bool $activo = "yes"
print($puerto)
print($precio)
print($activo)
```

### Tabela de entradas textuais aceitas:

| Tipo destino | Cadeias de texto que Joss converte automaticamente |
|---|---|
| `int` | Textos com dígitos inteiros (`"9000"`, `"-42"`), ou números flutuantes sem parte fracionária (`"100.0"`). |
| `float` | Qualquer texto com notação decimal válida (`"3.1416"`, `"-0.05"`). |
| `decimal` | Textos numéricos limpos (`"49.99"`, `"120.50m"`). |
| `bool` | Aceita sem diferenciar maiúsculas: `"true"`, `"1"`, `"yes"` (como `true`); e `"false"`, `"0"`, `"no"`, `""` (como `false`). |

Se o texto não puder ser convertido (por exemplo `int $x = "manzana"`), o analisador emite um erro estático `JOSS-TYPE-002` ou o runtime o rejeita de forma defensiva.

---

## 9. Precisão numérica e defesas do runtime

| Regra de segurança | Comportamento em Joss | Diagnóstico |
|---|---|---|
| Estouro de inteiro | As operações com inteiros de 64 bits (`+`, `-`, `*`) são verificadas contra overflow. Se superarem os limites de 64 bits, a execução é interrompida imediatamente. | `JOSS-ARITH-001` |
| Divisão por zero | Dividir ou calcular o resto de um número por zero (`$n / 0` ou `$n % 0`) produz um erro controlado em vez de valores `NaN` ou `Infinity` inesperados. | `JOSS-ARITH-002` |
| Perda de precisão implícita | Misturar `int` e `float`, `float` e `decimal`, ou dividir inteiros fora do intervalo exato IEEE-754 sem conversão explícita é rejeitado. | `JOSS-ARITH-003` |
| Índice fora do intervalo | Acessar um índice negativo ou superior ao comprimento de uma lista ou cadeia interrompe o programa de forma estruturada. | `JOSS-INDEX-001` |

---

## 10. Diagnósticos comuns do sistema de tipos

| Código | Significado | Solução habitual |
|---|---|---|
| `JOSS-TYPE-001` | Reatribuição com tipo incompatível (ex. `var $x = 1; $x = "hola"`). | Mantenha o tipo homogêneo ou declare a variável explicitamente como `mixed $x`. |
| `JOSS-TYPE-002` | Valor inicial incompatível com a anotação de tipo. | Corrija o valor inicial para coincidir com o tipo declarado. |
| `JOSS-TYPE-008` | O valor retornado por uma função não coincide com o tipo prometido em `: Tipo`. | Revise a expressão de `return` para que entregue o tipo prometido. |
| `JOSS-TYPE-009` | Nome de tipo ou classe inexistente (inclui aliases retirados como `integer`). | Substitua `integer`, `double`, `boolean`, `any` por seus nomes canônicos: `int`, `float`, `bool`, `mixed`. |
| `JOSS-TYPE-010` | Uma função tipada pode terminar sem executar um `return` ou `throw`. | Certifique-se de que todas as ramificações ternárias concluam com um valor devolvido. |
| `JOSS-TYPE-011` | Foi declarado um parâmetro sem tipo (`func($x)`). | Escreva o tipo do parâmetro: `func(int $x)` ou `func(mixed $x)`. |
| `JOSS-TYPE-014` | Uma função ou método com nome omitiu seu tipo de retorno. | Adicione `: Tipo` ou `: void` na assinatura da função. |

---

## Próximo passo

Para consultar todos os códigos de diagnóstico do analisador e seus exemplos de correção, continue com: [Catálogo e referência de diagnósticos](DIAGNOSTICOS.md).
