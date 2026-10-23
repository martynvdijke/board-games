# Backup and restore

Everything that matters lives under `/data` in the container:

- `boardgames.db` — users, shelf, plays, sync history.
- `boardgames.db-wal` / `boardgames.db-shm` — SQLite WAL sidecars.
- `covers/` — cached BGG images (regenerable, but nice to keep).

## Consistent online backup

The safest way to copy a live database is SQLite's own backup command. From the
host:

```sh
docker compose exec -T shelf sh -c \
  'sqlite3 /data/boardgames.db ".backup /data/backup-$(date +%F).db"'
```

The `sqlite3` CLI is not installed in the runtime image by default. Either use
the image's WAL-aware file copy while the app is stopped, or install the CLI in
a derived image. A simple, safe alternative is to pause writes:

```sh
docker compose stop shelf
docker run --rm -v shelf-data:/data -v "$PWD/backups:/backup" alpine \
  sh -c 'tar czf /backup/shelf-$(date +%F).tar.gz -C /data boardgames.db boardgames.db-wal boardgames.db-shm covers'
docker compose start shelf
```

For a hot copy without stopping, use Python's `sqlite3` backup API from a
temporary container:

```sh
docker run --rm -v shelf-data:/data -v "$PWD/backups:/backup" python:3.12-alpine \
  python -c "import sqlite3,datetime; s=sqlite3.connect('/data/boardgames.db'); d=sqlite3.connect('/backup/shelf-'+datetime.date.today().isoformat()+'.db'); s.backup(d); d.close(); s.close()"
```

Keep copies off the host. A daily cron or systemd timer is plenty for a
personal shelf.

## Restore

```sh
docker compose stop shelf
docker run --rm -v shelf-data:/data -v "$PWD/backups:/backup" alpine \
  sh -c 'rm -f /data/boardgames.db*; cp /backup/shelf-YYYY-MM-DD.db /data/boardgames.db'
docker compose start shelf
```

If you restore only the database without `covers/`, the image proxy will refill
covers on demand from BoardGameGeek.

## Verify a backup

```sh
sqlite3 backups/shelf-YYYY-MM-DD.db 'PRAGMA integrity_check;'
```

Expected output: `ok`.

## What is not backed up

- OIDC sessions: users simply sign in again.
- API tokens: they are in the database and survive a restore.
- Environment secrets: keep `.env` in your password manager or secret store.
