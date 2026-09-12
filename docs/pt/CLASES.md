# Classes, objetos, métodos e herança

Antes: [Funções e encerramentos](FUNCIONES.md), [Coleções](COLECCIONES.md). Depois: [Tratamento de erros e exceções](ERRORES.md).
Referência técnica: [Tipo de sistema](SISTEMA_TIPOS.md), [Catálogo nativo](CATALOGO_NATIVO.md).

---

## O que você vai aprender aqui?

À medida que uma aplicação cresce, ter variáveis espalhadas de um lado e funções soltas do outro pode se tornar caótico:
- Você pode ter uma variável `$usuario_nombre`, outra `$usuario_email`, outra `$usuario_rol`.
- Se você tem 100 usuários, como mantém os dados de cada um junto com as operações que lhes correspondem (como autenticação, alteração de senha ou envio de notificação)?

A **Programação Orientada a Objetos (OOP)** resolve esse problema empacotando dados e comportamentos relacionados em uma única unidade conceitual.

Neste guia você aprenderá:
1. O que é uma **classe** (o plano de design) e o que é um **objeto** ou **instância** (a entidade real).
2. Como definir **propriedades** (atributos) e **métodos** (funções de classe).
3. Como inicializar objetos com **construtores** e `Init`.
4. O papel da variável especial **`$this`**.
5. Níveis de visibilidade e encapsulamento: `public`, `protected` e `private`.
6. Membros estáticos e o operador de resolução de escopo (`::`).
7. Como reutilizar e especializar código através de **herança** com `extends`.
8. O operador de navegação seguro para nulos (`?->`).

---

## 1. Classes e Objetos: A planta e a casa

Para entender a orientação a objetos, a melhor analogia é a arquitetura:
- Uma **aula** é a **planta arquitetônica**: descreve quais cômodos a casa terá, quantas portas e quais funções ela possui. O avião não ocupa terreno físico nem você pode morar nele.
- Um **objeto** (ou **instância**) é a **casa física real construída** em um terreno daquele plano. Você pode construir dez casas na mesma planta; Pintar uma casa de azul não altera a cor das outras.

Vejamos um exemplo mínimo com um contador:<!-- joss-run: ["1", "2"] -->
```joss
public class Contador {
    public int $valor = 0

    public func incrementar(): int {
        $this->valor = $this->valor + 1
        return $this->valor
    }
}
$contador = new Contador()
print($contador->incrementar())
print($contador->incrementar())
```
### Quais elementos compõem esse código?

1. `public class Contador`: Declare uma classe pública chamada `Contador`. Em Joss, as classes em nível de arquivo requerem um modificador de visibilidade (`public` ou `private`).
2. `public int $valor = 0`: É uma **propriedade** (uma informação que cada instância de `Contador` irá lembrar).
3. `public func incrementar(): int`: É um **método** (uma função que pertence à classe e que pode manipular suas propriedades).
4. `$this`: É uma palavra reservada que significa "este objeto específico". Ao executar `$this->valor`, você está acessando a propriedade `$valor` da instância que está executando o método.
5. `new Contador()`: A palavra-chave `new` cria uma nova instância real na memória.
6. `$contador->incrementar()`: O operador de seta `->` é usado para acessar propriedades e métodos de uma instância.

---

## 2. Inicialização de objetos: Construtores

Ao criar um objeto, quase sempre é necessário configurá-lo com dados iniciais (por exemplo, o nome de uma pessoa ou credenciais do banco de dados).

