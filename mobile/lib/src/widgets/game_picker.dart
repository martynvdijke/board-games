import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../providers.dart';
import '../widgets/common.dart';

Future<Game?> showGamePicker(BuildContext context) {
  return showModalBottomSheet<Game>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (_) => const _GamePickerSheet(),
  );
}

class _GamePickerSheet extends ConsumerWidget {
  const _GamePickerSheet();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final games = ref.watch(gamesProvider(const GameQuery()));
    return FractionallySizedBox(
      heightFactor: .82,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 4, 20, 10),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'What hit the table?',
                  style: Theme.of(context).textTheme.headlineSmall,
                ),
                const SizedBox(height: 4),
                Text(
                  'Pick a game from your shelf.',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          Expanded(
            child: games.when(
              data: (page) => page.items.isEmpty
                  ? const Padding(
                      padding: EdgeInsets.all(20),
                      child: EmptyState(
                        title: 'No games yet',
                        message: 'Sync your BoardGameGeek collection first.',
                      ),
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.fromLTRB(16, 4, 16, 20),
                      itemCount: page.items.length,
                      separatorBuilder: (_, _) => const SizedBox(height: 6),
                      itemBuilder: (context, index) {
                        final game = page.items[index];
                        return ListTile(
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(14),
                          ),
                          leading: SizedBox(
                            width: 34,
                            child: GameCover(
                              imageUrl: game.imageUrl,
                              name: game.name,
                              radius: 6,
                            ),
                          ),
                          title: Text(game.name),
                          subtitle: Text(game.playerLabel),
                          trailing: const Icon(Icons.chevron_right),
                          onTap: () => Navigator.pop(context, game),
                        );
                      },
                    ),
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, _) => Padding(
                padding: const EdgeInsets.all(20),
                child: ErrorView(message: '$error'),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
