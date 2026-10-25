/// API models are intentionally tolerant: most payloads use lower_snake_case
/// while a few legacy stats structs use PascalCase.
Object? _lowerValue(Object? value) {
  if (value is Map) {
    return value.map(
      (key, item) => MapEntry(key.toString().toLowerCase(), _lowerValue(item)),
    );
  }
  if (value is List) return value.map(_lowerValue).toList();
  return value;
}

Map<String, dynamic> lowerKeys(Object? value) {
  if (value is! Map) return const {};
  return value.map(
    (key, item) => MapEntry(key.toString().toLowerCase(), _lowerValue(item)),
  );
}

Map<String, dynamic> asMap(Object? value) {
  if (value is Map) return lowerKeys(value);
  return const {};
}

List<Map<String, dynamic>> asList(Object? value) {
  if (value is List) {
    return value.map(asMap).where((m) => m.isNotEmpty).toList();
  }
  return const [];
}

String stringOf(Map<String, dynamic> json, String key, [String fallback = '']) {
  final value = json[key];
  if (value == null) return fallback;
  return value.toString();
}

int intOf(Map<String, dynamic> json, String key, [int fallback = 0]) {
  final value = json[key];
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '') ?? fallback;
}

double doubleOf(Map<String, dynamic> json, String key, [double fallback = 0]) {
  final value = json[key];
  if (value is num) return value.toDouble();
  return double.tryParse(value?.toString() ?? '') ?? fallback;
}

bool boolOf(Map<String, dynamic> json, String key, [bool fallback = false]) {
  final value = json[key];
  if (value is bool) return value;
  if (value is num) return value != 0;
  final text = value?.toString().toLowerCase();
  if (text == 'true' || text == '1') return true;
  if (text == 'false' || text == '0') return false;
  return fallback;
}

class UserProfile {
  const UserProfile({
    required this.id,
    required this.email,
    required this.name,
    required this.avatarUrl,
    required this.role,
    required this.scopes,
  });

  final String id;
  final String email;
  final String name;
  final String avatarUrl;
  final String role;
  final List<String> scopes;

  String get initials {
    final parts = name
        .trim()
        .split(RegExp(r'\s+'))
        .where((part) => part.isNotEmpty)
        .toList();
    if (parts.isEmpty) return '?';
    if (parts.length == 1) {
      return String.fromCharCode(parts.first.runes.first).toUpperCase();
    }
    return '${String.fromCharCode(parts.first.runes.first)}${String.fromCharCode(parts.last.runes.first)}'
        .toUpperCase();
  }

  String get firstName {
    final parts = name.trim().split(RegExp(r'\s+'));
    return parts.isEmpty || parts.first.isEmpty ? 'there' : parts.first;
  }

  factory UserProfile.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    return UserProfile(
      id: stringOf(map, 'id'),
      email: stringOf(map, 'email'),
      name: stringOf(map, 'name'),
      avatarUrl: stringOf(map, 'avatar_url'),
      role: stringOf(map, 'role'),
      scopes:
          (map['scopes'] as List?)?.map((e) => e.toString()).toList() ??
          const [],
    );
  }
}

class Game {
  const Game({
    required this.id,
    required this.bggId,
    required this.name,
    required this.year,
    required this.imageUrl,
    required this.bggUrl,
    required this.minPlayers,
    required this.maxPlayers,
    required this.minPlaytime,
    required this.maxPlaytime,
    required this.minAge,
    required this.weight,
    required this.bggRating,
    required this.bggRank,
    required this.userRating,
    required this.owned,
    required this.wishlist,
    required this.wantToPlay,
    required this.preordered,
    required this.forTrade,
    required this.favorite,
    required this.bggNumPlays,
    required this.localPlays,
    required this.totalPlays,
    required this.addedAt,
    required this.isExpansion,
    required this.notes,
  });

  final int id;
  final int bggId;
  final String name;
  final int year;
  final String imageUrl;
  final String bggUrl;
  final int minPlayers;
  final int maxPlayers;
  final int minPlaytime;
  final int maxPlaytime;
  final int minAge;
  final double weight;
  final double bggRating;
  final int bggRank;
  final double? userRating;
  final bool owned;
  final bool wishlist;
  final bool wantToPlay;
  final bool preordered;
  final bool forTrade;
  final bool favorite;
  final int bggNumPlays;
  final int localPlays;
  final int totalPlays;
  final String addedAt;
  final bool isExpansion;
  final String notes;

