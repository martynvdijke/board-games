import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../auth/auth_controller.dart';
import '../theme/theme_controller.dart';
import '../theme/tokens.dart';
import '../widgets/common.dart';

class OnboardingScreen extends ConsumerStatefulWidget {
  const OnboardingScreen({super.key});

  @override
  ConsumerState<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends ConsumerState<OnboardingScreen> {
  late final TextEditingController _server;
  final _token = TextEditingController();
  bool _busy = false;
  bool _tokenVisible = false;

  @override
  void initState() {
    super.initState();
    _server = TextEditingController(
      text: ref.read(authControllerProvider).serverUrl,
    );
  }

  @override
  void dispose() {
    _server.dispose();
    _token.dispose();
    super.dispose();
  }

  Future<void> _connect() async {
    FocusScope.of(context).unfocus();
    setState(() => _busy = true);
    final ok = await ref
        .read(authControllerProvider.notifier)
        .signIn(_server.text, _token.text);
    if (mounted) {
      setState(() => _busy = false);
      if (!ok) {
        final message = ref.read(authControllerProvider).error;
        if (message.isNotEmpty) showShelfSnack(context, message, error: true);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final auth = ref.watch(authControllerProvider);
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Scaffold(
      body: LayoutBuilder(
        builder: (context, constraints) {
          final wide = constraints.maxWidth > 700;
          final heroHeight = wide
              ? 330.0
              : (constraints.maxHeight * .44).clamp(280.0, 390.0);
          return SingleChildScrollView(
            child: ConstrainedBox(
              constraints: BoxConstraints(minHeight: constraints.maxHeight),
              child: Column(
                children: [
                  Container(
                    height: heroHeight,
                    width: double.infinity,
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                        colors: isDark
                            ? [const Color(0xFF1E3A2F), ShelfColors.night]
                            : [ShelfColors.greenDeep, ShelfColors.green],
                      ),
                    ),
                    child: Stack(
                      children: [
                        Positioned(
                          right: -70,
                          bottom: -90,
                          child: Container(
                            width: 240,
                            height: 240,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              border: Border.all(
                                color: Colors.white.withValues(alpha: .12),
                                width: 1,
                              ),
                            ),
                          ),
                        ),
                        Positioned(
                          left: -40,
                          top: 70,
                          child: Container(
                            width: 130,
                            height: 130,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              color: ShelfColors.brass.withValues(alpha: .12),
                            ),
                          ),
                        ),
                        SafeArea(
                          child: Padding(
                            padding: EdgeInsets.symmetric(
                              horizontal: wide ? 48 : 24,
                              vertical: 12,
                            ),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(
                                  children: [
                                    const ShelfLogo(size: 40, onDark: true),
                                    const SizedBox(width: 12),
                                    Text(
                                      'The Shelf',
                                      style: TextStyle(
                                        fontFamily: 'Newsreader',
                                        fontSize: 24,
                                        fontWeight: FontWeight.w600,
                                        color: ShelfColors.cream,
                                      ),
                                    ),
                                    const Spacer(),
                                    IconButton(
                                      tooltip: isDark
                                          ? 'Use light theme'
                                          : 'Use dark theme',
                                      onPressed: () => ref
                                          .read(
                                            themeControllerProvider.notifier,
                                          )
                                          .set(
                                            isDark
                                                ? ThemeMode.light
                                                : ThemeMode.dark,
                                          ),
                                      icon: Icon(
                                        isDark
                                            ? Icons.light_mode_outlined
                                            : Icons.dark_mode_outlined,
                                        color: ShelfColors.cream.withValues(
                                          alpha: .85,
                                        ),
                                      ),
                                    ),
                                  ],
                                ),
                                const Spacer(),
                                Text(
                                  'YOUR COLLECTION, CONSIDERED',
                                  style: TextStyle(
                                    color: ShelfColors.cream.withValues(
                                      alpha: .62,
                                    ),
                                    fontSize: 10,
                                    letterSpacing: 2,
                                    fontWeight: FontWeight.w600,
                                  ),
                                ),
                                const SizedBox(height: 10),
                                Text(
                                  'Every game.\nOne quiet shelf.',
                                  style: TextStyle(
                                    fontFamily: 'Newsreader',
                                    fontSize: wide ? 50 : 40,
                                    height: 1.02,
                                    letterSpacing: -.6,
                                    fontWeight: FontWeight.w500,
                                    color: ShelfColors.cream,
                                  ),
                                ),
                                const SizedBox(height: 12),
                                Text(
                                  'The games you own, the plays you remember, and the ones still waiting for their first outing.',
                                  style: TextStyle(
                                    color: ShelfColors.cream.withValues(
                                      alpha: .76,
                                    ),
                                    fontSize: 13.5,
                                    height: 1.4,
                                  ),
                                ),
                                const SizedBox(height: 16),
                              ],
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                  Container(
                    width: double.infinity,
                    transform: Matrix4.translationValues(0, -22, 0),
                    padding: EdgeInsets.fromLTRB(
                      wide ? 48 : 22,
                      28,
                      wide ? 48 : 22,
                      28,
                    ),
                    decoration: BoxDecoration(
                      color: scheme.surface,
                      borderRadius: const BorderRadius.vertical(
                        top: Radius.circular(28),
                      ),
                    ),
                    child: Center(
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 460),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Eyebrow('Welcome back'),
                            const SizedBox(height: 8),
                            Text(
                              'Sign in to your shelf',
                              style: Theme.of(context).textTheme.headlineMedium,
                            ),
                            const SizedBox(height: 8),
                            Text(
                              'Use the address of your self-hosted Shelf and a personal API token.',
                              style: Theme.of(context).textTheme.bodyMedium
                                  ?.copyWith(
                                    color: scheme.onSurfaceVariant,
                                    height: 1.45,
                                  ),
                            ),
                            const SizedBox(height: 24),
                            TextField(
                              controller: _server,
                              keyboardType: TextInputType.url,
                              textInputAction: TextInputAction.next,
                              autocorrect: false,
                              decoration: const InputDecoration(
                                labelText: 'Shelf address',
                                hintText: 'https://shelf.example.com',
                                prefixIcon: Icon(Icons.public_outlined),
                              ),
                            ),
                            const SizedBox(height: 14),
                            TextField(
                              controller: _token,
                              obscureText: !_tokenVisible,
                              autocorrect: false,
                              enableSuggestions: false,
                              decoration: InputDecoration(
                                labelText: 'Personal API token',
                                hintText: 'bg_…',
                                prefixIcon: const Icon(Icons.key_outlined),
                                suffixIcon: IconButton(
                                  onPressed: () => setState(
                                    () => _tokenVisible = !_tokenVisible,
                                  ),
                                  icon: Icon(
                                    _tokenVisible
                                        ? Icons.visibility_off_outlined
                                        : Icons.visibility_outlined,
                                  ),
                                  tooltip: _tokenVisible
                                      ? 'Hide token'
                                      : 'Show token',
                                ),
                              ),
                              onSubmitted: (_) => _busy ? null : _connect(),
                            ),
                            if (auth.error.isNotEmpty) ...[
                              const SizedBox(height: 14),
                              ErrorView(message: auth.error),
                            ],
                            const SizedBox(height: 20),
                            SizedBox(
                              width: double.infinity,
                              child: FilledButton(
                                onPressed: _busy ? null : _connect,
                                child: _busy
                                    ? const SizedBox(
                                        width: 18,
                                        height: 18,
                                        child: CircularProgressIndicator(
                                          strokeWidth: 2,
                                        ),
                                      )
                                    : const Row(
                                        mainAxisAlignment:
                                            MainAxisAlignment.center,
                                        children: [
                                          Text('Connect to my shelf'),
                                          SizedBox(width: 8),
                                          Icon(Icons.arrow_forward, size: 18),
                                        ],
                                      ),
                              ),
                            ),
                            const SizedBox(height: 18),
                            Container(
                              padding: const EdgeInsets.all(14),
                              decoration: BoxDecoration(
                                color: scheme.surfaceContainerLow,
                                borderRadius: BorderRadius.circular(14),
                                border: Border.all(
                                  color: scheme.outlineVariant,
                                ),
                              ),
                              child: Row(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Icon(
                                    Icons.lock_outline,
                                    size: 16,
                                    color: scheme.onSurfaceVariant,
                                  ),
                                  const SizedBox(width: 10),
                                  Expanded(
                                    child: Text(
                                      'Create a token on the web under Settings → API tokens. The token is stored in Android secure storage and never leaves your device except to your own server.',
                                      style: Theme.of(context)
                                          .textTheme
                                          .bodySmall
                                          ?.copyWith(
                                            color: scheme.onSurfaceVariant,
                                            height: 1.45,
                                          ),
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }
}
