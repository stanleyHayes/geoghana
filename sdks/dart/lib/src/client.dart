import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:http/http.dart' as http;

import 'cancel_token.dart';
import 'exception.dart';
import 'models.dart';

typedef TelemetrySink = void Function(Map<String, Object?> event);
typedef DatasetChangesTransport = Stream<DatasetChange> Function(
    String cursor, CancelToken? cancelToken);

final class GhanaGeoClient {
  GhanaGeoClient({
    Uri? baseUrl,
    this.apiKey,
    int retry = 2,
    this.telemetry = true,
    this.telemetrySink,
    this.userAgent = 'ghanageo-dart/0.1.0',
    this.maxResponseBytes = 8 * 1024 * 1024,
    this.maxDownloadBytes = 128 * 1024 * 1024,
    http.Client? httpClient,
    DatasetChangesTransport? datasetChangesTransport,
  })  : baseUrl = baseUrl ?? Uri.parse('https://api-geo.digitalghana.dev/v1'),
        retry = retry < 0 ? 0 : (retry > 5 ? 5 : retry),
        _http = httpClient ?? http.Client(),
        _datasetChangesTransport = datasetChangesTransport;

  static const apiVersion = 'v1';
  static const testedDatasetVersion = '2026.08.3-ulid';

  final Uri baseUrl;
  final String? apiKey;
  final int retry;
  final bool telemetry;
  final TelemetrySink? telemetrySink;
  final String userAgent;
  final int maxResponseBytes;
  final int maxDownloadBytes;
  final http.Client _http;
  final DatasetChangesTransport? _datasetChangesTransport;

  void close() => _http.close();

  Future<Page<Region>> regions(
          {String? cursor, int? limit, CancelToken? cancelToken}) =>
      _page('/regions', {'cursor': cursor, 'limit': limit}, Region.fromJson,
          cancelToken);

  Future<Region> region(String id, {CancelToken? cancelToken}) async =>
      Region.fromJson(await _json(
          '/regions/${Uri.encodeComponent(id)}', const {}, cancelToken));

  Future<Page<District>> regionDistricts(String id,
          {String? cursor, int? limit, CancelToken? cancelToken}) =>
      _page('/regions/${Uri.encodeComponent(id)}/districts',
          {'cursor': cursor, 'limit': limit}, District.fromJson, cancelToken);

  Future<Page<District>> districts(
          {String? regionId,
          String? query,
          String? cursor,
          int? limit,
          CancelToken? cancelToken}) =>
      _page(
          '/districts',
          {'regionId': regionId, 'q': query, 'cursor': cursor, 'limit': limit},
          District.fromJson,
          cancelToken);

  Future<District> district(String id, {CancelToken? cancelToken}) async =>
      District.fromJson(await _json(
          '/districts/${Uri.encodeComponent(id)}', const {}, cancelToken));

  Future<Page<Place>> districtPlaces(String id,
          {String? type,
          String? cursor,
          int? limit,
          CancelToken? cancelToken}) =>
      _page(
          '/districts/${Uri.encodeComponent(id)}/places',
          {'type': type, 'cursor': cursor, 'limit': limit},
          Place.fromJson,
          cancelToken);

  Future<Page<Place>> places(
          {String? districtId,
          String? regionId,
          String? type,
          String? query,
          String? cursor,
          int? limit,
          CancelToken? cancelToken}) =>
      _page(
          '/places',
          {
            'districtId': districtId,
            'regionId': regionId,
            'type': type,
            'q': query,
            'cursor': cursor,
            'limit': limit
          },
          Place.fromJson,
          cancelToken);

  Future<Place> place(String id, {CancelToken? cancelToken}) async =>
      Place.fromJson(await _json(
          '/places/${Uri.encodeComponent(id)}', const {}, cancelToken));

  Future<Page<SearchResult>> search(String query,
          {String? regionId,
          String? districtId,
          String? type,
          int? limit,
          CancelToken? cancelToken}) =>
      _page(
          '/search',
          {
            'q': query,
            'regionId': regionId,
            'districtId': districtId,
            'type': type,
            'limit': limit
          },
          SearchResult.fromJson,
          cancelToken);

