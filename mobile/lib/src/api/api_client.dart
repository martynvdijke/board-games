import 'dart:convert';

import 'package:http/http.dart' as http;

import '../telemetry/telemetry.dart';
import 'api_exception.dart';
import 'models.dart';

class GamesPage {
  const GamesPage({
    required this.items,
    required this.total,
    required this.hasMore,
  });
  final List<Game> items;
  final int total;
  final bool hasMore;
}

class PlaysPage {
  const PlaysPage({
    required this.items,
    required this.total,
    required this.hasMore,
  });
  final List<Play> items;
  final int total;
  final bool hasMore;
}

/// Thin, testable wrapper around the public `/api/v1` contract.
class ApiClient {
  ApiClient({
    required String serverUrl,
    required this.token,
    http.Client? client,
  }) : baseUrl = normalizeServerUrl(serverUrl),
       _client = client ?? http.Client();

  final String baseUrl;
  final String token;
  final http.Client _client;

  static String normalizeServerUrl(String raw) {
    var value = raw.trim();
    while (value.endsWith('/')) {
      value = value.substring(0, value.length - 1);
    }
    if (value.isEmpty) return value;
    if (!value.startsWith('http://') && !value.startsWith('https://')) {
      value = 'https://$value';
    }
    if (value.endsWith('/api/v1')) value = value.substring(0, value.length - 7);
    return value;
  }

  Uri _uri(String path, [Map<String, String>? query]) {
    final uri = Uri.parse('$baseUrl/api/v1$path');
    if (query == null || query.isEmpty) return uri;
    return uri.replace(queryParameters: {...uri.queryParameters, ...query});
  }

  Future<UserProfile> me() async {
    final data = await _send('GET', '/me', route: '/me');
    return UserProfile.fromJson(asMap(data['data']));
  }

  Future<GamesPage> games({
    String query = '',
    String status = '',
    String sort = 'name',
    int limit = 48,
    int offset = 0,
    int players = 0,
    int maxPlaytime = 0,
  }) async {
    final params = <String, String>{
      'limit': '$limit',
      'offset': '$offset',
      'sort': sort,
      if (query.trim().isNotEmpty) 'q': query.trim(),
      if (status.isNotEmpty) 'status': status,
      if (players > 0) 'players': '$players',
      if (maxPlaytime > 0) 'max_playtime': '$maxPlaytime',
    };
    final data = await _send('GET', '/games', query: params, route: '/games');
    final pagination = asMap(data['pagination']);
    return GamesPage(
      items: asList(data['data']).map(Game.fromJson).toList(),
      total: intOf(pagination, 'total'),
      hasMore: boolOf(pagination, 'has_more'),
    );
  }

  Future<GameDetail> game(int id) async {
    final data = await _send('GET', '/games/$id', route: '/games/{id}');
    return GameDetail.fromJson(asMap(data['data']));
  }

  Future<GameDetail> patchGame(int id, Map<String, Object?> patch) async {
    final data = await _send(
      'PATCH',
      '/games/$id',
      body: patch,
      route: '/games/{id}',
    );
    return GameDetail.fromJson(asMap(data['data']));
  }

  Future<PlaysPage> plays({
    int? gameId,
    int? year,
    int limit = 50,
    int offset = 0,
  }) async {
    final params = <String, String>{
      'limit': '$limit',
      'offset': '$offset',
      if (gameId != null) 'game_id': '$gameId',
      if (year != null && year > 0) 'year': '$year',
    };
    final data = await _send('GET', '/plays', query: params, route: '/plays');
    final pagination = asMap(data['pagination']);
    return PlaysPage(
      items: asList(data['data']).map(Play.fromJson).toList(),
      total: intOf(pagination, 'total'),
      hasMore: boolOf(pagination, 'has_more'),
    );
  }

  Future<Play> createPlay(PlayInput input) async {
    final data = await _send(
      'POST',
      '/plays',
      body: input.toJson(),
      route: '/plays',
    );
    return Play.fromJson(asMap(data['data']));
  }

  Future<Play> updatePlay(int id, PlayInput input) async {
    final data = await _send(
      'PATCH',
      '/plays/$id',
      body: input.toJson(),
      route: '/plays/{id}',
    );
    return Play.fromJson(asMap(data['data']));
  }

  Future<void> deletePlay(int id) async {
    await _send('DELETE', '/plays/$id', route: '/plays/{id}');
  }

  Future<Stats> stats({int? year}) async {
    final data = await _send(
      'GET',
      '/stats',
      query: {if (year != null && year > 0) 'year': '$year'},
      route: '/stats',
    );
    return Stats.fromJson(asMap(data['data']));
  }

  Future<SyncRun?> sync() async {
    final data = await _send('GET', '/sync', route: '/sync');
    final value = data['data'];
    if (value is! Map) return null;
    return SyncRun.fromJson(asMap(value));
  }

  Future<SyncRun?> startSync() async {
    final data = await _send('POST', '/sync', route: '/sync');
    final value = data['data'];
    if (value is! Map) return null;
    return SyncRun.fromJson(asMap(value));
  }

  Future<Map<String, dynamic>> _send(
    String method,
    String path, {
    Map<String, String>? query,
    Map<String, Object?>? body,
    required String route,
  }) async {
    final span = Telemetry.instance.beginSpan(
      'HTTP $method $route',
      attributes: {
        'http.request.method': method,
        'url.path': '/api/v1$route',
        'shelf.api': true,
      },
      kind: 3,
    );
    final headers = <String, String>{
      'Authorization': 'Bearer $token',
      'Accept': 'application/json',
      'traceparent': span.traceparent,
      if (body != null) 'Content-Type': 'application/json',
    };
    try {
      final request = http.Request(method, _uri(path, query))
        ..headers.addAll(headers);
      if (body != null) request.body = jsonEncode(body);
      final streamed = await _client
          .send(request)
          .timeout(const Duration(seconds: 25));
      final response = await http.Response.fromStream(streamed);
      final decoded = _decode(response.body);
      if (response.statusCode < 200 || response.statusCode >= 300) {
        final error = asMap(decoded['error']);
        final fields = <String, String>{};
        for (final entry in asMap(error['fields']).entries) {
          fields[entry.key] = entry.value.toString();
        }
        throw ApiException(
          stringOf(error, 'message', 'That request could not be completed.'),
          statusCode: response.statusCode,
          code: stringOf(error, 'code', 'request_failed'),
          fields: fields,
        );
      }
      span.end(httpStatus: response.statusCode);
      return decoded;
    } on ApiException catch (error) {
      span.end(httpStatus: error.statusCode, failure: error);
      rethrow;
    } catch (error) {
      span.end(failure: error);
      if (error is http.ClientException) {
        throw ApiException(
          'Could not reach the server. Check the URL and your connection.',
        );
      }
      throw ApiException(
        'Could not reach the server. Check the URL and your connection.',
      );
    }
  }

  Map<String, dynamic> _decode(String body) {
    if (body.trim().isEmpty) return const {};
    try {
      final decoded = jsonDecode(body);
      return decoded is Map ? lowerKeys(decoded) : const {};
    } on FormatException {
      return const {};
    }
  }

  void close() => _client.close();
}
