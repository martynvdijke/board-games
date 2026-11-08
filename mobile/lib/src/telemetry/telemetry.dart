import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;

/// Configuration is compile-time so secrets never live in app storage.
abstract final class TelemetryConfig {
  static const otelEndpoint = String.fromEnvironment(
    'OTEL_EXPORTER_OTLP_ENDPOINT',
  );
  static const otelHeaders = String.fromEnvironment(
    'OTEL_EXPORTER_OTLP_HEADERS',
  );
  static const serviceName = String.fromEnvironment(
    'OTEL_SERVICE_NAME',
    defaultValue: 'the-shelf-android',
  );
  static const serviceVersion = String.fromEnvironment(
    'OTEL_SERVICE_VERSION',
    defaultValue: '1.0.0',
  );
  static const umamiUrl = String.fromEnvironment('UMAMI_URL');
  static const umamiWebsiteId = String.fromEnvironment('UMAMI_WEBSITE_ID');

  static bool get otelEnabled => otelEndpoint.trim().isNotEmpty && !kIsWeb;
  static bool get umamiEnabled =>
      umamiUrl.trim().isNotEmpty && umamiWebsiteId.trim().isNotEmpty;
}

/// Allowlisted analytics events. Keys and values are enforced at the boundary,
/// so game names, notes, player names, and searches can never be sent.
const Map<String, Set<String>> umamiEventAllowlist = {
  'screen_view': {'screen'},
  'shelf_search': {'has_query'},
  'play_submitted': {'mode'},
  'sync_started': {},
  'theme_changed': {'theme'},
};

const Set<String> _screens = {
  'shelf',
  'game',
  'plays',
  'play_form',
  'stats',
  'settings',
  'onboarding',
};
const Set<String> _themes = {'system', 'light', 'dark'};
const Set<String> _modes = {'create', 'edit'};

Map<String, Object>? sanitizeUmamiEvent(
  String event, [
  Map<String, Object?> props = const {},
]) {
  final allowed = umamiEventAllowlist[event];
  if (allowed == null) return null;
  final clean = <String, Object>{};
  for (final entry in props.entries) {
    if (!allowed.contains(entry.key)) continue;
    final value = entry.value;
    if (value is bool || value is num) {
      clean[entry.key] = value as Object;
      continue;
    }
    if (value is! String || value.length > 64) continue;
    if (entry.key == 'screen' && !_screens.contains(value)) continue;
    if (entry.key == 'theme' && !_themes.contains(value)) continue;
    if (entry.key == 'mode' && !_modes.contains(value)) continue;
    clean[entry.key] = value;
  }
  if (allowed.isNotEmpty && clean.isEmpty) return null;
  return clean;
}

String randomHex(int bytes) {
  final random = Random.secure();
  final buffer = StringBuffer();
  for (var i = 0; i < bytes; i++) {
    buffer.write(random.nextInt(256).toRadixString(16).padLeft(2, '0'));
  }
  return buffer.toString();
}

String buildTraceparent(String traceId, String spanId) =>
    '00-$traceId-$spanId-01';

/// Runtime OpenTelemetry settings published by the web API.
///
/// Parsing is tolerant on purpose: the server may use either the short or the
/// `otel_`-prefixed key names, and an older server may omit the per-signal
/// flags entirely. An empty endpoint always means "telemetry is off".
class OtelSettings {
  const OtelSettings({
    required this.endpoint,
    required this.enabled,
    required this.tracesEnabled,
    this.metricsEnabled = false,
    this.logsEnabled = false,
    this.serviceName = '',
  });

  final String endpoint;
  final bool enabled;
  final bool tracesEnabled;
  final bool metricsEnabled;
  final bool logsEnabled;
  final String serviceName;

  factory OtelSettings.fromJson(Map<String, dynamic> json) {
    final endpoint = (json['endpoint'] ?? json['otel_endpoint'] ?? '')
        .toString()
        .trim();
    final enabled =
        _asBool(json['enabled'] ?? json['otel_enabled']) ||
        (json['enabled'] == null &&
            json['otel_enabled'] == null &&
            endpoint.isNotEmpty);
    final traces = _asBool(
      json['traces_enabled'],
      fallback: enabled,
    );
    final usable = enabled && endpoint.isNotEmpty;
    return OtelSettings(
      endpoint: endpoint,
      enabled: usable,
      tracesEnabled: usable && traces,
      metricsEnabled: usable && _asBool(json['metrics_enabled']),
      logsEnabled: usable && _asBool(json['logs_enabled']),
      serviceName: (json['service_name'] ?? json['otel_service_name'] ?? '')
          .toString()
          .trim(),
    );
  }

  static bool _asBool(Object? value, {bool fallback = false}) {
    if (value == null) return fallback;
    if (value is bool) return value;
    if (value is num) return value != 0;
    final text = value.toString().trim().toLowerCase();
    if (text == 'true' || text == '1' || text == 'yes' || text == 'on') {
      return true;
    }
    if (text == 'false' || text == '0' || text == 'no' || text == 'off') {
      return false;
    }
    return fallback;
  }
}

