#!/usr/bin/env bash
# The one entry point for the whole suite — used locally, by the branch guard
# before every push, and by CI. Stages:
#   1. shell tests (branch guard)
#   2. go vet + build
#   3. go test against a throwaway PostGIS
#   4. frontend lint + unit tests
#   5. playwright end-to-end against the compose stack
# Stages whose project is not scaffolded yet are skipped, not failed.
set -euo pipefail
cd "$(dirname "$0")"

# Each checkout gets its own compose project and host ports, so a suite running
# in a parallel worktree does not tear down this one's containers.
SLOT=$(( $(cksum <<< "$PWD" | cut -d' ' -f1) % 200 ))
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-ssk-$SLOT}"
export TEST_DB_PORT="${TEST_DB_PORT:-$((55000 + SLOT))}"
export API_PORT="${API_PORT:-$((8400 + SLOT))}"
export WEB_PORT="${WEB_PORT:-$((8600 + SLOT))}"

GO_IMAGE="golang:1.27-alpine"
NODE_IMAGE="node:26-alpine"

# Keep the module and npm caches outside the source tree but stable across runs,
# so the docker fallback is not slower than a local toolchain on every invocation.
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/schulsicherheitskarte"
mkdir -p "$CACHE_DIR/go-mod" "$CACHE_DIR/go-build" "$CACHE_DIR/npm"

go_run() {
  if command -v go > /dev/null; then
    (cd backend && go "$@")
  else
    docker run --rm --network host \
      -v "$PWD/backend:/src" -w /src \
      -v "$CACHE_DIR/go-mod:/go/pkg/mod" -v "$CACHE_DIR/go-build:/root/.cache/go-build" \
      -e TEST_DATABASE_URL -e CGO_ENABLED=0 \
      "$GO_IMAGE" go "$@"
  fi
}

npm_run() {
  if command -v npm > /dev/null; then
    (cd frontend && npm "$@")
  else
    docker run --rm --network host \
      -v "$PWD/frontend:/src" -w /src -v "$CACHE_DIR/npm:/root/.npm" \
      "$NODE_IMAGE" npm "$@"
  fi
}

echo "🐚 Shell script tests"
.claude/hooks/tests/branch-guard-test.sh

if [[ -f backend/go.mod ]]; then
  echo "🔨 Go vet and build"
  go_run vet ./...
  go_run build ./...

  echo "🧪 Go tests"
  docker compose -f deploy/docker-compose.test.yml up -d --wait
  trap 'docker compose -f deploy/docker-compose.test.yml down -v --remove-orphans > /dev/null 2>&1 || true' EXIT
  export TEST_DATABASE_URL="postgres://ssk:ssk@127.0.0.1:${TEST_DB_PORT}/ssk_test?sslmode=disable"
  go_run test ./...
else
  echo "⏭️  Go backend not scaffolded yet — skipping stages 2 and 3"
fi

if [[ -f frontend/package.json ]]; then
  echo "⚛️  Frontend lint and unit tests"
  [[ -d frontend/node_modules ]] || npm_run ci
  npm_run run lint
  npm_run run test
else
  echo "⏭️  Frontend not scaffolded yet — skipping stage 4"
fi

if [[ -f e2e/package.json ]]; then
  echo "🎭 End-to-end tests"
  (cd e2e && ./run.sh)
else
  echo "⏭️  No e2e suite yet — skipping stage 5"
fi

echo "✅ All tests passed"
