import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'api/api_client.dart';
import 'api/models.dart';
import 'auth/auth_controller.dart';

final authProfileProvider = Provider<UserProfile?>(
  (ref) => ref.watch(authControllerProvider).profile,
);

final apiClientProvider = Provider<ApiClient>((ref) {
  final auth = ref.watch(authControllerProvider);
  final client = ApiClient(serverUrl: auth.serverUrl, token: auth.token);
  ref.onDispose(client.close);
  return client;
});

final gamesProvider = FutureProvider.autoDispose.family<GamesPage, GameQuery>((
  ref,
  query,
) async {
  return ref
      .watch(apiClientProvider)
      .games(
        query: query.search,
        status: query.status,
        sort: query.sort,
        limit: 60,
      );
});

final gameProvider = FutureProvider.autoDispose.family<GameDetail, int>((
  ref,
  id,
) async {
  return ref.watch(apiClientProvider).game(id);
});

final playsProvider = FutureProvider.autoDispose.family<PlaysPage, PlayQuery>((
  ref,
  query,
) async {
  return ref
      .watch(apiClientProvider)
      .plays(gameId: query.gameId, year: query.year, limit: 100);
});

final statsProvider = FutureProvider.autoDispose.family<Stats, int>((
  ref,
  year,
) async {
  return ref.watch(apiClientProvider).stats(year: year);
});

final syncProvider = FutureProvider.autoDispose<SyncRun?>((ref) async {
  return ref.watch(apiClientProvider).sync();
});

class GameQuery {
  const GameQuery({this.search = '', this.status = '', this.sort = 'name'});
  final String search;
  final String status;
  final String sort;

  @override
  bool operator ==(Object other) =>
      other is GameQuery &&
      other.search == search &&
      other.status == status &&
      other.sort == sort;

  @override
  int get hashCode => Object.hash(search, status, sort);
}

class PlayQuery {
  const PlayQuery({this.gameId, this.year});
  final int? gameId;
  final int? year;

  @override
  bool operator ==(Object other) =>
      other is PlayQuery && other.gameId == gameId && other.year == year;

  @override
  int get hashCode => Object.hash(gameId, year);
}
