package main

import (
	"strings"
	"testing"
)

func TestParseMCPOptions(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		options, err := parseMCPOptions([]string{"--bundle", "/notes"})
		if err != nil {
			t.Fatal(err)
		}
		if options.listen != "127.0.0.1:7331" || options.topKDefault != 10 || options.stdio {
			t.Fatalf("unexpected defaults: %+v", options)
		}
	})

	t.Run("stdio", func(t *testing.T) {
		options, err := parseMCPOptions([]string{"--bundle", "/notes", "--stdio", "--token", "secret"})
		if err != nil {
			t.Fatal(err)
		}
		if !options.stdio || options.token != "secret" {
			t.Fatalf("unexpected stdio options: %+v", options)
		}
	})

	t.Run("custom listen", func(t *testing.T) {
		options, err := parseMCPOptions([]string{
			"--bundle", "/notes", "--listen", "127.0.0.1:9000", "--top-k-default", "20",
		})
		if err != nil {
			t.Fatal(err)
		}
		if options.listen != "127.0.0.1:9000" || options.topKDefault != 20 {
			t.Fatalf("unexpected listen options: %+v", options)
		}
	})

	t.Run("stdio and listen are exclusive", func(t *testing.T) {
		_, err := parseMCPOptions([]string{
			"--bundle", "/notes", "--stdio", "--listen", "127.0.0.1:9000",
		})
		if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("expected mutual exclusion error, got %v", err)
		}
	})

	t.Run("bundle is required", func(t *testing.T) {
		_, err := parseMCPOptions([]string{"--stdio"})
		if err == nil || !strings.Contains(err.Error(), "--bundle") {
			t.Fatalf("expected bundle error, got %v", err)
		}
	})

	t.Run("non-loopback requires token", func(t *testing.T) {
		_, err := parseMCPOptions([]string{"--bundle", "/notes", "--listen", "0.0.0.0:7331"})
		if err == nil || !strings.Contains(err.Error(), "--token") {
			t.Fatalf("expected token error, got %v", err)
		}
		options, err := parseMCPOptions([]string{
			"--bundle", "/notes", "--listen", "0.0.0.0:7331", "--token", "secret",
		})
		if err != nil || options.token != "secret" {
			t.Fatalf("expected non-loopback token config, got %+v %v", options, err)
		}
	})

	t.Run("top-k range", func(t *testing.T) {
		_, err := parseMCPOptions([]string{"--bundle", "/notes", "--top-k-default", "101"})
		if err == nil || !strings.Contains(err.Error(), "top-k-default") {
			t.Fatalf("expected top-k error, got %v", err)
		}
	})

	t.Run("watch is deferred", func(t *testing.T) {
		_, err := parseMCPOptions([]string{"--bundle", "/notes", "--watch"})
		if err == nil || !strings.Contains(err.Error(), "watch") {
			t.Fatalf("expected watch error, got %v", err)
		}
	})
}
