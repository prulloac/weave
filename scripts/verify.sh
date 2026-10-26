#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

bun run contract:types
bun run contract:export
bun run contract:check
bun test ./tests/unit
bun run typecheck
bun run lint:md

go test -race ./...
go vet ./...
go build -o /tmp/weave-verify ./cmd/weave
rm -f /tmp/weave-verify

unformatted="$(gofmt -l cmd contract engine mount render)"
if [ -n "$unformatted" ]; then
	echo "gofmt needed:"
	echo "$unformatted"
	exit 1
fi

echo "verify: ok"
