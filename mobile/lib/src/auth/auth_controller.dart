import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../api/models.dart';
import '../config.dart';
import 'storage.dart';

enum AuthStatus { loading, unauthenticated, authenticated }

class AuthState {
  const AuthState({
    required this.status,
    this.profile,
    this.serverUrl = defaultServerUrl,
    this.token = '',
    this.error = '',
  });

  final AuthStatus status;
  final UserProfile? profile;
  final String serverUrl;
  final String token;
  final String error;

  AuthState copyWith({
    AuthStatus? status,
    UserProfile? profile,
    String? serverUrl,
    String? token,
    String? error,
  }) {
    return AuthState(
      status: status ?? this.status,
      profile: profile ?? this.profile,
      serverUrl: serverUrl ?? this.serverUrl,
      token: token ?? this.token,
      error: error ?? this.error,
    );
  }
}

final authStorageProvider = Provider<AuthStorage>((ref) => AuthStorage());

final authControllerProvider = NotifierProvider<AuthController, AuthState>(
  AuthController.new,
);

class AuthController extends Notifier<AuthState> {
  @override
  AuthState build() {
    scheduleMicrotask(_restore);
    return const AuthState(status: AuthStatus.loading);
  }

  AuthStorage get _storage => ref.read(authStorageProvider);

  Future<void> _restore() async {
    final server = await _storage.readServer();
    final token = await _storage.readToken();
    if (server == null || server.isEmpty || token == null || token.isEmpty) {
      state = AuthState(
        status: AuthStatus.unauthenticated,
        serverUrl: resolveInitialServerUrl(server),
      );
      return;
    }
    final client = ApiClient(serverUrl: server, token: token);
    try {
      final profile = await client.me();
      state = AuthState(
        status: AuthStatus.authenticated,
        profile: profile,
        serverUrl: ApiClient.normalizeServerUrl(server),
        token: token,
      );
    } on ApiException catch (error) {
      if (error.statusCode == 401 || error.statusCode == 403) {
        await _storage.clear();
        state = AuthState(
          status: AuthStatus.unauthenticated,
          serverUrl: server,
          error: 'That token is no longer valid. Sign in again with a fresh token.',
        );
      } else {
        state = AuthState(
          status: AuthStatus.unauthenticated,
          serverUrl: server,
          error: error.message,
        );
      }
    } finally {
      client.close();
    }
  }

  Future<bool> signIn(
    String rawServer,
    String rawToken, {
    http.Client? client,
  }) async {
    final server = ApiClient.normalizeServerUrl(rawServer);
    final token = rawToken.trim();
    if (server.isEmpty || token.isEmpty) {
      state = state.copyWith(
        error: 'Enter your shelf address and a personal API token.',
      );
      return false;
    }
    state = state.copyWith(error: '', serverUrl: server, token: token);
    final apiClient = ApiClient(
      serverUrl: server,
      token: token,
      client: client,
    );
    try {
      final profile = await apiClient.me();
      await _storage.save(server, token);
      state = AuthState(
        status: AuthStatus.authenticated,
        profile: profile,
        serverUrl: server,
        token: token,
      );
      return true;
    } on ApiException catch (error) {
      state = AuthState(
        status: AuthStatus.unauthenticated,
        serverUrl: server,
        token: '',
        error: error.message,
      );
      return false;
    } catch (_) {
      state = AuthState(
        status: AuthStatus.unauthenticated,
        serverUrl: server,
        token: '',
        error: 'Could not reach that server. Check the address and try again.',
      );
      return false;
    } finally {
      if (client == null) apiClient.close();
    }
  }

  /// Signs in with the public demo account advertised by the server. The
  /// config request is unauthenticated and no stored token is ever reused.
  Future<bool> signInWithDemo(String rawServer, {http.Client? client}) async {
    final server = ApiClient.normalizeServerUrl(rawServer);
    if (server.isEmpty) {
      state = state.copyWith(
        status: AuthStatus.unauthenticated,
        error: 'Enter your shelf address to try the demo.',
      );
      return false;
    }
    state = state.copyWith(error: '', serverUrl: server);
    AuthConfig config;
    try {
      config = await ApiClient.fetchAuthConfig(server, client: client);
    } on ApiException catch (error) {
      state = AuthState(
        status: AuthStatus.unauthenticated,
        serverUrl: server,
        error: error.message.isEmpty
            ? 'Could not load the demo account. Try again in a moment.'
            : error.message,
      );
      return false;
    } catch (_) {
      state = AuthState(
        status: AuthStatus.unauthenticated,
        serverUrl: server,
        error: 'Could not load the demo account. Try again in a moment.',
      );
      return false;
    }
    if (!config.demo.enabled || !config.demo.hasToken) {
      state = AuthState(
        status: AuthStatus.unauthenticated,
        serverUrl: server,
        error: 'The demo account is not available on this server.',
      );
      return false;
    }
    return signIn(server, config.demo.token, client: client);
  }

  Future<void> signOut() async {
    await _storage.clear();
    state = const AuthState(
      status: AuthStatus.unauthenticated,
      serverUrl: defaultServerUrl,
    );
  }

  /// Deletes the server account and only then clears the local token. If the
  /// request fails the credentials stay so the user can retry.
  Future<void> deleteAccount() async {
    final client = ApiClient(serverUrl: state.serverUrl, token: state.token);
    try {
      await client.deleteAccount();
    } finally {
      client.close();
    }
    await _storage.clear();
    state = const AuthState(
      status: AuthStatus.unauthenticated,
      serverUrl: defaultServerUrl,
    );
  }

  void clearError() {
    if (state.error.isNotEmpty) state = state.copyWith(error: '');
  }
}
