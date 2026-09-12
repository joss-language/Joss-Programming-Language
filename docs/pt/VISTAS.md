# Visualizações

[Índice](README.md)

`view("dashboard.index", $data)` (ou `View::render`) procura por `app/views/dashboard/index.joss.html` e depois `.html`.```html
<h1>{{ $title }}</h1>
<div>{{! $trusted_html }}</div>
```
`{{ expr }}` escapa do HTML. `{{! expr }}` insere saída sem escape e deve receber apenas conteúdo confiável. `{{ csrf_field() }}` é transformado em saída bruta para gerar a entrada CSRF.

## Layouts e inclui```html
@extends('layouts.master')
@section('content')
    @include('partials.alert')
@endsection
```
O layout usa `@yield('content')`. `@extends` só é reconhecido no início lógico da visualização. As inclusões são resolvidas antes da compilação do modelo.

## Foreach e condicionais```html
@foreach($users as $user)
    <p>{{ $user.name }}</p>
    {{ ($user.active) ? { <span>Activo</span> } : { <span>Inactivo</span> } }}
@endforeach

<!-- Soporte para expresiones anidadas y colecciones indexadas -->
@foreach($order["items"] as $it)
    <div>{{ $it["product"]["title"] }} - ${{ $it["unit_price"] }}</div>
@endforeach
```
O compilador processa recursivamente o corpo de cada `@foreach` (suportando expressões arbitrárias, variáveis ​​simples, arrays aninhados ou acessos a propriedades) e ternários de bloco podem interagir livremente com as variáveis ​​iteradas. Lembrete: Em Joss `@if`, `@else` e ​​`@endif` não existem; Os ternários funcionais são usados ​​`($cond) ? { ... } : { ... }`.

A notação `$map.key` nas expressões de visualização se traduz em `$map->key`. O avaliador permite ler mapas e instâncias com essa forma.

## Diretivas e comentários em modelos

1. Diretiva **`@json($data)`**:
   Permite despejar objetos, mapas ou matrizes com segurança em atributos JavaScript:   ```html
   <script>
       const config = @json($appConfig);
   </script>
   ```
2. **Comentários do modelo `{{-- Comentario --}}`**:
   Os blocos de comentários do blade são removidos antes da renderização e não alcançam o cliente HTML:   ```html
   {{-- Este comentario no se muestra en el navegador ni en el código fuente --}}
   ```
## Métodos nativos de `View`

- **`View::exists("vista.nombre")`**: Retorna `true` ou `false` dependendo da existência do arquivo de visualização tanto no disco quanto no VFS.
- **`View::share("key", $value)`** ou **`View::share($map)`**: Compartilhe variáveis ​​globais disponíveis para todas as visualizações renderizadas durante a aplicação (por exemplo, configurações do site, dados do usuário, marca).
- **`View::render("vista.nombre", $data)`**: Renderiza a visualização com os dados fornecidos.

## Dados globais automáticos

O renderizador injeta automaticamente `auth_check`, `auth_guest`, `auth_user`, `auth_role`, `auth_email`, `csrf_token` e mensagens flash quando elas existem.