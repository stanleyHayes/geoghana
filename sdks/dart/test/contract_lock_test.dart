import 'dart:convert';
import 'dart:io';

import 'package:test/test.dart';

void main() {
  test('contract lock matches the canonical conformance export', () {
    final root = Directory('../..').absolute;
    if (!File('${root.path}/tools/conformance/export_cases.rb').existsSync()) {
      return;
    }
    final exported = Process.runSync(
      'ruby',
      ['tools/conformance/export_cases.rb'],
      workingDirectory: root.path,
    );
    expect(exported.exitCode, 0, reason: exported.stderr as String?);
    final contract =
        jsonDecode(exported.stdout as String) as Map<String, Object?>;
    final lock = jsonDecode(File('contract.lock.json').readAsStringSync())
        as Map<String, Object?>;
    final conformance = Map<String, Object?>.from(
        lock['conformance']! as Map<Object?, Object?>);
    expect(conformance['digest'], contract['contractDigest']);
  });
}
