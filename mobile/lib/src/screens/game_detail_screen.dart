import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/api_client.dart';
import '../api/models.dart';
import '../providers.dart';
import '../telemetry/telemetry.dart';
import '../widgets/common.dart';
import '../widgets/play_tile.dart';
import 'play_form_screen.dart';

class GameDetailScreen extends ConsumerStatefulWidget {
  const GameDetailScreen({super.key, required this.gameId, this.initial});

  final int gameId;
  final Game? initial;

  @override
  ConsumerState<GameDetailScreen> createState() => _GameDetailScreenState();
}

class _GameDetailScreenState extends ConsumerState<GameDetailScreen> {
  @override
  void initState() {
    super.initState();
    Telemetry.instance.screen('game_detail');
  }

  Future<void> _refresh() async {
    try {
      ref.invalidate(gameProvider(widget.gameId));
      await ref.read(gameProvider(widget.gameId).future);
      ref.invalidate(playsProvider(PlayQuery(gameId: widget.gameId)));
    } catch (_) {}
  }

  void _openPlayForm(Game game) {
    Navigator.of(context)
        .push(MaterialPageRoute(builder: (_) => PlayFormScreen(game: game)));
  }

  Future<void> _editShelf(GameDetail detail) async {
    final result = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Theme.of(context).colorScheme.surface,
      builder: (_) => _ShelfEditor(detail: detail),
    );
    if (result == true && mounted) {
      ref.invalidate(gameProvider(widget.gameId));
      ref.invalidate(gamesProvider);
      ref.invalidate(statsProvider(DateTime.now().year));
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final async = ref.watch(gameProvider(widget.gameId));
    final plays = ref
        .watch(playsProvider(PlayQuery(gameId: widget.gameId)))
        .value;
    final detail = async.value;
    final game = detail?.game ?? widget.initial;
    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: async.when(
          loading: () => game == null
              ? const Center(child: CircularProgressIndicator())
              : _buildContent(
                  context,
                  scheme,
                  game,
                  detail,
                  plays,
                  loading: true,
                ),
          error: (error, _) => Column(
            children: [
              _TopBar(onBack: () => Navigator.of(context).pop()),
              Padding(
                padding: const EdgeInsets.all(20),
                child: ErrorView(
                  message: '$error',
                  onRetry: () => ref.invalidate(gameProvider(widget.gameId)),
                ),
              ),
            ],
          ),
          data: (detail) {
            final plays = ref
                .watch(playsProvider(PlayQuery(gameId: widget.gameId)))
                .value;
            return _buildContent(context, scheme, detail.game, detail, plays);
          },
        ),
      ),
    );
  }

  Widget _buildContent(
    BuildContext context,
    ColorScheme scheme,
    Game game,
    GameDetail? detail,
    PlaysPage? plays, {
    bool loading = false,
  }) {
    final recent = (plays?.items ?? []).take(4).toList();
    return RefreshIndicator(
      onRefresh: _refresh,
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          SliverToBoxAdapter(
            child: _TopBar(onBack: () => Navigator.of(context).pop()),
          ),
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(20, 6, 20, 0),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      SizedBox(
                        width: 126,
                        child: Stack(
                          children: [
                            GameCover(
                              imageUrl: game.imageUrl,
                              name: game.name,
                              radius: 14,
                            ),
                            if (game.favorite)
                              Positioned(
                                top: 7,
                                right: 7,
                                child: Container(
                                  padding: const EdgeInsets.all(5),
                                  decoration: BoxDecoration(
                                    color: scheme.surface.withValues(
                                      alpha: .92,
                                    ),
                                    shape: BoxShape.circle,
                                  ),
                                  child: Icon(
                                    Icons.favorite,
                                    size: 13,
                                    color: scheme.secondary,
                                  ),
                                ),
                              ),
                          ],
                        ),
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            if (game.year > 0)
                              Eyebrow(
                                '${game.year}${game.isExpansion ? ' · Expansion' : ''}',
                              ),
                            const SizedBox(height: 5),
                            Text(
                              game.name,
                              style: TextStyle(
                                fontFamily: 'Newsreader',
                                fontSize: 28,
                                height: 1.06,
                                letterSpacing: -.4,
                                fontWeight: FontWeight.w500,
                                color: scheme.onSurface,
                              ),
                            ),
                            const SizedBox(height: 10),
                            Wrap(
                              spacing: 6,
                              runSpacing: 6,
                              children: [
                                if (game.playerLabel.isNotEmpty)
                                  MetaChip(
                                    game.playerLabel,
                                    icon: Icons.people_outline,
                                  ),
                                if (game.timeLabel.isNotEmpty)
                                  MetaChip(
                                    game.timeLabel,
                                    icon: Icons.schedule,
                                  ),
                                if (game.weight > 0) MetaChip(game.weightLabel),
                                if (game.userRating != null)
                                  Container(
                                    padding: const EdgeInsets.symmetric(
                                      horizontal: 9,
                                      vertical: 5,
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
                                          size: 13,
                                          color: scheme.onSecondary,
                                        ),
                                        const SizedBox(width: 3),
                                        Text(
                                          game.userRating!.toStringAsFixed(1),
                                          style: Theme.of(context)
                                              .textTheme
                                              .labelSmall
                                              ?.copyWith(
                                                color: scheme.onSecondary,
                                                fontWeight: FontWeight.w700,
                                              ),
                                        ),
                                      ],
                                    ),
                                  ),
                              ],
                            ),
                            if (loading) ...[
                              const SizedBox(height: 10),
                              LinearProgressIndicator(color: scheme.primary),
                            ],
                          ],
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 18),
                  SizedBox(
                    width: double.infinity,
                    child: FilledButton.icon(
                      onPressed: () => _openPlayForm(game),
                      icon: const Icon(Icons.add, size: 18),
                      label: const Text('Log a play'),
                    ),
                  ),
                  const SizedBox(height: 8),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.storage_outlined,
                        size: 13,
                        color: scheme.onSurfaceVariant,
                      ),
                      const SizedBox(width: 5),
                      Text(
                        '${game.totalPlays} ${game.totalPlays == 1 ? 'play' : 'plays'} · BGG #${game.bggRank}',
                        style: Theme.of(context).textTheme.labelSmall
                            ?.copyWith(color: scheme.onSurfaceVariant),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
          if (detail != null && detail.description.isNotEmpty)
            SliverToBoxAdapter(
              child: _Section(
                title: 'About the game',
                child: Text(
                  _plain(detail.description),
                  style: Theme.of(context).textTheme.bodyMedium
                      ?.copyWith(height: 1.6),
                ),
              ),
            ),
          if (detail != null)
            SliverToBoxAdapter(
              child: _ShelfPanel(
                detail: detail,
                onEdit: () => _editShelf(detail),
              ),
            ),
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(20, 22, 20, 6),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      'Recent plays',
                      style: Theme.of(context).textTheme.headlineSmall,
                    ),
                  ),
                  Text(
                    '${game.totalPlays} total',
                    style: Theme.of(context).textTheme.labelSmall
                        ?.copyWith(color: scheme.onSurfaceVariant),
                  ),
                ],
              ),
            ),
          ),
          if (recent.isEmpty)
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(20, 4, 20, 32),
                child: EmptyState(
                  title: 'No plays yet',
                  message: 'Be the first to put this one on the table.',
                  icon: Icons.casino_outlined,
                  actionLabel: 'Log the first play',
                  onAction: () => _openPlayForm(game),
                ),
              ),
            )
          else
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(20, 4, 20, 36),
              sliver: SliverList.separated(
                itemCount: recent.length,
                separatorBuilder: (_, _) => const SizedBox(height: 8),
                itemBuilder: (context, index) => PlayTile(
                  play: recent[index],
                  showGame: false,
                  onTap: () => _openPlayForm(game),
                  onEdit: recent[index].isLocal
                      ? () => Navigator.of(context).push(
                          MaterialPageRoute(
                            builder: (_) =>
                                PlayFormScreen(game: game, play: recent[index]),
                          ),
                        )
                      : null,
                  onDelete: recent[index].isLocal
                      ? () => _delete(recent[index])
                      : null,
                ),
              ),
            ),
        ],
      ),
    );
  }

  Future<void> _delete(Play play) async {
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
      ref.invalidate(playsProvider(PlayQuery(gameId: widget.gameId)));
      ref.invalidate(gameProvider(widget.gameId));
      ref.invalidate(gamesProvider);
      if (mounted) showShelfSnack(context, 'Play deleted.');
    } catch (error) {
      if (mounted) showShelfSnack(context, '$error', error: true);
    }
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({required this.onBack});
  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(8, 4, 16, 4),
      child: Row(
        children: [
          IconButton(
            onPressed: onBack,
            icon: const Icon(Icons.arrow_back),
            tooltip: 'Back to shelf',
          ),
          const Spacer(),
          Text(
            'THE SHELF',
            style: Theme.of(context).textTheme.labelSmall?.copyWith(
              letterSpacing: 1.8,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}

class _Section extends StatelessWidget {
  const _Section({required this.title, required this.child});
  final String title;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 24, 20, 0),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.all(18),
        decoration: BoxDecoration(
          color: scheme.surface,
          borderRadius: BorderRadius.circular(18),
          border: Border.all(color: scheme.outlineVariant),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [Eyebrow(title), const SizedBox(height: 8), child],
        ),
      ),
    );
  }
}

