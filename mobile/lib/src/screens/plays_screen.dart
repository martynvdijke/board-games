import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../providers.dart';
import '../utils.dart';
import '../widgets/common.dart';
import '../widgets/game_picker.dart';
import '../widgets/play_tile.dart';
import 'game_detail_screen.dart';
import 'play_form_screen.dart';

class PlaysScreen extends ConsumerStatefulWidget {
  const PlaysScreen({super.key, this.gameId, this.title = 'Play log'});

  final int? gameId;
  final String title;

  @override
  ConsumerState<PlaysScreen> createState() => _PlaysScreenState();
}

class _PlaysScreenState extends ConsumerState<PlaysScreen> {
  int? _year;

  Future<void> _refresh() async {
    try {
      final query = PlayQuery(gameId: widget.gameId, year: _year);
      ref.invalidate(playsProvider(query));
      await ref.read(playsProvider(query).future);
    } catch (_) {}
  }

  Future<void> _newPlay() async {
    final game = await showGamePicker(context);
    if (game == null || !mounted) return;
    final saved = await Navigator.of(
      context,
    ).push<bool>(MaterialPageRoute(builder: (_) => PlayFormScreen(game: game)));
    if (saved == true) _afterChange();
  }

  Future<void> _editPlay(Play play) async {
    final detail = await ref.read(gameProvider(play.gameId).future);
    if (!mounted) return;
    final saved = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => PlayFormScreen(game: detail.game, play: play),
      ),
    );
    if (saved == true) _afterChange();
  }

  void _afterChange() {
    ref.invalidate(playsProvider);
    ref.invalidate(statsProvider);
    ref.invalidate(gamesProvider);
  }

  Future<void> _deletePlay(Play play) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete this play?'),
        content: Text(
          '${play.gameName} · ${play.whenLabel}. This cannot be undone.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(apiClientProvider).deletePlay(play.id);
      _afterChange();
      if (mounted) showShelfSnack(context, 'Play deleted.');
    } catch (error) {
      if (mounted) showShelfSnack(context, '$error', error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final query = PlayQuery(gameId: widget.gameId, year: _year);
    final plays = ref.watch(playsProvider(query));
    final thisYear = DateTime.now().year;
    final years = [for (var i = 0; i < 6; i++) thisYear - i];
    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: RefreshIndicator(
          onRefresh: _refresh,
          child: CustomScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            slivers: [
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(20, 16, 20, 8),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Eyebrow('Dice, cards, and good company'),
                      const SizedBox(height: 6),
                      Text(
                        '${widget.title}.',
                        style: TextStyle(
                          fontFamily: 'Newsreader',
                          fontSize: 40,
                          height: 1,
                          letterSpacing: -.7,
                          fontWeight: FontWeight.w500,
                          color: scheme.onSurface,
                        ),
                      ),
                      const SizedBox(height: 10),
                      Text(
                        plays.value == null
                            ? 'Every session worth remembering.'
                            : '${plays.value!.total} ${plays.value!.total == 1 ? 'play' : 'plays'} on the record.',
                        style: Theme.of(context).textTheme.bodyMedium
                            ?.copyWith(color: scheme.onSurfaceVariant),
                      ),
                      const SizedBox(height: 16),
                      Row(
                        children: [
                          ChoiceChip(
                            label: const Text('All years'),
                            selected: _year == null,
                            onSelected: (_) => setState(() => _year = null),
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: SingleChildScrollView(
                              scrollDirection: Axis.horizontal,
                              child: Row(
                                children: [
                                  for (final year in years)
                                    Padding(
                                      padding: const EdgeInsets.only(right: 8),
                                      child: ChoiceChip(
                                        label: Text('$year'),
                                        selected: _year == year,
                                        onSelected: (_) =>
                                            setState(() => _year = year),
                                      ),
                                    ),
                                ],
                              ),
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
              ...switch (plays) {
                AsyncData(:final value) when value.items.isEmpty => [
                  SliverPadding(
                    padding: const EdgeInsets.fromLTRB(20, 20, 20, 32),
                    sliver: SliverToBoxAdapter(
                      child: EmptyState(
                        title: 'No plays here yet',
                        message: widget.gameId == null
                            ? 'The first game night is the beginning of the story.'
                            : 'This one is still waiting for its first outing.',
                        icon: Icons.casino_outlined,
                        actionLabel: 'Log a play',
                        onAction: _newPlay,
                      ),
                    ),
                  ),
                ],
                AsyncData(:final value) => _groupedSlivers(value.items),
                AsyncError(:final error) => [
                  SliverPadding(
                    padding: const EdgeInsets.all(20),
                    sliver: SliverToBoxAdapter(
                      child: ErrorView(
                        message: '$error',
                        onRetry: () => ref.invalidate(playsProvider(query)),
                      ),
                    ),
                  ),
                ],
                _ => [
                  const SliverFillRemaining(
                    hasScrollBody: false,
                    child: Center(child: CircularProgressIndicator()),
                  ),
                ],
              },
            ],
          ),
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _newPlay,
        icon: const Icon(Icons.add),
        label: const Text('Log a play'),
      ),
    );
  }

  List<Widget> _groupedSlivers(List<Play> items) {
    final groups = <String, List<Play>>{};
    for (final play in items) {
      groups.putIfAbsent(monthLabel(play.playedAt), () => []).add(play);
    }
    final slivers = <Widget>[];
    groups.forEach((month, entries) {
      slivers.add(
        SliverToBoxAdapter(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 18, 20, 8),
            child: Row(
              children: [
                Text(month, style: Theme.of(context).textTheme.titleMedium),
                const SizedBox(width: 8),
                Expanded(
                  child: Divider(
                    color: Theme.of(context).colorScheme.outlineVariant,
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  '${entries.length}',
                  style: Theme.of(context).textTheme.labelSmall?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
        ),
      );
      slivers.add(
        SliverPadding(
          padding: const EdgeInsets.symmetric(horizontal: 20),
          sliver: SliverList.separated(
            itemCount: entries.length,
            separatorBuilder: (_, _) => const SizedBox(height: 8),
            itemBuilder: (context, index) {
              final play = entries[index];
              return PlayTile(
                play: play,
                onTap: () => Navigator.of(context).push(
                  MaterialPageRoute(
                    builder: (_) => GameDetailScreen(gameId: play.gameId),
                  ),
                ),
                onEdit: play.isLocal ? () => _editPlay(play) : null,
                onDelete: play.isLocal ? () => _deletePlay(play) : null,
              );
            },
          ),
        ),
      );
    });
    slivers.add(const SliverToBoxAdapter(child: SizedBox(height: 100)));
    return slivers;
  }
}
