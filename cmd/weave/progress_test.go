package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

type progressBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	wrote  chan struct{}
}

func newProgressBuffer() *progressBuffer {
	return &progressBuffer{wrote: make(chan struct{}, 1)}
}

func (buffer *progressBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	written, err := buffer.buffer.Write(data)
	select {
	case buffer.wrote <- struct{}{}:
	default:
	}
	return written, err
}

func (buffer *progressBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.String()
}

func TestProgressWritesForWriteOperations(t *testing.T) {
	writer := newProgressBuffer()
	stop := startProgressEvery(writer, contractv1.OpIngest, true, time.Millisecond)
	select {
	case <-writer.wrote:
	case <-time.After(time.Second):
		stop()
		t.Fatal("progress did not write within one second")
	}
	stop()
	if !strings.Contains(writer.String(), "weave: ingest") {
		t.Fatalf("progress missing: %q", writer.String())
	}
}

func TestProgressDisabledForReads(t *testing.T) {
	writer := newProgressBuffer()
	stop := startProgressEvery(writer, contractv1.OpSearch, true, time.Millisecond)
	stop()
	if writer.String() != "" {
		t.Fatalf("unexpected progress: %q", writer.String())
	}
}

func TestWriterIsTerminalRejectsBuffer(t *testing.T) {
	if writerIsTerminal(&bytes.Buffer{}) {
		t.Fatal("bytes.Buffer must not be treated as a terminal")
	}
}
