# Classes, objetos, métodos e herança

Antes: [Tratamento de erros e exceções](ERRORES.md). Depois: [Concorrência, assincronia e canais](CONCURRENCIA.md).
Referência técnica: [Sistema de tipos](SISTEMA_TIPOS.md), [Catálogo nativo](CATALOGO_NATIVO.md).

---

## O que você vai aprender aqui?

À medida que uma aplicação cresce, ter variáveis espalhadas de um lado e funções soltas de outro pode se tornar caótico:
- Você pode ter uma variável `$usuario_nombre`, outra `$usuario_email`, outra `$usuario_rol`.
- Se você tiver 100 usuários, como mantém unidos os dados de cada um com as operações que lhes correspondem (como autenticar-se, alterar senha ou enviar notificação)?

A **Programação Orientada a Objetos (POO)** resolve esse problema empacotando dados e comportamentos relacionados em uma única unidade conceitual.

Neste guia você aprenderá:
1. O que é uma **classe** (a planta de projeto) e o que é um **objeto** ou **instância** (a entidade real).
2. Como definir **propriedades** (atributos) e **métodos** (funções da classe).
3. Como inicializar objetos com **construtores** e `Init`.
4. O papel da variável especial **`$this`**.
5. Os níveis de visibilidade e encapsulamento: `public`, `protected` e `private`.
6. Membros estáticos e o operador de resolução de escopo (`::`).
7. Como reutilizar e especializar código mediante **herança (`extends`)**, **interfaces (`implements`)** e **classes abstratas (`abstract`)**.
8. Classes genéricas (`<T>`), `record` imutáveis e o operador de navegação segura contra nulos (`?->`).

---

## 1. Classes e Objetos: A planta e a casa

Para entender a orientação a objetos, a melhor analogia é a arquitetura:
- Uma **classe** é a **planta arquitetônica**: descreve quais cômodos a casa terá, quantas portas e quais funções possui. A planta não ocupa um terreno físico nem você pode morar nela.
- Um **objeto** (ou **instância**) é a **casa física real construída** em um terreno a partir dessa planta. Você pode construir dez casas a partir da mesma planta; pintar uma casa de azul não muda a cor das demais.

Vejamos um exemplo mínimo com um contador:

<!-- joss-run: ["1", "2"] -->
```joss
public class Contador {
    public int $valor = 0

    public func incrementar(): int {
        $this->valor = $this->valor + 1
        return $this->valor
    }
}
var $contador = new Contador()
print($contador->incrementar())
print($contador->incrementar())
```

Saída no terminal:

```text
1
2
```

### Quais elementos compõem este código?

1. `public class Contador`: Declara uma classe pública chamada `Contador`. No Joss, as classes em nível de arquivo exigem um modificador de visibilidade (`public` ou `private`).
2. `public int $valor = 0`: É uma **propriedade** (um dado que cada instância de `Contador` lembrará).
3. `public func incrementar(): int`: É um **método** (uma função que pertence à classe e que pode manipular suas propriedades).
4. `$this`: É uma palavra reservada que significa "este objeto em particular". Quando você executa `$this->valor`, está acessando a propriedade `$valor` da instância que está executando o método.
5. `new Contador()`: A palavra-chave `new` cria uma nova instância real na memória.
6. `$contador->incrementar()`: O operador seta `->` é utilizado para acessar propriedades e métodos de uma instância.

---

## 2. Inicialização de objetos: Construtores

Quando você cria um objeto, quase sempre precisa configurá-lo com dados iniciais (por exemplo, o nome de uma pessoa ou as credenciais de um banco de dados).

No Joss você pode definir um método `public func constructor(...)` ou um bloco `Init(...)`:

<!-- joss-run: ["Hola, Ada"] -->
```joss
public class Persona {
    private string $nombre = ""

    public func constructor(string $nombre) {
        $this->nombre = $nombre
    }

    public func saludar(): string {
        return "Hola, " . $this->nombre
    }
}
var $persona = new Persona("Ada")
print($persona->saludar())
```

