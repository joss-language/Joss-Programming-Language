
/// Resultado estructurado de la ejecución de código Joss.
class JossResult {
  final bool isSuccess;
  final String stdout;
  final String stderr;
  final String? error;
  final bool timedOut;
  final int durationMs;
  final List<JossDiagnostic> diagnostics;

  const JossResult({
    required this.isSuccess,
    required this.stdout,
    required this.stderr,
    this.error,
    this.timedOut = false,
    required this.durationMs,
    this.diagnostics = const [],
  });

  factory JossResult.fromJson(Map<String, dynamic> json) {
    final diags = <JossDiagnostic>[];
    if (json['diagnostics'] is List) {
      for (final item in json['diagnostics']) {
        if (item is Map<String, dynamic>) {
          diags.add(JossDiagnostic.fromJson(item));
        }
      }
    }

    return JossResult(
      isSuccess: json['success'] == true,
      stdout: json['stdout'] ?? '',
      stderr: json['stderr'] ?? '',
      error: json['error'],
      timedOut: json['timed_out'] == true,
      durationMs: json['duration_ms'] ?? 0,
      diagnostics: diags,
    );
  }

  factory JossResult.error(String message, {int durationMs = 0}) {
    return JossResult(
      isSuccess: false,
      stdout: '',
      stderr: message,
      error: message,
      durationMs: durationMs,
    );
  }

  @override
  String toString() {
    return 'JossResult(success: $isSuccess, duration: ${durationMs}ms, stdout: ${stdout.trim()})';
  }
}

/// Diagnóstico de análisis semántico o sintáctico de Joss.
class JossDiagnostic {
  final String code;
  final String severity;
  final String message;
  final int line;
  final int column;
  final String? explanation;
  final String? suggestion;

  const JossDiagnostic({
    required this.code,
    required this.severity,
    required this.message,
    required this.line,
    required this.column,
    this.explanation,
    this.suggestion,
  });

  factory JossDiagnostic.fromJson(Map<String, dynamic> json) {
    return JossDiagnostic(
      code: json['code'] ?? '',
      severity: json['severity'] ?? 'error',
      message: json['message'] ?? '',
      line: json['line'] ?? 0,
      column: json['column'] ?? 0,
      explanation: json['explanation'],
      suggestion: json['suggestion'],
    );
  }
}