  Future<Page<SearchResult>> autocomplete(String query,
          {int? limit, CancelToken? cancelToken}) =>
      _page('/autocomplete', {'q': query, 'limit': limit},
          SearchResult.fromJson, cancelToken);

  Future<Page<SearchResult>> geocode(String query,
          {int? limit, CancelToken? cancelToken}) =>
      _page('/geocode', {'q': query, 'limit': limit}, SearchResult.fromJson,
          cancelToken);

  Future<ReverseResult> reverseGeocode(
          {required double latitude,
          required double longitude,
          CancelToken? cancelToken}) =>
      _json('/reverse', {'lat': latitude, 'lng': longitude}, cancelToken)
          .then(ReverseResult.fromJson);

  Future<Page<Place>> nearby(
          {required double latitude,
          required double longitude,
          int radius = 5000,
          int? limit,
          CancelToken? cancelToken}) =>
      _page(
          '/nearby',
          {'lat': latitude, 'lng': longitude, 'radius': radius, 'limit': limit},
          Place.fromJson,
          cancelToken);

  Future<BoundaryFeature> boundary(String id, {CancelToken? cancelToken}) =>
      _json('/boundaries/${Uri.encodeComponent(id)}', const {}, cancelToken)
          .then(BoundaryFeature.fromJson);

  Future<DatasetPage> datasets({CancelToken? cancelToken}) =>
      _json('/datasets', const {}, cancelToken).then(DatasetPage.fromJson);

  Future<DownloadList> datasetDownloads(String version,
          {CancelToken? cancelToken}) =>
      _json('/datasets/${Uri.encodeComponent(version)}/downloads', const {},
              cancelToken)
          .then(DownloadList.fromJson);

  Future<DatasetArtifact> downloadDatasetArtifact(
      String version, String entity, String format,
      {CancelToken? cancelToken}) async {
    const entities = {'regions', 'districts', 'places'};
    const formats = {'json', 'csv', 'geojson'};
    if (!entities.contains(entity) || !formats.contains(format)) {
      throw const GhanaGeoException(
          code: 'INVALID_ARGUMENT', message: 'Unsupported dataset artifact');
    }
    final response = await _send(
        '/datasets/${Uri.encodeComponent(version)}/downloads/$entity.$format',
        const {},
        cancelToken,
        maxDownloadBytes);
    return DatasetArtifact(
        bytes: response.$1, contentType: response.$2.headers['content-type']);
  }

  Future<JsonMap> roads({CancelToken? cancelToken}) =>
      _json('/roads', const {}, cancelToken);
  Future<JsonMap> pointsOfInterest({CancelToken? cancelToken}) =>
      _json('/pois', const {}, cancelToken);

  /// Advanced GET escape hatch for newly added contract operations.
  ///
  /// It retains the same authentication, cancellation, retry, size and error
  /// semantics as every typed method and never accepts an absolute URL.
  Future<JsonMap> request(String path,
      {Map<String, Object?> query = const {}, CancelToken? cancelToken}) {
    if (!path.startsWith('/') || path.startsWith('//')) {
      throw const GhanaGeoException(
          code: 'INVALID_ARGUMENT',
          message: 'Request path must be API-relative');
    }
    return _json(path, query, cancelToken);
  }

  Stream<DatasetChange> streamDatasetChanges(
      {String cursor = '', CancelToken? cancelToken}) {
    final transport = _datasetChangesTransport;
    if (transport == null) {
      throw const GhanaGeoException(
        code: 'TRANSPORT_NOT_CONFIGURED',
        message:
            'Configure a native gRPC DatasetChangesTransport to stream changes',
      );
    }
    return transport(cursor, cancelToken);
  }

