import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:ghanageo/ghanageo.dart';
import 'package:http/http.dart' as http;

final class CaptureClient extends http.BaseClient {
  CaptureClient(this.caseId) : _inner = http.Client();
  final String caseId;
  final http.Client _inner;
  final requests = <Map<String, Object?>>[];
  final responses = <Map<String, Object?>>[];

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    request.headers['x-conformance-case'] = caseId;
    requests.add({
      'method': request.method,
      'path': request.url.path,
      'query': request.url.query,
      'authorizationSent': request.headers.containsKey('authorization')
    });
    final response = await _inner.send(request);
    final body = await response.stream.toBytes();
    responses.add({
      'status': response.statusCode,
      'requestId': response.headers['x-request-id'],
      'quota': response.headers['x-conformance-quota'],
      'bodySha256': sha256.convert(body).toString(),
    });
    return http.StreamedResponse(
      Stream<List<int>>.value(body),
      response.statusCode,
      contentLength: body.length,
      request: response.request,
      headers: response.headers,
      isRedirect: response.isRedirect,
      persistentConnection: response.persistentConnection,
      reasonPhrase: response.reasonPhrase,
    );
  }

  @override
  void close() => _inner.close();
}

Future<void> main(List<String> arguments) async {
  final rootArgument = arguments.isNotEmpty && !arguments.first.startsWith('--')
      ? arguments.first
      : '../..';
  final root = Directory(rootArgument).absolute.path;
  final baseUrl = _argument(arguments, '--base-url') ??
      Platform.environment['GHANAGEO_CONFORMANCE_URL'];
  final output =
      _argument(arguments, '--output') ?? 'dart-conformance-report.json';
  if (baseUrl == null || baseUrl.isEmpty) {
    stderr.writeln(
        '--base-url or GHANAGEO_CONFORMANCE_URL is required; mocked conformance is forbidden.');
    exitCode = 2;
    return;
  }
  final export = Process.runSync('ruby', ['tools/conformance/export_cases.rb'],
      workingDirectory: root);
  if (export.exitCode != 0) {
    stderr.write(export.stderr);
    exitCode = export.exitCode;
    return;
  }
  final contract = Map<String, Object?>.from(
      jsonDecode(export.stdout as String) as Map<Object?, Object?>);
  final cases = (contract['cases']! as List<Object?>)
      .map((item) => Map<String, Object?>.from(item! as Map<Object?, Object?>))
      .where((item) => (item['protocols']! as List<Object?>).contains('rest'));
  final results = <Map<String, Object?>>[];
  final evidence = <Map<String, Object?>>[];
  final versions = <String>{};
  for (final item in cases) {
    final caseId = item['id']! as String;
    final input = _map(item['input']);
    final auth = input['auth'] as String?;
    final capture = CaptureClient(caseId);
    final client = GhanaGeoClient(
        baseUrl: Uri.parse(baseUrl),
        apiKey: auth == null || auth == 'omitted' ? null : auth,
        retry: 0,
        httpClient: capture);
    Object? value;
    Object? runnerFailure;
    final semanticFailures = <String>[];
    var cancelled = false;
    try {
      if (caseId == 'semantic.cursor-pagination') {
        final pages = await client.placePages(limit: 2).take(2).toList();
        if (pages.length != 2) {
          semanticFailures.add('pagination did not produce two pages');
          value = pages.isEmpty ? <String, Object?>{} : _normalize(pages.first);
        } else {
          final firstIds = pages.first.data.map((place) => place.id).toSet();
          final secondIds = pages.last.data.map((place) => place.id).toSet();
          if (firstIds.intersection(secondIds).isNotEmpty) {
            semanticFailures.add('pagination returned duplicate identities');
          }
          final cursor = pages.first.nextCursor;
          if (cursor == null ||
              cursor.length < 8 ||
              int.tryParse(cursor) != null) {
            semanticFailures.add('cursor is not opaque');
          }
          final replay = await client.places(limit: 2);
          if (replay.nextCursor != cursor) {
            semanticFailures.add('cursor is not stable on replay');
          }
          value = _normalize(pages.first);
        }
      } else if (caseId == 'semantic.cancellation-propagates') {
        final token = CancelToken();
        final pending = _invoke(client, item, token);
        Timer(
            Duration(
                milliseconds:
                    (input['cancelAfterMilliseconds'] as num?)?.toInt() ?? 10),
            token.cancel);
        try {
          await pending;
        } on GhanaGeoCancelledException {
          cancelled = true;
        }
        value = <String, Object?>{};
      } else {
        value = _normalize(await _invoke(client, item, null));
      }
    } on GhanaGeoException catch (error) {
      value = {
        'error': {
          'code': error.code,
          'message': error.message,
          'requestId': error.requestId,
          'docs': error.docs,
          'details': error.details
        }
      };
    } catch (error) {
      runnerFailure = error;
      value = {'runnerFailure': '$error'};
    } finally {
      client.close();
    }
    _collectVersions(value, versions);
    final expected = _map(item['expect']);
    final failures = <String>[...semanticFailures];
    final outcome = value is Map<String, Object?> && value.containsKey('error')
        ? 'error'
        : 'success';
    if (outcome != expected['outcome']) {
      failures.add('expected ${expected['outcome']}, received $outcome');
    }
    final status =
        capture.responses.isEmpty ? null : capture.responses.last['status'];
    if (expected['httpStatus'] != null && status != expected['httpStatus']) {
      failures.add('expected HTTP ${expected['httpStatus']}, received $status');
    }
    final quotaHeader =
        capture.responses.isEmpty ? null : capture.responses.last['quota'];
    if (quotaHeader is String && quotaHeader.isNotEmpty) {
      final actualQuota = jsonDecode(quotaHeader);
      if (jsonEncode(actualQuota) != jsonEncode(expected['quotaCost'])) {
        failures.add('quota cost differs');
      }
    } else if (caseId != 'semantic.cancellation-propagates') {
      failures.add('fixture quota evidence is absent');
    }
    if (runnerFailure != null) failures.add('runner failure: $runnerFailure');
    for (final required
        in (_map(expected['shape'])['required'] as List<Object?>? ??
            const [])) {
      if (_nested(value, required! as String) == null) {
        failures.add('missing result field $required');
      }
    }
    final expectedError = _map(expected['error']);
    if (expectedError.isNotEmpty) {
      if (_nested(value, 'error.code') != expectedError['code']) {
        failures.add('canonical error code differs');
      }
      for (final required
          in (expectedError['requiredFields'] as List<Object?>? ?? const [])) {
        if (_nested(value, 'error.$required') == null) {
          failures.add('missing error field $required');
        }
      }
    }
    final semantics = expected['semantics'] as List<Object?>? ?? const [];
    if (semantics.contains('datasetVersionPresent') &&
        _nested(value, 'datasetVersion') is! String) {
      failures.add('datasetVersion is absent');
    }
    if (semantics.contains('preservesGhanaianOrthography') &&
        !jsonEncode(value).contains('Mampɔŋ')) {
      failures.add('orthography was not preserved');
    }
    if (semantics.contains('anonymousByDefault') &&
        capture.requests
            .any((request) => request['authorizationSent'] == true)) {
      failures.add('anonymous request sent a key');
    }
    if (semantics.contains('cancellationPropagates') && !cancelled) {
      failures.add('cancellation did not propagate');
    }
    if (semantics.contains('emptyOutsideGhana') &&
        (_nested(value, 'region') != null ||
            _nested(value, 'district') != null ||
            jsonEncode(_nested(value, 'nearby')) != '[]')) {
      failures.add('outside-Ghana result was not empty');
    }
    results.add({
      'caseId': caseId,
      'status': failures.isEmpty ? 'passed' : 'failed',
      'protocols': ['rest'],
      if (failures.isNotEmpty) 'message': failures.join('; ')
    });
    evidence.add({
      'caseId': caseId,
      'protocol': 'rest',
      'requestDigest': _digest(capture.requests),
      // Each captured response contains the canonical body SHA-256 alongside
      // its status and selected headers, so this digest is wire-body-bound.
      'responseDigest': _digest(capture.responses),
    });
  }
  final failed = results.where((item) => item['status'] == 'failed').length;
  final report = <String, Object?>{
    'schemaVersion': 2,
    'contract': {
      'digest': contract['contractDigest'],
      'runnerVersion': '1.0.0'
    },
    'sdk': {
      'language': 'dart',
      'name': 'ghanageo',
      'version': '0.1.0',
      'supportedProtocols': ['rest']
    },
    'apiVersion': GhanaGeoClient.apiVersion,
    'datasetVersion': versions.length == 1 ? versions.single : 'unobserved',
    'summary': {
      'passed': results.length - failed,
      'failed': failed,
      'skipped': 0
    },
    'evidenceDigest': _digest(evidence),
    'evidence': evidence,
    'results': results,
  };
  File(output).writeAsStringSync(
      '${const JsonEncoder.withIndent('  ').convert(report)}\n');
  stdout.writeln(
      'Dart conformance: ${results.length - failed} passed, $failed failed');
  if (failed != 0 ||
      versions.length != 1 ||
      _singleOrNull(versions) != GhanaGeoClient.testedDatasetVersion) {
    exitCode = 1;
  }
}

