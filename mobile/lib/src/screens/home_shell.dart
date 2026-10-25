import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../telemetry/telemetry.dart';
import '../widgets/common.dart';
import '../widgets/game_picker.dart';
import 'play_form_screen.dart';
import 'plays_screen.dart';
import 'settings_screen.dart';
import 'shelf_screen.dart';
import 'stats_screen.dart';

class HomeShell extends ConsumerStatefulWidget {
  const HomeShell({super.key});

  @override
  ConsumerState<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends ConsumerState<HomeShell> {
  int _index = 0;

  static const _screens = <Widget>[
    ShelfScreen(),
    PlaysScreen(),
    StatsScreen(),
    SettingsScreen(),
  ];

  static const _names = ['shelf', 'plays', 'stats', 'settings'];

  @override
  void initState() {
    super.initState();
    Telemetry.instance.screen('shelf');
  }

  void _select(int index) {
    if (_index == index) return;
    setState(() => _index = index);
    Telemetry.instance.screen(_names[index]);
  }

  Future<void> _logPlay() async {
    final game = await showGamePicker(context);
    if (game == null || !mounted) return;
    await _openPlayForm(game);
  }

  Future<void> _openPlayForm(Game game, {Play? play}) async {
    final saved = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => PlayFormScreen(game: game, play: play),
      ),
    );
    if (saved == true && mounted) {
      showShelfSnack(context, play == null ? 'Play logged.' : 'Play updated.');
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      body: IndexedStack(index: _index, children: _screens),
      floatingActionButton: _index == 0 || _index == 3
          ? null
          : FloatingActionButton.extended(
              onPressed: _logPlay,
              backgroundColor: scheme.primary,
              foregroundColor: scheme.onPrimary,
              icon: const Icon(Icons.casino_outlined),
              label: const Text('Log a play'),
            ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: _select,
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.grid_view_outlined),
            selectedIcon: Icon(Icons.grid_view),
            label: 'Shelf',
          ),
          NavigationDestination(
            icon: Icon(Icons.history_outlined),
            selectedIcon: Icon(Icons.history),
            label: 'Plays',
          ),
          NavigationDestination(
            icon: Icon(Icons.insights_outlined),
            selectedIcon: Icon(Icons.insights),
            label: 'Stats',
          ),
          NavigationDestination(
            icon: Icon(Icons.tune_outlined),
            selectedIcon: Icon(Icons.tune),
            label: 'Settings',
          ),
        ],
      ),
    );
  }
}
