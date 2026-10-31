#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

go mod tidy
git diff --exit-code -- go.mod go.sum

go run ./cmd/contract-export
git diff --exit-code -- contract/v1/golden/expected
go run ./cmd/schema-diff

go test -count=1 -shuffle=on -race -timeout=15m ./...
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /tmp/weave-verify ./cmd/weave
go test ./cmd/weave -run '^TestSmoke$' -count=1
rm -f /tmp/weave-verify

go test ./engine/okf -run '^$' -fuzz '^FuzzExtractFrontmatter$' -fuzztime=5s
go test ./engine/okf -run '^$' -fuzz '^FuzzExtractLinks$' -fuzztime=5s
go test ./engine/okf -run '^$' -fuzz '^FuzzConceptID$' -fuzztime=5s

go run ./cmd/docs-lint

unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
	echo "gofmt needed:"
	echo "$unformatted"
	exit 1
fi

go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7

echo "verify: ok"
