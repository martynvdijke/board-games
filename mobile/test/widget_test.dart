import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/src/api/api_client.dart';
import 'package:shelf/src/auth/auth_controller.dart';
import 'package:shelf/src/auth/storage.dart';
import 'package:shelf/src/config.dart';
import 'package:shelf/src/screens/onboarding_screen.dart';
import 'package:shelf/src/theme/shelf_theme.dart';

void main() {
  test('server URL normalization accepts common pastes', () {
    expect(
      ApiClient.normalizeServerUrl('shelf.example.com'),
      'https://shelf.example.com',
    );
    expect(
      ApiClient.normalizeServerUrl('https://shelf.example.com/'),
      'https://shelf.example.com',
    );
    expect(
      ApiClient.normalizeServerUrl('http://localhost:6290/api/v1/'),
      'http://localhost:6290',
    );
  });

  testWidgets('warm theme exposes the editorial palette', (tester) async {
    final theme = ShelfTheme.light();
    expect(theme.colorScheme.primary, const Color(0xFF1F5A45));
    expect(theme.scaffoldBackgroundColor, const Color(0xFFF7F2E8));
    expect(theme.textTheme.displaySmall?.fontFamily, 'Newsreader');
  });

  test('server defaults to the hosted instance until one is saved', () {
    expect(
      const AuthState(status: AuthStatus.unauthenticated).serverUrl,
      defaultServerUrl,
    );
    expect(resolveInitialServerUrl(null), defaultServerUrl);
    expect(resolveInitialServerUrl('  '), defaultServerUrl);
    expect(
      resolveInitialServerUrl('https://games.example.com'),
      'https://games.example.com',
    );
  });

  test('a saved server URL wins over the default', () async {
    final container = ProviderContainer(
      overrides: [
        authStorageProvider.overrideWithValue(
          _FakeAuthStorage(server: 'https://games.example.com'),
        ),
      ],
    );
    addTearDown(container.dispose);
    container.read(authControllerProvider);
    await Future<void>.delayed(Duration.zero);
    expect(
      container.read(authControllerProvider).serverUrl,
      'https://games.example.com',
    );
  });

  testWidgets('onboarding prefills the default server on first run', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [authStorageProvider.overrideWithValue(_FakeAuthStorage())],
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(home: OnboardingScreen()),
      ),
    );
    await tester.pumpAndSettle();
    final field = tester.widget<TextField>(find.byType(TextField).first);
    expect(field.controller?.text, defaultServerUrl);
  });
}

class _FakeAuthStorage extends AuthStorage {
  _FakeAuthStorage({this.server});

  final String? server;

  @override
  Future<String?> readServer() async => server;

  @override
  Future<String?> readToken() async => null;

  @override
  Future<String> readTheme() async => 'system';
}
