class ApiException implements Exception {
  const ApiException(
    this.message, {
    this.statusCode,
    this.code,
    this.fields = const {},
  });

  final String message;
  final int? statusCode;
  final String? code;
  final Map<String, String> fields;

  bool get isUnauthorized => statusCode == 401;
  bool get isConflict => statusCode == 409;

  @override
  String toString() => message;
}
