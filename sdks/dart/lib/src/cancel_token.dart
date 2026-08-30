import 'dart:async';

/// A cooperative token that also aborts an in-flight HTTP request.
final class CancelToken {
  final Completer<void> _cancelled = Completer<void>();

  bool get isCancelled => _cancelled.isCompleted;
  Future<void> get whenCancelled => _cancelled.future;

  void cancel() {
    if (!_cancelled.isCompleted) _cancelled.complete();
  }

  void throwIfCancelled() {
    if (isCancelled) throw const GhanaGeoCancelledException();
  }
}

final class GhanaGeoCancelledException implements Exception {
  const GhanaGeoCancelledException();

  @override
  String toString() => 'GhanaGeo request cancelled';
}
