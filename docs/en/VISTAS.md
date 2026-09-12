# Views

[Index](README.md)

`view("dashboard.index", $data)` (or `View::render`) looks for `app/views/dashboard/index.joss.html` and then `.html`.```html
<h1>{{ $title }}</h1>
<div>{{! $trusted_html }}</div>
```
`{{ expr }}` escapes HTML. `{{! expr }}` inserts unescaped output and should only receive trusted content. `{{ csrf_field() }}` is transformed into raw output to generate the CSRF input.

## Layouts and includes```html
@extends('layouts.master')
@section('content')
    @include('partials.alert')
@endsection
```
The layout uses `@yield('content')`. `@extends` is only recognized at the logical start of the view. The includes are resolved before the template is compiled.

## Foreach and conditionals```html
@foreach($users as $user)
    <p>{{ $user.name }}</p>
    {{ ($user.active) ? { <span>Activo</span> } : { <span>Inactivo</span> } }}
@endforeach

<!-- Soporte para expresiones anidadas y colecciones indexadas -->
@foreach($order["items"] as $it)
    <div>{{ $it["product"]["title"] }} - ${{ $it["unit_price"] }}</div>
@endforeach
```
The compiler recursively processes the body of each `@foreach` (supporting arbitrary expressions, simple variables, nested arrays, or property accesses) and block ternaries can freely interact with the iterated variables. Reminder: In Joss `@if`, `@else` and `@endif` do not exist; Are functional ternaries used `($cond) ? { ... } : { ... }`.

The `$map.key` notation within view expressions translates to `$map->key`. The evaluator allows you to read maps and instances with that shape.

## Directives and Comments in Templates

1. **`@json($data)`** directive:
   Allows you to safely dump objects, maps or arrays into JavaScript attributes:   ```html
   <script>
       const config = @json($appConfig);
   </script>
   ```
2. **Template Comments `{{-- Comentario --}}`**:
   Blade comment blocks are removed before rendering and do not reach the HTML client:   ```html
   {{-- Este comentario no se muestra en el navegador ni en el código fuente --}}
   ```
## Native `View` Methods

- **`View::exists("vista.nombre")`**: Returns `true` or `false` depending on the existence of the view file both on disk and in the VFS.
- **`View::share("key", $value)`** or **`View::share($map)`**: Share global variables available for all views rendered during the application (e.g. site configurations, user data, branding).
- **`View::render("vista.nombre", $data)`**: Renders the view with the provided data.

## Automatic global data

The renderer automatically injects `auth_check`, `auth_guest`, `auth_user`, `auth_role`, `auth_email`, `csrf_token` and flash messages when they exist.