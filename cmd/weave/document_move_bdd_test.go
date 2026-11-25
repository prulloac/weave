package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func moveWrite(t *testing.T, root, name, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func moveRead(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func moveIngest(t *testing.T, root, db string) {
	t.Helper()
	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--strict", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
}

func moveInvoke(t *testing.T, root, from, to string, extra ...string) (int, testEnvelope, string) {
	t.Helper()
	args := append([]string{"mv", root, from, to}, extra...)
	args = append(args, "--json")
	code, stdout, stderr := Invoke(t, args...)
	return code, decodeEnvelope(t, stdout), stderr
}

func moveResult(t *testing.T, envelope testEnvelope) contractv1.MoveResult {
	t.Helper()
	var result contractv1.MoveResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("decode move result: %v\n%s", err, envelope.Result)
	}
	return result
}

func moveFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	moveWrite(t, root, "notes/old.md", "---\ntype: Concept\ntitle: \"Old\"\nkeywords:\n  - \"old\"\n---\n\n# Old\n")
	moveWrite(t, root, "notes/home.md", "---\ntype: Concept\ntitle: \"Index\"\nkeywords:\n  - \"index\"\n---\n\nSee [old](old.md) and [other](other.md).\n")
	moveWrite(t, root, "notes/other.md", "---\ntype: Concept\ntitle: \"Other\"\nkeywords:\n  - \"other\"\n---\n\nBack to [old](../notes/old.md).\n")
	moveWrite(t, root, "notes/assets/logo.svg", "<svg/>\n")
	return root
}

