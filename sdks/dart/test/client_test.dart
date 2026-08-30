import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:ghanageo/ghanageo.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:test/test.dart';

void main() {
  test('is anonymous by default and decodes a page', () async {
    final client = GhanaGeoClient(
      baseUrl: Uri.parse('https://example.test/v1'),
      httpClient: MockClient((request) async {
        expect(request.headers.containsKey('authorization'), isFalse);
        return http.Response(
            jsonEncode({
              'data': [
                {
                  'id': 'gh-region-ashanti',
                  'name': 'Ashanti',
                  'countryCode': 'GH',
                  'status': 'ACTIVE',
                  'verificationStatus': 'REFERENCE',
                  'provenance': {'sourceId': 'fixture'},
                  'datasetVersion': 'test',
                }
              ],
              'datasetVersion': 'test'
            }),
            200);
      }),
    );
    final page = await client.regions(limit: 1);
    expect(page.data.single.name, 'Ashanti');
    expect(page.data.single.status, Status.active);
    expect(page.data.single.provenance.sourceId, 'fixture');
    expect(page.datasetVersion, 'test');
  });

  test('maps catalog errors without losing unknown codes', () async {
    final client = GhanaGeoClient(
      baseUrl: Uri.parse('https://example.test/v1'),
      retry: 0,
      httpClient: MockClient((_) async => http.Response(
          jsonEncode({
            'error': {
              'code': 'NEW_CODE',
              'message': 'Nope',
              'requestId': 'req-1',
              'details': {'x': 1}
            }
          }),
          418)),
    );
    await expectLater(
        client.regions(),
        throwsA(isA<GhanaGeoException>()
            .having((e) => e.code, 'code', 'NEW_CODE')
            .having((e) => e.requestId, 'requestId', 'req-1')));
  });

  test('sends an explicitly supplied key as a Bearer credential', () async {
    final client = GhanaGeoClient(
      baseUrl: Uri.parse('https://example.test/v1'),
      apiKey: 'fixture-key',
      httpClient: MockClient((request) async {
        expect(request.headers['authorization'], 'Bearer fixture-key');
        return http.Response(
            jsonEncode({'data': <Object?>[], 'datasetVersion': 'test'}), 200);
      }),
    );
    await client.regions();
  });

  test('stops repeated cursors', () async {
    var calls = 0;
    final client = GhanaGeoClient(
      baseUrl: Uri.parse('https://example.test/v1'),
      httpClient: MockClient((_) async {
        calls++;
        return http.Response(
            jsonEncode({
              'data': <Object?>[],
              'datasetVersion': 'test',
              'nextCursor': 'same'
            }),
            200);
      }),
    );
    await expectLater(
        client.regionPages().toList(),
        throwsA(isA<GhanaGeoException>()
            .having((e) => e.code, 'code', 'PAGINATION_LOOP')));
    expect(calls, 2);
  });

  test('cancel token aborts retry delay', () async {
    final token = CancelToken();
    final client = GhanaGeoClient(
      baseUrl: Uri.parse('https://example.test/v1'),
      httpClient: MockClient((_) async =>
          http.Response('{}', 503, headers: {'retry-after': '30'})),
    );
    final future = client.regions(cancelToken: token);
    scheduleMicrotask(token.cancel);
    await expectLater(future, throwsA(isA<GhanaGeoCancelledException>()));
  });

  test('cancel token aborts an in-flight IO request', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final accepted = Completer<void>();
    server.listen((request) async {
      accepted.complete();
      await Future<void>.delayed(const Duration(seconds: 5));
      request.response.write('{}');
      await request.response.close();
    });
    final client = GhanaGeoClient(
        baseUrl: Uri.parse('http://${server.address.host}:${server.port}/v1'));
    final token = CancelToken();
    final pending = client.regions(cancelToken: token);
    await accepted.future;
    token.cancel();
    await expectLater(pending, throwsA(isA<GhanaGeoCancelledException>()));
    client.close();
    await server.close(force: true);
  });

  test('rejects an oversized response body', () async {
    final client = GhanaGeoClient(
      baseUrl: Uri.parse('https://example.test/v1'),
      maxResponseBytes: 4,
      httpClient: MockClient((_) async => http.Response('12345', 200)),
    );
    await expectLater(
        client.regions(),
        throwsA(isA<GhanaGeoException>()
            .having((e) => e.code, 'code', 'RESPONSE_TOO_LARGE')));
  });

  test('ships exactly sixteen offline regions with separate versions', () {
    expect(offlineRegions, hasLength(16));
    expect(offlineRegionsDatasetVersion, isNot(offlineRegionsPackageVersion));
  });

  test('retries transient reads but never a normal 4xx', () async {
    var calls = 0;
    final client = GhanaGeoClient(
      baseUrl: Uri.parse('https://example.test/v1'),
      retry: 2,
      httpClient: MockClient((_) async {
        calls++;
        return calls == 1
            ? http.Response('{}', 503)
            : http.Response(
                jsonEncode({'data': <Object?>[], 'datasetVersion': 'test'}),
                200);
      }),
    );
    await client.regions();
    expect(calls, 2);
    calls = 0;
    final bad = GhanaGeoClient(
      baseUrl: Uri.parse('https://example.test/v1'),
      httpClient: MockClient((_) async {
        calls++;
        return http.Response('{}', 400);
      }),
    );
    await expectLater(bad.regions(), throwsA(isA<GhanaGeoException>()));
    expect(calls, 1);
  });

  test('telemetry can be disabled and contains metadata only when enabled',
      () async {
    final events = <Map<String, Object?>>[];
    final transport = MockClient((_) async => http.Response(
        jsonEncode({'data': <Object?>[], 'datasetVersion': 'test'}), 200));
    final disabled = GhanaGeoClient(
        baseUrl: Uri.parse('https://example.test/v1'),
        telemetry: false,
        telemetrySink: events.add,
        httpClient: transport);
    await disabled.search('private address');
    expect(events, isEmpty);
    final enabled = GhanaGeoClient(
        baseUrl: Uri.parse('https://example.test/v1'),
        telemetrySink: events.add,
        httpClient: transport);
    await enabled.search('private address');
    expect(events.single.keys,
        unorderedEquals(['operation', 'status', 'durationMs', 'sdk']));
    expect(jsonEncode(events), isNot(contains('private address')));
  });

  test('dataset change stream forwards cursor and cancellation token',
      () async {
    final token = CancelToken();
    String? receivedCursor;
    CancelToken? receivedToken;
    final client = GhanaGeoClient(
      datasetChangesTransport: (cursor, cancelToken) {
        receivedCursor = cursor;
        receivedToken = cancelToken;
        return Stream.value(const DatasetChange(change: {'entity': 'region'}));
      },
    );
    final change = await client
        .streamDatasetChanges(cursor: 'opaque', cancelToken: token)
        .single;
    expect(change.change['entity'], 'region');
    expect(receivedCursor, 'opaque');
    expect(receivedToken, same(token));
  });

  test('decodes the complete OpenAPI place shape without losing orthography',
      () {
    final place = Place.fromJson(_placeJson());
    expect(place.normalizedName, 'mampong');
    expect(place.type, PlaceType.town);
    expect(place.aliases.single.value, 'Mampɔŋ');
    expect(place.aliases.single.isPreferred, isTrue);
    expect(place.region?.name, 'Ashanti');
    expect(place.centroid?.longitude, -1.4);
    expect(place.population, 42000);
    expect(place.verificationStatus, VerificationStatus.canonical);
    expect(
        place.provenance.sourceUrl, Uri.parse('https://example.test/source'));
  });

  test('decodes typed boundary and dataset download models', () {
    final boundary = BoundaryFeature.fromJson({
      'type': 'Feature',
      'geometry': {'type': 'Polygon', 'coordinates': <Object?>[]},
      'properties': {'id': 'gh-region-ashanti', 'datasetVersion': 'test'}
    });
    expect(boundary.type, 'Feature');
    expect(boundary.geometry.type, 'Polygon');
    final downloads = DownloadList.fromJson({
      'version': 'test',
      'downloads': [
        {
          'format': 'geojson',
          'url': 'https://example.test/data.geojson',
          'checksum': 'sha256:abc',
          'sizeBytes': 42
        }
      ]
    });
    expect(downloads.downloads.single.format, DownloadFormat.geojson);
    expect(downloads.downloads.single.sizeBytes, 42);
  });
}

Map<String, Object?> _placeJson() => {
      'id': 'gh-place-mampong',
      'name': 'Mampɔŋ',
      'normalizedName': 'mampong',
      'type': 'TOWN',
      'region': {'id': 'gh-region-ashanti', 'name': 'Ashanti'},
      'district': {'id': 'gh-district-mampong', 'name': 'Mampong Municipal'},
      'parentPlaceId': 'gh-place-parent',
      'aliases': [
        {
          'value': 'Mampɔŋ',
          'type': 'official',
          'language': 'tw',
          'isPreferred': true
        }
      ],
      'centroid': {'latitude': 7.06, 'longitude': -1.4},
      'population': 42000,
      'status': 'ACTIVE',
      'verificationStatus': 'CANONICAL',
      'provenance': {
        'sourceId': 'fixture',
        'sourceUrl': 'https://example.test/source',
        'retrievedAt': '2026-08-30T00:00:00Z'
      },
      'datasetVersion': 'test'
    };
