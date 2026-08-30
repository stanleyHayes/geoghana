final class GhanaGeoException implements Exception {
  const GhanaGeoException({
    required this.code,
    required this.message,
    this.status,
    this.requestId,
    this.docs,
    this.details = const <String, Object?>{},
  });

  final String code;
  final String message;
  final int? status;
  final String? requestId;
  final String? docs;
  final Map<String, Object?> details;

  @override
  String toString() => 'GhanaGeoException($code): $message';
}