  String get playerLabel => playersLabel;

  String get playLabel => playsLabel;

  String get playersLabel {
    if (minPlayers <= 0 && maxPlayers <= 0) return '';
    if (minPlayers == maxPlayers && minPlayers > 0) {
      return '$minPlayers players';
    }
    if (maxPlayers <= 0) return '$minPlayers+ players';
    return '$minPlayers–$maxPlayers players';
  }

  String get timeLabel {
    if (minPlaytime <= 0 && maxPlaytime <= 0) return '';
    if (maxPlaytime <= 0 || minPlaytime == maxPlaytime) {
      return '$minPlaytime min';
    }
    return '$minPlaytime–$maxPlaytime min';
  }

  String get weightLabel {
    if (weight <= 0) return '';
    final label = weight < 1.8
        ? 'Light'
        : weight < 2.6
        ? 'Medium-light'
        : weight < 3.4
        ? 'Medium'
        : weight < 4.2
        ? 'Medium-heavy'
        : 'Heavy';
    return '$label · ${weight.toStringAsFixed(1)}';
  }

  String get playsLabel {
    if (totalPlays == 0) return 'Unplayed';
    return totalPlays == 1 ? '1 play' : '$totalPlays plays';
  }

  factory Game.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    final rating = map['user_rating'];
    return Game(
      id: intOf(map, 'id'),
      bggId: intOf(map, 'bgg_id'),
      name: stringOf(map, 'name'),
      year: intOf(map, 'year'),
      imageUrl: stringOf(map, 'image_url'),
      bggUrl: stringOf(map, 'bgg_url'),
      minPlayers: intOf(map, 'min_players'),
      maxPlayers: intOf(map, 'max_players'),
      minPlaytime: intOf(map, 'min_playtime'),
      maxPlaytime: intOf(map, 'max_playtime'),
      minAge: intOf(map, 'min_age'),
      weight: doubleOf(map, 'weight'),
      bggRating: doubleOf(map, 'bgg_rating'),
      bggRank: intOf(map, 'bgg_rank'),
      userRating: rating is num
          ? rating.toDouble()
          : double.tryParse(rating?.toString() ?? ''),
      owned: boolOf(map, 'owned'),
      wishlist: boolOf(map, 'wishlist'),
      wantToPlay: boolOf(map, 'want_to_play'),
      preordered: boolOf(map, 'preordered'),
      forTrade: boolOf(map, 'for_trade'),
      favorite: boolOf(map, 'favorite'),
      bggNumPlays: intOf(map, 'bgg_num_plays'),
      localPlays: intOf(map, 'local_plays'),
      totalPlays: intOf(map, 'total_plays'),
      addedAt: stringOf(map, 'added_at'),
      isExpansion: boolOf(map, 'is_expansion'),
      notes: stringOf(map, 'notes'),
    );
  }
}

class GameDetail {
  const GameDetail({
    required this.game,
    required this.description,
    required this.categories,
    required this.mechanics,
    required this.designers,
    required this.publishers,
    required this.location,
    required this.condition,
    required this.pricePaidCents,
  });

  final Game game;
  final String description;
  final List<String> categories;
  final List<String> mechanics;
  final List<String> designers;
  final List<String> publishers;
  final String location;
  final String condition;
  final int? pricePaidCents;

  factory GameDetail.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    final nested = map['game'];
    final gameJson = nested is Map ? lowerKeys(nested) : map;
    return GameDetail(
      game: Game.fromJson(gameJson),
      description: stringOf(map, 'description'),
      categories: _strings(map['categories']),
      mechanics: _strings(map['mechanics']),
      designers: _strings(map['designers']),
      publishers: _strings(map['publishers']),
      location: stringOf(map, 'location'),
      condition: stringOf(map, 'condition'),
      pricePaidCents: map['price_paid_cents'] is num
          ? intOf(map, 'price_paid_cents')
          : null,
    );
  }
}

