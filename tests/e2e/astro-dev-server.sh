#!/usr/bin/env bash
set -e

PORT="${1:-4323}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Start astro dev in background
"$REPO/node_modules/.bin/astro" dev --port "$PORT" --host &
ASTRO_PID=$!

# Wait for the server to be ready
for i in $(seq 1 30); do
  if curl -s -o /dev/null "http://localhost:$PORT" 2>/dev/null; then
    echo "Astro dev server ready on port $PORT"
    # Keep this script alive so Playwright doesn't see it exit
    wait $ASTRO_PID
    exit $?
  fi
  sleep 1
done

echo "Astro dev server failed to start on port $PORT"
kill $ASTRO_PID 2>/dev/null
exit 1
