package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

// Behavior-level tests for `weave source diff`. See docs/bdd/source-diff.md.

func sourceDiffWorkspace(t *testing.T, fixture string) string {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, ".weave", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("testdata", "extensions", fixture))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "weave-git"), data, 0o755); err != nil {
		t.Fatal(err)
	}
	trustTestExtension(t, dir, "git")
	manifest := "version: 1\nsources:\n  docs:\n    git: /tmp/repo\n"
	if err := os.WriteFile(filepath.Join(dir, workspaceManifestName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(dir, ".weave", "index.db")
	store, err := sqlitestore.Open(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertSource(context.Background(), sqlitestore.SourceRecord{
		Alias: "docs", Path: "/tmp/repo", Status: sqlitestore.SourceActive, Generation: 1, Provider: "git",
	}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	store.Close()
	return dir
}

func decodeSourceDiff(t *testing.T, stdout string) contractv1.SourceDiffResult {
	t.Helper()
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("unexpected failure: %+v", envelope.Error)
	}
	var result contractv1.SourceDiffResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("decode diff result: %v\n%s", err, stdout)
	}
	return result
}

func TestSourceDiffJSON(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	dir := sourceDiffWorkspace(t, "source-diff-git.sh")
	code, stdout, stderr := Invoke(t, "source", "diff", "docs", "--workspace", dir,
		"--from", "a", "--to", "b", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	result := decodeSourceDiff(t, stdout)
	if result.From != "1111111" || result.To != "2222222" {
		t.Fatalf("unexpected refs: %+v", result)
	}
	if len(result.Changes) != 4 {
		t.Fatalf("changes = %+v", result.Changes)
	}
	order := []string{"assets/logo.bin", "docs/changed.md", "docs/new-name.md", "docs/new.md"}
	for index, path := range order {
		if result.Changes[index].Path != path {
			t.Fatalf("changes are not sorted: %+v", result.Changes)
		}
	}
	for _, change := range result.Changes {
		if change.Binary && change.Patch != "" {
			t.Fatalf("binary change carries a patch: %+v", change)
		}
	}
}

func TestSourceDiffPatch(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	dir := sourceDiffWorkspace(t, "source-diff-git.sh")
	code, stdout, stderr := Invoke(t, "source", "diff", "docs", "--workspace", dir,
		"--from", "a", "--to", "b", "--format", "patch")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	for _, want := range []string{"diff --git", "rename from docs/old-name.md", "+beta"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("patch missing %q:\n%s", want, stdout)
		}
	}
}

func TestSourceDiffPatchOutFile(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	dir := sourceDiffWorkspace(t, "source-diff-git.sh")
	out := filepath.Join(t.TempDir(), "change.patch")
	code, stdout, stderr := Invoke(t, "source", "diff", "docs", "--workspace", dir,
		"--from", "a", "--to", "b", "--format", "patch", "--out", out)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout must be empty with --out, got %q", stdout)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "diff --git") {
		t.Fatalf("unexpected patch file:\n%s", data)
	}
}

func TestSourceDiffPathFilter(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	dir := sourceDiffWorkspace(t, "source-diff-git.sh")
	code, stdout, stderr := Invoke(t, "source", "diff", "docs", "--workspace", dir,
		"--from", "a", "--to", "b", "--path", "docs/**", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	result := decodeSourceDiff(t, stdout)
	for _, change := range result.Changes {
		if !strings.HasPrefix(change.Path, "docs/") {
			t.Fatalf("path filter leaked %q", change.Path)
		}
	}
}

func TestSourceDiffUnknownAlias(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	dir := sourceDiffWorkspace(t, "source-diff-git.sh")
	code, stdout, _ := Invoke(t, "source", "diff", "missing", "--workspace", dir,
		"--from", "a", "--to", "b", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrSourceNotFound {
		t.Fatalf("expected SOURCE_NOT_FOUND, got %+v", envelope.Error)
	}
}

func TestSourceDiffUnsupportedProvider(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	dir := sourceDiffWorkspace(t, "source-no-diff.sh")
	code, stdout, _ := Invoke(t, "source", "diff", "docs", "--workspace", dir,
		"--from", "a", "--to", "b", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrSourceUnsupported {
		t.Fatalf("expected SOURCE_UNSUPPORTED, got %+v", envelope.Error)
	}
}

func TestSourceDiffMissingRefs(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	dir := sourceDiffWorkspace(t, "source-diff-git.sh")
	code, stdout, _ := Invoke(t, "source", "diff", "docs", "--workspace", dir,
		"--from", "a", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrInvalidRequest {
		t.Fatalf("expected INVALID_REQUEST, got %+v", envelope.Error)
	}
}

func TestSourceDiffPatchWithJSON(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	dir := sourceDiffWorkspace(t, "source-diff-git.sh")
	code, _, stderr := Invoke(t, "source", "diff", "docs", "--workspace", dir,
		"--from", "a", "--to", "b", "--format", "patch", "--json")
	if code != 2 || !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected a usage error, got %d %q", code, stderr)
	}
}

func TestSourceDiffProviderFailures(t *testing.T) {
	cases := []struct {
		mode string
		code contractv1.ErrorCode
	}{
		{"fail", contractv1.ErrExtensionFailed},
		{"timeout", contractv1.ErrExtensionTimeout},
		{"malformed", contractv1.ErrExtensionProto},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			withCoreVersion(t, "0.11.0")
			t.Setenv("TEST_DIFF_MODE", c.mode)
			dir := sourceDiffWorkspace(t, "source-diff-git.sh")
			code, stdout, _ := Invoke(t, "source", "diff", "docs", "--workspace", dir,
				"--from", "a", "--to", "b", "--timeout", "100ms", "--json")
			if code != 1 {
				t.Fatalf("expected exit 1, got %d stdout=%s", code, stdout)
			}
			envelope := decodeEnvelope(t, stdout)
			if envelope.Error == nil || envelope.Error.Code != c.code {
				t.Fatalf("expected %s, got %+v", c.code, envelope.Error)
			}
		})
	}
}

func TestSourceDiffDeterministic(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	dir := sourceDiffWorkspace(t, "source-diff-git.sh")
	firstCode, first, firstStderr := Invoke(t, "source", "diff", "docs", "--workspace", dir,
		"--from", "a", "--to", "b", "--format", "patch")
	secondCode, second, secondStderr := Invoke(t, "source", "diff", "docs", "--workspace", dir,
		"--from", "a", "--to", "b", "--format", "patch")
	if firstCode != 0 || secondCode != 0 {
		t.Fatalf("expected exit 0, got %d/%d stderr=%s/%s", firstCode, secondCode, firstStderr, secondStderr)
	}
	if first != second {
		t.Fatal("diff output is not deterministic")
	}
}

func TestSourceDiffDoesNotWrite(t *testing.T) {
	withCoreVersion(t, "0.11.0")
	dir := sourceDiffWorkspace(t, "source-diff-git.sh")
	before := treeDigest(t, dir)
	if code, _, stderr := Invoke(t, "source", "diff", "docs", "--workspace", dir,
		"--from", "a", "--to", "b", "--json"); code != 0 {
		t.Fatalf("expected exit 0, stderr=%s", stderr)
	}
	after := treeDigest(t, dir)
	if before != after {
		t.Fatalf("diff changed the workspace: %s != %s", before, after)
	}
}

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	digest := sha256.New()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest.Write([]byte(filepath.ToSlash(relative)))
		digest.Write(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}
