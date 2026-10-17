# Funções globais

[Índice](README.md) · Antes: [funções](FUNCIONES.md) · [Classes nativas](MODULOS_NATIVOS.md)

Estas funções estão integradas no runtime (`pkg/core/builtins.go`), sem imports nem instalação.
A assinatura desta página descreve o uso admitido pelo dispatcher; nem todos
os parâmetros estão publicados ao analisador. `[x]` indica um argumento
opcional e `...` vários argumentos. Não são caracteres que você deva copiar.

As famílias compartilham exemplos ao final. Os aliases de funções como
`doubleval` continuam existindo; isso não restaura o tipo-fonte eliminado
`double`. Os retornos declarados para tooling podem ser consultados no
[catálogo gerado](CATALOGO_NATIVO.md); abaixo descreve-se o resultado em runtime.

## Conversão e consultas de tipo

| Assinatura | Resultado e limites | Exemplo |
|---|---|---|
| `intval(valor)` | Inteiro; trunca float/decimal, bool passa a 0/1; inválido ou ausente dá 0. | `intval("12")` → 12 |
| `floatval(valor)`, `doubleval(valor)` | Float aproximado; inválido dá 0. | `floatval("1.5")` |
| `decimal([valor])` | Decimal a partir de número/texto/bool; inválido ou null dá zero. Sufixos textuais m/M/d/D são removidos. | `decimal("1.25")` |
| `strval(valor)` | Texto por formatação Go, null dá texto vazio. Não é JSON. | `strval(12)` |
| `boolval(valor)` | Bool segundo verdade em runtime. Float zero e map vazio têm diferenças em relação a outras linguagens. | `boolval("")` → false |
| `is_numeric(valor)` | Bool: número ou texto interpretável como float/decimal. Não valida um domínio como “idade”. | `is_numeric("12")` |
| `is_int(valor)`, `is_integer(valor)` | Bool; inteiro, sem converter strings. | `is_int("12")` → false |
| `is_float(valor)`, `is_double(valor)` | Bool; float, sem converter. | `is_float(1.5)` |
| `is_decimal(valor)` | Bool; decimal concreto. | `is_decimal(1m)` |
| `is_string(valor)` | Bool; texto concreto. | `is_string("a")` |
| `is_array(valor)` | Bool; array/slice, não map. | `is_array([1])` |
| `is_null(valor)` | Bool; ausência de valor; sem argumento também true. | `is_null(null)` |
| `isset(expresiones...)` | Forma especial do parser: consulta existência sem erro por variável ausente. Uma variável ligada a null conta como existente; não serve como verificação geral de chaves de map. | `isset($nombre)` |
| `empty(expresion)` | Forma especial: true se não existir ou se o valor for falso segundo as regras de runtime. | `empty($ausente)` |

As conversões explícitas são permissivas; não substituem a validação.
Para condições exatas compare contra `0`, `null` ou `""` conforme o que
precisar. Veja [tipos](SISTEMA_TIPOS.md) e [sintaxe](SINTAXIS.md).

## Arrays, mapas e coleções funcionais

