# Deploying The Shelf

The Shelf is designed for one small instance behind an existing reverse proxy.
It listens on port `6290`, stores everything in one SQLite file, and needs no
external services except an OIDC provider.

## 1. Prepare the environment

```sh
cp .env.example .env
```

Set at least:

```env
ENV=production
BASE_URL=https://shelf.example.com
SESSION_SECRET=<openssl rand -base64 48>
OIDC_ISSUER=https://auth.example.com
OIDC_CLIENT_ID=the-shelf
OIDC_CLIENT_SECRET=...
BGG_USER_AGENT=TheShelf/1.0 (you@example.com)
TRUSTED_PROXIES=172.16.0.0/12
```

`BASE_URL` must be the public HTTPS origin. The OIDC redirect URI is
`${BASE_URL}/auth/callback` unless you set `OIDC_REDIRECT_URL` explicitly.
Register that exact URI with your provider.

`TRUSTED_PROXIES` should contain only the network your proxy connects from.
Loopback is the default, which is right when the proxy and container share a
host network namespace or a local bridge. Do not use `0.0.0.0/0`.

## 2. Start the container

```sh
docker compose up -d --build
docker compose logs -f shelf
```

Compose binds to `127.0.0.1:6290`. To use a different host address, set
`BIND_ADDR`. The named volume `shelf-data` holds `/data/boardgames.db` and the
cover cache at `/data/covers`.

The image runs as an unprivileged `shelf` user. The only writable paths it needs
are `/data` and `/tmp`.

## 3. Reverse proxy

### Caddy

```caddyfile
shelf.example.com {
    encode zstd gzip
    reverse_proxy 127.0.0.1:6290 {
        header_up X-Request-ID {http.request.uuid}
    }
}
```

Caddy sets `X-Forwarded-For` and `X-Forwarded-Proto` correctly by default. If
Caddy runs in a container, add its Docker subnet to `TRUSTED_PROXIES`.

### nginx

```nginx
server {
    listen 443 ssl http2;
    server_name shelf.example.com;

    client_max_body_size 2m;

    location / {
        proxy_pass http://127.0.0.1:6290;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_read_timeout 90s;
    }

    location /static/ {
        proxy_pass http://127.0.0.1:6290;
        proxy_cache_valid 200 1h;
        add_header Cache-Control "public, max-age=3600";
    }
}
```

The app already sets immutable cache headers for hashed assets; letting the
proxy pass them through is sufficient.

## 4. First sign-in

Visit `https://shelf.example.com/login`. The first successfully authenticated
user becomes an admin. If you configured `ALLOWED_EMAILS` or
`ALLOWED_DOMAINS`, only matching accounts can enter.

Then open **Settings → BoardGameGeek**, enter your public BGG username, save,
and run the first sync. Private BGG collections are not supported.

## 5. Health and metrics

- `GET /healthz` — process is alive.
- `GET /readyz` — SQLite responds.
- `GET /metrics` — Prometheus, if `METRICS_ENABLED=true`.

Point your uptime monitor at `/readyz`. Keep `/metrics` on the private network
unless you add proxy authentication.

## 6. Upgrades

```sh
git pull
docker compose build
docker compose up -d
```

Migrations run automatically and transactionally at startup. Back up
`/data/boardgames.db` first (see [backup.md](backup.md)). The container gets a
15-second HTTP drain and a 20-second compose stop grace period.

## 7. Observability and analytics

Set `OTEL_EXPORTER_OTLP_ENDPOINT` (for example `http://otel-collector:4318`) to
export traces and metrics. Logs are JSON in production and include request IDs
and trace correlation.

To enable Umami, set `UMAMI_URL` and `UMAMI_WEBSITE_ID`. The script is injected
only when both are present, and the CSP automatically allows that origin for
both script loading and the `/api/send` beacon. Events are limited to
`theme_changed`, `shelf_search` (a `has_query` boolean, never the query text),
`play_submitted`, and `sync_started`. No game, player, or note data is sent.

## Single-user versus multi-user

The schema is multi-user and OIDC allows more people in, but v1 intentionally
has no user administration. For a private single-user instance, set
`ALLOWED_EMAILS` to your address. For a household, list each address or use a
domain allowlist. Everyone sees their own shelf and plays; the game catalogue
is shared and read-only.
