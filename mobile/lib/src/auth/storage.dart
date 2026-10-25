import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

class AuthStorage {
  AuthStorage({FlutterSecureStorage? secure})
    : _secure = secure ?? const FlutterSecureStorage();

  static const _serverKey = 'shelf.server';
  static const _tokenKey = 'shelf.token';
  static const _themeKey = 'shelf.theme';

  final FlutterSecureStorage _secure;

  Future<String?> readServer() => _secure.read(key: _serverKey);
  Future<String?> readToken() => _secure.read(key: _tokenKey);

  Future<void> save(String serverUrl, String token) async {
    await _secure.write(key: _serverKey, value: serverUrl);
    await _secure.write(key: _tokenKey, value: token);
  }

  Future<void> clear() async {
    await _secure.delete(key: _serverKey);
    await _secure.delete(key: _tokenKey);
  }

  Future<String> readTheme() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_themeKey) ?? 'system';
  }

  Future<void> writeTheme(String value) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_themeKey, value);
  }
}