Em Joss você pode definir um método `constructor` ou um bloco `Init`:<!-- joss-run: ["Hola, Ada"] -->
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
$persona = new Persona("Ada")
print($persona->saludar())
```
Quando você digita `new Persona("Ada")`, Joss automaticamente chama o construtor, dando-lhe o argumento `"Ada"`, que é armazenado com segurança dentro da propriedade privada `$this->nombre`.

> [!NOTA]
> Joss também suporta a sintaxe de bloco `Init(string $nombre) { ... }`. Os blocos `Init` não carregam modificadores de visibilidade (`public` ou `private`).

### Promoção de propriedade para construtores

Para evitar ter que declarar a propriedade, receber o parâmetro e escrever `$this->prop = $prop` manualmente, Joss permite que você declare visibilidade (`public`, `protected` ou `private`) e constância (`const`) diretamente nos parâmetros `Init` ou `constructor`. Joss criará e atribuirá a propriedade automaticamente:<!-- joss-run: ["Ada", "30"] -->
```joss
public class Usuario {
    Init (
        public string $nombre,
        public int $edad = 30
    ) {}
}

$u = new Usuario("Ada")
print($u->nombre)
print($u->edad)
```
Também é possível declarar propriedades constantes promovidas com `public const Tipo $campo` para protegê-las de reatribuição posterior.

---

## 3. Modificadores de encapsulamento e visibilidade

**Encapsulamento** é o princípio de proteção dos dados internos de um objeto para evitar que código externo o modifique incorretamente ou o corrompa.

Joss oferece três modificadores de visibilidade explícitos:

| Modificador | Onde você pode acessar | Uso recomendado |
|---|---|---|
| `public` | De **qualquer parte** do programa (dentro da classe, em subclasses e de fora do código). | Para a API pública do objeto: métodos que os usuários da sua classe precisam invocar. |
| `protected` | Somente **dentro da própria classe** e **dentro das subclasses** que herdam dela com `extends`. | Para métodos e propriedades internas que as classes filhas precisam se especializar ou consultar. |
| `private` | **Somente dentro da classe exata** onde foi declarado. Ninguém mais pode ver ou modificá-lo. | Para detalhes íntimos de implementação (senhas, conexões brutas, sinalizadores de status). |```joss
public class CuentaBancaria {
    private decimal $saldo = 0.0m

    public func depositar(decimal $monto) {
        ($monto > 0.0m) ? {
            $this->saldo = $this->saldo + $monto
        }
    }

    public func obtenerSaldo(): decimal {
        return $this->saldo
    }
}
```
Ao tornar `$saldo` privado, ninguém pode escrever `$cuenta->saldo = -5000.0m` de fora, garantindo que o dinheiro só seja modificado de acordo com as regras do método `depositar`.

---

## 4. Membros estáticos e o operador `::`

Nem todas as propriedades ou métodos pertencem a uma casa individual; algumas operações pertencem ao conceito geral da classe ou não requerem a criação de uma instância com `new`.

Esses elementos são chamados de **static** e são declarados com a palavra `static`:```joss
public class Utilidades {
    public static func limpiarTexto(string $t): string {
        return trim($t)
    }
}
```
Para invocar um método estático ou ler uma propriedade estática, você não usa `->`, mas sim o operador de dois pontos duplos **`::`**:```joss
$limpio = Utilidades::limpiarTexto("  hola  ")
```
Em Joss, classes de sistema nativas (como `Auth::user()`, `GranDB::table()`, `Route::get()`, `Cache::put()`) são fachadas que normalmente são invocadas por `::`.

---

## 5. Herança com `extends`

**Herança** permite criar uma nova classe baseada em uma classe existente, reutilizando todos os seus métodos e propriedades públicos e protegidos sem precisar reescrevê-los:<!-- joss-run: ["hola"] -->
```joss
public class Mensaje {
    public func texto(): string { return "hola" }
}
public class Aviso extends Mensaje {}
$aviso = new Aviso()
print($aviso->texto())
```
- A classe `Mensaje` é a **classe base** (ou superclasse).
- A classe `Aviso` é a **classe derivada** (ou subclasse).
- `Aviso` herda automaticamente o método `texto()` de `Mensaje`.

> [!TIP]
> **Quando usar herança versus quando usar composição**:
> Use herança somente quando existir um relacionamento estrito "é um" (por exemplo, `Gato extends Animal` ou `AdminUser extends User`). Se você deseja apenas reutilizar uma função utilitária, não use herança; use funções ou injete uma classe de serviço.

---

## 6. Interfaces e Polimorfismo (`interface` e `implements`)

Quando você trabalha em aplicativos modulares ou arquitetura limpa, muitas vezes você deseja definir **o que** um componente deve fazer sem estar vinculado a **como** ele faz isso.

Uma **interface** é um **contrato formal**:
- Declarar apenas os protótipos dos métodos públicos (nome, parâmetros digitados e tipo de retorno) sem corpo.
- Uma interface não pode ser instanciada diretamente com `new`.
- Qualquer classe que declara `implements NombreInterfaz` é **forçada pelo analisador semântico e pelo tempo de execução** a implementar todos os métodos prometidos com assinaturas compatíveis.
- Uma classe pode herdar de uma classe base e ao mesmo tempo implementar **múltiplas interfaces** separadas por vírgulas: `public class MiClase extends Base implements I1, I2`.
- Uma interface pode estender uma ou mais interfaces: `public interface IDerivada extends IBase1, IBase2`.

Vejamos um exemplo de polimorfismo executável:<!-- joss-run: ["50", "36"] -->
```joss
public interface IFigura {
    public func calcularArea(): int;
}

