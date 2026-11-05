import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shelf/src/auth/auth_controller.dart';
import 'package:shelf/src/auth/storage.dart';

void main() {
  test('signInWithDemo signs in with the advertised token', () async {
    final requests = <http.Request>[];
    final client = MockClient((request) async {
      requests.add(request);
      if (request.url.path == '/api/v1/auth/config') {
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
      }
      return http.Response(
        jsonEncode({
          'data': {
            'id': 'demo',
            'email': 'demo@theshelf.local',
            'name': 'Shelf Demo',
            'role': 'demo',
          },
        }),
        200,
        headers: {'content-type': 'application/json'},
      );
    });
    final storage = _FakeAuthStorage();
    final container = ProviderContainer(
      overrides: [authStorageProvider.overrideWithValue(storage)],
    );
    addTearDown(container.dispose);
    container.read(authControllerProvider);
    await Future<void>.delayed(Duration.zero);

    final ok = await container
        .read(authControllerProvider.notifier)
        .signInWithDemo('shelf.example.com', client: client);

    expect(ok, isTrue);
    final state = container.read(authControllerProvider);
    expect(state.status, AuthStatus.authenticated);
    expect(state.token, 'bg_demo_token');
    expect(state.profile?.name, 'Shelf Demo');
    expect(storage.savedToken, 'bg_demo_token');
    expect(requests.first.url.path, '/api/v1/auth/config');
    expect(requests.first.headers.containsKey('Authorization'), isFalse);
    expect(requests.last.headers['Authorization'], 'Bearer bg_demo_token');
  });

  test('signInWithDemo reports when the demo is disabled', () async {
    final client = MockClient(
      (_) async => http.Response(
        jsonEncode({
          'data': {
            'demo': {'enabled': false},
          },
        }),
        200,
        headers: {'content-type': 'application/json'},
      ),
    );
    final container = ProviderContainer(
      overrides: [authStorageProvider.overrideWithValue(_FakeAuthStorage())],
    );
    addTearDown(container.dispose);
    container.read(authControllerProvider);
    await Future<void>.delayed(Duration.zero);

    final ok = await container
        .read(authControllerProvider.notifier)
        .signInWithDemo('shelf.example.com', client: client);

    expect(ok, isFalse);
    final state = container.read(authControllerProvider);
    expect(state.status, AuthStatus.unauthenticated);
    expect(state.token, isEmpty);
    expect(state.error, contains('not available'));
  });
}

class _FakeAuthStorage extends AuthStorage {
  String? savedToken;

  @override
  Future<String?> readServer() async => null;

  @override
  Future<String?> readToken() async => null;

  @override
  Future<String> readTheme() async => 'system';

  @override
  Future<void> save(String serverUrl, String token) async {
    savedToken = token;
  }
}