/// Resolves the OTLP endpoint from the web API. Never throws: offline, slow,
/// malformed, or non-200 responses all yield `null` so startup can continue
/// with the compile-time configuration.
Future<OtelSettings?> fetchOtelSettings(
  String baseUrl, {
  http.Client? client,
  Duration timeout = const Duration(seconds: 3),
}) async {
  final owned = client == null;
  final httpClient = client ?? http.Client();
  try {
    final base = baseUrl.trim().replaceAll(RegExp(r'/+$'), '');
    if (base.isEmpty) return null;
    final uri = Uri.tryParse('$base/api/v1/otel-settings');
    if (uri == null) return null;
    final response = await httpClient
        .get(uri, headers: {'Accept': 'application/json'})
        .timeout(timeout);
    if (response.statusCode != 200) return null;
    final decoded = jsonDecode(response.body);
    if (decoded is! Map) return null;
    final map = decoded['data'] is Map
        ? Map<String, dynamic>.from(decoded['data'] as Map)
        : Map<String, dynamic>.from(decoded);
    return OtelSettings.fromJson(map);
  } catch (_) {
    return null;
  } finally {
    if (owned) httpClient.close();
  }
}

class TraceSpan {
  TraceSpan({
    required this.name,
    required this.attributes,
    required this.traceId,
    required this.spanId,
    required this.startNanos,
    this.kind = 1,
  });

  final String name;
  final Map<String, Object> attributes;
  final String traceId;
  final String spanId;
  final int startNanos;
  final int kind;
  bool _ended = false;
  int? statusCode;
  String error = '';

  String get traceparent => buildTraceparent(traceId, spanId);

  void end({
    int? httpStatus,
    Object? failure,
    Map<String, Object> extra = const {},
  }) {
    if (_ended) return;
    _ended = true;
    statusCode = httpStatus;
    if (failure != null) error = failure.toString();
    final attrs = <String, Object>{...attributes, ...extra};
    if (httpStatus != null) attrs['http.response.status_code'] = httpStatus;
    if (error.isNotEmpty) attrs['error.message'] = _short(error);
    Telemetry.instance._finish(
      this,
      attrs,
      error.isNotEmpty || (httpStatus != null && httpStatus >= 500),
    );
  }

  String _short(String value) =>
      value.length > 200 ? '${value.substring(0, 200)}…' : value;
}

Map<String, Object?> buildOtlpPayload({
  required String serviceName,
  required String serviceVersion,
  required TraceSpan span,
  required Map<String, Object> attributes,
  required bool failed,
  required int endNanos,
}) {
  List<Map<String, Object?>> attrs(Map<String, Object> values) => values.entries
      .map(
        (entry) => {
          'key': entry.key,
          'value': switch (entry.value) {
            final bool value => {'boolValue': value},
            final int value => {'intValue': value.toString()},
            final double value => {'doubleValue': value},
            _ => {'stringValue': entry.value.toString()},
          },
        },
      )
      .toList();

  return {
    'resourceSpans': [
      {
        'resource': {
          'attributes': attrs({
            'service.name': serviceName,
            'service.version': serviceVersion,
            'telemetry.sdk.name': 'shelf-mobile',
            'telemetry.sdk.language': 'dart',
          }),
        },
        'scopeSpans': [
          {
            'scope': {'name': 'shelf.mobile', 'version': serviceVersion},
            'spans': [
              {
                'traceId': span.traceId,
                'spanId': span.spanId,
                'name': span.name,
                'kind': span.kind,
                'startTimeUnixNano': span.startNanos.toString(),
                'endTimeUnixNano': endNanos.toString(),
                'attributes': attrs(attributes),
                'status': {'code': failed ? 2 : 1},
              },
            ],
          },
        ],
      },
    ],
  };
}

class Telemetry {
  Telemetry._();

  static final Telemetry instance = Telemetry._();

  http.Client? _client;
  Timer? _timer;
  final List<Map<String, Object?>> _queue = [];
  bool _started = false;
  String _tracesEndpoint = '';
  Map<String, String> _headers = const {};

  bool get otelEnabled => _tracesEndpoint.isNotEmpty;
  bool get umamiEnabled => TelemetryConfig.umamiEnabled;

  Future<void> start({String? endpoint, Map<String, String>? headers}) async {
    if (_started) return;
    _started = true;
    final raw = (endpoint ?? TelemetryConfig.otelEndpoint).trim();
    final usable = raw.isNotEmpty && !kIsWeb;
    if (usable) {
      _client = http.Client();
      _tracesEndpoint = normalizeEndpoint(raw);
      _headers = headers ?? parseOtelHeaders(TelemetryConfig.otelHeaders);
      _timer = Timer.periodic(const Duration(seconds: 5), (_) => _flush());
    }
    if (otelEnabled) {
      final span = beginSpan('app.start');
      span.end();
    }
  }

  /// Starts the exporter from the endpoint published by the web API.
  ///
  /// Uses the server settings when they enable traces, otherwise falls back to
  /// the compile-time `--dart-define` configuration and finally stays idle.
  /// Never throws, so it is safe to call unawaited during startup.
  Future<void> startFromServer(String baseUrl, {http.Client? client}) async {
    OtelSettings? settings;
    try {
      settings = await fetchOtelSettings(baseUrl, client: client);
    } catch (_) {
      settings = null;
    }
    if (settings != null && settings.enabled && settings.tracesEnabled) {
      await start(endpoint: settings.endpoint);
      return;
    }
    await start();
  }

