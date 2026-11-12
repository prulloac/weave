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
printf '%s' '{"v":"weave.source/v1","type":"source_response","snapshot":{"path":"/cache/weave/sources/abc/worktree","source_id":"abc","commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c","ref":"main","uri":"git+https://example.com/team/weave.git","read_only":true},"repository":{"id":"_repo","name":"weave","title":"Weave","description":"Local-first database","readme":"README.md","host":"example.com","owner":"team"},"warnings":[]}'
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
	if response.Snapshot.SourceID != "abc" || response.Snapshot.Commit == "" {
		t.Fatalf("unexpected snapshot: %+v", response.Snapshot)
	}
	if response.Repository.ID != "_repo" || response.Repository.Name != "weave" {
		t.Fatalf("unexpected repository: %+v", response.Repository)
	}
}

func TestMaterializeSourceValidation(t *testing.T) {
	cases := map[string]string{
		"wrong-version": `{"v":"weave.cli/v1","snapshot":{"path":"/cache/worktree","source_id":"abc","commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c"},"repository":{"id":"_repo","name":"weave"}}`,
		"source-id":     `{"v":"weave.source/v1","snapshot":{"path":"/cache/worktree","source_id":"other","commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c"},"repository":{"id":"_repo","name":"weave"}}`,
		"no-commit":     `{"v":"weave.source/v1","snapshot":{"path":"/cache/worktree","source_id":"abc"},"repository":{"id":"_repo","name":"weave"}}`,
		"bad-commit":    `{"v":"weave.source/v1","snapshot":{"path":"/cache/worktree","source_id":"abc","commit":"main"},"repository":{"id":"_repo","name":"weave"}}`,
		"relative-path": `{"v":"weave.source/v1","snapshot":{"path":"worktree","source_id":"abc","commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c"},"repository":{"id":"_repo","name":"weave"}}`,
		"no-repository": `{"v":"weave.source/v1","snapshot":{"path":"/cache/worktree","source_id":"abc","commit":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c"}}`,
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
