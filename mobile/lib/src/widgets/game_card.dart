import 'package:flutter/material.dart';

import '../api/models.dart';
import '../theme/tokens.dart';
import 'common.dart';

class GameCard extends StatelessWidget {
  const GameCard({super.key, required this.game, required this.onTap});

  final Game game;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Semantics(
      button: true,
      label: '${game.name}, ${game.totalPlays} plays',
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(18),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Stack(
                fit: StackFit.expand,
                children: [
                  GameCover(imageUrl: game.imageUrl, name: game.name),
                  if (game.favorite)
                    Positioned(
                      top: 8,
                      right: 8,
                      child: Container(
                        padding: const EdgeInsets.all(6),
                        decoration: BoxDecoration(
                          color: scheme.surface.withValues(alpha: .92),
                          shape: BoxShape.circle,
                        ),
                        child: Icon(
                          Icons.favorite,
                          size: 14,
                          color: scheme.secondary,
                        ),
                      ),
                    ),
                  if (game.totalPlays == 0)
                    Positioned(
                      top: 8,
                      left: -5,
                      child: Transform.rotate(
                        angle: -.06,
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 9,
                            vertical: 4,
                          ),
                          color: scheme.secondary,
                          child: Text(
                            'UNPLAYED',
                            style: Theme.of(context).textTheme.labelSmall
                                ?.copyWith(
                                  color: scheme.onSecondary,
                                  fontWeight: FontWeight.w700,
                                  letterSpacing: .6,
                                ),
                          ),
                        ),
                      ),
                    ),
                  if (game.userRating != null)
                    Positioned(
                      left: 8,
                      bottom: 8,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 8,
                          vertical: 4,
                        ),
                        decoration: BoxDecoration(
                          color: scheme.secondary,
                          borderRadius: BorderRadius.circular(999),
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(
                              Icons.star_rounded,
                              size: 12,
                              color: scheme.onSecondary,
                            ),
                            const SizedBox(width: 3),
                            Text(
                              game.userRating!.toStringAsFixed(1),
                              style: Theme.of(context).textTheme.labelSmall
                                  ?.copyWith(
                                    color: scheme.onSecondary,
                                    fontWeight: FontWeight.w700,
                                  ),
                            ),
                          ],
                        ),
                      ),
                    ),
                ],
              ),
            ),
            const SizedBox(height: 10),
            Text(
              game.name,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.titleMedium
                  ?.copyWith(height: 1.2),
            ),
            const SizedBox(height: 4),
            GameMetaLine(game: game, color: scheme.onSurfaceVariant),
            const SizedBox(height: 6),
            LayoutBuilder(
              builder: (context, constraints) {
                final showWeight =
                    constraints.maxWidth >= 210 && game.weightLabel.isNotEmpty;
                return Row(
                  children: [
                    Expanded(
                      child: Text(
                        game.playLabel,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: Theme.of(context).textTheme.labelSmall?.copyWith(
                          color: game.totalPlays == 0
                              ? ShelfColors.brass
                              : scheme.onSurfaceVariant,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                    if (showWeight) ...[
                      const SizedBox(width: 6),
                      Flexible(
                        child: Text(
                          game.weightLabel,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          textAlign: TextAlign.right,
                          style: Theme.of(context).textTheme.labelSmall
                              ?.copyWith(color: scheme.onSurfaceVariant),
                        ),
                      ),
                    ],
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}
