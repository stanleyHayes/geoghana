import 'dart:io';

Future<void> main() async {
  final temporary =
      await Directory.systemTemp.createTemp('ghanageo-dart-artifact-');
  try {
    final artifact = Directory('${temporary.path}/artifact')..createSync();
    _stagePackage(Directory.current, artifact);
    final consumer = Directory('${temporary.path}/consumer')..createSync();
    File('${consumer.path}/pubspec.yaml').writeAsStringSync('''
name: ghanageo_clean_consumer
publish_to: none
environment:
  sdk: ">=3.4.0 <4.0.0"
dependencies:
  ghanageo:
    path: ${artifact.path}
''');
    Directory('${consumer.path}/lib').createSync();
    File('${consumer.path}/lib/main.dart').writeAsStringSync('''
import 'package:ghanageo/ghanageo.dart';

Future<String> firstRegion() async {
  final client = GhanaGeoClient(telemetry: false);
  try { return (await client.regions(limit: 1)).data.first.name; }
  finally { client.close(); }
}
''');
    await _run(consumer.path, ['dart', 'pub', 'get']);
    await _run(consumer.path, ['dart', 'analyze']);
    stdout.writeln('Verified isolated staged Dart artifact: ${artifact.path}');
  } finally {
    temporary.deleteSync(recursive: true);
  }
}

void _stagePackage(Directory source, Directory target) {
  for (final name in [
    'pubspec.yaml',
    'README.md',
    'CHANGELOG.md',
    'LICENSE',
    'contract.lock.json'
  ]) {
    File('${source.path}/$name').copySync('${target.path}/$name');
  }
  _copyDirectory(
      Directory('${source.path}/lib'), Directory('${target.path}/lib'));
}

void _copyDirectory(Directory source, Directory target) {
  target.createSync(recursive: true);
  for (final entity in source.listSync()) {
    final name = entity.uri.pathSegments.where((part) => part.isNotEmpty).last;
    final destination = '${target.path}/$name';
    if (entity is File) entity.copySync(destination);
    if (entity is Directory) _copyDirectory(entity, Directory(destination));
  }
}

Future<void> _run(String directory, List<String> command) async {
  final result = await Process.run(command.first, command.sublist(1),
      workingDirectory: directory, runInShell: true);
  stdout.write(result.stdout);
  stderr.write(result.stderr);
  if (result.exitCode != 0) exit(result.exitCode);
}