Saída no terminal:

```text
Hola, Ada
```

Ao escrever `new Persona("Ada")`, o Joss chama automaticamente o construtor entregando-lhe o argumento `"Ada"`, o qual fica guardado de forma segura dentro da propriedade privada `$this->nombre`.

> [!NOTE]
> O Joss também admite a sintaxe de bloco `Init(string $nombre) { ... }`. Os blocos `Init` não levam modificadores de visibilidade (`public` nem `private`). A forma histórica combinada `Init constructor(...)` continua sendo aceita por compatibilidade, mas emite o aviso de transição `JOSS-DECL-008`; prefira `public func constructor(...)` ou `Init(...)`.

### Promoção de propriedades no construtor (Constructor Property Promotion)

Para evitar ter que declarar a propriedade, receber o parâmetro e escrever `$this->prop = $prop` manualmente, o Joss permite declarar a visibilidade (`public`, `protected` ou `private`) e constância (`const`) diretamente nos parâmetros de `Init` ou do `constructor`. O Joss criará e atribuirá a propriedade automaticamente:

<!-- joss-run: ["Ada", "30"] -->
```joss
public class Usuario {
    Init(
        public string $nombre,
        public int $edad = 30
    ) {}
}

var $u = new Usuario("Ada")
print($u->nombre)
print($u->edad)
```

Saída no terminal:

```text
Ada
30
```

Também é possível declarar propriedades constantes promovidas com `public const Tipo $campo` para protegê-las contra reatribuições posteriores.

---

## 3. Encapsulamento e modificadores de visibilidade

O **encapsulamento** é o princípio de proteger os dados internos de um objeto para evitar que código externo os modifique de forma incorreta ou corrompida.

O Joss oferece três modificadores de visibilidade explícitos:

| Modificador | Onde pode ser acessado | Uso recomendado |
|---|---|---|
| `public` | De **qualquer parte** do programa (dentro da classe, em subclasses e a partir de código externo). | Para a API pública do objeto: métodos que os usuários da sua classe precisam invocar. |
| `protected` | Apenas **dentro da própria classe** e **dentro das subclasses** que herdem dela com `extends`. | Para métodos e propriedades internas que as classes filhas precisam especializar ou consultar. |
| `private` | **Unicamente dentro da classe exata** onde foi declarada. Ninguém mais pode vê-la nem modificá-la. | Para detalhes íntimos de implementação (senhas, conexões brutas, flags de estado). |

<!-- joss-run: ["150.5"] -->
```joss
public class CuentaBancaria {
    private decimal $saldo = 0.0m

    public func depositar(decimal $monto): void {
        ($monto > 0.0m) ? {
            $this->saldo = $this->saldo + $monto
        }
    }

    public func obtenerSaldo(): decimal {
        return $this->saldo
    }
}

var $cuenta = new CuentaBancaria()
$cuenta->depositar(150.50m)
print($cuenta->obtenerSaldo())
```

Saída no terminal:

```text
150.5
```

Ao tornar `$saldo` privado, ninguém pode escrever `$cuenta->saldo = -5000.0m` de fora, garantindo que o dinheiro só seja modificado sob as regras do método `depositar`.

---

## 4. Membros estáticos e o operador `::`

Nem todas as propriedades ou métodos pertencem a uma casa individual; algumas operações pertencem ao conceito geral da classe ou não exigem criar uma instância com `new`.

Esses elementos são chamados de **estáticos** e são declarados com a palavra `static` (lembre-se de que `static` sempre acompanha um modificador de visibilidade como `public static`):

<!-- joss-run: ["hola"] -->
```joss
public class Utilidades {
    public static func limpiarTexto(string $t): string {
        return trim($t)
    }
}

var $limpio = Utilidades::limpiarTexto("  hola  ")
print($limpio)
```

Saída no terminal:

```text
hola
```