  TraceSpan beginSpan(
    String name, {
    Map<String, Object> attributes = const {},
    int kind = 1,
  }) {
    return TraceSpan(
      name: name,
      attributes: attributes,
      traceId: randomHex(16),
      spanId: randomHex(8),
      startNanos: DateTime.now().microsecondsSinceEpoch * 1000,
      kind: kind,
    );
  }

  void screen(String name) {
    final span = beginSpan('screen $name', attributes: {'screen.name': name});
    span.end();
    track('screen_view', {'screen': name});
  }

  void track(String event, [Map<String, Object?> props = const {}]) {
    final clean = sanitizeUmamiEvent(event, props);
    if (clean == null || !TelemetryConfig.umamiEnabled) return;
    final client = _client ??= http.Client();
    final base = TelemetryConfig.umamiUrl.replaceAll(RegExp(r'/+$'), '');
    final uri = Uri.tryParse('$base/api/send');
    if (uri == null) return;
    final body = {
      'type': 'event',
      'payload': {
        'website': TelemetryConfig.umamiWebsiteId,
        'hostname': 'shelf.app',
        'url': '/app/$event',
        'event': event,
        'data': clean,
      },
    };
    unawaited(
      client
          .post(
            uri,
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode(body),
          )
          .then((_) {}, onError: (Object _) {}),
    );
  }

  void _finish(TraceSpan span, Map<String, Object> attributes, bool failed) {
    if (!otelEnabled) return;
    final payload = buildOtlpPayload(
      serviceName: TelemetryConfig.serviceName,
      serviceVersion: TelemetryConfig.serviceVersion,
      span: span,
      attributes: attributes,
      failed: failed,
      endNanos: DateTime.now().microsecondsSinceEpoch * 1000,
    );
    _queue.add(payload);
    if (_queue.length >= 20) _flush();
  }

  void _flush() {
    if (_queue.isEmpty || _client == null || _tracesEndpoint.isEmpty) return;
    final spans = <Map<String, Object?>>[];
    for (final payload in _queue) {
      final resourceSpans = payload['resourceSpans'];
      if (resourceSpans is List && resourceSpans.isNotEmpty) {
        final resource = resourceSpans.first;
        if (resource is Map) {
          final scopeSpans = resource['scopeSpans'];
          if (scopeSpans is List && scopeSpans.isNotEmpty) {
            final scope = scopeSpans.first;
            if (scope is Map && scope['spans'] is List) {
              spans.addAll(
                (scope['spans'] as List).cast<Map<String, Object?>>(),
              );
            }
          }
        }
      }
    }
    _queue.clear();
    if (spans.isEmpty) return;
    final payload = {
      'resourceSpans': [
        {
          'resource': {
            'attributes': [
              {
                'key': 'service.name',
                'value': {'stringValue': TelemetryConfig.serviceName},
              },
              {
                'key': 'service.version',
                'value': {'stringValue': TelemetryConfig.serviceVersion},
              },
            ],
          },
          'scopeSpans': [
            {
              'scope': {'name': 'shelf.mobile'},
              'spans': spans,
            },
          ],
        },
      ],
    };
    unawaited(
      _client!
          .post(
            Uri.parse(_tracesEndpoint),
            headers: {'Content-Type': 'application/json', ..._headers},
            body: jsonEncode(payload),
          )
          .then(
            (response) {
              if (response.statusCode >= 400 && kDebugMode) {
                debugPrint('OTLP export failed: ${response.statusCode}');
              }
            },
            onError: (Object error) {
              if (kDebugMode) debugPrint('OTLP export failed: $error');
            },
          ),
    );
  }

  Future<void> shutdown() async {
    _timer?.cancel();
    _flush();
    await Future<void>.delayed(const Duration(milliseconds: 200));
    _client?.close();
    _client = null;
  }

  /// Clears runtime state so tests can exercise startup gating more than once.
  @visibleForTesting
  void resetForTest() {
    _timer?.cancel();
    _timer = null;
    _client?.close();
    _client = null;
    _queue.clear();
    _started = false;
    _tracesEndpoint = '';
    _headers = const {};
  }

  static String normalizeEndpoint(String raw) {
    final base = raw.trim().replaceAll(RegExp(r'/+$'), '');
    if (base.endsWith('/v1/traces')) return base;
    return '$base/v1/traces';
  }

  static Map<String, String> parseOtelHeaders(String raw) {
    final out = <String, String>{};
    for (final pair in raw.split(',')) {
      final idx = pair.indexOf('=');
      if (idx <= 0) continue;
      final key = pair.substring(0, idx).trim();
      var value = pair.substring(idx + 1).trim();
      try {
        value = Uri.decodeComponent(value);
      } catch (_) {}
      if (key.isNotEmpty && value.isNotEmpty) out[key] = value;
    }
    return out;
  }
}
