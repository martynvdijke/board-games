import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/src/api/models.dart';

void main() {
  test('Game parses snake_case payload and derives labels', () {
    final game = Game.fromJson({
      'id': 17,
      'bgg_id': 109,
      'name': 'A Feast for Odin',
      'year': 2016,
      'min_players': 1,
      'max_players': 4,
      'min_playtime': 30,
      'max_playtime': 120,
      'weight': 3.85,
      'user_rating': 8.5,
      'total_plays': 2,
      'favorite': true,
    });
    expect(game.name, 'A Feast for Odin');
    expect(game.playersLabel, '1–4 players');
    expect(game.timeLabel, '30–120 min');
    expect(game.playsLabel, '2 plays');
    expect(game.userRating, 8.5);
    expect(game.favorite, isTrue);
  });

  test('Stats accepts PascalCase nested structs from the Go API', () {
    final stats = Stats.fromJson({
      'year': 2026,
      'monthly': [
        {'Month': 1, 'Label': 'Jan', 'Count': 3},
      ],
      'total_plays': 3,
      'total_sessions': 2,
      'top_games': [
        {'Name': 'Wingspan', 'Count': 2},
      ],
      'leaderboard': [
        {'Name': 'Alex', 'Wins': 2, 'Plays': 3},
      ],
      'weight_bands': [
        {'Label': 'Light', 'Count': 1},
      ],
      'shelf': {'owned': 12, 'unplayed': 4, 'plays_this_year': 3},
    });
    expect(stats.monthly.single.label, 'Jan');
    expect(stats.topGames.single.name, 'Wingspan');
    expect(stats.leaderboard.single.wins, 2);
    expect(stats.shelf.owned, 12);
    expect(stats.maxMonth, 3);
  });

  test('Play parses players and labels local origin', () {
    final play = Play.fromJson({
      'id': 4,
      'game_id': 17,
      'game_name': 'Wingspan',
      'played_at': '2026-10-20',
      'origin': 'local',
      'players': [
        {'name': 'Alex', 'score': '42', 'is_winner': true},
      ],
    });
    expect(play.isLocal, isTrue);
    expect(play.playersLabel, 'Alex');
    expect(play.whenLabel, '20 Oct 2026');
  });

  test('Profile exposes initials', () {
    expect(
      const UserProfile(
        id: '1',
        email: 'a@b.c',
        name: 'Alex Doe',
        avatarUrl: '',
        role: 'admin',
        scopes: [],
      ).initials,
      'AD',
    );
  });
}