  Stream<Page<Region>> regionPages({int? limit, CancelToken? cancelToken}) =>
      _pages(
          (cursor) =>
              regions(cursor: cursor, limit: limit, cancelToken: cancelToken),
          cancelToken);
  Stream<Page<District>> districtPages(
          {String? regionId,
          String? query,
          int? limit,
          CancelToken? cancelToken}) =>
      _pages(
          (cursor) => districts(
              regionId: regionId,
              query: query,
              cursor: cursor,
              limit: limit,
              cancelToken: cancelToken),
          cancelToken);
  Stream<Page<Place>> placePages(
          {String? districtId,
          String? regionId,
          String? type,
          String? query,
          int? limit,
          CancelToken? cancelToken}) =>
      _pages(
          (cursor) => places(
              districtId: districtId,
              regionId: regionId,
              type: type,
              query: query,
              cursor: cursor,
              limit: limit,
              cancelToken: cancelToken),
          cancelToken);

  Stream<Page<T>> _pages<T>(Future<Page<T>> Function(String? cursor) load,
      CancelToken? cancelToken) async* {
    String? cursor;
    final seen = <String>{};
    do {
      cancelToken?.throwIfCancelled();
      final page = await load(cursor);
      yield page;
      final next = page.nextCursor;
      if (next != null && next.isNotEmpty && !seen.add(next)) {
        throw const GhanaGeoException(
            code: 'PAGINATION_LOOP',
            message: 'The API repeated a pagination cursor');
      }
      cursor = next;
    } while (cursor != null && cursor.isNotEmpty);
  }

  Future<Page<T>> _page<T>(String path, Map<String, Object?> query,
      T Function(JsonMap) decode, CancelToken? token) async {
    final json = await _json(path, query, token);
    final raw = (json['data'] as List<Object?>? ?? const <Object?>[]);
    return Page<T>(
      data: raw
          .map((item) =>
              decode(Map<String, Object?>.from(item! as Map<Object?, Object?>)))
          .toList(growable: false),
      datasetVersion:
          (json['datasetVersion'] ?? json['dataset_version'] ?? '') as String,
      nextCursor: (json['nextCursor'] ?? json['next_cursor']) as String?,
    );
  }

  Future<JsonMap> _json(
      String path, Map<String, Object?> query, CancelToken? token) async {
    final response = await _send(path, query, token, maxResponseBytes);
    final decoded = jsonDecode(utf8.decode(response.$1));
    if (decoded is! Map<Object?, Object?>) {
      throw const GhanaGeoException(
          code: 'INVALID_RESPONSE', message: 'Expected a JSON object');
    }
    return Map<String, Object?>.from(decoded);
  }

  Future<(List<int>, http.StreamedResponse)> _send(String path,
      Map<String, Object?> query, CancelToken? token, int byteLimit) async {
    token?.throwIfCancelled();
    final filtered = <String, String>{
      for (final entry in query.entries)
        if (entry.value != null) entry.key: '${entry.value}'
    };
    final basePath = baseUrl.path.endsWith('/')
        ? baseUrl.path.substring(0, baseUrl.path.length - 1)
        : baseUrl.path;
    final uri = baseUrl.replace(
        path: '$basePath$path',
        queryParameters: filtered.isEmpty ? null : filtered);
    Object? lastFailure;
    for (var attempt = 0; attempt <= retry; attempt++) {
      token?.throwIfCancelled();
      try {
        final request = http.AbortableRequest('GET', uri,
            abortTrigger: token?.whenCancelled)
          ..headers['accept'] =
              'application/json, application/geo+json, application/octet-stream'
          ..headers['user-agent'] = userAgent;
        if (apiKey != null && apiKey!.isNotEmpty) {
          request.headers['authorization'] = 'Bearer $apiKey';
        }
        final started = DateTime.now();
        final response = await _http.send(request);
        final bytes = await _boundedBytes(response, byteLimit, token);
        _emit(path, response.statusCode, DateTime.now().difference(started));
        if (response.statusCode >= 200 && response.statusCode < 300) {
          return (bytes, response);
        }
        final error = _error(response, bytes);
        if (!_retryableStatus(response.statusCode) || attempt == retry) {
          throw error;
        }
        await _delay(attempt, response.headers['retry-after'], token);
      } on http.RequestAbortedException {
        throw const GhanaGeoCancelledException();
      } on GhanaGeoException {
        rethrow;
      } catch (error) {
        lastFailure = error;
        if (attempt == retry) break;
        await _delay(attempt, null, token);
      }
    }
    throw GhanaGeoException(
        code: 'TRANSPORT_ERROR',
        message: 'Request failed',
        details: {'cause': '$lastFailure'});
  }

