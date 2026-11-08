import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'src/app.dart';
import 'src/auth/storage.dart';
import 'src/config.dart';
import 'src/telemetry/telemetry.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const ProviderScope(child: ShelfApp()));
  // Telemetry must never delay the first frame or fail startup when offline.
  unawaited(_startTelemetry());
}

/// Resolves the saved (or default) server address, then asks it for the OTLP
/// endpoint. Failures leave the exporter on its compile-time configuration.
Future<void> _startTelemetry() async {
  String? saved;
  try {
    saved = await AuthStorage().readServer();
  } catch (_) {
    saved = null;
  }
  await Telemetry.instance.startFromServer(resolveInitialServerUrl(saved));
}