func TestMoveRewritesRelativeLinks(t *testing.T) {
	root := moveFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, envelope, stderr := moveInvoke(t, root, "notes/old.md", "notes/archive/old.md", "--db", db)
	if code != 0 {
		t.Fatalf("mv exit=%d stderr=%q", code, stderr)
	}
	result := moveResult(t, envelope)
	if result.From != "notes/old.md" || result.To != "notes/archive/old.md" || result.DryRun {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(moveRead(t, root, "notes/home.md"), "[old](archive/old.md)") {
		t.Fatalf("index link not rewritten:\n%s", moveRead(t, root, "notes/home.md"))
	}
	if !strings.Contains(moveRead(t, root, "notes/other.md"), "[old](archive/old.md)") {
		t.Fatalf("other link not rewritten:\n%s", moveRead(t, root, "notes/other.md"))
	}
	if len(result.Rewritten) != 3 {
		t.Fatalf("rewritten = %+v", result.Rewritten)
	}
	if result.Rewritten[0].Path != "notes/archive/old.md" || result.Rewritten[0].References != 0 {
		t.Fatalf("moved document entry = %+v", result.Rewritten[0])
	}
}

func TestMovePreservesFragmentTitleAndImages(t *testing.T) {
	root := t.TempDir()
	moveWrite(t, root, "notes/old.md", "---\ntype: Concept\ntitle: \"Old\"\nkeywords:\n  - \"old\"\n---\n\n# Old\n\n## Section\n")
	moveWrite(t, root, "notes/home.md", "---\ntype: Concept\ntitle: \"Index\"\nkeywords:\n  - \"index\"\n---\n\n[section](old.md#section \"Section title\")\n\n![figure](old.md#figure)\n")
	code, _, stderr := moveInvoke(t, root, "notes/old.md", "notes/archive/old.md")
	if code != 0 {
		t.Fatalf("mv exit=%d stderr=%q", code, stderr)
	}
	content := moveRead(t, root, "notes/home.md")
	if !strings.Contains(content, `[section](archive/old.md#section "Section title")`) {
		t.Fatalf("fragment or title lost:\n%s", content)
	}
	if !strings.Contains(content, `![figure](archive/old.md#figure)`) {
		t.Fatalf("image target not rewritten:\n%s", content)
	}
}

func TestMoveRootRelativeAndReferenceDefinitions(t *testing.T) {
	root := t.TempDir()
	moveWrite(t, root, "notes/old.md", "---\ntype: Concept\ntitle: \"Old\"\nkeywords:\n  - \"old\"\n---\n\n# Old\n")
	moveWrite(t, root, "notes/home.md", "---\ntype: Concept\ntitle: \"Index\"\nkeywords:\n  - \"index\"\n---\n\n[root](/notes/old.md#top)\n\n[ref]: old.md \"Ref title\"\n\nUse [ref].\n\n![image][ref]\n")
	code, _, stderr := moveInvoke(t, root, "notes/old.md", "archive/old.md")
	if code != 0 {
		t.Fatalf("mv exit=%d stderr=%q", code, stderr)
	}
	content := moveRead(t, root, "notes/home.md")
	if !strings.Contains(content, "[root](/archive/old.md#top)") {
		t.Fatalf("root-relative link not rewritten:\n%s", content)
	}
	if !strings.Contains(content, `[ref]: ../archive/old.md "Ref title"`) {
		t.Fatalf("reference definition not rewritten:\n%s", content)
	}
	if !strings.Contains(content, "Use [ref].") || !strings.Contains(content, "![image][ref]") {
		t.Fatalf("reference uses must stay unchanged:\n%s", content)
	}
}

func TestMoveLeavesProtectedReferences(t *testing.T) {
	root := t.TempDir()
	moveWrite(t, root, "notes/old.md", "---\ntype: Concept\ntitle: \"Old\"\nkeywords:\n  - \"old\"\n---\n\n# Old\n")
	unchanged := "---\ntype: Concept\ntitle: \"Index\"\nkeywords:\n  - \"index\"\n---\n\n" +
		"[new](new.md) [external](https://example.com/old.md) [absolute](/elsewhere/old.md#x)\n" +
		"[anchor](#section)\n\n" +
		"```md\n[code](old.md)\n```\n\nInline `[code](old.md)` stays.\n\n<!-- [comment](old.md) -->\n"
	moveWrite(t, root, "notes/home.md", unchanged)
	moveWrite(t, root, "notes/new.md", "---\ntype: Concept\ntitle: \"New\"\nkeywords:\n  - \"new\"\n---\n\n# New\n")
	code, _, stderr := moveInvoke(t, root, "notes/old.md", "notes/renamed.md")
	if code != 0 {
		t.Fatalf("mv exit=%d stderr=%q", code, stderr)
	}
	if got := moveRead(t, root, "notes/home.md"); got != unchanged {
		t.Fatalf("protected references changed:\n--- want ---\n%s\n--- got ---\n%s", unchanged, got)
	}
}

func TestMoveRewritesRelated(t *testing.T) {
	root := t.TempDir()
	moveWrite(t, root, "notes/old.md", "---\ntype: Concept\ntitle: \"Old\"\nkeywords:\n  - \"old\"\n---\n\n# Old\n")
	moveWrite(t, root, "notes/home.md", "---\ntype: Concept\ntitle: \"Index\"\nkeywords:\n  - \"index\"\ntags:\n  - keep\nrelated:\n  - old\n  - ./notes/old\n  - notes/old\nother: untouched\n---\n\n# Index\n")
	code, _, stderr := moveInvoke(t, root, "notes/old.md", "notes/archive/old.md")
	if code != 0 {
		t.Fatalf("mv exit=%d stderr=%q", code, stderr)
	}
	content := moveRead(t, root, "notes/home.md")
	for _, want := range []string{"  - notes/archive/old", "  - ./notes/archive/old", "  - notes/archive/old", "other: untouched", "  - keep"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in:\n%s", want, content)
		}
	}
}

