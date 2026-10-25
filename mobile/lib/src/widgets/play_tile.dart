import 'package:flutter/material.dart';

import '../api/models.dart';
import '../theme/tokens.dart';

class PlayTile extends StatelessWidget {
  const PlayTile({
    super.key,
    required this.play,
    this.showGame = true,
    this.onTap,
    this.onEdit,
    this.onDelete,
  });

  final Play play;
  final bool showGame;
  final VoidCallback? onTap;
  final VoidCallback? onEdit;
  final VoidCallback? onDelete;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final title = showGame ? play.gameName : play.playersLabel;
    final meta = [
      play.whenLabel,
      if (showGame && play.playersLabel.isNotEmpty) play.playersLabel,
      if (!showGame && play.location.isNotEmpty) play.location,
      if (play.durationMinutes > 0) '${play.durationMinutes} min',
    ].where((e) => e.isNotEmpty).join(' · ');
    return Material(
      color: scheme.surface,
      borderRadius: BorderRadius.circular(16),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(16),
        child: Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(16),
            border: Border.all(color: scheme.outlineVariant),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 42,
                height: 42,
                decoration: BoxDecoration(
                  color: scheme.primaryContainer,
                  borderRadius: BorderRadius.circular(12),
                ),
                alignment: Alignment.center,
                child: Icon(
                  Icons.casino_outlined,
                  color: scheme.primary,
                  size: 20,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            title.isEmpty ? 'A game' : title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: Theme.of(context).textTheme.titleSmall,
                          ),
                        ),
                        if (play.origin == 'bgg')
                          Container(
                            margin: const EdgeInsets.only(left: 6),
                            padding: const EdgeInsets.symmetric(
                              horizontal: 6,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: scheme.surfaceContainerHigh,
                              borderRadius: BorderRadius.circular(5),
                            ),
                            child: Text(
                              'BGG',
                              style: Theme.of(context).textTheme.labelSmall
                                  ?.copyWith(
                                    fontSize: 9,
                                    letterSpacing: .7,
                                    color: scheme.onSurfaceVariant,
                                  ),
                            ),
                          ),
                        if (onEdit != null || onDelete != null)
                          PopupMenuButton<String>(
                            padding: EdgeInsets.zero,
                            icon: Icon(
                              Icons.more_horiz,
                              size: 18,
                              color: scheme.onSurfaceVariant,
                            ),
                            onSelected: (value) {
                              if (value == 'edit') onEdit?.call();
                              if (value == 'delete') onDelete?.call();
                            },
                            itemBuilder: (_) => const [
                              PopupMenuItem(
                                value: 'edit',
                                child: Text('Edit play'),
                              ),
                              PopupMenuItem(
                                value: 'delete',
                                child: Text('Delete play'),
                              ),
                            ],
                          ),
                      ],
                    ),
                    const SizedBox(height: 2),
                    Text(
                      meta,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.bodySmall
                          ?.copyWith(color: scheme.onSurfaceVariant),
                    ),
                    if (play.notes.isNotEmpty) ...[
                      const SizedBox(height: 5),
                      Text(
                        play.notes,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: scheme.onSurfaceVariant,
                          height: 1.35,
                        ),
                      ),
                    ],
                    if (play.players.any(
                      (p) => p.isWinner || p.score.isNotEmpty,
                    )) ...[
                      const SizedBox(height: 6),
                      Wrap(
                        spacing: 6,
                        runSpacing: 4,
                        children: [
                          for (final player in play.players)
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 7,
                                vertical: 2,
                              ),
                              decoration: BoxDecoration(
                                color: player.isWinner
                                    ? scheme.secondaryContainer
                                    : scheme.surfaceContainerHigh,
                                borderRadius: BorderRadius.circular(999),
                              ),
                              child: Text(
                                [
                                  player.name,
                                  if (player.score.isNotEmpty) player.score,
                                  if (player.isWinner) 'won',
                                ].join(' · '),
                                style: Theme.of(context).textTheme.labelSmall
                                    ?.copyWith(
                                      color: player.isWinner
                                          ? ShelfColors.brass
                                          : scheme.onSurfaceVariant,
                                      fontWeight: player.isWinner
                                          ? FontWeight.w600
                                          : FontWeight.w400,
                                    ),
                              ),
                            ),
                        ],
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
