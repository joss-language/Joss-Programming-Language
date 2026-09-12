# Projeto web completo: arquitetura MVC com pilha nativa

Antes: [projeto de console](PROYECTO_CONSOLA.md). Depois: [Controladores e HTTP](CONTROLADORES.md).
Referência técnica: [View Engine](VISTAS.md), [Native Server](SERVIDOR.md), [GranDB Models](MODELOS.md).

---

## O que você vai construir aqui?

Um **aplicativo web** é um programa executado em um servidor que escuta solicitações de rede (geralmente de um navegador ou aplicativo móvel) e responde com páginas HTML interativas ou dados no formato JSON.

Ao contrário de outras linguagens onde você precisa instalar servidores externos complexos (como pacotes Apache, Nginx ou Node.js pesados), **Joss inclui seu próprio servidor HTTP de alto desempenho, roteador dinâmico e mecanismo de modelagem HTML**.

Neste tutorial guiado você construirá sua primeira aplicação web baseada no padrão de arquitetura **MVC (Model - View - Controller)**:
1. Crie a estrutura do projeto usando o construtor de modelos `joss new web`.
2. Compreenda o fluxo de uma solicitação da web em Joss.
3. Defina uma rota com parâmetros dinâmicos em `routes.joss`.
4. Crie um controlador em `app/controllers/` que processe a lógica.
5. Projete uma visualização HTML segura em `app/views/` com escape automático contra injeções de XSS.
6. Inicie o servidor e teste a aplicação no navegador.

---

## 1. O fluxo de uma solicitação da web em Joss

Quando um usuário digita um endereço em seu navegador, ocorre a seguinte sequência:```text
1. Navegador web
   │  Solicita: GET http://127.0.0.1:8080/saludo/Ana
   ▼
2. Servidor HTTP nativo de Joss (construido sobre Go de alta concurrencia)
   │
   ▼
3. Router (routes.joss)
   │  Encuentra coincidencia: "/saludo/{nombre}"
   │  Extrae el parámetro: $nombre = "Ana"
   ▼
4. Controlador (app/controllers/SaludoController.joss)
   │  Ejecuta el método: SaludoController@show($nombre)
   │  Prepara los datos y llama a la vista: view("saludo", {"nombre": ...})
   ▼
5. Motor de Vistas (app/views/saludo.joss.html)
   │  Interpola las variables y escapa caracteres peligrosos
   ▼
6. Respuesta HTTP (HTML renderizado devuelto al navegador del usuario)
```
---

## 2. Gere o projeto web

Abra seu terminal e execute o gerador oficial:```bash
joss new web saludo_web
cd saludo_web
```
Este comando criará a estrutura padrão de uma aplicação web Joss:
- `main.joss`: Ponto de entrada do servidor.
- `env.joss`: Variáveis ​​de configuração (porta, banco de dados, chaves secretas).
- `routes.joss`: O mapa de URL do aplicativo.
- `app/controllers/`: Onde residem os controladores.
- `app/models/`: Modelos de banco de dados com GranDB.
- `app/views/`: Modelos HTML dinâmicos (`.joss.html`).
- `public/`: Arquivos estáticos diretos (folhas de estilo CSS, scripts JS, imagens).

---

## 3. Configure a porta do servidor

Abra o arquivo `env.joss` na raiz do projeto. Você verá uma linha que define a porta. Certifique-se de configurar uma porta disponível (por exemplo `8080`):```joss
PORT = "8080"
APP_NAME = "Mi Web Joss"
```
---

## 4. Defina a rota dinâmica

Abra o arquivo `routes.joss` e adicione a seguinte definição:<!-- joss-check: fragmento de routes.joss -->
```joss
Router::get("/saludo/{nombre}", "SaludoController@show")
```
### O que esta linha significa?
- `Router::get(...)`: Indica que responderemos às solicitações HTTP do tipo `GET` (aquelas feitas pelo navegador ao abrir um link).
- `"/saludo/{nombre}"`: O padrão de URL. `{nombre}` é um **segmento dinâmico**: qualquer texto que o usuário digitar naquela posição na URL será capturado.
- `"SaludoController@show"`: O destinatário. Diz ao roteador para instanciar a classe `SaludoController` e executar seu método `show`.

