import 'dart:convert';
import 'dart:io';

void main() {
  final companion = jsonDecode(File('contract.lock.json').readAsStringSync())
      as Map<String, Object?>;
  final core = jsonDecode(File('../../contract.lock.json').readAsStringSync())
      as Map<String, Object?>;
  final conformance =
      Map<String, Object?>.from(core['conformance']! as Map<Object?, Object?>);
  final openapi =
      Map<String, Object?>.from(core['openapi']! as Map<Object?, Object?>);
  final protobuf =
      Map<String, Object?>.from(core['protobuf']! as Map<Object?, Object?>);
  final mismatches = <String>[
    if (companion['conformanceDigest'] != conformance['digest'])
      'conformance digest',
    if (companion['openapiSha256'] != openapi['sha256']) 'OpenAPI digest',
    if (companion['protobufSha256'] != protobuf['sha256']) 'protobuf digest',
  ];
  if (mismatches.isNotEmpty) {
    stderr.writeln('Flutter companion lock drifted: ${mismatches.join(', ')}');
    exitCode = 1;
    return;
  }
  stdout.writeln('Flutter companion contract lock verified');
}