String? _argument(List<String> args, String name) {
  final index = args.indexOf(name);
  return index >= 0 && index + 1 < args.length ? args[index + 1] : null;
}

Map<String, Object?> _map(Object? value) => value is Map<Object?, Object?>
    ? Map<String, Object?>.from(value)
    : <String, Object?>{};

Future<Object?> _invoke(
    GhanaGeoClient client, Map<String, Object?> item, CancelToken? token) {
  final operation = item['operation']! as String;
  final input = _map(item['input']);
  final path = _map(input['path']);
  final query = _map(input['query']);
  if (item['id'] == 'error.invalid-argument') {
    return client.request('/places',
        query: {'limit': query['limit']}, cancelToken: token);
  }
  final limitValue = query['limit'];
  final limit = limitValue is num ? limitValue.toInt() : null;
  return switch (operation) {
    'listRegions' => client.regions(
        cursor: query['cursor'] as String?, limit: limit, cancelToken: token),
    'getRegion' => client.region(path['id'] as String, cancelToken: token),
    'listRegionDistricts' => client.regionDistricts(path['id'] as String,
        cursor: query['cursor'] as String?, limit: limit, cancelToken: token),
    'listDistricts' => client.districts(
        regionId: query['regionId'] as String?,
        query: query['q'] as String?,
        cursor: query['cursor'] as String?,
        limit: limit,
        cancelToken: token),
    'getDistrict' => client.district(path['id'] as String, cancelToken: token),
    'listDistrictPlaces' => client.districtPlaces(path['id'] as String,
        type: query['type'] as String?,
        cursor: query['cursor'] as String?,
        limit: limit,
        cancelToken: token),
    'listPlaces' => client.places(
        regionId: query['regionId'] as String?,
        districtId: query['districtId'] as String?,
        type: query['type'] as String?,
        query: query['q'] as String?,
        cursor: query['cursor'] as String?,
        limit: limit,
        cancelToken: token),
    'getPlace' => client.place(path['id'] as String, cancelToken: token),
    'search' => client.search((query['q'] ?? '') as String,
        regionId: query['regionId'] as String?,
        districtId: query['districtId'] as String?,
        type: query['type'] as String?,
        limit: limit,
        cancelToken: token),
    'autocomplete' => client.autocomplete((query['q'] ?? '') as String,
        limit: limit, cancelToken: token),
    'geocode' => client.geocode((query['q'] ?? '') as String,
        limit: limit, cancelToken: token),
    'reverseGeocode' => client.reverseGeocode(
        latitude: (query['lat'] as num).toDouble(),
        longitude: (query['lng'] as num).toDouble(),
        cancelToken: token),
    'nearby' => client.nearby(
        latitude: ((query['lat'] ?? 0) as num).toDouble(),
        longitude: ((query['lng'] ?? 0) as num).toDouble(),
        radius: ((query['radius'] ?? 5000) as num).toInt(),
        limit: limit,
        cancelToken: token),
    'getBoundary' => client.boundary(path['id'] as String, cancelToken: token),
    'listDatasets' => client.datasets(cancelToken: token),
    'listDatasetDownloads' =>
      client.datasetDownloads(path['version'] as String, cancelToken: token),
    'downloadDatasetArtifact' => client.downloadDatasetArtifact(
        path['version'] as String,
        path['entity'] as String,
        path['format'] as String,
        cancelToken: token),
    'listRoads' => client.roads(cancelToken: token),
    'listPointsOfInterest' => client.pointsOfInterest(cancelToken: token),
    _ => Future<Object?>.error(
        UnsupportedError('No Dart REST mapping for $operation')),
  };
}

