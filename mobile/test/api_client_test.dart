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
}
