import 'dart:io';

Future<void> main() async {
  final temporary =
      await Directory.systemTemp.createTemp('ghanageo-flutter-artifact-');
  try {
    final coreArtifact = Directory('${temporary.path}/ghanageo')..createSync();
    final flutterArtifact = Directory('${temporary.path}/ghanageo_flutter')
      ..createSync();
    _stagePackage(Directory('../..').absolute, coreArtifact);
    _stagePackage(Directory.current, flutterArtifact);
    final consumer = Directory('${temporary.path}/consumer')..createSync();
    File('${consumer.path}/pubspec.yaml').writeAsStringSync('''
name: ghanageo_flutter_clean_consumer
publish_to: none
environment:
  sdk: ">=3.4.0 <4.0.0"
dependencies:
  flutter:
    sdk: flutter
  ghanageo_flutter:
    path: ${flutterArtifact.path}
dependency_overrides:
  ghanageo:
    path: ${coreArtifact.path}
''');
    Directory('${consumer.path}/lib').createSync();
    File('${consumer.path}/lib/main.dart').writeAsStringSync('''
import 'package:flutter/widgets.dart';
import 'package:ghanageo_flutter/ghanageo_flutter.dart';

Widget picker(GhanaGeoClient client) => GhanaGeoLocationPicker(client: client, onSelected: (_) {});
''');
    await _run(consumer.path, ['flutter', 'pub', 'get']);
    await _run(consumer.path, ['flutter', 'analyze']);
    stdout.writeln(
        'Verified isolated staged Flutter artifact: ${flutterArtifact.path}');
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
