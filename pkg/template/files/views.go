package files

import "path/filepath"

func GetViewFiles(path string) map[string]string {
	return map[string]string{
		filepath.Join(path, "app", "views", "profile", "index.joss.html"): `@extends('layouts.master')

@section('content')
<div class="max-w-4xl mx-auto space-y-8">
    
    {{ ($success) ? {
    <div class="flex items-center p-4 text-green-800 rounded-xl bg-green-50/50 dark:bg-green-950/10 dark:text-green-400 border border-green-200 dark:border-green-900/30" role="alert">
        <i class="fas fa-check-circle mr-2"></i>
        <span class="text-sm font-medium">{{ $success }}</span>
    </div>
    } }}

    {{ ($error) ? {
    <div class="flex items-center p-4 text-red-800 rounded-xl bg-red-50/50 dark:bg-red-950/10 dark:text-red-400 border border-red-200 dark:border-red-900/30" role="alert">
        <i class="fas fa-exclamation-circle mr-2"></i>
        <span class="text-sm font-medium">{{ $error }}</span>
    </div>
    } }}
    
    <!-- Profile Banner Card -->
    <div class="relative overflow-hidden rounded-2xl bg-gradient-to-r from-blue-600 to-indigo-700 p-8 text-white shadow-lg">
        <div class="absolute inset-0 bg-black/10 backdrop-blur-[1px]"></div>
        <div class="relative z-10 flex flex-col md:flex-row items-center gap-6">
            <img src="https://ui-avatars.com/api/?name={{ $auth_user }}&background=ffffff&color=2563eb&size=128&bold=true" alt="Avatar" class="w-24 h-24 rounded-full border-4 border-white/20 shadow-xl">
            <div class="text-center md:text-left">
                <div class="flex flex-wrap gap-2 items-center justify-center md:justify-start">
                    <h2 class="text-3xl font-extrabold tracking-tight">{{ $first_name }} {{ $last_name }}</h2>
                    <span class="bg-white/20 text-xs px-2.5 py-0.5 rounded-full border border-white/10 uppercase font-semibold">{{ $auth_role }}</span>
                </div>
                <p class="text-white/80 mt-1"><i class="far fa-envelope mr-2"></i>{{ $email }}</p>
            </div>
        </div>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-3 gap-8">
        
        <!-- Info column -->
        <div class="md:col-span-1 space-y-6">
            <div class="p-6 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-sm">
                <h3 class="font-bold text-gray-900 dark:text-white border-b border-gray-100 dark:border-gray-700 pb-3 mb-4">Detalles</h3>
                <div class="space-y-3 text-sm">
                    <div>
                        <span class="text-gray-400 block">Usuario</span>
                        <span class="font-medium text-gray-800 dark:text-gray-200">{{ $auth_user }}</span>
                    </div>
                    <div>
                        <span class="text-gray-400 block">Teléfono</span>
                        <span class="font-medium text-gray-800 dark:text-gray-200">{{ $phone ? $phone : 'No registrado' }}</span>
                    </div>
                </div>
            </div>

            <!-- Danger Zone -->
            <div class="p-6 bg-red-50/50 dark:bg-red-950/10 border border-red-100 dark:border-red-900/30 rounded-2xl">
                <h3 class="font-bold text-red-700 dark:text-red-500 pb-2">Zona de Peligro</h3>
                <p class="text-xs text-red-600 dark:text-red-400 mb-4 leading-relaxed">Una vez eliminada la cuenta, tus datos no podrán recuperarse.</p>
                <form action="/profile/delete" method="POST" onsubmit="return confirm('¿Seguro que deseas eliminar tu cuenta permanentemente?');">
                     {{ csrf_field() }}
                     <button type="submit" class="w-full py-2.5 px-4 text-sm font-semibold text-white bg-red-600 hover:bg-red-700 rounded-xl transition duration-150">Eliminar Cuenta</button>
                </form>
            </div>
        </div>

        <!-- Form column -->
        <div class="md:col-span-2 p-6 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-sm">
            <h3 class="font-bold text-gray-900 dark:text-white border-b border-gray-100 dark:border-gray-700 pb-3 mb-6">Actualizar Información</h3>
            
            <form action="/profile/update" method="POST" class="space-y-5">
                {{ csrf_field() }}
                
                <div class="grid grid-cols-2 gap-4">
                    <div>
                        <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Nombre</label>
                        <input type="text" name="first_name" class="w-full p-2.5 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" value="{{ $first_name }}" required>
                    </div>
                    <div>
                        <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Apellido</label>
                        <input type="text" name="last_name" class="w-full p-2.5 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" value="{{ $last_name }}" required>
                    </div>
                </div>

                <div>
                    <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Teléfono</label>
                    <input type="text" name="phone" class="w-full p-2.5 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" value="{{ $phone }}">
                </div>

                <div>
                    <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Contraseña Nueva</label>
                    <input type="password" name="password" class="w-full p-2.5 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" placeholder="••••••••">
                    <p class="text-[11px] text-gray-400 mt-1">Dejar en blanco para mantener la contraseña actual.</p>
                </div>

                <button type="submit" class="w-full py-3 px-4 font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl shadow-lg shadow-blue-500/20 transition duration-150">Guardar Cambios</button>
            </form>

            <!-- 2FA Setup Card -->
            <div class="mt-8 pt-8 border-t border-gray-100 dark:border-gray-700">
                <h3 class="font-bold text-gray-900 dark:text-white mb-6">Autenticación de Doble Factor (2FA)</h3>
                
                {{ ($mfa_enabled) ? {
                    <div class="p-6 bg-green-500/10 border border-green-500/20 rounded-2xl flex flex-col sm:flex-row items-center justify-between gap-4">
                        <div class="flex items-center gap-3">
                            <div class="p-2 bg-green-500/20 text-green-500 rounded-xl">
                                <i class="fas fa-shield-alt text-lg"></i>
                            </div>
                            <div>
                                <span class="font-bold text-green-800 dark:text-green-400 block text-sm">Autenticación 2FA Activa</span>
                                <span class="text-xs text-green-700 dark:text-green-450 leading-relaxed">Tu cuenta está protegida con código TOTP adicional al iniciar sesión.</span>
                            </div>
                        </div>
                        <form action="/profile/2fa/deactivate" method="POST">
                            {{ csrf_field() }}
                            <button type="submit" class="px-4 py-2 text-xs font-bold text-red-500 hover:text-white hover:bg-red-650 border border-red-500/30 rounded-xl transition">Desactivar</button>
                        </form>
                    </div>
                } : {
                    <div class="grid grid-cols-1 sm:grid-cols-3 gap-6 items-center">
                        <div class="sm:col-span-1 flex justify-center">
                            {{ ($qr_code) ? {
                                <img src="{{ $qr_code }}" alt="QR 2FA" class="w-40 h-40 bg-white p-2 rounded-xl border border-gray-200">
                            } }}
                        </div>
                        <div class="sm:col-span-2 space-y-4">
                            <p class="text-xs text-gray-400 leading-relaxed">Escanea este código QR con tu aplicación autenticadora (Google Authenticator o Authy) e introduce el código de 6 dígitos abajo para activarlo.</p>
                            <form action="/profile/2fa/activate" method="POST" class="flex gap-2">
                                {{ csrf_field() }}
                                <input type="text" name="code" class="flex-1 p-2.5 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm tracking-widest text-center font-mono" placeholder="000000" maxlength="6" required>
                                <button type="submit" class="px-6 py-2.5 font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl transition">Activar</button>
                            </form>
                        </div>
                    </div>
                } }}
            </div>
        </div>
    </div>
</div>
@endsection`,
		filepath.Join(path, "app", "views", "welcome.joss.html"): `@extends('layouts.master')

@section('content')
<div class="max-w-3xl mx-auto text-center py-16 space-y-8">
    <div class="inline-flex p-4 bg-blue-600/10 border border-blue-500/20 rounded-2xl text-blue-500 mb-2">
        <i class="fas fa-cube text-4xl"></i>
    </div>
    
    <h1 class="text-5xl md:text-6xl font-extrabold tracking-tight text-gray-950 dark:text-white">
        Bienvenido a <span class="text-transparent bg-clip-text bg-gradient-to-r from-blue-500 to-indigo-600">Joss</span>
    </h1>
    
    <p class="text-lg md:text-xl text-gray-500 dark:text-gray-400 leading-relaxed max-w-2xl mx-auto">
        El framework y lenguaje de programación de alto rendimiento, seguro, tipado y optimizado nativamente para la nube.
    </p>
    
    <div class="flex flex-wrap justify-center gap-4 pt-4">
        <a href="/login" class="px-8 py-3.5 bg-blue-600 hover:bg-blue-700 text-white font-bold rounded-xl shadow-lg shadow-blue-500/20 transition">Iniciar Sesión</a>
        <a href="/register" class="px-8 py-3.5 bg-gray-100 hover:bg-gray-200 dark:bg-gray-800 dark:hover:bg-gray-750 text-gray-800 dark:text-white font-bold rounded-xl border border-gray-250/10 transition">Crear Cuenta</a>
    </div>

    <div class="pt-12">
        <span class="bg-gray-100 dark:bg-gray-900/50 text-gray-500 dark:text-gray-400 text-xs font-mono px-3 py-1.5 rounded-full border border-gray-200 dark:border-gray-800">
            Joss Compiler v{{ $version }}
        </span>
    </div>
</div>
@endsection`,
		filepath.Join(path, "app", "views", "auth", "login.joss.html"): `@extends('layouts.master')

@section('content')
<div class="max-w-md mx-auto bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-xl overflow-hidden p-8">
    <div class="text-center mb-8">
        <h2 class="text-2xl font-bold text-gray-950 dark:text-white">{{ $title }}</h2>
        <p class="text-sm text-gray-400 mt-1">Introduce tus credenciales para ingresar</p>
    </div>

    {{ ($error) ? {
    <div class="flex items-center p-4 mb-6 text-red-800 rounded-xl bg-red-50/50 dark:bg-red-950/10 dark:text-red-400 border border-red-200 dark:border-red-900/30" role="alert">
        <i class="fas fa-exclamation-circle mr-2"></i>
        <span class="text-sm font-medium">{{ $error }}</span>
    </div>
    } }}

    {{ ($success) ? {
    <div class="flex items-center p-4 mb-6 text-green-800 rounded-xl bg-green-50/50 dark:bg-green-950/10 dark:text-green-400 border border-green-200 dark:border-green-900/30" role="alert">
        <i class="fas fa-check-circle mr-2"></i>
        <span class="text-sm font-medium">{{ $success }}</span>
    </div>
    } }}

    <form method="POST" action="/login" class="space-y-5">
        {{ csrf_field() }}
        <div>
            <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Correo Electrónico</label>
            <input type="email" name="email" class="w-full p-3 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" required placeholder="ejemplo@correo.com">
        </div>
        <div>
            <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Contraseña</label>
            <input type="password" name="password" class="w-full p-3 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" required placeholder="••••••">
        </div>
        <button type="submit" class="w-full py-3 px-4 font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl shadow-lg shadow-blue-500/20 transition duration-150">Entrar</button>
    </form>
    
    <div class="mt-6 text-center text-sm border-t border-gray-100 dark:border-gray-700 pt-5">
        <p class="text-gray-400">¿No tienes cuenta? <a href="/register" class="text-blue-500 hover:underline">Regístrate aquí</a></p>
    </div>
</div>
@endsection`,
		filepath.Join(path, "app", "views", "auth", "register.joss.html"): `@extends('layouts.master')

@section('content')
<div class="max-w-md mx-auto bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-xl overflow-hidden p-8">
    <div class="text-center mb-8">
        <h2 class="text-2xl font-bold text-gray-950 dark:text-white">{{ $title }}</h2>
        <p class="text-sm text-gray-400 mt-1">Crea tu cuenta de Joss Red en segundos</p>
    </div>

    {{ ($error) ? {
    <div class="flex items-center p-4 mb-6 text-red-800 rounded-xl bg-red-50/50 dark:bg-red-950/10 dark:text-red-400 border border-red-200 dark:border-red-900/30" role="alert">
        <i class="fas fa-exclamation-circle mr-2"></i>
        <span class="text-sm font-medium">{{ $error }}</span>
    </div>
    } }}

    <form method="POST" action="/register" class="space-y-5">
        {{ csrf_field() }}
        <div class="grid grid-cols-2 gap-4">
            <div>
                <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Nombre</label>
                <input type="text" name="first_name" class="w-full p-2.5 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" required placeholder="Juan">
            </div>
            <div>
                <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Apellidos</label>
                <input type="text" name="last_name" class="w-full p-2.5 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" required placeholder="Pérez">
            </div>
        </div>
        <div>
            <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Nombre de Usuario</label>
            <input type="text" name="username" class="w-full p-3 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" required placeholder="juanperez">
        </div>
        <div>
            <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Teléfono</label>
            <input type="tel" name="phone" class="w-full p-3 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" placeholder="+52...">
        </div>
        <div>
            <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Correo Electrónico</label>
            <input type="email" name="email" class="w-full p-3 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" required placeholder="ejemplo@correo.com">
        </div>
        <div>
            <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Contraseña</label>
            <input type="password" name="password" class="w-full p-3 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" required placeholder="••••••">
        </div>
        <button type="submit" class="w-full py-3 px-4 font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl shadow-lg shadow-blue-500/20 transition duration-150">Crear Cuenta</button>
    </form>
    
    <div class="mt-6 text-center text-sm border-t border-gray-100 dark:border-gray-700 pt-5">
        <p class="text-gray-400">¿Ya tienes cuenta? <a href="/login" class="text-blue-500 hover:underline">Inicia sesión</a></p>
    </div>
</div>
@endsection`,
		filepath.Join(path, "app", "views", "auth", "forgot.joss.html"): `@extends('layouts.master')

@section('content')
<div class="max-w-md mx-auto bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-xl overflow-hidden p-8">
    <div class="text-center mb-8">
        <h2 class="text-2xl font-bold text-gray-950 dark:text-white">Recuperar Contraseña</h2>
        <p class="text-sm text-gray-400 mt-1">Te enviaremos un enlace de recuperación</p>
    </div>

    {{ ($success) ? {
    <div class="flex items-center p-4 mb-6 text-green-800 rounded-xl bg-green-50/50 dark:bg-green-950/10 dark:text-green-400 border border-green-200 dark:border-green-900/30" role="alert">
        <i class="fas fa-check-circle mr-2"></i>
        <span class="text-sm font-medium">{{ $success }}</span>
    </div>
    } }}

    {{ ($error) ? {
    <div class="flex items-center p-4 mb-6 text-red-800 rounded-xl bg-red-50/50 dark:bg-red-950/10 dark:text-red-400 border border-red-200 dark:border-red-900/30" role="alert">
        <i class="fas fa-exclamation-circle mr-2"></i>
        <span class="text-sm font-medium">{{ $error }}</span>
    </div>
    } }}

    <form method="POST" action="/password/email" class="space-y-5">
        {{ csrf_field() }}
        <div>
            <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Email</label>
            <input type="email" name="email" class="w-full p-3 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" required placeholder="ejemplo@correo.com">
        </div>
        <button type="submit" class="w-full py-3 px-4 font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl shadow-lg shadow-blue-500/20 transition duration-150">Enviar Enlace</button>
    </form>
</div>
@endsection`,
		filepath.Join(path, "app", "views", "auth", "reset.joss.html"): `@extends('layouts.master')

@section('content')
<div class="max-w-md mx-auto bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-xl overflow-hidden p-8">
    <div class="text-center mb-8">
        <h2 class="text-2xl font-bold text-gray-950 dark:text-white">Establecer Nueva Contraseña</h2>
    </div>

    {{ ($error) ? {
    <div class="flex items-center p-4 mb-6 text-red-800 rounded-xl bg-red-50/50 dark:bg-red-950/10 dark:text-red-400 border border-red-200 dark:border-red-900/30" role="alert">
        <i class="fas fa-exclamation-circle mr-2"></i>
        <span class="text-sm font-medium">{{ $error }}</span>
    </div>
    } }}

    <form method="POST" action="/password/reset" class="space-y-5">
        {{ csrf_field() }}
        <input type="hidden" name="token" value="{{ $token }}">
        <div>
            <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">Nueva Contraseña</label>
            <input type="password" name="password" class="w-full p-3 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm" required placeholder="••••••">
        </div>
        <button type="submit" class="w-full py-3 px-4 font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl shadow-lg shadow-blue-500/20 transition duration-150">Cambiar Contraseña</button>
    </form>
</div>
@endsection`,
		filepath.Join(path, "app", "views", "dashboard", "index.joss.html"): `@extends('layouts.master')

@section('content')
<div class="space-y-8">
    <!-- Welcome Header -->
    <div class="p-8 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-sm flex flex-col md:flex-row justify-between items-center gap-6">
        <div>
            <h1 class="text-3xl font-extrabold tracking-tight text-gray-950 dark:text-white">Bienvenido, {{ $user_name }}</h1>
            <p class="text-gray-400 text-sm mt-1">Has iniciado sesión correctamente en el sistema Joss Enterprise.</p>
        </div>
        <div class="flex items-center gap-3">
            <span class="bg-blue-500/10 border border-blue-500/20 text-blue-500 text-xs px-3 py-1 rounded-full font-bold uppercase tracking-wider">{{ $role }}</span>
            <a href="/logout" class="px-4 py-2 text-xs font-bold text-white bg-red-600 hover:bg-red-700 rounded-xl transition">Cerrar Sesión</a>
        </div>
    </div>

    {{ ($isAdmin) ? {
    <div class="flex items-start p-4 text-blue-800 rounded-2xl bg-blue-50/50 dark:bg-gray-800/50 dark:text-blue-400 border border-blue-200 dark:border-blue-900/30" role="alert">
        <i class="fas fa-info-circle text-xl mr-3 mt-0.5"></i>
        <div>
            <span class="font-bold block text-sm">Panel de Administrador</span>
            <span class="text-xs text-blue-700 dark:text-blue-400/80 leading-relaxed">Tienes acceso total al sistema. Puedes gestionar repositorios, aplicaciones y notificaciones globales.</span>
        </div>
    </div>
    } }}

    <!-- Stat cards -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div class="p-6 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-sm flex items-center justify-between">
            <div>
                <span class="text-xs text-gray-400 uppercase font-semibold">Proyectos</span>
                <h3 class="text-3xl font-extrabold text-gray-900 dark:text-white mt-1">12</h3>
            </div>
            <div class="p-3 bg-blue-500/10 text-blue-500 rounded-xl">
                <i class="fas fa-folder-open text-xl"></i>
            </div>
        </div>
        
        <div class="p-6 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-sm flex items-center justify-between">
            <div>
                <span class="text-xs text-gray-400 uppercase font-semibold">Alertas</span>
                <h3 class="text-3xl font-extrabold text-gray-900 dark:text-white mt-1">3</h3>
            </div>
            <div class="p-3 bg-amber-500/10 text-amber-500 rounded-xl">
                <i class="fas fa-bell text-xl"></i>
            </div>
        </div>

        <div class="p-6 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-sm flex items-center justify-between">
            <div>
                <span class="text-xs text-gray-400 uppercase font-semibold">Usuarios</span>
                <h3 class="text-3xl font-extrabold text-gray-900 dark:text-white mt-1">150</h3>
            </div>
            <div class="p-3 bg-green-500/10 text-green-500 rounded-xl">
                <i class="fas fa-users text-xl"></i>
            </div>
        </div>
    </div>
</div>
@endsection`,
		filepath.Join(path, "app", "views", "layouts", "master.joss.html"): `<!DOCTYPE html>
<html lang="es" class="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ $title ?? "Joss" }}</title>
    <!-- Tailwind CSS (Play CDN) -->
    <script src="https://cdn.tailwindcss.com"></script>
    <script>
        tailwind.config = {
            darkMode: 'class',
            theme: {
                extend: {
                    fontFamily: {
                        sans: ['Outfit', 'sans-serif'],
                    }
                }
            }
        }
    </script>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;500;600;700&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    
    <style>
        body {
            background-color: #03060b; 
            color: #fff;
            background-image: radial-gradient(circle at 50% 0%, #172554 0%, #03060b 80%);
            min-height: 100vh;
        }
    </style>
</head>
<body class="antialiased">
    <!-- Navbar -->
    <nav class="fixed w-full z-50 top-0 start-0 border-b border-white/5 bg-[#03060b]/70 backdrop-blur-md">
      <div class="max-w-screen-xl flex flex-wrap items-center justify-between mx-auto p-4">
        
        <!-- Logo -->
        <a href="/" class="flex items-center space-x-3 group">
            <div class="relative flex items-center justify-center w-9 h-9 rounded-lg bg-gradient-to-br from-blue-600 to-indigo-700 shadow-md group-hover:scale-105 transition-transform duration-300">
                <i class="fas fa-cube text-white text-md"></i>
            </div>
            <span class="self-center text-xl font-bold whitespace-nowrap text-white tracking-tight">
                JOSS<span class="text-blue-500">APP</span>
            </span>
        </a>

        <!-- Right Menu -->
        <div class="flex items-center md:order-2 space-x-4">
            {{ ($auth_check) ? {
                <!-- User Profile Summary -->
                <a href="/profile" class="flex items-center space-x-2 text-sm text-gray-300 hover:text-white bg-white/5 px-3 py-1.5 rounded-xl border border-white/5">
                    <img class="w-6 h-6 rounded-full border border-gray-700" src="https://ui-avatars.com/api/?name={{ $auth_user }}&background=3b82f6&color=fff&bold=true" alt="avatar">
                    <span class="hidden sm:inline font-medium text-xs">{{ $auth_user }}</span>
                </a>
                <a href="/logout" class="text-xs font-bold text-red-400 hover:text-red-300 transition"><i class="fas fa-sign-out-alt"></i></a>
            } : {
                 <div class="flex space-x-3">
                    <a href="/login" class="text-white hover:text-blue-400 text-xs font-semibold px-3 py-2 transition">Login</a>
                    <a href="/register" class="text-white bg-blue-600 hover:bg-blue-700 text-xs font-bold rounded-xl px-4 py-2 transition shadow-lg shadow-blue-500/20">Sign up</a>
                </div>
            } }}
        </div>

        <!-- Center Menu -->
        <div class="items-center justify-between hidden w-full md:flex md:w-auto md:order-1">
            <ul class="flex flex-col font-medium p-4 md:p-0 mt-4 border border-white/5 rounded-lg md:space-x-8 md:flex-row md:mt-0 md:border-0">
                <li><a href="/" class="block py-2 px-3 text-white rounded-lg md:bg-transparent">Inicio</a></li>
                {{ ($auth_check) ? {
                    <li><a href="/dashboard" class="block py-2 px-3 text-gray-300 hover:text-white">Dashboard</a></li>
                    <!-- Injected Links Here -->
                } }}
            </ul>
        </div>
      </div>
    </nav>

    <!-- Main Content Wrapper -->
    <div class="pt-32 pb-16 px-4 max-w-screen-xl mx-auto min-h-screen">
       @yield('content')
    </div>
</body>
</html>`,

		filepath.Join(path, "app", "views", "auth", "verify_2fa.joss.html"): `@extends('layouts.master')

@section('content')
<section class="bg-gray-50 dark:bg-gray-900 py-12 flex justify-center items-center">
  <div class="w-full max-w-md bg-white dark:bg-gray-800 rounded-2xl shadow-xl border border-gray-100 dark:border-gray-700 p-8 space-y-6">
      
      <div class="text-center space-y-2">
          <div class="w-12 h-12 rounded-2xl bg-blue-500/10 text-blue-600 dark:text-blue-400 mx-auto flex items-center justify-center text-2xl shadow-inner">
              <i class="fas fa-shield-alt"></i>
          </div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
              Verificación 2FA
          </h1>
          <p class="text-xs text-gray-500 dark:text-gray-400">
              Introduce el código de 6 dígitos generado por tu aplicación autenticadora.
          </p>
      </div>
      
      {{ ($error) ? {
      <div class="p-4 text-sm text-red-800 rounded-xl bg-red-50 dark:bg-red-950/20 dark:text-red-400 border border-red-200 dark:border-red-900/30" role="alert">
          <i class="fas fa-exclamation-circle mr-1"></i> {{ $error }}
      </div>
      } }}

      <form action="/2fa/verify" method="POST" class="space-y-6">
          {{ csrf_field() }}
          
          <div>
              <label class="block mb-2 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase text-center">Código de Seguridad</label>
              <input type="text" 
                     name="code" 
                     maxlength="6" 
                     autocomplete="one-time-code" 
                     inputmode="numeric" 
                     pattern="[0-9]*" 
                     required 
                     autofocus
                     placeholder="000000"
                     class="w-full p-4 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-2xl tracking-[0.5em] text-center font-mono font-bold text-gray-900 dark:text-white shadow-inner">
          </div>
          
          <button type="submit" class="w-full py-3.5 font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl shadow-lg shadow-blue-500/20 transition duration-150">
              Verificar e Ingresar
          </button>
          
          <div class="text-center">
              <a href="/login" class="text-xs text-gray-500 hover:text-blue-600 transition">Volver a iniciar sesión</a>
          </div>
      </form>
  </div>
</section>
@endsection`,
	}
}
