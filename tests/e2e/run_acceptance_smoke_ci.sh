#!/bin/bash

set -euo pipefail

POSTGRES_IMAGE="${POSTGRES_IMAGE:-postgres:15-alpine}"
REDIS_IMAGE="${REDIS_IMAGE:-redis:7-alpine}"
ACCEPTANCE_PHASE="${ACCEPTANCE_PHASE:-auth}"
POSTGRES_CONTAINER="${ACCEPTANCE_PG_CONTAINER:-auto-healing-postgres-${ACCEPTANCE_PHASE}}"
REDIS_CONTAINER="${ACCEPTANCE_REDIS_CONTAINER:-auto-healing-redis-${ACCEPTANCE_PHASE}}"
INIT_ADMIN_PASSWORD="${INIT_ADMIN_PASSWORD:-admin123456}"
READINESS_TIMEOUT_SECONDS="${READINESS_TIMEOUT_SECONDS:-30}"

resolve_host_port() {
  local container="$1"
  local container_port="$2"
  docker port "$container" "${container_port}/tcp" | head -n 1 | awk -F: '{print $NF}'
}

cleanup() {
  docker rm -f "$POSTGRES_CONTAINER" "$REDIS_CONTAINER" >/dev/null 2>&1 || true
}

wait_for_postgres() {
  local i
  for ((i = 1; i <= READINESS_TIMEOUT_SECONDS; i++)); do
    if docker exec "$POSTGRES_CONTAINER" pg_isready -U postgres >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "postgres readiness timeout" >&2
  return 1
}

wait_for_redis() {
  local i
  for ((i = 1; i <= READINESS_TIMEOUT_SECONDS; i++)); do
    if docker exec "$REDIS_CONTAINER" redis-cli ping >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "redis readiness timeout" >&2
  return 1
}

trap cleanup EXIT

cleanup

if [ -n "${HOST_POSTGRES_PORT:-}" ]; then
  docker run -d --name "$POSTGRES_CONTAINER" \
    -p "${HOST_POSTGRES_PORT}:5432" \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=postgres \
    -e POSTGRES_DB=postgres \
    "$POSTGRES_IMAGE" >/dev/null
else
  docker run -d --name "$POSTGRES_CONTAINER" \
    -p "127.0.0.1::5432" \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=postgres \
    -e POSTGRES_DB=postgres \
    "$POSTGRES_IMAGE" >/dev/null
  HOST_POSTGRES_PORT="$(resolve_host_port "$POSTGRES_CONTAINER" 5432)"
fi

if [ -n "${HOST_REDIS_PORT:-}" ]; then
  docker run -d --name "$REDIS_CONTAINER" \
    -p "${HOST_REDIS_PORT}:6379" \
    "$REDIS_IMAGE" >/dev/null
else
  docker run -d --name "$REDIS_CONTAINER" \
    -p "127.0.0.1::6379" \
    "$REDIS_IMAGE" >/dev/null
  HOST_REDIS_PORT="$(resolve_host_port "$REDIS_CONTAINER" 6379)"
fi

wait_for_postgres
wait_for_redis

INIT_ADMIN_PASSWORD="$INIT_ADMIN_PASSWORD" \
ACCEPTANCE_PG_CONTAINER="$POSTGRES_CONTAINER" \
ACCEPTANCE_REDIS_CONTAINER="$REDIS_CONTAINER" \
DATABASE_PORT="$HOST_POSTGRES_PORT" \
REDIS_PORT="$HOST_REDIS_PORT" \
KEEP_ACCEPTANCE_ARTIFACTS=0 \
python3.11 tests/e2e/test_acceptance_real.py --phase "$ACCEPTANCE_PHASE"