class _ShelfPanel extends StatelessWidget {
  const _ShelfPanel({required this.detail, required this.onEdit});
  final GameDetail detail;
  final VoidCallback onEdit;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final game = detail.game;
    return _Section(
      title: 'On your shelf',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: [
              if (game.owned) const MetaChip('Owned', icon: Icons.check),
              if (game.wishlist)
                const MetaChip('Wishlist', icon: Icons.bookmark_outline),
              if (game.wantToPlay)
                const MetaChip('Want to play', icon: Icons.local_play_outlined),
              if (game.forTrade)
                const MetaChip('For trade', icon: Icons.swap_horiz),
              if (game.preordered)
                const MetaChip('Preordered', icon: Icons.inventory_2_outlined),
            ],
          ),
          if (detail.location.isNotEmpty) ...[
            const SizedBox(height: 14),
            _DetailRow(
              icon: Icons.place_outlined,
              label: 'Location',
              value: detail.location,
            ),
          ],
          if (detail.condition.isNotEmpty) ...[
            const SizedBox(height: 10),
            _DetailRow(
              icon: Icons.auto_awesome_outlined,
              label: 'Condition',
              value: detail.condition,
            ),
          ],
          if (game.notes.isNotEmpty) ...[
            const SizedBox(height: 12),
            Text(
              game.notes,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                color: scheme.onSurfaceVariant,
                height: 1.45,
                fontStyle: FontStyle.italic,
              ),
            ),
          ],
          const SizedBox(height: 12),
          Align(
            alignment: Alignment.centerLeft,
            child: OutlinedButton.icon(
              onPressed: onEdit,
              icon: const Icon(Icons.edit_outlined, size: 15),
              label: const Text('Edit shelf details'),
            ),
          ),
        ],
      ),
    );
  }
}

