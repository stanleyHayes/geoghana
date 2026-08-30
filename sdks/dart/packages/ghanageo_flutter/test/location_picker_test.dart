import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ghanageo_flutter/ghanageo_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

void main() {
  testWidgets('announces results and selects with keyboard navigation',
      (tester) async {
    Place? selected;
    final client = _client((_) async => _response(['Kumasi', 'Kumawu']));
    await tester.pumpWidget(_app(GhanaGeoLocationPicker(
        client: client,
        debounce: Duration.zero,
        onSelected: (value) => selected = value)));
    await tester.enterText(find.byType(TextField), 'Kum');
    await tester.pumpAndSettle();
    expect(find.bySemanticsLabel('2 location suggestions'), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pump();
    expect(selected?.name, 'Kumawu');
    expect(tester.widget<TextField>(find.byType(TextField)).controller?.text,
        'Kumawu');
  });

  testWidgets('cancels and ignores a stale slower response', (tester) async {
    final firstStarted = Completer<void>();
    final releaseFirst = Completer<void>();
    final client = _client((request) async {
      if (request.url.queryParameters['q'] == 'Kum') {
        firstStarted.complete();
        await releaseFirst.future;
        return _response(['Kumasi']);
      }
      return _response(['Accra']);
    });
    await tester.pumpWidget(_app(GhanaGeoLocationPicker(
        client: client, debounce: Duration.zero, onSelected: (_) {})));
    await tester.enterText(find.byType(TextField), 'Kum');
    await tester.pump();
    await firstStarted.future;
    await tester.enterText(find.byType(TextField), 'Acc');
    await tester.pumpAndSettle();
    expect(find.text('Accra'), findsOneWidget);
    releaseFirst.complete();
    await tester.pumpAndSettle();
    expect(find.text('Accra'), findsOneWidget);
    expect(find.text('Kumasi'), findsNothing);
  });

  testWidgets('announces loading and errors as live semantics', (tester) async {
    final pending = Completer<http.Response>();
    final client = _client((_) => pending.future);
    await tester.pumpWidget(_app(GhanaGeoLocationPicker(
        client: client, debounce: Duration.zero, onSelected: (_) {})));
    await tester.enterText(find.byType(TextField), 'Bad');
    await tester.pump();
    expect(
        find.bySemanticsLabel('Loading location suggestions'), findsOneWidget);
    pending.complete(http.Response(
        jsonEncode({
          'error': {
            'code': 'INTERNAL',
            'message': 'failed',
            'requestId': 'req-1',
            'docs': '/errors/internal'
          }
        }),
        500));
    await tester.pumpAndSettle();
    expect(find.text('Locations could not be loaded'), findsOneWidget);
    expect(
        find.bySemanticsLabel('Locations could not be loaded'), findsWidgets);
  });

  testWidgets('updates initial value while preserving focus and caret',
      (tester) async {
    final client = _client((_) async => _response(const []));
    final first = _place('Kumasi');
    final second = _place('Accra');
    await tester.pumpWidget(_app(GhanaGeoLocationPicker(
        client: client, initialValue: first, onSelected: (_) {})));
    await tester.tap(find.byType(TextField));
    await tester.pump();
    expect(tester.testTextInput.isVisible, isTrue);
    await tester.pumpWidget(_app(GhanaGeoLocationPicker(
        client: client, initialValue: second, onSelected: (_) {})));
    await tester.pump();
    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.controller?.text, 'Accra');
    expect(field.controller?.selection.baseOffset, 5);
    expect(field.focusNode?.hasFocus, isTrue);
  });

  testWidgets('tap selection returns the complete typed place', (tester) async {
    Place? selected;
    final client = _client((_) async => _response(['Mampɔŋ']));
    await tester.pumpWidget(_app(GhanaGeoLocationPicker(
        client: client,
        debounce: Duration.zero,
        onSelected: (value) => selected = value)));
    await tester.enterText(find.byType(TextField), 'Mamp');
    await tester.pumpAndSettle();
    await tester.tap(find.text('Mampɔŋ'));
    expect(selected?.normalizedName, 'mampɔŋ');
    expect(selected?.provenance.sourceId, 'fixture');
  });
}

GhanaGeoClient _client(Future<http.Response> Function(http.Request) handler) =>
    GhanaGeoClient(
      baseUrl: Uri.parse('https://example.test/v1'),
      retry: 0,
      httpClient: MockClient(handler),
    );

Widget _app(Widget child) => MaterialApp(home: Scaffold(body: child));

http.Response _response(List<String> names) => http.Response(
      jsonEncode(
          {'data': names.map(_placeJson).toList(), 'datasetVersion': 'test'}),
      200,
      headers: {'content-type': 'application/json'},
    );

Map<String, Object?> _placeJson(String name) => {
      'id': 'gh-place-${name.toLowerCase()}',
      'name': name,
      'normalizedName': name.toLowerCase(),
      'type': 'CITY',
      'aliases': <Object?>[],
      'status': 'ACTIVE',
      'verificationStatus': 'REFERENCE',
      'provenance': {'sourceId': 'fixture'},
      'datasetVersion': 'test',
      'score': 1.0,
      'matchReason': 'prefix match',
    };

Place _place(String name) => Place.fromJson(_placeJson(name));