---

## 5. Crie o controlador

Crie o arquivo `app/controllers/SaludoController.joss` com o seguinte código:<!-- joss-check: fragmento de controlador web -->
```joss
public class SaludoController {
    public func show(string $nombre) {
        $nombreLimpio = trim($nombre)
        return view("saludo", {"nombre": $nombreLimpio})
    }
}
```
### Anatomia do Controlador:
1. `public class SaludoController`: Classe pública que Joss descobre automaticamente sem a necessidade de `import`.
2. `public func show(string $nombre)`: O método recebe o parâmetro dinâmico `{nombre}` capturado pela rota. Em Joss, os parâmetros do controlador devem declarar seu tipo (`string $nombre`).
3. `$nombreLimpio = trim($nombre)`: Limpa possíveis espaços em branco nas extremidades.
4. `return view("saludo", {"nombre": $nombreLimpio})`:
   - Chame a função nativa `view(...)`.
   - O primeiro argumento é o nome do arquivo de modelo (procurará por `app/views/saludo.joss.html`).
   - O segundo argumento é um mapa associativo com os dados que queremos injetar no HTML.

---

## 6. Estilize a visualização HTML

Crie o arquivo de modelo `app/views/saludo.joss.html`:```html
<!doctype html>
<html lang="es">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Saludo - Mi Web Joss</title>
    <style>
        body { font-family: system-ui, sans-serif; margin: 40px; background: #f8fafc; color: #1e293b; }
        .card { background: white; padding: 30px; border-radius: 12px; box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1); max-width: 500px; }
        h1 { color: #0f766e; margin-top: 0; }
    </style>
</head>
<body>
    <div class="card">
        <h1>¡Hola, {{ $nombre }}!</h1>
        <p>Bienvenido a tu primera aplicación web construida con <strong>Joss</strong>.</p>
    </div>
</body>
</html>
```
### Segurança nativa contra ataques XSS:
Observe a sintaxe `{{ $nombre }}`:
- Mecanismo de modelagem Joss ** escapa automaticamente de caracteres HTML perigosos ** como `<`, `>`, `"` ou `&`.
- Se um usuário mal-intencionado tentar inserir o nome `<script>alert('hack')</script>`, Joss irá convertê-los em entidades seguras (`&lt;script&gt;...`), protegendo seus visitantes de vulnerabilidades de Cross-Site Scripting (XSS).

---

## 7. Inicie e teste o servidor

No seu terminal, localizado na raiz do projeto `saludo_web`, digite:```bash
joss server start
```
Você verá a mensagem de confirmação do servidor HTTP:```text
[CLI] Ejecutando script de inicio (main.joss)...
[Joss Server] Servidor HTTP escuchando en http://127.0.0.1:8080
[Joss Server] Presiona 'q' o Ctrl+C para detener el servidor.
```
Abra seu navegador e visite:

`http://127.0.0.1:8080/saludo/Ana`

Você verá o cartão estilizado cumprimentando Ana. Agora tente alterar o URL para:

`http://127.0.0.1:8080/saludo/Carlos`

O servidor responderá instantaneamente com a nova saudação.

Para parar o servidor a qualquer momento, basta pressionar a tecla **`q`** em seu terminal.

---

## 8. Verificação e verificação estática

Antes de implantar em produção, você pode auditar todo o projeto com comandos de verificação Joss:```bash
joss analyze main.joss
joss check .
```
---

## Próxima etapa

Agora que você conhece o ciclo completo de uma solicitação da web, pode explorar os recursos avançados da pilha da web Joss: autenticação de usuário, middlewares de segurança, bancos de dados GranDB e WebSockets em tempo real:

Continue com: [HTTP e Drivers](CONTROLADORES.md), [Middlewares e Segurança](MIDDLEWARE.md) e [Modelos GrandDB](MODELOS.md).