func TestMoveRebasesMovedDocumentLinks(t *testing.T) {
	root := t.TempDir()
	moveWrite(t, root, "notes/old.md", "---\ntype: Concept\ntitle: \"Old\"\nkeywords:\n  - \"old\"\n---\n\n[other](other.md)\n\n![logo](assets/logo.svg)\n")
	moveWrite(t, root, "notes/other.md", "---\ntype: Concept\ntitle: \"Other\"\nkeywords:\n  - \"other\"\n---\n\n# Other\n")
	moveWrite(t, root, "notes/assets/logo.svg", "<svg/>\n")
	code, _, stderr := moveInvoke(t, root, "notes/old.md", "notes/archive/old.md")
	if code != 0 {
		t.Fatalf("mv exit=%d stderr=%q", code, stderr)
	}
	content := moveRead(t, root, "notes/archive/old.md")
	if !strings.Contains(content, "[other](../other.md)") {
		t.Fatalf("document link not rebased:\n%s", content)
	}
	if !strings.Contains(content, "![logo](../assets/logo.svg)") {
		t.Fatalf("asset link not rebased:\n%s", content)
	}
}

func TestMoveDryRun(t *testing.T) {
	root := moveFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	moveIngest(t, root, db)
	before := moveRead(t, root, "notes/home.md")
	code, envelope, stderr := moveInvoke(t, root, "notes/old.md", "notes/archive/old.md", "--db", db, "--dry-run")
	if code != 0 {
		t.Fatalf("mv exit=%d stderr=%q", code, stderr)
	}
	result := moveResult(t, envelope)
	if !result.DryRun || len(result.Rewritten) == 0 {
		t.Fatalf("result = %+v", result)
	}
	if moveRead(t, root, "notes/home.md") != before {
		t.Fatal("dry run changed a file")
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "archive", "old.md")); !os.IsNotExist(err) {
		t.Fatal("dry run created the destination")
	}
	code, stdout, stderr := Invoke(t, "show", root, "notes/old", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("old document missing after dry run: exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
}

func TestMoveDestinationExists(t *testing.T) {
	root := moveFixture(t)
	moveWrite(t, root, "notes/archive/old.md", "---\ntype: Concept\ntitle: \"Taken\"\nkeywords:\n  - \"taken\"\n---\n\n# Taken\n")
	before := moveRead(t, root, "notes/old.md")
	code, envelope, _ := moveInvoke(t, root, "notes/old.md", "notes/archive/old.md")
	if code != 1 {
		t.Fatalf("mv exit=%d, want 1", code)
	}
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrDestinationExists {
		t.Fatalf("error = %+v", envelope.Error)
	}
	if moveRead(t, root, "notes/old.md") != before {
		t.Fatal("source changed after refusal")
	}
}

func TestMoveInvalidInputs(t *testing.T) {
	root := moveFixture(t)
	moveWrite(t, root, "notes/dir/nested.md", "---\ntype: Concept\ntitle: \"Nested\"\nkeywords:\n  - \"nested\"\n---\n\n# Nested\n")
	if err := os.Symlink(filepath.Join(root, "notes", "old.md"), filepath.Join(root, "notes", "link.md")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	for _, test := range []struct {
		from string
		to   string
	}{
		{"notes/old.txt", "notes/new.md"},
		{"notes/old.md", "notes/new.txt"},
		{"notes/dir", "notes/new.md"},
		{"notes/link.md", "notes/new.md"},
		{"../outside.md", "notes/new.md"},
		{"notes/old.md", "../outside.md"},
	} {
		code, envelope, _ := moveInvoke(t, root, test.from, test.to)
		if code != 2 {
			t.Fatalf("mv %s -> %s exit=%d, want 2", test.from, test.to, code)
		}
		if envelope.Error == nil || envelope.Error.Code != contractv1.ErrInvalidRequest {
			t.Fatalf("mv %s -> %s error = %+v", test.from, test.to, envelope.Error)
		}
	}
}

func TestMoveMissingSource(t *testing.T) {
	root := moveFixture(t)
	code, envelope, _ := moveInvoke(t, root, "notes/missing.md", "notes/new.md")
	if code != 1 {
		t.Fatalf("mv exit=%d, want 1", code)
	}
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrConceptNotFound {
		t.Fatalf("error = %+v", envelope.Error)
	}
}

func TestMoveUpdatesIndex(t *testing.T) {
	root := moveFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	moveIngest(t, root, db)
	code, _, stderr := moveInvoke(t, root, "notes/old.md", "notes/archive/old.md", "--db", db)
	if code != 0 {
		t.Fatalf("mv exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := Invoke(t, "show", root, "notes/archive/old", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("new path not indexed: exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	code, stdout, _ = Invoke(t, "show", root, "notes/old", "--db", db, "--json")
	if code != 1 {
		t.Fatalf("old path still indexed: exit=%d stdout=%s", code, stdout)
	}
	code, stdout, stderr = Invoke(t, "backlinks", root, "notes/archive/old", "--db", db, "--json")
	if code != 0 {
		t.Fatalf("backlinks exit=%d stderr=%q", code, stderr)
	}
	var result contractv1.BacklinksResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &result); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, backlink := range result.Backlinks {
		if backlink.From == "notes/home" {
			found = true
		}
	}
	if !found {
		raw := extensionRaw(t, db)
		var links string
		_ = raw.QueryRow("SELECT links_json FROM documents WHERE id = ?", "notes/home").Scan(&links)
		raw.Close()
		t.Fatalf("backlink missing: %+v\nhome links: %s\nhome:\n%s\nother:\n%s", result.Backlinks, links,
			moveRead(t, root, "notes/home.md"), moveRead(t, root, "notes/other.md"))
	}
}

func TestMoveIndexFailureRestoresFiles(t *testing.T) {
	root := moveFixture(t)
	before := moveRead(t, root, "notes/home.md")
	db := filepath.Join(t.TempDir(), "index.db")
	if err := os.WriteFile(db, []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, envelope, _ := moveInvoke(t, root, "notes/old.md", "notes/archive/old.md", "--db", db)
	if code != 1 {
		t.Fatalf("mv exit=%d, want 1", code)
	}
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrMoveFailed {
		t.Fatalf("error = %+v", envelope.Error)
	}
	if moveRead(t, root, "notes/home.md") != before {
		t.Fatal("rewritten file not restored")
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "archive", "old.md")); !os.IsNotExist(err) {
		t.Fatal("destination was not removed")
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "old.md")); err != nil {
		t.Fatalf("source was not restored: %v", err)
	}
}

func TestMoveWithoutDatabase(t *testing.T) {
	root := moveFixture(t)
	db := filepath.Join(t.TempDir(), "missing", "index.db")
	code, _, stderr := moveInvoke(t, root, "notes/old.md", "notes/new.md", "--db", db)
	if code != 0 {
		t.Fatalf("mv exit=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatal("mv must not create a database")
	}
}

func TestMoveCaseOnlyRename(t *testing.T) {
	root := moveFixture(t)
	code, _, stderr := moveInvoke(t, root, "notes/old.md", "notes/Old.md")
	if code != 0 {
		t.Fatalf("mv exit=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "Old.md")); err != nil {
		t.Fatalf("case-only destination missing: %v", err)
	}
}

func TestMoveProtectedDocument(t *testing.T) {
	root := moveFixture(t)
	protected := "---\ntype: Concept\ntitle: \"Secret\"\nkeywords:\n  - \"secret\"\nprivacy:\n  policy: encrypt\n  provider: aes-256-gcm\n  key_id: ops\n---\n\nenc:v1:AAAA\n"
	moveWrite(t, root, "notes/secret.md", protected)
	before := moveRead(t, root, "notes/home.md")
	code, envelope, _ := moveInvoke(t, root, "notes/old.md", "notes/new.md")
	if code != 1 {
		t.Fatalf("mv exit=%d, want 1", code)
	}
	if envelope.Error == nil || envelope.Error.Code != contractv1.ErrPrivacyDirective {
		t.Fatalf("error = %+v", envelope.Error)
	}
	if moveRead(t, root, "notes/home.md") != before {
		t.Fatal("files changed after privacy refusal")
	}
}
