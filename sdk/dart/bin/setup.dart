import 'dart:io';
import 'package:joss/joss.dart';

void main(List<String> args) async {
  stdout.writeln('==============================================');
  stdout.writeln('🔧 Joss SDK: Configuración para Flutter/Android');
  stdout.writeln('==============================================');

  final currentDir = Directory.current;
  final androidMain = Directory('${currentDir.path}/android/app/src/main');

  if (androidMain.existsSync()) {
    stdout.writeln('📱 Proyecto Android detectado en: ${androidMain.path}');
    final targetDir = Directory('${androidMain.path}/assets/joss');
    if (!targetDir.existsSync()) {
      targetDir.createSync(recursive: true);
    }

    stdout.writeln('⬇️  Descargando y empaquetando motor Joss v${Joss.version} para Android...');
    try {
      final file = await JossDownloader.downloadAndConfigure(
        version: Joss.version,
        targetDir: targetDir,
        onProgress: (p) {
          final pct = (p * 100).toStringAsFixed(1);
          stdout.write('\rProgreso: $pct%');
        },
      );
      stdout.writeln('\n✅ Motor Joss instalado exitosamente en: ${file.path}');
    } catch (e) {
      stderr.writeln('\n❌ Error al configurar Joss para Android: $e');
      exit(1);
    }
  } else {
    stdout.writeln('ℹ️  Configurando motor Joss en directorio local...');
    try {
      final file = await JossDownloader.downloadAndConfigure(
        version: Joss.version,
        onProgress: (p) {
          final pct = (p * 100).toStringAsFixed(1);
          stdout.write('\rProgreso: $pct%');
        },
      );
      stdout.writeln('\n✅ Motor Joss configurado en: ${file.path}');
    } catch (e) {
      stderr.writeln('\n❌ Error: $e');
      exit(1);
    }
  }

  stdout.writeln('\n🎉 Configuración finalizada. ¡Listo para ejecutar código Joss!');
}
