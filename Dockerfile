# syntax=docker/dockerfile:1

FROM node:24-alpine AS frontend
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json vite.config.ts ./
COPY web ./web
RUN npm run build

FROM golang:1.27-alpine AS build
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/internal/web/static ./internal/web/static
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/shelf ./cmd/server

FROM alpine:3.24
ARG VERSION=dev
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S shelf && adduser -S -G shelf -h /app shelf && \
    mkdir -p /data && chown shelf:shelf /data
WORKDIR /app
COPY --from=build /out/shelf /app/shelf
USER shelf
ENV PORT=6290 \
    DATABASE_PATH=/data/boardgames.db \
    TZ=UTC \
    VERSION=${VERSION}
EXPOSE 6290
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:6290/healthz >/dev/null || exit 1
ENTRYPOINT ["/app/shelf"]
