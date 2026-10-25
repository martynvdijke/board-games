import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/src/api/api_client.dart';
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
}