| Assinatura | Resultado, mutação e erros | Exemplo |
|---|---|---|
| `len(valor)`, `count(valor)` | Comprimento de array/map; bytes de string; outros valores dão 0. | `count([2,3])` → 2 |
| `keys(map)`, `array_keys(map)` | Array de chaves sem ordem garantida; inválido dá []. | `keys({"a":1})` |
| `values(map)`, `array_values(map)` | Array de valores sem ordem garantida; inválido dá []. | `values({"a":1})` |
| `explode(separador, texto)` | Array de segmentos; requer duas strings, inválido dá null. | `explode(",", "a,b")` |
| `append(array, elemento)` | Retorna array ampliado; reatribua o resultado. Inválido dá null. | `$a = append($a, 2)` |
| `merge(array1, array2)` | Novo contêiner com ambas as sequências; inválido dá null. | `merge([1],[2])` |
| `array_merge(primero, otros...)` | Array concatenado ou map combinado conforme o primeiro argumento; últimas chaves vencem. Ignora seguintes de outro tipo; inválido dá []. | `array_merge({"a":1},{"a":2})` |
| `array_push(array, elementos...)` | Array ampliado; não devolve comprimento. Reatribua. Inválido dá null. | `$a = array_push($a,2,3)` |
| `end(array)`, `array_pop(array)` | Último elemento ou null. **Não reduzem o array**. | `array_pop([1,2])` → 2 |
| `array_shift(array)` | Primeiro elemento ou null. **Não reduz o array**. | `array_shift([1,2])` → 1 |
| `array_slice(array,inicio,[longitud])` | Visão superficial; negativos relativos ao final; fora dá []. Compartilha armazenamento. | `array_slice([1,2,3],1)` |
| `array_unique(array)` | Novo array, igualdade por representação textual; pode fundir valores de tipos diferentes. | `array_unique([1,1,2])` |
| `array_reverse(array)` | Novo array invertido; inválido dá []. | `array_reverse([1,2])` |
| `array_column(array,clave)` | Valores dessa chave em elementos map; omite outros elementos/chaves ausentes. | `array_column([{"id":1}],"id")` |
| `in_array(valor,array)` | Bool; em arrays Joss admite igualdade profunda ou textual. | `in_array(1,["1"])` → true |
| `array_key_exists(clave,map)` | Bool de existência; não confundir com valor não nulo. | `array_key_exists("a",{"a":null})` |
| `map(array, closure)` | Retorna um novo array aplicando `closure` a cada elemento. | `map([1,2], func(int $x): int { return $x * 2 })` |
| `filter(array, closure)` | Retorna um novo array com os elementos onde `closure` devolver verdadeiro. | `filter([1,2,3], func(int $x): bool { return $x > 1 })` |
| `sum(array)` | Soma numérica dos elementos; preserva `decimal`, verifica overflow inteiro e rejeita misturar `float` com `decimal` (`JOSS-ARITH-003`). | `sum([10, 20, 30])` → 60 |

## Texto e formatação

| Assinatura | Resultado e limites | Exemplo |
|---|---|---|
| `print(valores...)`, `echo(valores...)` | Imprimem cada argumento com quebra de linha; retorno null. | `print("Hola")` |
| `printf(formato,valores...)` | Formato Go, sem quebra implícita; retorno null. | `printf("%s: %d\\n","Edad",20)` |
| `strlen(texto)` | Quantidade de pontos Unicode; null dá 0. | `strlen("é")` → 1 |
| `str_contains(texto,parte)`, `contains(texto,parte)` | Bool; argumentos são convertidos para texto. | `contains("casa","as")` |
| `str_starts_with(texto,prefijo)`, `starts_with(...)` | Bool de prefixo. | `starts_with("abc","a")` |
| `str_ends_with(texto,sufijo)`, `ends_with(...)` | Bool de sufixo. | `ends_with("abc","c")` |
| `str_replace(buscar,reemplazo,texto)` | Substitui todas as ocorrências. A ordem difere de Str::replace. | `str_replace("a","o","casa")` |
| `strtolower(texto)`, `to_lower(texto)` | Minúsculas Unicode. | `to_lower("HOLA")` |
| `strtoupper(texto)`, `to_upper(texto)` | Maiúsculas Unicode. | `to_upper("hola")` |
| `ucfirst(texto)`, `lcfirst(texto)` | Alteram o primeiro ponto Unicode para maiúscula/minúscula. | `ucfirst("hola")` |
| `ucwords(texto)` | Capitalização segundo strings.Title de Go; não é análise linguística. | `ucwords("hola mundo")` |
| `trim(texto,[conjunto])` | Remove espaços Unicode ou caracteres do conjunto nas extremidades. | `trim(" hola ")` |
| `ltrim(texto,[conjunto])`, `rtrim(texto,[conjunto])` | Recorte em uma extremidade; padrão espaços ASCII. | `ltrim(" hola")` |
| `substr(texto,inicio,[longitud])` | Pontos Unicode; negativos relativos ao final; fora dá "". | `substr("abc",-2)` → bc |
| `strpos(texto,parte)` | Posição em pontos Unicode ou false se ausente. Zero é uma ocorrência válida. | `strpos("abc","a")` → 0 |
| `implode(separador,array)`, `join(separador,array)` | Converte elementos em texto e une; inválido dá "". | `join("-",[1,2])` |
| `str_pad(texto,longitud,[relleno])` | Adiciona preenchimento à direita por **bytes**, por padrão espaço. Preenchimento vazio pode causar panic; não usar. | `str_pad("7",3,"0")` → 700 |
| `str_repeat(texto,cantidad)` | Texto repetido; negativo equivale a zero. | `str_repeat("a",3)` |
| `html_escape(valor)` | Escapa &, <, > e aspas para HTML; null dá "". | `html_escape("<b>")` |
| `md5(texto)`, `sha1(texto)`, `sha256(texto)` | Digest hexadecimal de bytes; não é criptografia nem hash de senha. | `sha256("dato")` |
| `base64_encode(texto)` | Texto Base64, sem segredo criptográfico. | `base64_encode("a")` → YQ== |
| `base64_decode(texto)` | String de bytes ou false se inválido. | `base64_decode("YQ==")` |

