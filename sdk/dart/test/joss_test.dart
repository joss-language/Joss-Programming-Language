import "package:joss/joss.dart";
import "package:test/test.dart";

void main() {
  test("Joss.run executes code and returns stdout", () async {
    final result = await Joss.run(r'echo "Hola desde Joss SDK!\n"; int $val = 100; echo $val;');
    expect(result.isSuccess, isTrue);
    expect(result.stdout, contains("Hola desde Joss SDK!"));
    expect(result.stdout, contains("100"));
  });

  test("Joss.analyze detects syntax error without executing", () async {
    final analysis = await Joss.analyze(r'int $a = ;');
    expect(analysis.isValid, isFalse);
    expect(analysis.diagnostics, isNotEmpty);
  });
}
