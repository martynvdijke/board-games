import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'auth/auth_controller.dart';
import 'screens/home_shell.dart';
import 'screens/onboarding_screen.dart';
import 'theme/shelf_theme.dart';
import 'theme/theme_controller.dart';

class ShelfApp extends ConsumerWidget {
  const ShelfApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authControllerProvider);
    final mode = ref.watch(themeControllerProvider);
    return MaterialApp(
      title: 'The Shelf',
      debugShowCheckedModeBanner: false,
      theme: ShelfTheme.light(),
      darkTheme: ShelfTheme.dark(),
      themeMode: mode,
      home: switch (auth.status) {
        AuthStatus.loading => const _SplashScreen(),
        AuthStatus.unauthenticated => const OnboardingScreen(),
        AuthStatus.authenticated => const HomeShell(),
      },
    );
  }
}

class _SplashScreen extends StatelessWidget {
  const _SplashScreen();

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 72,
              height: 72,
              decoration: BoxDecoration(
                color: scheme.primary,
                borderRadius: BorderRadius.circular(20),
              ),
              alignment: Alignment.center,
              child: Text(
                'S',
                style: Theme.of(context).textTheme.displaySmall?.copyWith(
                  color: scheme.onPrimary,
                  fontFamily: 'Newsreader',
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
            const SizedBox(height: 20),
            Text('The Shelf', style: Theme.of(context).textTheme.headlineSmall),
            const SizedBox(height: 8),
            Text(
              'Opening the curtains…',
              style: Theme.of(context).textTheme.bodyMedium,
            ),
            const SizedBox(height: 24),
            SizedBox(
              width: 20,
              height: 20,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                color: scheme.primary,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
