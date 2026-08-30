import 'dart:io';

void main() {
  const expected = '3.13.2';
  final actual = Platform.version.split(' ').first;
  if (actual != expected) {
    stderr.writeln(
        'Expected Dart $expected, found $actual. Use the stable SDK pinned in contract.lock.json.');
    exitCode = 1;
    return;
  }
  stdout.writeln('Dart toolchain verified: $actual');
}
