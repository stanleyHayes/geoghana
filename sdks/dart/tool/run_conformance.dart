import 'dart:async';
import 'dart:convert';
import 'dart:io';

Future<void> main(List<String> arguments) async {
  final package = Directory.current.absolute;
  final root = Directory('${package.path}/../..').absolute;
  final output = File(arguments.isEmpty
          ? '${package.path}/dart-conformance-report.json'
          : arguments.first)
      .absolute;
  output.parent.createSync(recursive: true);
  final fixture = await Process.start(
    'node',
    [
      '--experimental-strip-types',
      '${root.path}/tools/conformance/fixture-server.ts'
    ],
    workingDirectory: root.path,
  );
  final errors = fixture.stderr
      .transform(const SystemEncoding().decoder)
      .listen(stderr.write);
  try {
    final portText = await fixture.stdout
        .transform(const SystemEncoding().decoder)
        .transform(const LineSplitter())
        .first
        .timeout(const Duration(seconds: 15));
    final port = int.tryParse(portText);
    if (port == null) {
      throw StateError('Fixture returned an invalid port: $portText');
    }
    await _run(package.path, [
      'dart',
      'run',
      'conformance/runner.dart',
      '--base-url',
      'http://127.0.0.1:$port/v1',
      '--output',
      output.path,
    ]);
    await _run(root.path,
        ['ruby', 'tools/conformance/validate.rb', '--report', output.path]);
    stdout.writeln('Strict normalized report: ${output.path}');
  } finally {
    fixture.kill(ProcessSignal.sigterm);
    await fixture.exitCode.timeout(const Duration(seconds: 5), onTimeout: () {
      fixture.kill(ProcessSignal.sigkill);
      return fixture.exitCode;
    });
    await errors.cancel();
  }
}

Future<void> _run(String directory, List<String> command) async {
  final result = await Process.run(command.first, command.sublist(1),
      workingDirectory: directory, runInShell: true);
  stdout.write(result.stdout);
  stderr.write(result.stderr);
  if (result.exitCode != 0) exit(result.exitCode);
}