public class Rectangulo implements IFigura {
    public int $ancho = 0
    public int $alto = 0

    Init constructor(int $ancho, int $alto) {
        $this->ancho = $ancho
        $this->alto = $alto
    }

    public func calcularArea(): int {
        return $this->ancho * $this->alto
    }
}

public class Cuadrado implements IFigura {
    public int $lado = 0

    Init constructor(int $lado) {
        $this->lado = $lado
    }

    public func calcularArea(): int {
        return $this->lado * $this->lado
    }
}

public func imprimirArea(IFigura $figura): int {
    return $figura->calcularArea()
}

$r = new Rectangulo(5, 10)
$c = new Cuadrado(6)
print(imprimirArea($r))
print(imprimirArea($c))
```
### Vantagens do Polimorfismo com Interfaces:
1. **Desacoplamento**: A função `imprimirArea(IFigura $figura)` não precisa saber se recebe um `Rectangulo`, um `Cuadrado` ou qualquer figura futura; ele apenas confia que cumpre o contrato `IFigura`.
2. **Validação estática exaustiva**: Se você esquecer de implementar um método em uma classe ou declarar um parâmetro com um tipo diferente, o analisador semântico emite imediatamente `JOSS-DECL-005`.

---

## 7. Classes e métodos abstratos (`abstract`)

Uma **classe abstrata** (`public abstract class`) serve como modelo base para outras classes, mas **não pode ser instanciada diretamente** com `new` (ela emitirá `JOSS-DECL-004`).

Classes abstratas podem conter:
- Propriedades e métodos completos com implementação a ser herdada.
- Métodos abstratos (`abstract func nombre(...): Tipo`) que não possuem corpo e forçam subclasses a implementá-los (`JOSS-DECL-003`).<!-- joss-run: ["Guau!"] -->
```joss
public abstract class Animal {
    public abstract func hablar(): string
}

public class Perro extends Animal {
    public func hablar(): string {
        return "Guau!"
    }
}

$perro = new Perro()
print($perro->hablar())
```
---

## 8. Verificação de tipo e instância: `is` e `instanceof`

Para verificar em tempo de execução se um objeto pertence a uma classe específica, herda de uma classe base ou implementa uma interface, use os operadores equivalentes **`is`** ou **`instanceof`**:<!-- joss-run: ["true", "true", "false"] -->
```joss
public interface IMovible {}
public class Auto implements IMovible {}

