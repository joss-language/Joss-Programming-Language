import 'dart:ffi';
import 'dart:io';
import 'package:archive/archive.dart';
import 'package:http/http.dart' as http;
import 'package:path/path.dart' as p;

class JossDownloader {
  static const String repoOwner = 'joss-language';
  static const String repoName = 'Joss-Programming-Language';

  /// Determina el nombre del zip de release según el sistema operativo.
  static String getReleaseZipName() {
    if (Platform.isAndroid) return 'jossecurity-android.zip';
    if (Platform.isWindows) return 'jossecurity-windows.zip';
    if (Platform.isMacOS) return 'jossecurity-macos.zip';
    if (Platform.isLinux) return 'jossecurity-linux.zip';
    throw UnsupportedError('Sistema operativo no soportado para descarga automática: ${Platform.operatingSystem}');
  }

  /// Determina el nombre del archivo ejecutable dentro del zip para la plataforma actual.
  static String getExecutableNameInZip() {
    final abi = Abi.current();

    if (Platform.isAndroid) {
      if (abi == Abi.androidArm64) return 'joss-android-arm64';
      if (abi == Abi.androidArm) return 'joss-android-armv7';
      return 'joss-android-arm64'; // default
    }

    if (Platform.isWindows) {
      if (abi == Abi.windowsArm64) return 'joss-windows-arm64.exe';
      return 'joss.exe';
    }

    if (Platform.isLinux) {
      if (abi == Abi.linuxArm64) return 'joss-linux-arm64';
      if (abi == Abi.linuxArm) return 'joss-linux-armv7';
      return 'joss-linux-amd64';
    }

    if (Platform.isMacOS) {
      if (abi == Abi.macosArm64) return 'joss-macos-arm64';
      return 'joss-macos-amd64';
    }

    return 'joss';
  }

  /// Directorio predeterminado de caché para el motor de Joss.
  static Directory getDefaultInstallDirectory() {
    if (Platform.isAndroid) {
      // En Android, los ejecutables deben vivir en el almacenamiento interno de la app
      final filesDir = Directory('/data/data/com.aprende_mas/files');
      if (filesDir.existsSync()) return filesDir;
    }

    final home = Platform.environment['HOME'] ?? Platform.environment['USERPROFILE'];
    if (home != null && home.isNotEmpty) {
      return Directory(p.join(home, '.joss', 'bin'));
    }

    return Directory(p.join(Directory.systemTemp.path, 'joss_bin'));
  }

  /// Descarga y configura el ejecutable de Joss para la versión indicada.
  static Future<File> downloadAndConfigure({
    required String version,
    Directory? targetDir,
    void Function(double progress)? onProgress,
  }) async {
    final installDir = targetDir ?? getDefaultInstallDirectory();
    if (!installDir.existsSync()) {
      installDir.createSync(recursive: true);
    }

    final exeName = Platform.isWindows ? 'joss.exe' : 'joss';
    final targetFile = File(p.join(installDir.path, exeName));

    final zipName = getReleaseZipName();
    final internalExeName = getExecutableNameInZip();

    final normalizedVersion = version.startsWith('v') || version.startsWith('V') ? version : 'v$version';
    final url = Uri.parse(
      'https://github.com/$repoOwner/$repoName/releases/download/$normalizedVersion/$zipName',
    );

    final client = http.Client();
    try {
      final response = await client.send(http.Request('GET', url));
      if (response.statusCode != 200) {
        throw HttpException('Error al descargar Joss ($zipName): HTTP ${response.statusCode}');
      }

      final contentLength = response.contentLength ?? 0;
      final bytes = <int>[];
      var downloaded = 0;

      await for (final chunk in response.stream) {
        bytes.addAll(chunk);
        downloaded += chunk.length;
        if (contentLength > 0 && onProgress != null) {
          onProgress(downloaded / contentLength);
        }
      }

      // Descomprimir el archivo en memoria
      final archive = ZipDecoder().decodeBytes(bytes);
      ArchiveFile? targetArchiveFile;

      for (final file in archive) {
        if (file.name == internalExeName || p.basename(file.name) == internalExeName) {
          targetArchiveFile = file;
          break;
        }
      }

      // Si no encontró el nombre específico, busca cualquier ejecutable joss
      if (targetArchiveFile == null) {
        for (final file in archive) {
          if (p.basename(file.name).startsWith('joss')) {
            targetArchiveFile = file;
            break;
          }
        }
      }

      if (targetArchiveFile == null) {
        throw StateError('No se encontró el ejecutable $internalExeName dentro de $zipName');
      }

      targetFile.writeAsBytesSync(targetArchiveFile.content as List<int>, flush: true);

      // En Android, Linux y macOS, asegurar permisos de ejecución (+x)
      if (!Platform.isWindows) {
        try {
          await Process.run('chmod', ['755', targetFile.path]);
        } catch (_) {}
      }

      return targetFile;
    } finally {
      client.close();
    }
  }
}
