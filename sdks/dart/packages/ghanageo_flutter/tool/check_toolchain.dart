import 'dart:convert';
import 'dart:io';

Future<void> main() async {
  const expectedDart = '3.13.2';
  const expectedFlutter = '3.47.2';
  final result = await Process.run('flutter', ['--version', '--machine']);
  if (result.exitCode != 0) {
    stderr.write(result.stderr);
    exit(result.exitCode);
  }
  final version = jsonDecode(result.stdout as String) as Map<String, Object?>;
  final dart = version['dartSdkVersion'];
  final flutter = version['frameworkVersion'];
  if (dart != expectedDart || flutter != expectedFlutter) {
    stderr.writeln(
        'Expected Flutter $expectedFlutter / Dart $expectedDart, found Flutter $flutter / Dart $dart.');
    exitCode = 1;
    return;
  }
  stdout.writeln('Flutter toolchain verified: $flutter / Dart $dart');
}
