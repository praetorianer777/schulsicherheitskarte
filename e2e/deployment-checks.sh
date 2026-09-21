#!/usr/bin/env bash
# Checks that belong to the deployment rather than to a feature: that the stack
# an operator starts is healthy, unprivileged, and keeps its data. Called by
# run.sh with the stack already up; every check names what it found.
set -euo pipefail

COMPOSE="$1"
WEB="$2"
PROJECT="$3"

fail() { echo "❌ $*" >&2; exit 1; }

echo "   deployment: every service healthy or exited cleanly"
# A pipe as separator: a service without a healthcheck prints an empty health,
# which whitespace splitting would silently shift into the next column.
$COMPOSE ps -a --format '{{.Service}}|{{.State}}|{{.Health}}|{{.ExitCode}}' | while IFS='|' read -r service state health code; do
  case "$service:$state:$health:$code" in
    migrate:exited:*:0) ;;
    *:running:healthy:*) ;;
    *) fail "$service is $state ($health, exit $code)" ;;
  esac
done

echo "   deployment: /healthz answers through the web entry point"
body=$(curl -sf "$WEB/healthz") || fail "/healthz not reachable through the web port"
[[ "$body" == *'"status":"ok"'* ]] || fail "/healthz through the web port answered: $body"

echo "   deployment: the API is reached through the same origin as the page"
curl -sf "$WEB/api/extent" > /dev/null || fail "/api/extent not reachable through the web port"

echo "   deployment: no service runs as root"
# Checked on the images, not on containers: the importer is a job and has no
# container left once it has run, and the image is what a fresh machine gets.
for service in api migrate importer web; do
  # --profile tools, or the importer is not in the rendered config at all.
  image=$($COMPOSE --profile tools config --format json | python3 -c "import json,sys; print(json.load(sys.stdin)['services']['$service']['image'])")
  user=$(docker image inspect --format '{{.Config.User}}' "$image")
  case "$user" in
    ""|root|0|0:0) fail "$service ($image) runs as root" ;;
  esac
done

echo "   deployment: the importer can write its cache"
# The importer image has no shell, so the volume is probed from a throwaway
# container running as the same user the importer runs as.
volume=$(docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT" --filter "label=com.docker.compose.volume=importer-cache")
[[ -n "$volume" ]] || fail "no importer-cache volume in project $PROJECT"
docker run --rm --user 65532:65532 -v "$volume:/data/cache" alpine:3 \
  sh -c 'touch /data/cache/.probe && rm /data/cache/.probe' \
  || fail "the importer's cache volume is not writable by its user"
