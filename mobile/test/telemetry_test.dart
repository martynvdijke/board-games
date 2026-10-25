import 'package:flutter_test/flutter_test.dart';
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
}
