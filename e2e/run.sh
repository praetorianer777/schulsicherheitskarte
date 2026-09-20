#!/usr/bin/env bash
# Brings up the stack the way an operator would, seeds it with fixed data and
# drives it with Playwright. Used by run-tests.sh as its last stage.
set -euo pipefail
cd "$(dirname "$0")"

COMPOSE="docker compose -f ../deploy/docker-compose.yml -p ${COMPOSE_PROJECT_NAME:-ssk}-e2e"

# Its own ports, so an e2e run does not collide with a stack somebody left
# running, or with the database the Go tests use.
export WEB_PORT="${E2E_WEB_PORT:-$(( 8700 + ${SLOT:-0} ))}"
export API_PORT="${E2E_API_PORT:-$(( 8900 + ${SLOT:-0} ))}"
export API_BIND=127.0.0.1
export WEB_BIND=127.0.0.1
export MODERATION_TOKEN="e2e-moderation-token"
export REPORT_SALT="e2e-salt"

cleanup() {
  $COMPOSE down -v --remove-orphans > /dev/null 2>&1 || true
}
# Tear the stack down however this ends, or the next run inherits its state.
trap cleanup EXIT

echo "   starting the stack on web:$WEB_PORT api:$API_PORT"
$COMPOSE up -d --wait --build

echo "   seeding"
$COMPOSE exec -T postgres psql -v ON_ERROR_STOP=1 -q -U "${POSTGRES_USER:-ssk}" -d "${POSTGRES_DB:-ssk}" < seed.sql
# The hotspots are computed by the importer rather than seeded, so the suite
# exercises the real clustering instead of numbers somebody typed in.
$COMPOSE run --rm importer hotspots

[[ -d node_modules ]] || npm ci
npx playwright install --no-shell chromium > /dev/null
npx playwright test
