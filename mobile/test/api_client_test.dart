import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shelf/src/api/api_client.dart';
import 'package:shelf/src/api/api_exception.dart';
import 'package:shelf/src/api/models.dart';

void main() {
  test('games sends filters, bearer auth, and W3C trace context', () async {
    late http.Request captured;
    final client = ApiClient(
      serverUrl: 'https://shelf.example.com/api/v1/',
      token: 'bg_test',
      client: MockClient((request) async {
        captured = request;
        return http.Response(
          jsonEncode({
            'data': [
              {
                'id': 1,
                'name': 'Wingspan',
                'year': 2019,
                'total_plays': 4,
                'min_players': 1,
                'max_players': 5,
              },
            ],
            'pagination': {
              'limit': 25,
              'offset': 0,
              'total': 1,
              'has_more': false,
            },
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }),
    );

    final page = await client.games(
      query: 'wing',
      status: 'unplayed',
      sort: 'rating',
      limit: 25,
    );

    expect(page.items.single.name, 'Wingspan');
    expect(page.total, 1);
    expect(captured.headers['Authorization'], 'Bearer bg_test');
    expect(
      captured.headers['traceparent'],
      matches(RegExp(r'^00-[0-9a-f]{32}-[0-9a-f]{16}-01$')),
    );
    expect(captured.url.path, '/api/v1/games');
    expect(captured.url.queryParameters['q'], 'wing');
    expect(captured.url.queryParameters['status'], 'unplayed');
    expect(captured.url.queryParameters['sort'], 'rating');
    expect(captured.url.queryParameters['limit'], '25');
  });

  test('error envelope becomes a typed exception', () async {
    final client = ApiClient(
      serverUrl: 'https://shelf.example.com',
      token: 'bg_test',
      client: MockClient(
        (_) async => http.Response(
          jsonEncode({
            'error': {
              'code': 'validation_failed',
              'message': 'That play could not be saved.',
              'fields': {'played_at': 'Use YYYY-MM-DD.'},
            },
          }),
          422,
        ),
      ),
    );

    await expectLater(
      client.createPlay(const PlayInput(gameId: 1, playedAt: 'bad')),
      throwsA(
        isA<ApiException>()
            .having((e) => e.statusCode, 'statusCode', 422)
            .having((e) => e.code, 'code', 'validation_failed')
            .having(
              (e) => e.fields['played_at'],
              'field hint',
              'Use YYYY-MM-DD.',
            ),
      ),
    );
  });

  test(
    'fetchAuthConfig parses an enabled demo without sending a token',
    () async {
      late http.Request captured;
      final config = await ApiClient.fetchAuthConfig(
        'shelf.example.com/api/v1/',
        client: MockClient((request) async {
          captured = request;
          return http.Response(
            jsonEncode({
              'data': {
                'demo': {
                  'enabled': true,
                  'email': 'demo@theshelf.local',
                  'name': 'Shelf Demo',
                  'token': 'bg_demo_token',
                },
              },
            }),
            200,
            headers: {'content-type': 'application/json'},
          );
        }),
      );

      expect(config.demo.enabled, isTrue);
      expect(config.demo.email, 'demo@theshelf.local');
      expect(config.demo.name, 'Shelf Demo');
      expect(config.demo.token, 'bg_demo_token');
      expect(config.demo.hasToken, isTrue);
      expect(captured.method, 'GET');
      expect(
        captured.url.toString(),
        'https://shelf.example.com/api/v1/auth/config',
      );
      expect(captured.headers.containsKey('Authorization'), isFalse);
    },
  );

  test('fetchAuthConfig tolerates disabled and missing fields', () async {
    final config = await ApiClient.fetchAuthConfig(
      'https://shelf.example.com',
      client: MockClient(
        (_) async => http.Response(
          jsonEncode({
            'data': {
              'demo': {'enabled': false},
            },
          }),
          200,
          headers: {'content-type': 'application/json'},
        ),
      ),
    );

    expect(config.demo.enabled, isFalse);
    expect(config.demo.email, isEmpty);
    expect(config.demo.name, isEmpty);
    expect(config.demo.token, isEmpty);
    expect(config.demo.hasToken, isFalse);
  });

  test('fetchAuthConfig maps an error envelope to ApiException', () async {
    await expectLater(
      ApiClient.fetchAuthConfig(
        'https://shelf.example.com',
        client: MockClient(
          (_) async => http.Response(
            jsonEncode({
              'error': {
                'code': 'not_found',
                'message': 'Auth config is unavailable.',
              },
            }),
            404,
          ),
        ),
      ),
      throwsA(
        isA<ApiException>()
            .having((e) => e.statusCode, 'statusCode', 404)
            .having((e) => e.code, 'code', 'not_found')
            .having((e) => e.message, 'message', 'Auth config is unavailable.'),
      ),
    );
  });

  test('deleteAccount sends DELETE /me with the bearer token', () async {
    late http.Request captured;
    final client = ApiClient(
      serverUrl: 'https://shelf.example.com',
      token: 'bg_test',
      client: MockClient((request) async {
        captured = request;
        return http.Response(
          jsonEncode({
            'data': {'deleted': true},
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }),
    );

    await client.deleteAccount();

    expect(captured.method, 'DELETE');
    expect(captured.url.path, '/api/v1/me');
    expect(captured.headers['Authorization'], 'Bearer bg_test');
  });
}