$auto = new Auto()
print($auto is Auto)
print($auto is IMovible)
print($auto instanceof string)
```
Você também pode usar `is` com tipos primitivos como `int`, `string`, `bool`, etc. (por exemplo, `$x is int`).

---

## 9. Navegação segura para nulos (`?->`)

Se uma variável puder conter uma instância ou ser `null` (tipo `Persona?`), tentar acessar um método com `->` em um valor nulo pode causar um erro.

Joss inclui o operador **null-safe (`?->`)**:```joss
Persona? $usuario = obtenerUsuario(123)
$nombre = $usuario?->saludar()
```
Se `$usuario` for `null`, a chamada será abortada silenciosa e seguramente, e `$nombre` simplesmente receberá `null` sem parar o programa.

---

## 10. Ciclo de vida e autodestruição inteligente para proteção

No Joss, as classes são escritas de maneira padrão, sem sintaxe complicada. Internamente, o mecanismo anexa um finalizador de ciclo de vida a cada instância criada com `new`.

Quando um objeto esgota seu ciclo de vida e fica sem referências no programa:
1. **Destruidor Opcional**: Se a classe definir um método `destructor()`, `destroy()` ou `__destruct()`, o mecanismo o executa automaticamente de maneira isolada e segura.
2. **Fechamento de recursos nativos**: Caso a instância retenha recursos do sistema (arquivos, canais de comunicação, streams), eles serão fechados automaticamente, evitando vazamentos de descritores.
3. **Limpeza de memória (*Zeroização*)**: Os campos internos da instância são esvaziados e higienizados para evitar que dados confidenciais (tokens, senhas) persistam desnecessariamente na memória RAM.
4. **Proteção contra acesso de zumbis**: A instância é marcada como destruída. Se algum ponteiro residual tentar ler ou modificar seus membros, o mecanismo gerará um `SecurityError`, protegendo a integridade do sistema.<!-- joss-run: ["Conexión activa", "Cerrando sesión de forma segura..."] -->
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

$s = new SesionSegura()
$s->destructor()
```
---

## 11. Erros comuns em OOP com Joss

| Erro | Causa | Solução |
|---|---|---|
| Confundir `->` com `::` | Escreva `$objeto::metodo()` ou `Clase->metodo()`. | Use `->` para instâncias reais criadas com `new` e `::` para chamadas de classes estáticas. |
| Tente acessar um membro privado | `$cuenta->saldo` quando for `private`. | Crie um método *getter* público (como `obtenerSaldo()`) para consultar o valor. |
| Esqueça `new` ao instanciar | `$p = Persona()` em vez de `$p = new Persona()`. | A instanciação requer a palavra `new`. |
| Confundir uma instância com um mapa | Trate um objeto como um array associativo (`$objeto["campo"]`). | Objetos usam seta (`$objeto->campo`), mapas usam colchetes (`$mapa["campo"]`). |
| Contrato de interface de violação | A classe declara `implements` mas falta um método ou seus parâmetros não correspondem. | Implemente todos os métodos de interface com visibilidade `public` e tipos compatíveis (`JOSS-DECL-005`). |
| Acesso ao objeto destruído | Tentativa de ler ou gravar um objeto após a execução de seu ciclo de destruição. | Crie uma nova instância válida em vez de reutilizar um objeto já invalidado (`SecurityError`). |

---

## 12. Exercício prático

1. **Hierarquia de veículos**:
   - Crie uma classe `public class Vehiculo` com uma propriedade protegida `protected string $marca` e um método `public func obtenerMarca(): string`.
   - Crie uma classe derivada `public class Auto extends Vehiculo` que possui uma propriedade `public int $puertas = 4`.
   - Instancie um `Auto`, atribua uma marca a ele e exiba sua marca e número de porta no console.

---

## Próxima etapa

Mesmo no melhor código orientado a objetos, as coisas podem falhar: um arquivo pode não existir, um banco de dados pode estar offline ou um usuário pode inserir dados inválidos. Aprenderemos como interceptar e resolver esses problemas com elegância:

Continue com: [Erro, exceção e tratamento de tentativa/captura](ERRORES.md).