Para senhas utilize o contrato de Auth. Para caracteres percebidos
como emojis compostos, consulte [unidades de texto](COLECCIONES.md).

## Números e tempo

| Assinatura | Resultado e limites | Exemplo |
|---|---|---|
| `round(numero,[precision])` | Float arredondado; precisão padrão 0. Não processa decimal.Decimal. | `round(1.25,1)` |
| `floor(numero)`, `ceil(numero)` | Inteiro para baixo/cima; aceitam float/int, não decimal. | `floor(1.9)` → 1 |
| `abs(numero)` | Magnitude int/float. O mínimo int64 pode transbordar sem diagnóstico aqui. | `abs(-2)` |
| `min(valores...)`, `max(valores...)` | Também aceitam um array não vazio; sem argumentos null. Um único array vazio é devolvido como tal. | `max([1,3,2])` |
| `rand([min,max])` | Inteiro inclusivo; sem dois limites usa 0..MaxInt32. max<=min devolve min. Não criptográfico. | `rand(1,6)` |
| `time()` | Segundos Unix como inteiro. | `time()` |
| `microtime([comoFloat])` | Com true, float em segundos; por padrão texto "fração segundos". | `microtime(true)` |
| `date([formato,timestamp])` | Texto; por padrão Y-m-d H:i:s e hora atual. Timestamp admite número, texto ou valor time.Time do host. | `date("Y-m-d",0)` |
| `now([formato])` | Texto da data atual; mesmo formato padrão. | `now("H:i:s")` |
| `strtotime(texto,[base])` | Inteiro Unix ou null se não reconhecer o texto. Parser limitado de datas e deslocamentos. | `strtotime("+1 day",0)` |
| `sleep(segundos)`, `usleep(microsegundos)` | Bloqueiam a tarefa atual; retorno null. sleep admite frações float. | `sleep(0.01)` |

Os tokens de data e expressões relativas admitidos estão implementados em
[date_utils.go](../../pkg/core/date_utils.go). Não se incorpora toda a gramática
de datas do PHP. O fuso horário depende do processo; evite saídas de data
local em testes que devam ser idênticos em qualquer máquina.

## Arquivos, serialização e processos

| Assinatura | Resultado e erros | Exemplo |
|---|---|---|
| `file_exists(ruta)` | Bool; inclui diretórios; false em erro de stat. | `file_exists("datos.json")` |
| `is_dir(ruta)`, `is_file(ruta)` | Bool; is_file significa existente e não diretório. | `is_dir("storage")` |
| `file_get_contents(ruta)` | String de bytes ou null ao falhar. Apenas arquivos locais. | `file_get_contents("datos.json")` |
| `file_put_contents(ruta,texto)` | Sobrescreve/cria arquivo; true ou false. Não cria diretórios pais. | `file_put_contents("nota.txt","Hola")` |
| `unlink(ruta)`, `file_delete(ruta)` | Remove arquivo ou diretório vazio; bool de sucesso. | `unlink("nota.txt")` |
| `mkdir(ruta)` | Cria diretórios incluindo pais; bool. | `mkdir("storage/informes")` |
| `json_encode(valor)` | JSON com indentação segundo JsonEncode; "" em caso de erro. | `json_encode({"ok":true})` |
| `json_decode(texto)` | Valor nativo ou null por erro (também JSON null). Números são decodificados para float. | `json_decode("[1,2]")` |
| `json_verify(texto)` | Bool de sintaxe JSON, não da estrutura de negócio. | `json_verify("{}")` |
| `toon_encode(valor)` | Formato textual simplificado de registros; não binário nem escape completo. | `toon_encode([{"a":"b"}])` |
| `toon_decode(texto)` | Array de registros do subconjunto ou null. Valores textuais. | Usar saída compatível do encoder. |
| `toon_verify(texto)` | Verificação limitada de cabeçalho/estrutura; não integridade criptográfica. | Não usar para validar dados hostis. |
| `hive_read_box(ruta)` | Array de entradas ou null; imprime erro. Leitor parcial do formato Hive. | Requer arquivo Hive compatível. |
| `run(ruta,args...)` | Executa .py com python ou .php com php; saída combinada. Requer ALLOW_SYSTEM_RUN=true e executável instalado. Falha imprime e retorna saída/""; nem sempre lança exceção. | `run("script.py","dato")` |

