import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../providers.dart';
import '../telemetry/telemetry.dart';
import '../utils.dart';
import '../widgets/common.dart';

class PlayFormScreen extends ConsumerStatefulWidget {
  const PlayFormScreen({super.key, required this.game, this.play});

  final Game game;
  final Play? play;

  @override
  ConsumerState<PlayFormScreen> createState() => _PlayFormScreenState();
}

class _PlayerDraft {
  _PlayerDraft({String name = '', String score = '', this.winner = false})
    : name = TextEditingController(text: name),
      score = TextEditingController(text: score);

  final TextEditingController name;
  final TextEditingController score;
  bool winner;

  void dispose() {
    name.dispose();
    score.dispose();
  }
}

class _PlayFormScreenState extends ConsumerState<PlayFormScreen> {
  late DateTime _date;
  late final TextEditingController _location;
  late final TextEditingController _duration;
  late final TextEditingController _quantity;
  late final TextEditingController _notes;
  final List<_PlayerDraft> _players = [];
  bool _saving = false;

  bool get _editing => widget.play != null;

  @override
  void initState() {
    super.initState();
    final play = widget.play;
    _date = play != null
        ? DateTime.tryParse(play.playedAt) ?? DateTime.now()
        : DateTime.now();
    _location = TextEditingController(text: play?.location ?? '');
    _duration = TextEditingController(
      text: play != null && play.durationMinutes > 0
          ? '${play.durationMinutes}'
          : '',
    );
    _quantity = TextEditingController(text: '${play?.quantity ?? 1}');
    _notes = TextEditingController(text: play?.notes ?? '');
    if (play != null) {
      for (final p in play.players) {
        _players.add(
          _PlayerDraft(name: p.name, score: p.score, winner: p.isWinner),
        );
      }
    }
    if (_players.isEmpty) {
      _players.add(_PlayerDraft());
      _players.add(_PlayerDraft());
    }
  }

  @override
  void dispose() {
    _location.dispose();
    _duration.dispose();
    _quantity.dispose();
    _notes.dispose();
    for (final p in _players) {
      p.dispose();
    }
    super.dispose();
  }

