#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

go run ./cmd/contract-export
go run ./cmd/schema-diff
go test -race ./...
go vet ./...
go build -o /tmp/weave-verify ./cmd/weave
rm -f /tmp/weave-verify

go run ./cmd/docs-lint

unformatted="$(gofmt -l cmd contract engine)"
if [ -n "$unformatted" ]; then
	echo "gofmt needed:"
	echo "$unformatted"
	exit 1
fi

echo "verify: ok"