Object? _normalize(Object? value) => switch (value) {
      Page<Object?> page => {
          'data': page.data.map(_normalize).toList(),
          'datasetVersion': page.datasetVersion,
          if (page.nextCursor != null) 'nextCursor': page.nextCursor
        },
      Region region => region.raw,
      District district => district.raw,
      SearchResult result => result.raw,
      Place place => place.raw,
      ReverseResult result => {
          if (result.region != null)
            'region': {'id': result.region!.id, 'name': result.region!.name},
          if (result.district != null)
            'district': {
              'id': result.district!.id,
              'name': result.district!.name
            },
          'nearby': result.nearby.map((place) => place.raw).toList(),
          'datasetVersion': result.datasetVersion,
        },
      BoundaryFeature feature => {
          'type': feature.type,
          'geometry': {
            'type': feature.geometry.type,
            'coordinates': feature.geometry.coordinates
          },
          'properties': {
            if (feature.properties.id != null) 'id': feature.properties.id,
            if (feature.properties.name != null)
              'name': feature.properties.name,
            if (feature.properties.datasetVersion != null)
              'datasetVersion': feature.properties.datasetVersion,
            if (feature.properties.attribution != null)
              'attribution': feature.properties.attribution,
          },
        },
      DatasetPage page => {
          'data': page.data
              .map((item) =>
                  {'version': item.version, 'status': item.status.name})
              .toList()
        },
      DownloadList list => {
          'version': list.version,
          'downloads': list.downloads
              .map((item) => {
                    'format': item.format.name,
                    'url': '${item.url}',
                    'checksum': item.checksum
                  })
              .toList()
        },
      DatasetArtifact artifact => {'bytes': artifact.bytes.length},
      _ => value,
    };

Object? _nested(Object? value, String path) {
  Object? current = value;
  for (final part in path.split('.')) {
    if (current is! Map<Object?, Object?> || !current.containsKey(part)) {
      return null;
    }
    current = current[part];
  }
  return current;
}

void _collectVersions(Object? value, Set<String> versions) {
  if (value is Map<Object?, Object?>) {
    for (final entry in value.entries) {
      if (entry.key == 'datasetVersion' &&
          entry.value is String &&
          (entry.value! as String).isNotEmpty) {
        versions.add(entry.value! as String);
      }
      _collectVersions(entry.value, versions);
    }
  } else if (value is Iterable<Object?>) {
    for (final child in value) {
      _collectVersions(child, versions);
    }
  }
}

String _digest(Object? value) =>
    sha256.convert(utf8.encode(jsonEncode(value))).toString();
String? _singleOrNull(Set<String> values) =>
    values.length == 1 ? values.single : null;