  Future<List<int>> _boundedBytes(
      http.StreamedResponse response, int limit, CancelToken? token) async {
    final declared = response.contentLength;
    if (declared != null && declared > limit) {
      throw GhanaGeoException(
          code: 'RESPONSE_TOO_LARGE',
          message: 'Response exceeds $limit bytes',
          status: response.statusCode);
    }
    final bytes = <int>[];
    await for (final chunk in response.stream) {
      token?.throwIfCancelled();
      if (bytes.length + chunk.length > limit) {
        throw GhanaGeoException(
            code: 'RESPONSE_TOO_LARGE',
            message: 'Response exceeds $limit bytes',
            status: response.statusCode);
      }
      bytes.addAll(chunk);
    }
    return bytes;
  }

  GhanaGeoException _error(http.StreamedResponse response, List<int> bytes) {
    JsonMap body = const {};
    try {
      final decoded = jsonDecode(utf8.decode(bytes));
      if (decoded is Map<Object?, Object?>) {
        body = Map<String, Object?>.from(decoded);
      }
    } on FormatException {
      // Preserve the status even if a proxy returned non-JSON.
    }
    final raw = body['error'] is Map<Object?, Object?>
        ? Map<String, Object?>.from(body['error']! as Map<Object?, Object?>)
        : body;
    return GhanaGeoException(
      code: (raw['code'] ?? 'HTTP_${response.statusCode}') as String,
      message: (raw['message'] ?? 'GhanaGeo request failed') as String,
      status: response.statusCode,
      requestId: (raw['requestId'] ??
          raw['request_id'] ??
          response.headers['x-request-id']) as String?,
      docs: raw['docs'] as String?,
      details: raw['details'] is Map<Object?, Object?>
          ? Map<String, Object?>.from(raw['details']! as Map<Object?, Object?>)
          : const {},
    );
  }

  bool _retryableStatus(int status) =>
      status == 429 || status == 502 || status == 503 || status == 504;

  Future<void> _delay(
      int attempt, String? retryAfter, CancelToken? token) async {
    final random = Random();
    final retryAfterDuration = _retryAfter(retryAfter);
    final duration = retryAfterDuration ??
        Duration(
            milliseconds:
                min(8000, 250 * (1 << attempt)) + random.nextInt(101));
    if (token == null) return Future<void>.delayed(duration);
    await Future.any<void>([
      Future<void>.delayed(duration),
      token.whenCancelled
          .then<void>((_) => throw const GhanaGeoCancelledException()),
    ]);
  }

  Duration? _retryAfter(String? value) {
    if (value == null || value.isEmpty) return null;
    final seconds = int.tryParse(value);
    if (seconds != null) return Duration(seconds: min(max(seconds, 0), 30));
    final match = RegExp(
            r'^[A-Za-z]{3}, (\d{2}) ([A-Za-z]{3}) (\d{4}) (\d{2}):(\d{2}):(\d{2}) GMT$')
        .firstMatch(value);
    if (match == null) return null;
    const months = {
      'Jan': 1,
      'Feb': 2,
      'Mar': 3,
      'Apr': 4,
      'May': 5,
      'Jun': 6,
      'Jul': 7,
      'Aug': 8,
      'Sep': 9,
      'Oct': 10,
      'Nov': 11,
      'Dec': 12
    };
    final month = months[match.group(2)];
    if (month == null) return null;
    final target = DateTime.utc(
        int.parse(match.group(3)!),
        month,
        int.parse(match.group(1)!),
        int.parse(match.group(4)!),
        int.parse(match.group(5)!),
        int.parse(match.group(6)!));
    final milliseconds =
        target.difference(DateTime.now().toUtc()).inMilliseconds;
    return Duration(milliseconds: min(max(milliseconds, 0), 30000));
  }

  void _emit(String operation, int status, Duration elapsed) {
    if (!telemetry || telemetrySink == null) return;
    telemetrySink!({
      'operation': operation,
      'status': status,
      'durationMs': elapsed.inMilliseconds,
      'sdk': 'dart'
    });
  }
}
