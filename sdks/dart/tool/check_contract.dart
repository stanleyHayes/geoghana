import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';

void main() {
  final root = Directory('../..').absolute;
  final lock = jsonDecode(File('contract.lock.json').readAsStringSync())
      as Map<String, Object?>;
  final canonicalOpenApi = File('${root.path}/contracts/openapi/v1.yaml');
  if (!canonicalOpenApi.existsSync()) {
    final digest = Map<String, Object?>.from(
        lock['conformance']! as Map<Object?, Object?>)['digest'];
    if (digest is! String || !RegExp(r'^[a-f0-9]{64}$').hasMatch(digest)) {
      stderr.writeln('contract lock is invalid');
      exitCode = 1;
    } else {
      stdout.writeln(
          'standalone package: verified pinned contract-lock structure');
    }
    return;
  }
  for (final name in ['openapi', 'protobuf']) {
    final item =
        Map<String, Object?>.from(lock[name]! as Map<Object?, Object?>);
    final actual = sha256
        .convert(File('${root.path}/${item['path']}').readAsBytesSync())
        .toString();
    if (actual != item['sha256']) {
      stderr.writeln(
          '$name contract drifted; regenerate and review the Dart facade');
      exitCode = 1;
    }
  }
  final exported = Process.runSync(
      'ruby', ['tools/conformance/export_cases.rb'],
      workingDirectory: root.path);
  if (exported.exitCode != 0) {
    stderr.write(exported.stderr);
    exitCode = exported.exitCode;
    return;
  }
  final contract =
      jsonDecode(exported.stdout as String) as Map<String, Object?>;
  final expected = Map<String, Object?>.from(
      lock['conformance']! as Map<Object?, Object?>)['digest'];
  if (contract['contractDigest'] != expected) {
    stderr.writeln(
        'conformance contract drifted; update the runner and lock together');
    exitCode = 1;
  }
}
