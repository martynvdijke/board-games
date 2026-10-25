import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../providers.dart';
import '../telemetry/telemetry.dart';
import '../theme/tokens.dart';
import '../widgets/common.dart';

class StatsScreen extends ConsumerStatefulWidget {
  const StatsScreen({super.key});

  @override
  ConsumerState<StatsScreen> createState() => _StatsScreenState();
}

class _StatsScreenState extends ConsumerState<StatsScreen> {
  late int _year;

  @override
  void initState() {
    super.initState();
    _year = DateTime.now().year;
    Telemetry.instance.screen('stats');
  }

  @override
  Widget build(BuildContext context) {
    final stats = ref.watch(statsProvider(_year));
    final scheme = Theme.of(context).colorScheme;
    final years = [
      DateTime.now().year,
      DateTime.now().year - 1,
      DateTime.now().year - 2,
    ];
    return Scaffold(
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: () async {
            ref.invalidate(statsProvider(_year));
            await ref.read(statsProvider(_year).future);
          },
          child: ListView(
            padding: const EdgeInsets.fromLTRB(20, 24, 20, 120),
            children: [
              const Eyebrow('THE YEAR IN GAMES'),
              const SizedBox(height: 8),
              Text(
                'Plays in review.',
                style: Theme.of(context).textTheme.displaySmall,
              ),
              const SizedBox(height: 8),
              Text(
                'The table remembers what the calendar forgets.',
                style: Theme.of(context).textTheme.bodyMedium
                    ?.copyWith(color: scheme.onSurfaceVariant),
              ),
              const SizedBox(height: 20),
              Wrap(
                spacing: 8,
                children: years
                    .map(
                      (year) => ChoiceChip(
                        label: Text('$year'),
                        selected: _year == year,
                        onSelected: (_) => setState(() => _year = year),
                      ),
                    )
                    .toList(),
              ),
              const SizedBox(height: 20),
              stats.when(
                loading: () => const Padding(
                  padding: EdgeInsets.symmetric(vertical: 80),
                  child: Center(
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                ),
                error: (error, _) => ErrorView(
                  message: error.toString(),
                  onRetry: () => ref.invalidate(statsProvider(_year)),
                ),
                data: (data) => _StatsBody(stats: data),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _StatsBody extends StatelessWidget {
  const _StatsBody({required this.stats});
  final Stats stats;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final maxMonth = stats.maxMonth == 0 ? 1 : stats.maxMonth;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: _BigStat(label: 'Plays', value: '${stats.totalPlays}'),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: _BigStat(
                label: 'Sessions',
                value: '${stats.totalSessions}',
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: _BigStat(label: 'Owned', value: '${stats.shelf.owned}'),
            ),
          ],
        ),
        const SizedBox(height: 28),
        const SectionHeader(title: 'By month'),
        const SizedBox(height: 12),
        Container(
          height: 220,
          padding: const EdgeInsets.fromLTRB(10, 20, 10, 12),
          decoration: BoxDecoration(
            color: scheme.surface,
            borderRadius: BorderRadius.circular(20),
            border: Border.all(color: scheme.outlineVariant),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: stats.monthly.map((item) {
              final height = item.count == 0
                  ? 3.0
                  : 18 + (120 * item.count / maxMonth);
              return Expanded(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    Text(
                      '${item.count}',
                      style: Theme.of(context).textTheme.labelSmall?.copyWith(
                        fontFeatures: const [FontFeature.tabularFigures()],
                      ),
                    ),
                    const SizedBox(height: 4),
                    AnimatedContainer(
                      duration: const Duration(milliseconds: 300),
                      height: height,
                      margin: const EdgeInsets.symmetric(horizontal: 2),
                      decoration: BoxDecoration(
                        color: item.count == 0
                            ? scheme.outlineVariant
                            : scheme.primary,
                        borderRadius: const BorderRadius.vertical(
                          top: Radius.circular(4),
                        ),
                      ),
                    ),
                    const SizedBox(height: 6),
                    Text(
                      item.label.isEmpty ? '' : item.label.substring(0, 1),
                      style: Theme.of(context).textTheme.labelSmall
                          ?.copyWith(color: scheme.onSurfaceVariant),
                    ),
                  ],
                ),
              );
            }).toList(),
          ),
        ),
        const SizedBox(height: 28),
        if (stats.topGames.isNotEmpty) ...[
          const SectionHeader(title: 'Most played'),
          const SizedBox(height: 10),
          ...stats.topGames
              .take(5)
              .map(
                (item) => _RankRow(
                  label: item.name,
                  value: '${item.count}',
                  trailing: item.count == 1 ? 'play' : 'plays',
                ),
              ),
          const SizedBox(height: 22),
        ],
        if (stats.leaderboard.isNotEmpty) ...[
          const SectionHeader(title: 'At the table'),
          const SizedBox(height: 10),
          ...stats.leaderboard
              .take(8)
              .map(
                (entry) => _RankRow(
                  label: entry.name,
                  value: '${entry.wins}',
                  trailing: entry.wins == 1 ? 'win' : 'wins',
                ),
              ),
          const SizedBox(height: 22),
        ],
        if (stats.weightBands.isNotEmpty) ...[
          const SectionHeader(title: 'Weight of the shelf'),
          const SizedBox(height: 10),
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: scheme.surface,
              borderRadius: BorderRadius.circular(20),
              border: Border.all(color: scheme.outlineVariant),
            ),
            child: Column(
              children: stats.weightBands.map((band) {
                final total = stats.weightBands.fold<int>(
                  0,
                  (sum, b) => sum + b.count,
                );
                return Padding(
                  padding: const EdgeInsets.symmetric(vertical: 6),
                  child: Row(
                    children: [
                      SizedBox(
                        width: 108,
                        child: Text(
                          band.label,
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                      ),
                      Expanded(
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(3),
                          child: LinearProgressIndicator(
                            value: total == 0 ? 0 : band.count / total,
                            minHeight: 8,
                            backgroundColor: scheme.surfaceContainerHighest,
                          ),
                        ),
                      ),
                      const SizedBox(width: 10),
                      SizedBox(
                        width: 22,
                        child: Text(
                          '${band.count}',
                          textAlign: TextAlign.right,
                          style: Theme.of(context).textTheme.labelSmall,
                        ),
                      ),
                    ],
                  ),
                );
              }).toList(),
            ),
          ),
          const SizedBox(height: 22),
        ],
        if (stats.shelf.unplayed > 0)
          Container(
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(
              color: ShelfColors.brass.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(20),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(Icons.hourglass_bottom, color: ShelfColors.brass),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'The shelf of shame',
                        style: Theme.of(context).textTheme.titleMedium
                            ?.copyWith(
                              fontFamily: 'Newsreader',
                              fontWeight: FontWeight.w600,
                            ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        stats.shelf.longestUnplayed.isEmpty
                            ? '${stats.shelf.unplayed} games are still waiting for their first play.'
                            : '${stats.shelf.unplayed} unplayed. ${stats.shelf.longestUnplayed} has waited ${stats.shelf.longestWaitDays} days.',
                        style: Theme.of(context).textTheme.bodySmall
                            ?.copyWith(color: scheme.onSurfaceVariant),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
      ],
    );
  }
}

class _BigStat extends StatelessWidget {
  const _BigStat({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label.toUpperCase(),
            style: Theme.of(context).textTheme.labelSmall
                ?.copyWith(color: scheme.onSurfaceVariant, letterSpacing: 1.2),
          ),
          const SizedBox(height: 6),
          Text(
            value,
            style: Theme.of(context).textTheme.headlineMedium?.copyWith(
              fontFamily: 'Newsreader',
              fontFeatures: const [FontFeature.tabularFigures()],
            ),
          ),
        ],
      ),
    );
  }
}

class _RankRow extends StatelessWidget {
  const _RankRow({
    required this.label,
    required this.value,
    required this.trailing,
  });
  final String label;
  final String value;
  final String trailing;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Row(
        children: [
          Expanded(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ),
          Text(
            value,
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
              fontFamily: 'Newsreader',
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(width: 5),
          Text(
            trailing,
            style: Theme.of(context).textTheme.labelSmall
                ?.copyWith(color: scheme.onSurfaceVariant),
          ),
        ],
      ),
    );
  }
}
