package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestProgressWritesForWriteOperations(t *testing.T) {
	var stderr bytes.Buffer
	stop := startProgress(&stderr, contractv1.OpIngest, true)
	time.Sleep(650 * time.Millisecond)
	stop()
	if !strings.Contains(stderr.String(), "weave: ingest") {
		t.Fatalf("progress missing: %q", stderr.String())
	}
}

func TestProgressDisabledForReads(t *testing.T) {
	var stderr bytes.Buffer
	stop := startProgress(&stderr, contractv1.OpSearch, true)
	time.Sleep(600 * time.Millisecond)
	stop()
	if stderr.Len() != 0 {
		t.Fatalf("unexpected progress: %q", stderr.String())
	}
}

func TestWriterIsTerminalRejectsBuffer(t *testing.T) {
	if writerIsTerminal(&bytes.Buffer{}) {
		t.Fatal("bytes.Buffer must not be treated as a terminal")
	}
}