Para invocar um método estático ou ler uma propriedade estática, não se utiliza `->`, mas sim o operador de dois-pontos duplos **`::`**. No Joss, as classes nativas do sistema (como `Auth::user()`, `GranDB::table()`, `Router::get()`, `Cache::put()`) são fachadas invocadas habitualmente mediante `::`.

---

## 5. Herança com `extends`

A **herança** permite criar uma classe nova baseada em uma classe existente, reutilizando todos os seus métodos e propriedades públicas e protegidas sem ter que reescrevê-los:

<!-- joss-run: ["hola"] -->
```joss
public class Mensaje {
    public func texto(): string { return "hola" }
}
public class Aviso extends Mensaje {}
var $aviso = new Aviso()
print($aviso->texto())
```

Saída no terminal:

```text
hola
```

- A classe `Mensaje` é a **classe base** (ou superclasse).
- A classe `Aviso` é a **classe derivada** (ou subclasse).
- `Aviso` herda automaticamente o método `texto()` de `Mensaje`.

> [!TIP]
> **Quando usar herança vs quando usar composição**:
> Use herança apenas quando existir uma relação estrita do tipo "é um" (por exemplo, `Gato extends Animal` ou `AdminUser extends User`). Se você só quer reutilizar uma função utilitária, não use herança; use funções ou injete uma classe de serviço.

---

## 6. Interfaces e Polimorfismo (`interface` e `implements`)

Quando você trabalha em aplicações modulares ou arquitetura limpa, muitas vezes quer definir **o que** um componente deve fazer sem se prender a **como** ele faz.

Uma **interface** é um **contrato formal**:
- Declara apenas os protótipos dos métodos públicos (nome, parâmetros tipados e tipo de retorno) sem corpo.
- Uma interface não pode ser instanciada diretamente com `new`.
- Qualquer classe que declare `implements NombreInterfaz` é **obrigada pelo analisador semântico e pelo runtime** a implementar todos os métodos prometidos com assinaturas compatíveis.
- Uma classe pode herdar de uma classe base e ao mesmo tempo implementar **múltiplas interfaces** separadas por vírgulas: `public class MiClase extends Base implements I1, I2`.
- Uma interface pode estender uma ou várias interfaces: `public interface IDerivada extends IBase1, IBase2`.

Vejamos um exemplo de polimorfismo executável:

<!-- joss-run: ["50", "36"] -->
```joss
public interface IFigura {
    public func calcularArea(): int;
}

public class Rectangulo implements IFigura {
    public int $ancho = 0
    public int $alto = 0

    public func constructor(int $ancho, int $alto) {
        $this->ancho = $ancho
        $this->alto = $alto
    }

    public func calcularArea(): int {
        return $this->ancho * $this->alto
    }
}

public class Cuadrado implements IFigura {
    public int $lado = 0

    public func constructor(int $lado) {
        $this->lado = $lado
    }

    public func calcularArea(): int {
        return $this->lado * $this->lado
    }
}

public func imprimirArea(IFigura $figura): int {
    return $figura->calcularArea()
}

var $r = new Rectangulo(5, 10)
var $c = new Cuadrado(6)
print(imprimirArea($r))
print(imprimirArea($c))
```

Saída no terminal:

```text
50
36
```

### Vantagens do Polimorfismo com Interfaces:
1. **Desacoplamento**: A função `imprimirArea(IFigura $figura)` não precisa saber se recebe um `Rectangulo`, um `Cuadrado` ou qualquer figura futura; ela apenas confia que cumpre o contrato `IFigura`.
2. **Validação estática exaustiva**: Se você esquecer de implementar um método da interface em uma classe ou sua assinatura não coincidir, o analisador semântico emite `JOSS-DECL-004` (ou `JOSS-SEM-001` se um método implementado não for `public`).

---

## 7. Classes e métodos abstratos (`abstract`)

Uma **classe abstrata** (`public abstract class`) serve como modelo base para outras classes, mas **não pode ser instanciada diretamente** com `new` (tentar fazer isso emite o diagnóstico `JOSS-DECL-006`).

