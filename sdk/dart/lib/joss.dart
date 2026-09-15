import 'dart:convert';
import 'dart:io';
import 'package:path/path.dart' as p;

import 'src/downloader.dart';
import 'src/models.dart';

export 'src/downloader.dart';
export 'src/models.dart';

/// Punto de entrada principal del SDK de Joss para Dart y Flutter.
class Joss {
  static const String version = '3.6.7';
  static String? _configuredBinaryPath;

  /// Permite establecer manualmente la ruta del binario de Joss si ya existe.
  static void setBinaryPath(String path) {
    _configuredBinaryPath = path;
  }

  /// Busca el binario de Joss disponible en el sistema o en la caché de la app.
  static Future<String?> findBinaryPath({Directory? targetDir}) async {
    if (_configuredBinaryPath != null && File(_configuredBinaryPath!).existsSync()) {
      return _configuredBinaryPath;
    }

    // 1. Verificar directorio de instalación configurado
    final installDir = targetDir ?? JossDownloader.getDefaultInstallDirectory();
    final exeName = Platform.isWindows ? 'joss.exe' : 'joss';
    final localFile = File(p.join(installDir.path, exeName));
    if (localFile.existsSync()) {
      return localFile.path;
    }

    // 2. Verificar en PATH global del sistema
    try {
      final cmd = Platform.isWindows ? 'where' : 'which';
      final res = await Process.run(cmd, [exeName]);
      if (res.exitCode == 0) {
        final line = (res.stdout as String).split(RegExp(r'[\r\n]+')).firstWhere((l) => l.trim().isNotEmpty, orElse: () => '');
        if (line.isNotEmpty && File(line).existsSync()) {
          return line;
        }
      }
    } catch (_) {}

    return null;
  }

  /// Asegura que el motor de Joss esté instalado. Si no existe, lo descarga y configura automáticamente.
  static Future<String> ensureInstalled({
    String targetVersion = version,
    Directory? targetDir,
    void Function(double progress)? onProgress,
  }) async {
    final existing = await findBinaryPath(targetDir: targetDir);
    if (existing != null) {
      return existing;
    }

    final downloaded = await JossDownloader.downloadAndConfigure(
      version: targetVersion,
      targetDir: targetDir,
      onProgress: onProgress,
    );

    _configuredBinaryPath = downloaded.path;
    return downloaded.path;
  }

  /// Ejecuta código fuente Joss y retorna un [JossResult] estructurado.
  /// Si el motor de Joss no está presente, lo descarga y configura de forma transparente.
  static Future<JossResult> run(
    String code, {
    int timeoutMs = 5000,
    Directory? targetDir,
    String? customBinaryPath,
    void Function(double progress)? onDownloadProgress,
  }) async {
    final stopwatch = Stopwatch()..start();

    try {
      String binaryPath;
      if (customBinaryPath != null && File(customBinaryPath).existsSync()) {
        binaryPath = customBinaryPath;
      } else {
        binaryPath = await ensureInstalled(
          targetDir: targetDir,
          onProgress: onDownloadProgress,
        );
      }

      // Ejecutar usando 'joss eval --json <codigo>'
      final processResult = await Process.run(
        binaryPath,
        ['eval', '--json', code],
      ).timeout(
        Duration(milliseconds: timeoutMs),
        onTimeout: () {
          return ProcessResult(-1, -1, '', 'Execution timed out ($timeoutMs ms)');
        },
      );

      final outStr = processResult.stdout.toString().trim();
      final errStr = processResult.stderr.toString().trim();

      // Intentar decodificar la salida JSON directa del runtime
      if (outStr.startsWith('{') && outStr.endsWith('}')) {
        try {
          final decoded = jsonDecode(outStr) as Map<String, dynamic>;
          return JossResult.fromJson(decoded);
        } catch (_) {}
      }

      final success = processResult.exitCode == 0;
      return JossResult(
        isSuccess: success,
        stdout: outStr,
        stderr: errStr,
        error: success ? null : (errStr.isNotEmpty ? errStr : 'Código de salida: ${processResult.exitCode}'),
        durationMs: stopwatch.elapsedMilliseconds,
      );
    } catch (e) {
      return JossResult.error(
        'Error de ejecución en Joss: $e',
        durationMs: stopwatch.elapsedMilliseconds,
      );
    }
  }

  /// Realiza análisis estático del código fuente Joss sin ejecutarlo.
  static Future<JossAnalysisResult> analyze(
    String code, {
    Directory? targetDir,
    String? customBinaryPath,
  }) async {
    final result = await run(
      code,
      timeoutMs: 5000,
      targetDir: targetDir,
      customBinaryPath: customBinaryPath,
    );
    return JossAnalysisResult(
      isValid: result.diagnostics.every((d) => d.severity != 'error'),
      diagnostics: result.diagnostics,
    );
  }
}