List<String> _strings(Object? value) {
  if (value is List) return value.map((e) => e.toString()).toList();
  return const [];
}

class PlayPlayer {
  const PlayPlayer({
    required this.name,
    required this.score,
    required this.isWinner,
    required this.isFirstPlay,
  });

  final String name;
  final String score;
  final bool isWinner;
  final bool isFirstPlay;

  Map<String, dynamic> toJson() => {
    'name': name,
    'score': score,
    'is_winner': isWinner,
    'is_first_play': isFirstPlay,
  };

  factory PlayPlayer.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    return PlayPlayer(
      name: stringOf(map, 'name'),
      score: stringOf(map, 'score'),
      isWinner: boolOf(map, 'is_winner'),
      isFirstPlay: boolOf(map, 'is_first_play'),
    );
  }
}

class Play {
  const Play({
    required this.id,
    required this.gameId,
    required this.gameName,
    required this.imageUrl,
    required this.playedAt,
    required this.location,
    required this.durationMinutes,
    required this.quantity,
    required this.notes,
    required this.origin,
    required this.bggPlayId,
    required this.players,
    required this.createdAt,
    required this.updatedAt,
  });

  final int id;
  final int gameId;
  final String gameName;
  final String imageUrl;
  final String playedAt;
  final String location;
  final int durationMinutes;
  final int quantity;
  final String notes;
  final String origin;
  final int? bggPlayId;
  final List<PlayPlayer> players;
  final String createdAt;
  final String updatedAt;

  bool get isLocal => origin != 'bgg';

  String get whenLabel {
    final date = DateTime.tryParse(playedAt);
    if (date == null) return playedAt;
    const months = [
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'May',
      'Jun',
      'Jul',
      'Aug',
      'Sep',
      'Oct',
      'Nov',
      'Dec',
    ];
    return '${date.day} ${months[date.month - 1]} ${date.year}';
  }

  String get playersLabel =>
      players.map((p) => p.name).where((n) => n.isNotEmpty).join(', ');

  factory Play.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    return Play(
      id: intOf(map, 'id'),
      gameId: intOf(map, 'game_id'),
      gameName: stringOf(map, 'game_name'),
      imageUrl: stringOf(map, 'image_url'),
      playedAt: stringOf(map, 'played_at'),
      location: stringOf(map, 'location'),
      durationMinutes: intOf(map, 'duration_minutes'),
      quantity: intOf(map, 'quantity', 1),
      notes: stringOf(map, 'notes'),
      origin: stringOf(map, 'origin', 'local'),
      bggPlayId: map['bgg_play_id'] is num ? intOf(map, 'bgg_play_id') : null,
      players: asList(map['players']).map(PlayPlayer.fromJson).toList(),
      createdAt: stringOf(map, 'created_at'),
      updatedAt: stringOf(map, 'updated_at'),
    );
  }
}

class PlayInput {
  const PlayInput({
    required this.gameId,
    required this.playedAt,
    this.location = '',
    this.durationMinutes = 0,
    this.quantity = 1,
    this.notes = '',
    this.players = const [],
  });

  final int gameId;
  final String playedAt;
  final String location;
  final int durationMinutes;
  final int quantity;
  final String notes;
  final List<PlayPlayer> players;

  Map<String, dynamic> toJson() => {
    'game_id': gameId,
    'played_at': playedAt,
    'location': location,
    'duration_minutes': durationMinutes,
    'quantity': quantity,
    'notes': notes,
    'players': players.map((p) => p.toJson()).toList(),
  };
}

class MonthCount {
  const MonthCount(this.month, this.label, this.count);
  final int month;
  final String label;
  final int count;

  factory MonthCount.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    return MonthCount(
      intOf(map, 'month'),
      stringOf(map, 'label'),
      intOf(map, 'count'),
    );
  }
}

class CountItem {
  const CountItem(this.name, this.count);
  final String name;
  final int count;

  factory CountItem.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    return CountItem(stringOf(map, 'name'), intOf(map, 'count'));
  }
}

class LeaderEntry {
  const LeaderEntry(this.name, this.wins, this.plays);
  final String name;
  final int wins;
  final int plays;

