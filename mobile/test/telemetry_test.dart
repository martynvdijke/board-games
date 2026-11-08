import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shelf/src/telemetry/telemetry.dart';

void main() {
  test('Umami allowlist drops unknown events and private fields', () {
    expect(
      sanitizeUmamiEvent('shelf_search', {
        'has_query': true,
        'query': 'Wingspan',
      }),
      {'has_query': true},
    );
    expect(
      sanitizeUmamiEvent('play_submitted', {
        'mode': 'create',
        'game': 'Wingspan',
      }),
      {'mode': 'create'},
    );
    expect(sanitizeUmamiEvent('screen_view', {'screen': 'shelf'}), {
      'screen': 'shelf',
    });
    expect(sanitizeUmamiEvent('screen_view', {'screen': 'Wingspan'}), isNull);
    expect(sanitizeUmamiEvent('note_entered', {'text': 'secret'}), isNull);
  });

  test('Traceparent is valid W3C trace context', () {
    final trace = randomHex(16);
    final span = randomHex(8);
    final header = buildTraceparent(trace, span);
    expect(header, startsWith('00-$trace-$span-01'));
    expect(header.length, 55);
  });

  test('OTLP payload has resource and scope spans', () {
    final span = TraceSpan(
      name: 'app.start',
      attributes: const {'screen': 'shelf'},
      traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
      spanId: '00f067aa0ba902b7',
      startNanos: 1,
    );
    final payload = buildOtlpPayload(
      serviceName: 'shelf-android',
      serviceVersion: '1.0.0',
      span: span,
      attributes: span.attributes,
      failed: false,
      endNanos: 2,
    );
    expect(payload['resourceSpans'], isA<List>());
    final resource = (payload['resourceSpans'] as List).single as Map;
    final scope = (resource['scopeSpans'] as List).single as Map;
    expect((scope['spans'] as List).length, 1);
    final exported = (scope['spans'] as List).single as Map;
    expect(exported['traceId'], span.traceId);
    expect(exported['endTimeUnixNano'], '2');
  });

  group('OtelSettings.fromJson', () {
    test('parses the canonical server payload', () {
      final settings = OtelSettings.fromJson({
        'endpoint': 'https://collector.example.com/otlp',
        'enabled': true,
        'traces_enabled': true,
        'metrics_enabled': false,
        'logs_enabled': false,
        'service_name': 'the-shelf-api',
      });
      expect(settings.endpoint, 'https://collector.example.com/otlp');
      expect(settings.enabled, isTrue);
      expect(settings.tracesEnabled, isTrue);
      expect(settings.metricsEnabled, isFalse);
      expect(settings.logsEnabled, isFalse);
      expect(settings.serviceName, 'the-shelf-api');
    });

    test('accepts otel_ prefixed keys and string booleans', () {
      final settings = OtelSettings.fromJson({
        'otel_endpoint': ' https://collector.example.com/otlp ',
        'otel_enabled': 'true',
        'traces_enabled': '0',
      });
      expect(settings.endpoint, 'https://collector.example.com/otlp');
      expect(settings.enabled, isTrue);
      expect(settings.tracesEnabled, isFalse);
    });

    test('defaults tracesEnabled to enabled', () {
      final settings = OtelSettings.fromJson({
        'endpoint': 'https://collector.example.com/otlp',
      });
      expect(settings.enabled, isTrue);
      expect(settings.tracesEnabled, isTrue);
    });

    test('an empty endpoint is always disabled', () {
      final settings = OtelSettings.fromJson({
        'endpoint': '',
        'enabled': true,
        'traces_enabled': true,
      });
      expect(settings.enabled, isFalse);
      expect(settings.tracesEnabled, isFalse);
    });
  });

  group('fetchOtelSettings', () {
    test('reads the endpoint from /api/v1/otel-settings', () async {
      late Uri requested;
      final client = MockClient((request) async {
        requested = request.url;
        expect(request.headers['Accept'], 'application/json');
        return http.Response(
          jsonEncode({
            'endpoint': 'https://collector.example.com/otlp',
            'enabled': true,
            'traces_enabled': true,
            'service_name': 'the-shelf-api',
          }),
          200,
        );
      });
      final settings = await fetchOtelSettings(
        'https://shelf.example.com/',
        client: client,
      );
      expect(requested.path, '/api/v1/otel-settings');
      expect(settings, isNotNull);
      expect(settings!.endpoint, 'https://collector.example.com/otlp');
      expect(settings.tracesEnabled, isTrue);
    });

    test('reports the disabled state from an empty endpoint', () async {
      final client = MockClient(
        (_) async => http.Response(
          jsonEncode({
            'endpoint': '',
            'enabled': false,
            'traces_enabled': false,
          }),
          200,
        ),
      );
      final settings = await fetchOtelSettings(
        'https://shelf.example.com',
        client: client,
      );
      expect(settings, isNotNull);
      expect(settings!.enabled, isFalse);
      expect(settings.tracesEnabled, isFalse);
    });

    test('returns null on non-200, bad JSON, and transport errors', () async {
      final notFound = MockClient((_) async => http.Response('nope', 404));
      expect(
        await fetchOtelSettings('https://shelf.example.com', client: notFound),
        isNull,
      );

      final badJson = MockClient((_) async => http.Response('{not json', 200));
      expect(
        await fetchOtelSettings('https://shelf.example.com', client: badJson),
        isNull,
      );

      final failing = MockClient((_) async => throw const SocketLikeError());
      expect(
        await fetchOtelSettings('https://shelf.example.com', client: failing),
        isNull,
      );
    });

    test('returns null instead of hanging when the server is slow', () async {
      final slow = MockClient((_) async {
        await Future<void>.delayed(const Duration(milliseconds: 200));
        return http.Response('{}', 200);
      });
      final settings = await fetchOtelSettings(
        'https://shelf.example.com',
        client: slow,
        timeout: const Duration(milliseconds: 10),
      );
      expect(settings, isNull);
    });
  });

  group('startFromServer', () {
    tearDown(() => Telemetry.instance.resetForTest());

    test('stays disabled when the server disables telemetry', () async {
      final client = MockClient(
        (_) async => http.Response(
          jsonEncode({'endpoint': '', 'enabled': false}),
          200,
        ),
      );
      await Telemetry.instance.startFromServer(
        'https://shelf.example.com',
        client: client,
      );
      expect(Telemetry.instance.otelEnabled, isFalse);
    });

    test('adopts an enabled server endpoint', () async {
      final client = MockClient(
        (_) async => http.Response(
          jsonEncode({
            'endpoint': 'https://collector.example.com/otlp',
            'enabled': true,
            'traces_enabled': true,
          }),
          200,
        ),
      );
      await Telemetry.instance.startFromServer(
        'https://shelf.example.com',
        client: client,
      );
      expect(Telemetry.instance.otelEnabled, isTrue);
    });

    test('follows traces_enabled even when the server is enabled', () async {
      final client = MockClient(
        (_) async => http.Response(
          jsonEncode({
            'endpoint': 'https://collector.example.com/otlp',
            'enabled': true,
            'traces_enabled': false,
          }),
          200,
        ),
      );
      await Telemetry.instance.startFromServer(
        'https://shelf.example.com',
        client: client,
      );
      // Traces disabled => no OTLP exporter, and the dart-define fallback is
      // empty in tests, so telemetry stays off.
      expect(Telemetry.instance.otelEnabled, isFalse);
    });

    test('never throws when the server is unreachable', () async {
      final client = MockClient((_) async => throw const SocketLikeError());
      await expectLater(
        Telemetry.instance.startFromServer(
          'https://shelf.example.com',
          client: client,
        ),
        completes,
      );
      expect(Telemetry.instance.otelEnabled, isFalse);
    });
  });
}

/// Minimal stand-in for a transport failure, avoiding dart:io in tests.
class SocketLikeError implements Exception {
  const SocketLikeError();
}
