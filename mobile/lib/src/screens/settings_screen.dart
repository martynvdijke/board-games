import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/api_exception.dart';
import '../api/models.dart';
import '../auth/auth_controller.dart';
import '../providers.dart';
import '../telemetry/telemetry.dart';
import '../theme/theme_controller.dart';
import '../widgets/common.dart';

const appVersion = String.fromEnvironment('APP_VERSION', defaultValue: '1.0.0');

class SettingsScreen extends ConsumerStatefulWidget {
  const SettingsScreen({super.key});

  @override
  ConsumerState<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends ConsumerState<SettingsScreen> {
  bool _syncing = false;
  SyncRun? _run;
  String? _syncError;

  @override
  void initState() {
    super.initState();
    Telemetry.instance.screen('settings');
    _loadLatest();
  }

  Future<void> _loadLatest() async {
    try {
      final run = await ref.read(apiClientProvider).sync();
      if (mounted) setState(() => _run = run);
    } catch (_) {
      // Settings still works if sync history is unavailable.
    }
  }

  Future<void> _startSync() async {
    if (_syncing) return;
    setState(() {
      _syncing = true;
      _syncError = null;
    });
    Telemetry.instance.track('sync_started');
    try {
      final started = await ref.read(apiClientProvider).startSync();
      if (mounted) setState(() => _run = started);
      for (var attempt = 0; attempt < 45; attempt++) {
        await Future<void>.delayed(const Duration(seconds: 2));
        if (!mounted) return;
        final run = await ref.read(apiClientProvider).sync();
        if (mounted) setState(() => _run = run);
        if (run != null && !run.isRunning) break;
      }
      ref.invalidate(statsProvider(DateTime.now().year));
      ref.invalidate(gamesProvider);
    } on ApiException catch (error) {
      if (mounted) {
        setState(
          () => _syncError = error.code == 'sync_running'
              ? 'A sync is already running.'
              : error.message,
        );
      }
    } catch (error) {
      if (mounted) setState(() => _syncError = error.toString());
    } finally {
      if (mounted) setState(() => _syncing = false);
    }
  }

  Future<void> _signOut() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Sign out?'),
        content: const Text(
          'The server URL and token will be removed from this device.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Stay'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Sign out'),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      await ref.read(authControllerProvider.notifier).signOut();
    }
  }

