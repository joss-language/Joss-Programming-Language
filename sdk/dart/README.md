# Joss SDK para Dart y Flutter

SDK oficial para ejecutar y probar código del lenguaje de programación **Joss** en aplicaciones móviles y de escritorio (**Android**, **iOS**, **Windows**, **Linux**, **macOS**).

---

## 🚀 Instalación en Flutter / Dart

En tu `pubspec.yaml`:

```yaml
dependencies:
  joss:
    git:
      url: https://github.com/joss-language/Joss-Programming-Language.git
      path: sdk/dart
      ref: v3.6.7 # O la versión que utilices
```

Ejecuta:
```bash
flutter pub get
```

---

## 📱 Uso Rápido

```dart
import 'package:joss/joss.dart';

void probarCodigo() async {
  // Código Joss a ejecutar
  const codigo = '''
  public func main(): void {
      print("¡Hola desde Joss en Flutter!");
      int \$total = 10 + 25;
      cout << "Total calculado: " << \$total << endl;
  }
  main();
  ''';

  // Ejecución automática (descarga y configura el motor si es necesario)
  final result = await Joss.run(codigo, timeoutMs: 3000);

  if (result.isSuccess) {
    print('Salida:');
    print(result.stdout);
  } else {
    print('Error: ${result.error}');
  }
}
```

---

## ⚙️ Configuración Previa para Android (Opcional)

Si deseas pre-descargar el motor de Joss para que tu app Android funcione sin descargar nada en el primer inicio:

```bash
dart run joss:setup
```

Esto descargará el binario compilado de Android (`joss-android-arm64`) directamente dentro de `android/app/src/main/assets/joss/`.