As classes abstratas podem conter:
- Propriedades e métodos completos com implementação para serem herdados.
- Métodos abstratos (`public abstract func nombre(...): Tipo`) que não possuem corpo e obrigam as subclasses concretas a implementá-los (omiti-los emite `JOSS-DECL-005`).

<!-- joss-run: ["Guau!"] -->
```joss
public abstract class Animal {
    public abstract func hablar(): string
}

public class Perro extends Animal {
    public func hablar(): string {
        return "Guau!"
    }
}

var $perro = new Perro()
print($perro->hablar())
```

Saída no terminal:

```text
Guau!
```

---

## 8. Verificação de tipos e instâncias: `is` e `instanceof`

Para verificar em tempo de execução se um objeto pertence a uma classe concreta, herda de uma classe base ou implementa uma interface, utilize os operadores equivalentes **`is`** ou **`instanceof`**:

<!-- joss-run: ["true", "true", "false"] -->
```joss
public interface IMovible {}
public class Auto implements IMovible {}

var $auto = new Auto()
print($auto is Auto)
print($auto is IMovible)
print($auto instanceof string)
```

Saída no terminal:

```text
true
true
false
```

Você também pode usar `is` com tipos primitivos como `int`, `string`, `bool`, etc. (ex.: `$x is int`).

---

## 9. Navegação segura contra nulos (`?->`)

Se uma variável puder conter uma instância ou ser `null` (tipo `Persona?`), tentar acessar um método com `->` sobre um valor nulo produziria uma falha.

O Joss inclui o operador **null-safe (`?->`)**:

<!-- joss-run: ["Sin usuario"] -->
```joss
public class Perfil {
    public func saludo(): string {
        return "Hola"
    }
}

Perfil? $usuario = null
var $nombre = $usuario?->saludo()
print($nombre ?? "Sin usuario")
```

Saída no terminal:

```text
Sin usuario
```

Se `$usuario` for `null`, a chamada é cancelada de forma silenciosa e segura, e `$nombre` receberá simplesmente `null` sem interromper o programa.

---

## 10. Classes genéricas (`class Contenedor<T>`) e Records imutáveis (`record`)

### Classes genéricas
O Joss admite parâmetros formais de tipo `<T>` na declaração de classes:

```joss
public class Caja<T> {
    public T $contenido
    public func constructor(T $valor) {
        $this->contenido = $valor
    }
}
```

O analisador semântico valida nominalmente os parâmetros genéricos garantindo que `Caja<int>` não seja atribuível a `Caja<string>`.

### Records (`public record Nombre(campos...)`)
Um `record` é uma declaração concisa de classe orientada a dados imutáveis com construtor e campos automáticos:

<!-- joss-run: ["10", "20"] -->
```joss
public record Coordenada(int $x, int $y)

var $c = new Coordenada(10, 20)
print($c->x)
print($c->y)
```

Saída no terminal:

```text
10
20
```

Os campos declarados em um `record` ficam selados como constantes (`const`) após a inicialização do objeto, impedindo qualquer reatribuição acidental (`JOSS-CONST-001`).

---

## 11. Ciclo de vida e autodestruição inteligente por proteção

No Joss, as classes são escritas de forma padrão sem sintaxe complicada. Internamente, o motor acopla um finalizador de ciclo de vida a cada instância criada com `new`.

Quando um objeto esgota seu ciclo de vida e fica sem referências no programa:
1. **Destrutor opcional**: Se a classe definir um método `destructor()`, `destroy()` ou `__destruct()`, o motor o executa automaticamente de forma isolada e segura.
2. **Fechamento de recursos nativos**: Se a instância retinha recursos do sistema (arquivos, canais de comunicação, streams), eles são fechados automaticamente prevenindo vazamentos de descritores.
3. **Expurgo de memória (*Zeroization*)**: Os campos internos da instância são esvaziados e sanitizados para evitar que dados sensíveis (tokens, senhas) persistam desnecessariamente na memória RAM.
4. **Proteção contra acessos zumbis**: A instância fica marcada como destruída. Se algum ponteiro residual tentar ler ou modificar seus membros, o motor lança um `SecurityError` protegendo a integridade do sistema.