  @override
  Widget build(BuildContext context) {
    final profile = ref.watch(authProfileProvider);
    final auth = ref.watch(authControllerProvider);
    final mode = ref.watch(themeControllerProvider);
    final scheme = Theme.of(context).colorScheme;
    final run = _run;
    return Scaffold(
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 24, 20, 120),
          children: [
            const Eyebrow('THE BACK ROOM'),
            const SizedBox(height: 8),
            Text('Settings.', style: Theme.of(context).textTheme.displaySmall),
            const SizedBox(height: 8),
            Text(
              'Your shelf, your account, your rules.',
              style: Theme.of(context).textTheme.bodyMedium
                  ?.copyWith(color: scheme.onSurfaceVariant),
            ),
            const SizedBox(height: 24),
            Container(
              padding: const EdgeInsets.all(20),
              decoration: BoxDecoration(
                color: scheme.surface,
                borderRadius: BorderRadius.circular(20),
                border: Border.all(color: scheme.outlineVariant),
              ),
              child: Row(
                children: [
                  CircleAvatar(
                    radius: 26,
                    backgroundColor: scheme.primary,
                    foregroundColor: scheme.onPrimary,
                    child: Text(
                      (profile?.name.isNotEmpty ?? false)
                          ? profile!.name.characters.first.toUpperCase()
                          : '?',
                      style: Theme.of(context).textTheme.titleLarge
                          ?.copyWith(color: scheme.onPrimary),
                    ),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          profile?.name ?? 'Shelf owner',
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        const SizedBox(height: 2),
                        Text(
                          profile?.email ?? '',
                          style: Theme.of(context).textTheme.bodySmall
                              ?.copyWith(color: scheme.onSurfaceVariant),
                        ),
                      ],
                    ),
                  ),
                  if (profile != null) MetaChip(profile.role),
                ],
              ),
            ),
            const SizedBox(height: 24),
            const SectionHeader(title: 'Appearance'),
            const SizedBox(height: 10),
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: scheme.surface,
                borderRadius: BorderRadius.circular(20),
                border: Border.all(color: scheme.outlineVariant),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    'Warm paper by day, deep ink by night.',
                    style: Theme.of(context).textTheme.bodySmall
                        ?.copyWith(color: scheme.onSurfaceVariant),
                  ),
                  const SizedBox(height: 12),
                  SegmentedButton<ThemeMode>(
                    segments: const [
                      ButtonSegment(
                        value: ThemeMode.system,
                        icon: Icon(Icons.brightness_auto_outlined),
                        label: Text('System'),
                      ),
                      ButtonSegment(
                        value: ThemeMode.light,
                        icon: Icon(Icons.light_mode_outlined),
                        label: Text('Light'),
                      ),
                      ButtonSegment(
                        value: ThemeMode.dark,
                        icon: Icon(Icons.dark_mode_outlined),
                        label: Text('Dark'),
                      ),
                    ],
                    selected: {mode},
                    showSelectedIcon: false,
                    onSelectionChanged: (value) => ref
                        .read(themeControllerProvider.notifier)
                        .set(value.first),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 24),
            const SectionHeader(title: 'BoardGameGeek'),
            const SizedBox(height: 10),
            Container(
              padding: const EdgeInsets.all(18),
              decoration: BoxDecoration(
                color: scheme.surface,
                borderRadius: BorderRadius.circular(20),
                border: Border.all(color: scheme.outlineVariant),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          run == null
                              ? 'Never synced'
                              : 'Last sync · ${run.status}',
                          style: Theme.of(context).textTheme.titleSmall,
                        ),
                      ),
                      if (run != null)
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 8,
                            vertical: 4,
                          ),
                          decoration: BoxDecoration(
                            color: run.isRunning
                                ? scheme.primary.withValues(alpha: 0.12)
                                : scheme.surfaceContainerHighest,
                            borderRadius: BorderRadius.circular(20),
                          ),
                          child: Text(
                            run.isRunning
                                ? 'Running'
                                : (run.status == 'success'
                                      ? 'Up to date'
                                      : run.status),
                            style: Theme.of(context).textTheme.labelSmall,
                          ),
                        ),
                    ],
                  ),
                  if (run != null) ...[
                    const SizedBox(height: 8),
                    Text(
                      run.summary,
                      style: Theme.of(context).textTheme.bodySmall
                          ?.copyWith(color: scheme.onSurfaceVariant),
                    ),
                    if (run.phase.isNotEmpty)
                      Text(
                        'Phase: ${run.phase}',
                        style: Theme.of(context).textTheme.bodySmall
                            ?.copyWith(color: scheme.onSurfaceVariant),
                      ),
                    if (run.error.isNotEmpty) ...[
                      const SizedBox(height: 8),
                      Text(
                        run.error,
                        style: Theme.of(context).textTheme.bodySmall
                            ?.copyWith(color: scheme.error),
                      ),
                    ],
                  ],
                  if (_syncError != null) ...[
                    const SizedBox(height: 8),
                    Text(
                      _syncError!,
                      style: Theme.of(context).textTheme.bodySmall
                          ?.copyWith(color: scheme.error),
                    ),
                  ],
                  const SizedBox(height: 14),
                  FilledButton.icon(
                    onPressed: _syncing ? null : _startSync,
                    icon: _syncing
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.sync),
                    label: Text(_syncing ? 'Syncing…' : 'Sync now'),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Read-only. Plays and ratings you add here stay local.',
                    style: Theme.of(context).textTheme.labelSmall
                        ?.copyWith(color: scheme.onSurfaceVariant),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 24),
            const SectionHeader(title: 'Connection'),
            const SizedBox(height: 10),
            Container(
              padding: const EdgeInsets.all(18),
              decoration: BoxDecoration(
                color: scheme.surface,
                borderRadius: BorderRadius.circular(20),
                border: Border.all(color: scheme.outlineVariant),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _InfoRow(label: 'Server', value: auth.serverUrl),
                  const Divider(height: 24),
                  _InfoRow(label: 'Token', value: _maskedToken(auth.token)),
                  const Divider(height: 24),
                  _InfoRow(
                    label: 'Scopes',
                    value: (profile?.scopes ?? const []).join(', '),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 12),
            OutlinedButton.icon(
              onPressed: _signOut,
              icon: const Icon(Icons.logout),
              label: const Text('Sign out'),
            ),
            const SizedBox(height: 28),
            Center(
              child: Text(
                'The Shelf · Android $appVersion',
                style: Theme.of(context).textTheme.labelSmall
                    ?.copyWith(color: scheme.onSurfaceVariant),
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _maskedToken(String token) {
    if (token.length <= 12) return '••••';
    return '${token.substring(0, 7)}…${token.substring(token.length - 4)}';
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 76,
          child: Text(
            label,
            style: Theme.of(context).textTheme.bodySmall
                ?.copyWith(color: scheme.onSurfaceVariant),
          ),
        ),
        Expanded(
          child: Text(
            value.isEmpty ? 'Not set' : value,
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ),
      ],
    );
  }
}
