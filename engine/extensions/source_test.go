package extensions_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/prulloac/weave/engine/extensions"
)

func TestMaterializeSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "weave-git")
	writeScript(t, path, `cat >/dev/null
printf '%s' '{"v":"weave.source/v1","type":"source_response","snapshot":{"path":"/cache/weave/sources/abc/worktree","source_id":"abc","revision":"rev-8f3c","ref":"main","uri":"git+https://example.com/team/weave.git","read_only":true},"documents":[{"id":"_repo","type":"Repository","content_kind":"blob","title":"Weave"}],"warnings":[]}'
`)
	response, err := extensions.Runner{}.MaterializeSource(context.Background(), extensions.Extension{Name: "git", Path: path}, extensions.SourceRequest{
		SourceID: "abc",
		URI:      "git+https://example.com/team/weave.git",
		Ref:      "main",
		CacheDir: "/cache/weave",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Snapshot.SourceID != "abc" || response.Snapshot.Revision != "rev-8f3c" {
		t.Fatalf("unexpected snapshot: %+v", response.Snapshot)
	}
	if len(response.Documents) != 1 || response.Documents[0].ID != "_repo" {
		t.Fatalf("unexpected documents: %+v", response.Documents)
	}
}

func TestMaterializeSourceValidation(t *testing.T) {
	cases := map[string]string{
		"wrong-version":   `{"v":"weave.cli/v1","snapshot":{"path":"/cache/worktree","source_id":"abc","revision":"rev"},"documents":[{"id":"_repo","type":"Repository"}]}`,
		"source-id":       `{"v":"weave.source/v1","snapshot":{"path":"/cache/worktree","source_id":"other","revision":"rev"},"documents":[{"id":"_repo","type":"Repository"}]}`,
		"no-revision":     `{"v":"weave.source/v1","snapshot":{"path":"/cache/worktree","source_id":"abc"},"documents":[{"id":"_repo","type":"Repository"}]}`,
		"no-documents":    `{"v":"weave.source/v1","snapshot":{"path":"/cache/worktree","source_id":"abc","revision":"rev"}}`,
		"relative-path":   `{"v":"weave.source/v1","snapshot":{"path":"worktree","source_id":"abc","revision":"rev"},"documents":[{"id":"_repo","type":"Repository"}]}`,
		"unsafe-artifact": `{"v":"weave.source/v1","snapshot":{"path":"/cache/worktree","source_id":"abc","revision":"rev"},"documents":[{"id":"_repo","type":"Repository"}],"artifacts":[{"path":"../escape.json","content":{}}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "weave-git")
			writeScript(t, path, "cat >/dev/null\nprintf '%s' '"+body+"'\n")
			_, err := extensions.Runner{}.MaterializeSource(context.Background(), extensions.Extension{Name: "git", Path: path}, extensions.SourceRequest{
				SourceID: "abc",
				URI:      "git+https://example.com/team/weave.git",
				Ref:      "main",
			})
			if !errors.Is(err, extensions.ErrProtocol) {
				t.Fatalf("expected protocol error, got %v", err)
			}
		})
	}
}

func TestManifestSourceProviders(t *testing.T) {
	manifest := extensions.Manifest{SourceProviders: []string{"git", "archive"}}
	if !manifest.SupportsSourceProvider("git") {
		t.Fatal("expected git source provider")
	}
	if !manifest.SupportsSourceProvider("archive") {
		t.Fatal("expected archive source provider")
	}
	if manifest.SupportsSourceProvider("svn") {
		t.Fatal("unexpected source provider")
	}
}

func TestSourceIDIsStableAndScoped(t *testing.T) {
	workspace := extensions.WorkspaceSourceID("/workspace", "core", "git+https://example.com/team/weave.git")
	same := extensions.WorkspaceSourceID("/workspace", "core", "git+https://example.com/team/weave.git")
	if workspace != same {
		t.Fatal("source id must be stable")
	}
	if workspace == extensions.WorkspaceSourceID("/other", "core", "git+https://example.com/team/weave.git") {
		t.Fatal("workspace root must scope the source id")
	}
	if workspace == extensions.WorkspaceSourceID("/workspace", "docs", "git+https://example.com/team/weave.git") {
		t.Fatal("alias must scope the source id")
	}
	if workspace == extensions.SingleSourceID("git+https://example.com/team/other.git") {
		t.Fatal("uri must scope the source id")
	}
}