class _DetailRow extends StatelessWidget {
  const _DetailRow({
    required this.icon,
    required this.label,
    required this.value,
  });
  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 15, color: scheme.onSurfaceVariant),
        const SizedBox(width: 8),
        Text('$label: ', style: Theme.of(context).textTheme.bodySmall),
        Expanded(
          child: Text(
            value,
            style: Theme.of(context).textTheme.bodySmall
                ?.copyWith(color: scheme.onSurfaceVariant),
          ),
        ),
      ],
    );
  }
}

class _ShelfEditor extends ConsumerStatefulWidget {
  const _ShelfEditor({required this.detail});
  final GameDetail detail;

  @override
  ConsumerState<_ShelfEditor> createState() => _ShelfEditorState();
}

class _ShelfEditorState extends ConsumerState<_ShelfEditor> {
  late double? _rating;
  late final TextEditingController _notes;
  late final TextEditingController _location;
  late final TextEditingController _condition;
  late bool _favorite;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _rating = widget.detail.game.userRating;
    _notes = TextEditingController(text: widget.detail.game.notes);
    _location = TextEditingController(text: widget.detail.location);
    _condition = TextEditingController(text: widget.detail.condition);
    _favorite = widget.detail.game.favorite;
  }

  @override
  void dispose() {
    _notes.dispose();
    _location.dispose();
    _condition.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    final patch = <String, dynamic>{
      'favorite': _favorite,
      'notes': _notes.text.trim(),
      'location': _location.text.trim(),
      'condition': _condition.text.trim(),
      if (_rating == null) 'clear_rating': true else 'user_rating': _rating,
    };
    try {
      await ref.read(apiClientProvider).patchGame(widget.detail.game.id, patch);
      if (mounted) Navigator.pop(context, true);
    } catch (error) {
      if (mounted) {
        setState(() => _saving = false);
        showShelfSnack(context, '$error', error: true);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: EdgeInsets.fromLTRB(
        20,
        18,
        20,
        MediaQuery.viewInsetsOf(context).bottom + 20,
      ),
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                Text(
                  'Shelf details',
                  style: Theme.of(context).textTheme.headlineSmall,
                ),
                const Spacer(),
                IconButton(
                  onPressed: () => Navigator.pop(context),
                  icon: const Icon(Icons.close),
                ),
              ],
            ),
            const SizedBox(height: 12),
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              title: const Text('Favourite'),
              subtitle: const Text('Pin a little heart on this one'),
              value: _favorite,
              onChanged: (value) => setState(() => _favorite = value),
            ),
            const SizedBox(height: 8),
            DropdownButtonFormField<double?>(
              initialValue: _rating,
              decoration: const InputDecoration(labelText: 'Your rating'),
              items: [
                const DropdownMenuItem(value: null, child: Text('No rating')),
                for (var i = 1; i <= 20; i++)
                  DropdownMenuItem(value: i / 2, child: Text('${i / 2} / 10')),
              ],
              onChanged: (value) => setState(() => _rating = value),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _location,
              decoration: const InputDecoration(
                labelText: 'Location',
                hintText: 'Living room, game cupboard…',
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _condition,
              decoration: const InputDecoration(
                labelText: 'Condition',
                hintText: 'Excellent, shelf wear…',
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _notes,
              minLines: 3,
              maxLines: 6,
              decoration: const InputDecoration(
                labelText: 'Private notes',
                hintText: 'House rules, who loves it, what to remember…',
                alignLabelWithHint: true,
              ),
            ),
            const SizedBox(height: 18),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: _saving ? null : _save,
                child: _saving
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(
                        'Save shelf details',
                        style: TextStyle(color: scheme.onPrimary),
                      ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

String _plain(String value) {
  return value
      .replaceAll(RegExp(r'<br\s*/?>', caseSensitive: false), '\n')
      .replaceAll(RegExp(r'</p>', caseSensitive: false), '\n\n')
      .replaceAll(RegExp(r'<[^>]+>'), '')
      .replaceAll('&amp;', '&')
      .replaceAll('&quot;', '"')
      .replaceAll('&#39;', "'")
      .replaceAll('&nbsp;', ' ')
      .replaceAll(RegExp(r'\n{3,}'), '\n\n')
      .trim();
}
