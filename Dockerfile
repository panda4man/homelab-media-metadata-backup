# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/media-inventory ./cmd/media-inventory

FROM alpine:3.20
ARG TARGETARCH
ARG SUPERCRONIC_VERSION=v0.2.33

# rclone and tzdata come from Alpine's own repos; supercronic does not
# package for Alpine, so it is fetched directly from its GitHub release
# and the fetch tool (curl) is removed again in the same layer.
RUN apk add --no-cache ca-certificates tzdata rclone curl && \
    curl -fsSLo /usr/local/bin/supercronic \
      "https://github.com/aptible/supercronic/releases/download/${SUPERCRONIC_VERSION}/supercronic-linux-${TARGETARCH}" && \
    chmod +x /usr/local/bin/supercronic && \
    apk del curl && \
    addgroup -g 1000 -S app && \
    adduser -u 1000 -S -G app -H -h /app app

WORKDIR /app
COPY --from=builder /out/media-inventory /app/media-inventory
COPY cron/crontab /app/crontab

USER app

# CMD, not ENTRYPOINT: `docker compose run --rm media-inventory
# /app/media-inventory run` (the manual-execution path) replaces CMD
# entirely. An ENTRYPOINT here would instead append that command as
# extra arguments to supercronic, which is not what either invocation
# wants.
CMD ["/usr/local/bin/supercronic", "/app/crontab"]
