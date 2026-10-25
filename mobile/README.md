# The Shelf for Android

A warm, quiet companion for your BoardGameGeek shelf.

- Browse your collection with search, filters, and a cover-forward grid.
- Open a game for metadata, shelf notes, and recent plays.
- Log plays with players, scores, duration, and notes.
- Review your play timeline and yearly statistics.
- Connect other instances with a personal API token.

## Run

The app targets the same `/api/v1` API as the web app. Create a token under
**Settings → API tokens** on your Shelf instance, then enter the server URL and
token on first launch.

```sh
flutter pub get
flutter analyze
flutter test
flutter run
```

Release builds need Java 17:

```sh
JAVA_HOME=/usr/lib/jvm/java-17-openjdk flutter build apk --release
```

See [../docs/mobile.md](../docs/mobile.md) for signing, telemetry, and
configuration.
