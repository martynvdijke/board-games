import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../providers.dart';
import '../telemetry/telemetry.dart';
import '../theme/tokens.dart';
import '../widgets/common.dart';
import '../widgets/game_card.dart';
import '../widgets/game_picker.dart';
import 'game_detail_screen.dart';
import 'play_form_screen.dart';

class ShelfScreen extends ConsumerStatefulWidget {
  const ShelfScreen({super.key});

  @override
  ConsumerState<ShelfScreen> createState() => _ShelfScreenState();
}

class _ShelfScreenState extends ConsumerState<ShelfScreen> {
  final _search = TextEditingController();
  Timer? _debounce;
  String _status = '';
  String _sort = 'name';
  String _query = '';

  @override
  void dispose() {
    _debounce?.cancel();
    _search.dispose();
    super.dispose();
  }

  Future<void> _logPlay() async {
    final game = await showGamePicker(context);
    if (game == null || !mounted) return;
    final saved = await Navigator.of(
      context,
    ).push<bool>(MaterialPageRoute(builder: (_) => PlayFormScreen(game: game)));
    if (saved == true && mounted) showShelfSnack(context, 'Play logged.');
  }

  void _onSearch(String value) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 320), () {
      if (!mounted) return;
      setState(() => _query = value.trim());
      if (_query.isNotEmpty) {
        Telemetry.instance.track('shelf_search', {'has_query': true});
      }
    });
  }

  Future<void> _refresh() async {
    final query = GameQuery(search: _query, status: _status, sort: _sort);
    try {
      ref.invalidate(gamesProvider(query));
      await ref.read(gamesProvider(query).future);
      ref.invalidate(statsProvider(DateTime.now().year));
    } catch (_) {}
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final query = GameQuery(search: _query, status: _status, sort: _sort);
    final games = ref.watch(gamesProvider(query));
    final stats = ref.watch(statsProvider(DateTime.now().year)).value;
    final profile = ref.watch(authProfileProvider);
    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: RefreshIndicator(
          onRefresh: _refresh,
          color: scheme.primary,
          child: CustomScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            slivers: [
              SliverToBoxAdapter(
                child: _Header(
                  stats: stats,
                  initials: profile?.initials ?? 'S',
                  name: profile?.name ?? 'Reader',
                  onLogPlay: _logPlay,
                ),
              ),
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(20, 8, 20, 0),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      TextField(
                        controller: _search,
                        onChanged: _onSearch,
                        textInputAction: TextInputAction.search,
                        decoration: InputDecoration(
                          hintText: 'Search games, designers, mechanics…',
                          prefixIcon: const Icon(Icons.search),
                          suffixIcon: _search.text.isEmpty
                              ? null
                              : IconButton(
                                  tooltip: 'Clear search',
                                  icon: const Icon(Icons.close, size: 18),
                                  onPressed: () {
                                    _search.clear();
                                    _onSearch('');
                                  },
                                ),
                        ),
                      ),
                      const SizedBox(height: 14),
                      SingleChildScrollView(
                        scrollDirection: Axis.horizontal,
                        child: Row(
                          children: [
                            for (final status in const [
                              ('', 'All'),
                              ('unplayed', 'Unplayed'),
                              ('wishlist', 'Wishlist'),
                              ('favorite', 'Favourites'),
                            ])
                              Padding(
                                padding: const EdgeInsets.only(right: 8),
                                child: ChoiceChip(
                                  label: Text(status.$2),
                                  selected: _status == status.$1,
                                  onSelected: (_) =>
                                      setState(() => _status = status.$1),
                                ),
                              ),
                          ],
                        ),
                      ),
                      const SizedBox(height: 10),
                      Row(
                        children: [
                          Text(
                            games.value == null
                                ? 'Loading the shelf…'
                                : '${games.value!.total} ${games.value!.total == 1 ? 'game' : 'games'}${_query.isNotEmpty || _status.isNotEmpty ? ' matching' : ''}',
                            style: Theme.of(context).textTheme.labelMedium
                                ?.copyWith(color: scheme.onSurfaceVariant),
                          ),
                          const Spacer(),
                          PopupMenuButton<String>(
                            initialValue: _sort,
                            tooltip: 'Sort games',
                            onSelected: (value) =>
                                setState(() => _sort = value),
                            itemBuilder: (_) => const [
                              PopupMenuItem(value: 'name', child: Text('A–Z')),
                              PopupMenuItem(
                                value: 'rating',
                                child: Text('My rating'),
                              ),
                              PopupMenuItem(
                                value: 'added',
                                child: Text('Recently added'),
                              ),
                              PopupMenuItem(
                                value: 'plays',
                                child: Text('Most played'),
                              ),
                            ],
                            child: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Text(
                                  'Sort',
                                  style: Theme.of(context)
                                      .textTheme
                                      .labelMedium,
                                ),
                                const SizedBox(width: 2),
                                const Icon(Icons.expand_more, size: 17),
                              ],
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 10),
                    ],
                  ),
                ),
              ),
              ...switch (games) {
                AsyncData(:final value) when value.items.isEmpty => [
                  SliverPadding(
                    padding: const EdgeInsets.fromLTRB(20, 10, 20, 28),
                    sliver: SliverToBoxAdapter(
                      child: EmptyState(
                        title: _query.isNotEmpty || _status.isNotEmpty
                            ? 'No games fit that combination'
                            : 'Nothing here yet',
                        message: _query.isNotEmpty || _status.isNotEmpty
                            ? 'Try loosening a filter, or clear the search and browse the whole shelf.'
                            : 'Sync your BoardGameGeek collection from the web, or log a game manually.',
                        icon: Icons.search_off,
                        actionLabel: _query.isNotEmpty || _status.isNotEmpty
                            ? 'Clear filters'
                            : null,
                        onAction: () {
                          _search.clear();
                          setState(() {
                            _query = '';
                            _status = '';
                          });
                        },
                      ),
                    ),
                  ),
                ],
                AsyncData(:final value) => [
                  SliverPadding(
                    padding: const EdgeInsets.fromLTRB(20, 6, 20, 32),
                    sliver: SliverLayoutBuilder(
                      builder: (context, constraints) {
                        final width = constraints.crossAxisExtent;
                        final columns = width > 900
                            ? 5
                            : (width > 700 ? 4 : (width > 440 ? 3 : 2));
                        return SliverGrid(
                          gridDelegate:
                              SliverGridDelegateWithFixedCrossAxisCount(
                                crossAxisCount: columns,
                                crossAxisSpacing: 14,
                                mainAxisSpacing: 20,
                                childAspectRatio: columns >= 4 ? .56 : .55,
                              ),
                          delegate: SliverChildBuilderDelegate((
                            context,
                            index,
                          ) {
                            final game = value.items[index];
                            return GameCard(
                              game: game,
                              onTap: () => Navigator.of(context).push(
                                MaterialPageRoute(
                                  builder: (_) => GameDetailScreen(
                                    gameId: game.id,
                                    initial: game,
                                  ),
                                ),
                              ),
                            );
                          }, childCount: value.items.length),
                        );
                      },
                    ),
                  ),
                ],
                AsyncError(:final error) => [
                  SliverPadding(
                    padding: const EdgeInsets.all(20),
                    sliver: SliverToBoxAdapter(
                      child: ErrorView(
                        message: '$error',
                        onRetry: () => ref.invalidate(gamesProvider(query)),
                      ),
                    ),
                  ),
                ],
                _ => [
                  SliverPadding(
                    padding: const EdgeInsets.fromLTRB(20, 20, 20, 32),
                    sliver: SliverToBoxAdapter(
                      child: GridView.builder(
                        shrinkWrap: true,
                        physics: const NeverScrollableScrollPhysics(),
                        itemCount: 6,
                        gridDelegate:
                            const SliverGridDelegateWithFixedCrossAxisCount(
                              crossAxisCount: 2,
                              crossAxisSpacing: 14,
                              mainAxisSpacing: 20,
                              childAspectRatio: .55,
                            ),
                        itemBuilder: (_, _) => Column(
                          children: [
                            Expanded(
                              child: Container(
                                decoration: BoxDecoration(
                                  color: scheme.surfaceContainerHigh,
                                  borderRadius: BorderRadius.circular(16),
                                ),
                              ),
                            ),
                            const SizedBox(height: 10),
                            Container(
                              height: 12,
                              color: scheme.surfaceContainerHigh,
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                ],
              },
            ],
          ),
        ),
      ),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({
    required this.stats,
    required this.initials,
    required this.name,
    required this.onLogPlay,
  });

  final Stats? stats;
  final String initials;
  final String name;
  final VoidCallback onLogPlay;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final owned = stats?.shelf.owned ?? 0;
    final unplayed = stats?.shelf.unplayed ?? 0;
    final thisYear = stats?.shelf.playsThisYear ?? 0;
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 14, 20, 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const ShelfLogo(size: 36),
              const SizedBox(width: 10),
              Text(
                'The Shelf',
                style: TextStyle(
                  fontFamily: 'Newsreader',
                  fontSize: 22,
                  fontWeight: FontWeight.w600,
                  color: scheme.onSurface,
                ),
              ),
              const Spacer(),
              IconButton.filledTonal(
                onPressed: onLogPlay,
                tooltip: 'Log a play',
                icon: const Icon(Icons.casino_outlined, size: 20),
              ),
              const SizedBox(width: 6),
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: scheme.primaryContainer,
                  shape: BoxShape.circle,
                  border: Border.all(color: scheme.outlineVariant),
                ),
                alignment: Alignment.center,
                child: Text(
                  initials,
                  style: Theme.of(context).textTheme.labelMedium?.copyWith(
                    color: scheme.primary,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 26),
          Eyebrow('${greeting()}, ${name.split(' ').first}'),
          const SizedBox(height: 6),
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text(
                'Your shelf',
                style: TextStyle(
                  fontFamily: 'Newsreader',
                  fontSize: 42,
                  height: 1,
                  letterSpacing: -.8,
                  fontWeight: FontWeight.w500,
                  color: scheme.onSurface,
                ),
              ),
              Padding(
                padding: const EdgeInsets.only(bottom: 4, left: 2),
                child: Text(
                  '.',
                  style: TextStyle(
                    fontFamily: 'Newsreader',
                    fontSize: 42,
                    height: 1,
                    color: scheme.secondary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 10),
          Text(
            stats == null
                ? 'A quiet home for the games you own.'
                : '$owned games, $unplayed ${unplayed == 1 ? 'is' : 'are'} still waiting for their first play.',
            style: Theme.of(context).textTheme.bodyMedium
                ?.copyWith(color: scheme.onSurfaceVariant, height: 1.4),
          ),
          const SizedBox(height: 20),
          Container(
            padding: const EdgeInsets.symmetric(vertical: 14),
            decoration: BoxDecoration(
              border: Border(
                top: BorderSide(color: scheme.outlineVariant),
                bottom: BorderSide(color: scheme.outlineVariant),
              ),
            ),
            child: Row(
              children: [
                _Stat(label: 'Owned', value: '$owned'),
                _Stat(
                  label: 'Unplayed',
                  value: '$unplayed',
                  accent: ShelfColors.brass,
                ),
                _Stat(label: 'This year', value: '$thisYear'),
                _Stat(
                  label: 'Wishlist',
                  value: '${stats?.shelf.wishlist ?? 0}',
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Stat extends StatelessWidget {
  const _Stat({required this.label, required this.value, this.accent});

  final String label;
  final String value;
  final Color? accent;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Expanded(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            value,
            style: TextStyle(
              fontFamily: 'Newsreader',
              fontSize: 28,
              height: 1,
              fontWeight: FontWeight.w500,
              color: accent ?? scheme.onSurface,
              fontFeatures: const [FontFeature.tabularFigures()],
            ),
          ),
          const SizedBox(height: 4),
          Text(
            label.toUpperCase(),
            style: Theme.of(context).textTheme.labelSmall?.copyWith(
              fontSize: 9,
              letterSpacing: 1.1,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
