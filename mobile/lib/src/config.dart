/// The hosted Board Games instance the app connects to on first run.
const String defaultServerUrl = 'https://board-games.vandijke.xyz';

/// Returns the server URL to prefill, preferring a previously saved address.
String resolveInitialServerUrl(String? saved) {
  final value = saved?.trim() ?? '';
  return value.isEmpty ? defaultServerUrl : value;
}