  factory LeaderEntry.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    return LeaderEntry(
      stringOf(map, 'name'),
      intOf(map, 'wins'),
      intOf(map, 'plays'),
    );
  }
}

class BandCount {
  const BandCount(this.label, this.count);
  final String label;
  final int count;

  factory BandCount.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    return BandCount(stringOf(map, 'label'), intOf(map, 'count'));
  }
}

class ShelfSummary {
  const ShelfSummary({
    required this.owned,
    required this.unplayed,
    required this.wishlist,
    required this.playsThisYear,
    required this.totalPlays,
    required this.longestUnplayed,
    required this.longestWaitDays,
  });

  final int owned;
  final int unplayed;
  final int wishlist;
  final int playsThisYear;
  final int totalPlays;
  final String longestUnplayed;
  final int longestWaitDays;

  factory ShelfSummary.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    final longest = map['longest_unplayed'];
    var name = '';
    if (longest is Map) name = stringOf(lowerKeys(longest), 'name');
    if (longest is String) name = longest;
    return ShelfSummary(
      owned: intOf(map, 'owned'),
      unplayed: intOf(map, 'unplayed'),
      wishlist: intOf(map, 'wishlist'),
      playsThisYear: intOf(map, 'plays_this_year'),
      totalPlays: intOf(map, 'total_plays'),
      longestUnplayed: name,
      longestWaitDays: intOf(map, 'longest_wait_days'),
    );
  }
}

class Stats {
  const Stats({
    required this.year,
    required this.monthly,
    required this.totalPlays,
    required this.totalSessions,
    required this.topGames,
    required this.leaderboard,
    required this.weightBands,
    required this.shelf,
  });

  final int year;
  final List<MonthCount> monthly;
  final int totalPlays;
  final int totalSessions;
  final List<CountItem> topGames;
  final List<LeaderEntry> leaderboard;
  final List<BandCount> weightBands;
  final ShelfSummary shelf;

  int get maxMonth =>
      monthly.fold<int>(0, (max, item) => item.count > max ? item.count : max);

  factory Stats.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    return Stats(
      year: intOf(map, 'year'),
      monthly: asList(map['monthly']).map(MonthCount.fromJson).toList(),
      totalPlays: intOf(map, 'total_plays'),
      totalSessions: intOf(map, 'total_sessions'),
      topGames: asList(map['top_games']).map(CountItem.fromJson).toList(),
      leaderboard: asList(map['leaderboard'])
          .map(LeaderEntry.fromJson)
          .toList(),
      weightBands: asList(map['weight_bands']).map(BandCount.fromJson).toList(),
      shelf: ShelfSummary.fromJson(
        map['shelf'] is Map ? lowerKeys(map['shelf']) : const {},
      ),
    );
  }
}

class SyncRun {
  const SyncRun({
    required this.id,
    required this.status,
    required this.phase,
    required this.collectionCount,
    required this.thingsCount,
    required this.playsCount,
    required this.error,
    required this.startedAt,
    required this.finishedAt,
  });

  final String id;
  final String status;
  final String phase;
  final int collectionCount;
  final int thingsCount;
  final int playsCount;
  final String error;
  final String startedAt;
  final String finishedAt;

  bool get isRunning => status == 'running';
  bool get isFinished =>
      status == 'success' || status == 'partial' || status == 'failed';

  String get summary {
    final bits = <String>[];
    if (collectionCount > 0) bits.add('$collectionCount collection');
    if (thingsCount > 0) bits.add('$thingsCount enriched');
    if (playsCount > 0) bits.add('$playsCount plays');
    return bits.isEmpty ? 'No changes yet' : bits.join(' · ');
  }

  factory SyncRun.fromJson(Map<String, dynamic> json) {
    final map = lowerKeys(json);
    return SyncRun(
      id: stringOf(map, 'id'),
      status: stringOf(map, 'status', 'unknown'),
      phase: stringOf(map, 'phase'),
      collectionCount: intOf(map, 'collection_count'),
      thingsCount: intOf(map, 'things_count'),
      playsCount: intOf(map, 'plays_count'),
      error: stringOf(map, 'error'),
      startedAt: stringOf(map, 'started_at'),
      finishedAt: stringOf(map, 'finished_at'),
    );
  }
}
