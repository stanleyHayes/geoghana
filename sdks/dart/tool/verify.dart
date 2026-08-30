import 'dart:io';

Future<void> main() async {
  final failures = <String>[];
  for (final command in <List<String>>[
    ['dart', 'run', 'tool/check_toolchain.dart'],
    ['dart', 'run', 'tool/check_contract.dart'],
    [
      'dart',
      'format',
      '--output=none',
      '--set-exit-if-changed',
      'lib',
      'test',
      'example',
      'conformance',
      'tool'
    ],
    ['dart', 'analyze'],
    ['dart', 'test'],
    ['dart', 'run', 'tool/clean_consumer.dart'],
  ]) {
    final result =
        await Process.run(command.first, command.sublist(1), runInShell: true);
    stdout.write(result.stdout);
    stderr.write(result.stderr);
    if (result.exitCode != 0) failures.add(command.join(' '));
  }
  final publishIgnore = File('.pubignore');
  try {
    publishIgnore.writeAsStringSync('''
.dart_tool/
build/
coverage/
packages/
pubspec.lock
dart-conformance-report.json
''');
    final command = ['dart', 'pub', 'publish', '--dry-run'];
    final result =
        await Process.run(command.first, command.sublist(1), runInShell: true);
    stdout.write(result.stdout);
    stderr.write(result.stderr);
    if (result.exitCode != 0) failures.add(command.join(' '));
  } finally {
    if (publishIgnore.existsSync()) publishIgnore.deleteSync();
  }
  final credential = RegExp(
      r'''(api[_-]?key|authorization)\s*[:=]\s*["'][A-Za-z0-9_-]{16,}''',
      caseSensitive: false);
  for (final entity
      in Directory.current.listSync(recursive: true, followLinks: false)) {
    if (entity is! File ||
        entity.path.contains('/.dart_tool/') ||
        entity.path.endsWith('.lock')) {
      continue;
    }
    try {
      if (credential.hasMatch(entity.readAsStringSync())) {
        failures.add('secret scan: ${entity.path}');
      }
    } on FileSystemException {
      // Ignore binary/package-cache files; release inputs are source text.
    }
  }
  if (failures.isNotEmpty) {
    stderr.writeln('Verification failed: ${failures.join(', ')}');
    exitCode = 1;
  }
}
