import 'dart:io';

Future<void> main() async {
  final failures = <String>[];
  final override = File('pubspec_overrides.yaml');
  try {
    override.writeAsStringSync('''
dependency_overrides:
  ghanageo:
    path: ../..
    ''');
    for (final command in <List<String>>[
      ['flutter', 'pub', 'get'],
      ['dart', 'run', 'tool/check_contract.dart'],
      ['dart', 'run', 'tool/check_toolchain.dart'],
      [
        'dart',
        'format',
        '--output=none',
        '--set-exit-if-changed',
        'lib',
        'test',
        'tool'
      ],
      ['flutter', 'analyze'],
      ['flutter', 'test'],
      ['dart', 'run', 'tool/clean_consumer.dart'],
    ]) {
      await _check(command, failures);
    }
    await _checkPrepublicationDryRun(failures);
  } finally {
    if (override.existsSync()) override.deleteSync();
  }
  if (failures.isNotEmpty) {
    stderr.writeln('Verification failed: ${failures.join(', ')}');
    exitCode = 1;
  }
}

Future<void> _checkPrepublicationDryRun(List<String> failures) async {
  const command = ['dart', 'pub', 'publish', '--dry-run'];
  final result =
      await Process.run(command.first, command.sublist(1), runInShell: true);
  final output = '${result.stdout}\n${result.stderr}';
  stdout.write(result.stdout);
  stderr.write(result.stderr);
  final expectedHint = output.contains(
    'Non-dev dependencies are overridden in pubspec_overrides.yaml.',
  );
  final exactSummary = output.contains('Package has 0 warnings and 1 hint.');
  final hasUnexpectedFinding = RegExp(
    r'Package validation found the following (?:error|warning)',
  ).hasMatch(output);
  if (result.exitCode != 0 ||
      !expectedHint ||
      !exactSummary ||
      hasUnexpectedFinding) {
    failures.add(
        '${command.join(' ')} (expected only the local-core override hint)');
  }
}

Future<void> _check(List<String> command, List<String> failures) async {
  final result =
      await Process.run(command.first, command.sublist(1), runInShell: true);
  stdout.write(result.stdout);
  stderr.write(result.stderr);
  if (result.exitCode != 0) failures.add(command.join(' '));
}
