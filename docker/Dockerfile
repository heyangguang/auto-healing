# ── Stage 1: Build ──────────────────────────────────
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    -o /bin/server ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w" \
    -o /bin/init-admin ./cmd/init-admin

# ── Stage 2: Runtime ────────────────────────────────
FROM alpine:3.20

LABEL org.opencontainers.image.title="Auto-Healing Platform"
LABEL org.opencontainers.image.description="Enterprise-grade Intelligent IT Operations Self-Healing Platform"
LABEL org.opencontainers.image.source="https://github.com/heyangguang/auto-healing"
LABEL org.opencontainers.image.licenses="Apache-2.0"

RUN apk add --no-cache ca-certificates tzdata curl

RUN addgroup -g 1000 ahs && adduser -u 1000 -G ahs -s /bin/sh -D ahs

WORKDIR /app

COPY --from=builder /bin/server /app/server
COPY --from=builder /bin/init-admin /app/init-admin
COPY configs/config.yaml /app/configs/config.yaml
COPY migrations/ /app/migrations/

RUN chown -R ahs:ahs /app

USER ahs

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]