<!-- joss-run: ["Conexión activa", "Cerrando sesión de forma segura..."] -->
```joss
public class SesionSegura {
    public string $token = "tok_12345"

    public func constructor() {
        print("Conexión activa")
    }

    public func destructor() {
        print("Cerrando sesión de forma segura...")
    }
}

var $s = new SesionSegura()
$s->destructor()
```

Saída no terminal:

```text
Conexión activa
Cerrando sesión de forma segura...
```

---

## 12. Erros comuns em POO com Joss

| Erro | Causa | Solução |
|---|---|---|
| Confundir `->` com `::` | Escrever `$objeto::metodo()` ou `Clase->metodo()`. | Use `->` para instâncias reais criadas com `new` e `::` para chamadas estáticas a classes. |
| Tentar acessar um membro privado | `$cuenta->saldo` quando é `private`. | Crie um método público *getter* (como `obtenerSaldo()`) para consultar o valor. |
| Esquecer `new` ao instanciar | `var $p = Persona()` em vez de `var $p = new Persona()`. | A criação de instâncias exige a palavra `new`. |
| Confundir uma instância com um map | Tratar um objeto como array associativo (`$objeto["campo"]`). | Os objetos usam seta (`$objeto->campo`), os maps usam colchetes (`$mapa["campo"]`). |
| Descumprir contrato de interface | A classe declara `implements` mas falta um método ou seus parâmetros não coincidem. | Implementar todos os métodos da interface com visibilidade `public` e tipos compatíveis (`JOSS-DECL-004`). |
| Instanciar uma classe abstrata | Fazer `new ClaseAbstracta()` diretamente. | Estenda a classe abstrata em uma subclasse concreta e instancie a subclasse (`JOSS-DECL-006`). |
| Acesso a objeto destruído | Tentar ler ou escrever em um objeto após executar seu ciclo de destruição. | Criar uma nova instância válida em vez de reutilizar um objeto já invalidado (`SecurityError`). |

---

## 13. Exercícios práticos

### Nível 1 — Teste rápido
Declare um `public record Producto(string $nombre, decimal $precio)`. Instancie um produto `"Teclado"` com preço `45.50m` e imprima seu nome e seu preço usando `->`.

### Nível 2 — Exercício guiado
1. Crie uma classe `public class Vehiculo` com uma propriedade protegida `protected string $marca = ""` inicializada em `public func constructor(string $marca)` e um método `public func obtenerMarca(): string`.
2. Crie uma classe derivada `public class Auto extends Vehiculo` que tenha uma propriedade `public int $puertas = 4`.
3. Instancie um `Auto("Toyota")`, e imprima sua marca (usando `obtenerMarca()`) e seu número de portas no console.

### Nível 3 — Desafio
1. Defina uma interface `public interface INotificador` com o método `public func enviar(string $mensaje): string;`.
2. Crie duas classes que implementem `INotificador`:
   - `NotificadorEmail`: devolve `"[EMAIL] " . $mensaje`.
   - `NotificadorSMS`: devolve `"[SMS] " . $mensaje`.
3. Escreva uma função `public func despacharAlerta(INotificador $canal, string $texto): void` que imprima o resultado de `$canal->enviar($texto)`.
4. Teste-a passando uma instância de `NotificadorEmail` e outra de `NotificadorSMS`.

---

## Próximo passo

Agora que você domina classes, contratos de interfaces e objetos, está pronto para descobrir uma das capacidades mais poderosas do Joss: executar tarefas em paralelo, delegar operações para segundo plano e comunicar processos mediante canais seguros.

Continue com: [Concorrência, operações assíncronas, Future e Canais](CONCURRENCIA.md).