Os caminhos são relativos ao diretório de trabalho. As operações não têm
sandbox de arquivos. O exemplo completo está em [relatório de compras](PROYECTO_CONSOLA.md).

## Contexto web, testes e concorrência

| Assinatura | Contrato e disponibilidade |
|---|---|
| `env(clave,[default])`, `config(clave,[default])` | Consulta r.Env; devolve string ou default arbitrário se ausente/vazio. Não relê .env. |
| `view(nombre,[datos])` | Delega View::render; contexto de templates do projeto. |
| `json(datos,[status])` | Delega Response::json e devolve WebResponse; **não é serialização para string**. |
| `response(cuerpo,[status,mime,headers])` | Delega Response::raw. |
| `redirect(url,[status])`, `back()` | WebResponse de redirecionamento; back usa contexto da requisição. |
| `request([clave,default])` | Sem chave obtém Request::all; com chave Request::input. |
| `session([clave])` | Sem chave objeto de sessão ou null; com chave Session::get. Não incorpora uma sessão no console. |
| `__(clave)` | Tradução usando locale atual e gerenciador i18n. |
| `csrf_field()` | HTML do campo _token usando sessão; sem ela token vazio. |
| `async { instrucciones }` | Sintaxe que cria Future e executa uma closure em fork/goroutine. |
| `await(future)` | Aguarda e devolve resultado; propaga falha. |
| `make_chan([capacidad])` | Channel; capacidade 0 sincroniza emissor e receptor. |
| `send(channel,valor)` | Envia; pode bloquear, canal fechado provoca falha. Também channel << valor. |
| `recv(channel)` | Recebe; bloqueia até dado/fechamento; fechamento esgotado produz null. |
| `close(channel)` | Fecha; fechar duas vezes ou enviar depois produz falha. |
| `Ok(valor)`, `Err(error)` | Construtores do tipo `Result<T, E>` para retorno explícito de sucesso ou falha. |
| `test(nombre, closure)`, `it(nombre, closure)` | Registra e executa um caso de teste em arquivos `*_test.joss` (`joss test`). |
| `assert(cond,[msg])`, `assertTrue(...)`, `assertFalse(...)`, `assertEqual(a,b)`, `assertNotEqual(a,b)`, `assertNull(v)`, `assertNotNull(v)`, `assertThrows(closure)` | Asserções nativas integradas para testes automatizados com `joss test`. |

Consulte [HTTP e classes nativas](MODULOS_NATIVOS.md), [views](VISTAS.md) e
[concorrência](CONCURRENCIA.md) para exemplos com o contexto necessário.

## Exemplo executável de utilitários

<!-- joss-run: ["Ana", "3", "a-b", "true", "Hola", "2"] -->
```joss
print(ucfirst(trim(" ana ")))
print(strlen("sol"))
print(join("-", ["a", "b"]))
print(array_key_exists("id", {"id": null}))
print(base64_decode(base64_encode("Hola")))
print(floor(2.8))
```

## Diferenças entre metadados e implementação

Foram observados retornos publicados incompletos: boolval anuncia int mas
devolve bool; json anuncia string mas devolve WebResponse; microtime
anuncia float embora sem true devolva string; strpos e base64_decode podem
devolver false; array_merge também devolve map. Não atribua esses erros
ao usuário nem force anotações incompatíveis para satisfazer o catálogo.
O [relatório de auditoria](DOCUMENTATION_AUDIT.md) registra essa dívida.

Fontes: [catálogo](../../pkg/core/builtins.go),
[arrays e conversões](../../pkg/core/builtins_array.go),
[textos](../../pkg/core/builtins_string.go), [I/O](../../pkg/core/builtins_io.go),
[tempo](../../pkg/core/builtins_date.go), [async](../../pkg/core/builtins_async.go).