  Future<void> _pickDate() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: _date,
      firstDate: DateTime(1970),
      lastDate: DateTime.now(),
    );
    if (picked != null) setState(() => _date = picked);
  }

  Future<void> _save() async {
    final players = <PlayPlayer>[];
    for (final draft in _players) {
      final name = draft.name.text.trim();
      if (name.isEmpty) continue;
      players.add(
        PlayPlayer(
          name: name,
          score: draft.score.text.trim(),
          isWinner: draft.winner,
          isFirstPlay: false,
        ),
      );
    }
    final input = PlayInput(
      gameId: widget.game.id,
      playedAt: isoDate(_date),
      location: _location.text.trim(),
      durationMinutes: int.tryParse(_duration.text.trim()) ?? 0,
      quantity: int.tryParse(_quantity.text.trim()) ?? 1,
      notes: _notes.text.trim(),
      players: players,
    );
    setState(() => _saving = true);
    try {
      final api = ref.read(apiClientProvider);
      if (_editing) {
        await api.updatePlay(widget.play!.id, input);
      } else {
        await api.createPlay(input);
      }
      Telemetry.instance.track('play_submitted', {
        'mode': _editing ? 'edit' : 'create',
      });
      ref.invalidate(playsProvider);
      ref.invalidate(gameProvider(widget.game.id));
      ref.invalidate(gamesProvider);
      ref.invalidate(statsProvider);
      if (mounted) {
        Navigator.pop(context, true);
      }
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
    return Scaffold(
      appBar: AppBar(
        title: Text(_editing ? 'Edit play' : 'Log a play'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          tooltip: 'Cancel',
          onPressed: () => Navigator.pop(context),
        ),
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
          children: [
            Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: scheme.surface,
                borderRadius: BorderRadius.circular(16),
                border: Border.all(color: scheme.outlineVariant),
              ),
              child: Row(
                children: [
                  SizedBox(
                    width: 46,
                    child: GameCover(
                      imageUrl: widget.game.imageUrl,
                      name: widget.game.name,
                      radius: 8,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Eyebrow('The game'),
                        const SizedBox(height: 3),
                        Text(
                          widget.game.name,
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        const SizedBox(height: 2),
                        Text(
                          widget.game.playerLabel,
                          style: Theme.of(context).textTheme.bodySmall
                              ?.copyWith(color: scheme.onSurfaceVariant),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 22),
            Eyebrow('When and where'),
            const SizedBox(height: 10),
            InkWell(
              onTap: _pickDate,
              borderRadius: BorderRadius.circular(10),
              child: InputDecorator(
                decoration: const InputDecoration(
                  labelText: 'Date played',
                  prefixIcon: Icon(Icons.calendar_today_outlined),
                ),
                child: Text(dateLong(_date)),
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _location,
              decoration: const InputDecoration(
                labelText: 'Location',
                hintText: 'Home, club, convention…',
                prefixIcon: Icon(Icons.place_outlined),
              ),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _duration,
                    keyboardType: TextInputType.number,
                    decoration: const InputDecoration(
                      labelText: 'Minutes',
                      hintText: 'Optional',
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: TextField(
                    controller: _quantity,
                    keyboardType: TextInputType.number,
                    decoration: const InputDecoration(labelText: 'Rounds'),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 24),
            Row(
              children: [
                Expanded(
                  child: Text(
                    'Players',
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                TextButton.icon(
                  onPressed: _players.length >= 20
                      ? null
                      : () => setState(() => _players.add(_PlayerDraft())),
                  icon: const Icon(Icons.add, size: 17),
                  label: const Text('Add player'),
                ),
              ],
            ),
            const SizedBox(height: 4),
            for (var i = 0; i < _players.length; i++)
              Padding(
                padding: const EdgeInsets.only(bottom: 10),
                child: Row(
                  children: [
                    Container(
                      width: 24,
                      alignment: Alignment.center,
                      child: Text(
                        '${i + 1}',
                        style: Theme.of(context).textTheme.labelSmall
                            ?.copyWith(color: scheme.onSurfaceVariant),
                      ),
                    ),
                    Expanded(
                      child: TextField(
                        controller: _players[i].name,
                        textCapitalization: TextCapitalization.words,
                        decoration: const InputDecoration(
                          hintText: 'Player name',
                          isDense: true,
                        ),
                      ),
                    ),
                    const SizedBox(width: 8),
                    SizedBox(
                      width: 74,
                      child: TextField(
                        controller: _players[i].score,
                        decoration: const InputDecoration(
                          hintText: 'Pts',
                          isDense: true,
                        ),
                      ),
                    ),
                    IconButton(
                      tooltip: _players[i].winner
                          ? 'Not the winner'
                          : 'Mark as winner',
                      onPressed: () => setState(
                        () => _players[i].winner = !_players[i].winner,
                      ),
                      icon: Icon(
                        _players[i].winner
                            ? Icons.emoji_events
                            : Icons.emoji_events_outlined,
                        size: 20,
                        color: _players[i].winner
                            ? scheme.secondary
                            : scheme.onSurfaceVariant,
                      ),
                    ),
                    IconButton(
                      tooltip: 'Remove player',
                      onPressed: _players.length == 1
                          ? null
                          : () =>
                                setState(() => _players.removeAt(i).dispose()),
                      icon: const Icon(Icons.close, size: 18),
                    ),
                  ],
                ),
              ),
            Text(
              'Names are enough. Scores and winners are optional.',
              style: Theme.of(context).textTheme.labelSmall
                  ?.copyWith(color: scheme.onSurfaceVariant),
            ),
            const SizedBox(height: 22),
            TextField(
              controller: _notes,
              minLines: 3,
              maxLines: 6,
              decoration: const InputDecoration(
                labelText: 'Notes',
                hintText: 'What happened? House rules, best moment, who swore they would never play again…',
                alignLabelWithHint: true,
              ),
            ),
            const SizedBox(height: 24),
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
                    : Text(_editing ? 'Save changes' : 'Log this play'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
