package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func diffExtension(t *testing.T, response string) Extension {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script extensions are POSIX only")
	}
	path := filepath.Join(t.TempDir(), "weave-git")
	body := "#!/bin/sh\ncat >/dev/null\nprintf '%s' '" + response + "'\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return Extension{Name: "git", Path: path, Source: "bundle"}
}

func TestDiffSourceValidatesAndSorts(t *testing.T) {
	extension := diffExtension(t, `{"v":"weave.source/v1","type":"source_diff_response","from":"a","to":"b","changes":[{"kind":"modified","path":"z.md","patch":"x"},{"kind":"added","path":"a.md","binary":true,"patch":"ignored"}]}`)
	response, err := (Runner{Timeout: time.Second}).DiffSource(context.Background(), extension, SourceDiffRequest{
		SourceID: "src-test", URI: "/tmp/repo", From: "a", To: "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.From != "a" || response.To != "b" {
		t.Fatalf("refs = %s..%s", response.From, response.To)
	}
	if got := []string{response.Changes[0].Path, response.Changes[1].Path}; !reflect.DeepEqual(got, []string{"a.md", "z.md"}) {
		t.Fatalf("changes are not sorted: %v", got)
	}
	if response.Changes[0].Patch != "" || !response.Changes[0].Binary {
		t.Fatalf("binary change kept a patch: %+v", response.Changes[0])
	}
}

func TestDiffSourceRejectsInvalidResponses(t *testing.T) {
	cases := []struct {
		name     string
		response string
	}{
		{"unknown kind", `{"v":"weave.source/v1","type":"source_diff_response","from":"a","to":"b","changes":[{"kind":"teleported","path":"a.md"}]}`},
		{"unsafe path", `{"v":"weave.source/v1","type":"source_diff_response","from":"a","to":"b","changes":[{"kind":"added","path":"../a.md"}]}`},
		{"missing echo", `{"v":"weave.source/v1","type":"source_diff_response","changes":[]}`},
		{"duplicate path", `{"v":"weave.source/v1","type":"source_diff_response","from":"a","to":"b","changes":[{"kind":"added","path":"a.md"},{"kind":"modified","path":"a.md"}]}`},
		{"stray previous path", `{"v":"weave.source/v1","type":"source_diff_response","from":"a","to":"b","changes":[{"kind":"modified","path":"a.md","previous_path":"b.md"}]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			extension := diffExtension(t, c.response)
			_, err := (Runner{Timeout: time.Second}).DiffSource(context.Background(), extension, SourceDiffRequest{
				SourceID: "src-test", URI: "/tmp/repo", From: "a", To: "b",
			})
			if !errors.Is(err, ErrProtocol) {
				t.Fatalf("expected ErrProtocol, got %v", err)
			}
		})
	}
}

func TestDiffSourceRejectsMissingRefs(t *testing.T) {
	_, err := (Runner{Timeout: time.Second}).DiffSource(context.Background(), Extension{Name: "git"}, SourceDiffRequest{
		SourceID: "src-test", URI: "/tmp/repo", From: "", To: "b",
	})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected ErrProtocol, got %v", err)
	}
}

func TestManifestCapabilities(t *testing.T) {
	manifest := Manifest{Capabilities: []string{"repository", "diff"}}
	if !manifest.SupportsCapability("diff") {
		t.Fatal("diff capability not detected")
	}
	if manifest.SupportsCapability("merge") {
		t.Fatal("unexpected capability")
	}
}
