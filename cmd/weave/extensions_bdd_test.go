package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	_ "modernc.org/sqlite"
)

// Behavior-level tests for weave-<name> subprocess extensions.

type bddExtensionList struct {
	Extensions []struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Source string `json:"source"`
	} `json:"extensions"`
}

type bddRerankResponse struct {
	Keyword []struct {
		ID          string   `json:"id"`
		Score       float64  `json:"score"`
		RerankScore *float64 `json:"rerank_score"`
	} `json:"keyword"`
}

func extensionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		content := "---\ntype: Concept\ntitle: \"" + strings.ToUpper(name) + "\"\nkeywords:\n  - \"alpha\"\n---\n\n# " + strings.ToUpper(name) + "\n\nalpha content for " + name + "\n"
		if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

func installExtension(t *testing.T, root, name, script string) string {
	t.Helper()
	source := filepath.Join("testdata", "extensions", script)
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read fixture %s: %v", script, err)
	}
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "weave-"+name)
	if err := os.WriteFile(destination, data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	return destination
}

func extensionRaw(t *testing.T, path string) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return raw
}

func extensionRoot(t *testing.T, root string) string {
	t.Helper()
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(absolute)
}

func extensionIngestReport(t *testing.T, stdout string) contractv1.IngestReport {
	t.Helper()
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("ingest failed: %+v", envelope.Error)
	}
	var report contractv1.IngestReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	return report
}

func extensionShow(t *testing.T, root, id string) contractv1.ShowResult {
	t.Helper()
	code, stdout, stderr := invokeRaw(t, "show", root, id, "--no-db", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatalf("decode show: %v", err)
	}
	return view
}

func extensionSearchKeyword(t *testing.T, root, query string, extra ...string) bddRerankResponse {
	t.Helper()
	args := append([]string{"search", root, query, "--no-db", "--mode", "keyword"}, extra...)
	args = append(args, "--json")
	code, stdout, stderr := invokeRaw(t, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	var response bddRerankResponse
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	return response
}

func keywordIDs(response bddRerankResponse) []string {
	ids := make([]string, 0, len(response.Keyword))
	for _, hit := range response.Keyword {
		ids = append(ids, hit.ID)
	}
	return ids
}

func TestExtensionsList(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "echo", "echo.sh")

	pathDir := t.TempDir()
	for name, script := range map[string]string{"echo": "echo.sh", "other": "fail.sh"} {
		data, err := os.ReadFile(filepath.Join("testdata", "extensions", script))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(pathDir, "weave-"+name)
		if err := os.WriteFile(destination, data, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(destination, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	code, stdout, stderr := invokeRaw(t, "extensions", "list", root, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("list exit=%d stderr=%q", code, stderr)
	}
	var listed bddExtensionList
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Extensions) != 2 {
		t.Fatalf("expected 2 extensions, got %+v", listed.Extensions)
	}
	if listed.Extensions[0].Name != "echo" || listed.Extensions[0].Source != "bundle" {
		t.Fatalf("bundle extension must win: %+v", listed.Extensions[0])
	}
	if listed.Extensions[1].Name != "other" || listed.Extensions[1].Source != "path" {
		t.Fatalf("unexpected path extension: %+v", listed.Extensions[1])
	}
}

func TestExtensionsMissingFailsBeforeWrite(t *testing.T) {
	root := extensionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, _ := invokeRaw(t, "ingest", root, "--db", db, "--with-enricher", "ghost", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "EXTENSION_NOT_FOUND" {
		t.Fatalf("expected EXTENSION_NOT_FOUND, got %+v", envelope.Error)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatalf("database must not be written, stat err = %v", err)
	}
}

func TestExtensionsEnrichmentMergesKeywords(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "echo", "echo.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--with-enricher", "echo", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	extensionIngestReport(t, stdout)

	code, stdout, stderr = invokeRaw(t, "show", root, "a", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatal(err)
	}
	if view.Keywords[0] != "alpha" {
		t.Fatalf("declared keyword must stay first: %v", view.Keywords)
	}
	joined := strings.ToLower(strings.Join(view.Keywords, " "))
	if !strings.Contains(joined, "enriched") || !strings.Contains(joined, "sqlite") {
		t.Fatalf("missing inferred keywords: %v", view.Keywords)
	}
	if strings.Count(joined, "alpha") != 1 {
		t.Fatalf("duplicate inferred keyword was not removed: %v", view.Keywords)
	}

	response := extensionSearchKeyword(t, root, "sqlite", "--db", db)
	if len(response.Keyword) == 0 || response.Keyword[0].ID != "a" {
		t.Fatalf("inferred keyword is not searchable: %+v", response.Keyword)
	}

	raw := extensionRaw(t, db)
	var data string
	var enricher string
	if err := raw.QueryRow(
		"SELECT enricher_id, data_json FROM enrichments WHERE bundle_root = ? AND doc_id = ?",
		extensionRoot(t, root), "a",
	).Scan(&enricher, &data); err != nil {
		t.Fatalf("query enrichment: %v", err)
	}
	if enricher != "echo" || !strings.Contains(data, "enriched") {
		t.Fatalf("unexpected enrichment row: %q %q", enricher, data)
	}
}

func TestExtensionsEnrichmentFailureFallsBack(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "fail", "fail.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--with-enricher", "fail", "--json")
	if code != 0 {
		t.Fatalf("ingest must succeed on extension failure, exit=%d stderr=%q", code, stderr)
	}
	report := extensionIngestReport(t, stdout)
	if report.Warnings < 1 {
		t.Fatalf("expected warning count, got %+v", report)
	}
	if !strings.Contains(stderr, "fail") {
		t.Fatalf("stderr must name the extension: %q", stderr)
	}
	view := extensionShow(t, root, "a")
	if len(view.Keywords) != 1 || view.Keywords[0] != "alpha" {
		t.Fatalf("keywords must stay unchanged: %v", view.Keywords)
	}
}

func TestExtensionsEnrichmentProtocolFallsBack(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "bad", "badjson.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	code, stdout, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--with-enricher", "bad", "--json")
	if code != 0 {
		t.Fatalf("ingest must succeed, exit=%d stderr=%q", code, stderr)
	}
	if report := extensionIngestReport(t, stdout); report.Warnings < 1 {
		t.Fatalf("expected warning count, got %+v", report)
	}
	view := extensionShow(t, root, "a")
	if len(view.Keywords) != 1 || view.Keywords[0] != "alpha" {
		t.Fatalf("malformed response must not change keywords: %v", view.Keywords)
	}
}

func TestExtensionsEnrichmentTimeout(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "slow", "slow.sh")
	db := filepath.Join(t.TempDir(), "index.db")

	started := time.Now()
	code, stdout, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict",
		"--with-enricher", "slow", "--timeout", "50ms", "--json")
	elapsed := time.Since(started)
	if code != 0 {
		t.Fatalf("ingest must succeed, exit=%d stderr=%q", code, stderr)
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("timeout was not enforced: %s", elapsed)
	}
	if report := extensionIngestReport(t, stdout); report.Warnings < 1 {
		t.Fatalf("expected warning count, got %+v", report)
	}
	if !strings.Contains(stderr, "slow") {
		t.Fatalf("stderr must name the extension: %q", stderr)
	}
}

func TestExtensionsReadOnlyCommandsNeverInvoke(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "echo", "echo.sh")
	marker := filepath.Join(t.TempDir(), "invoked")
	t.Setenv("WEAVE_EXT_MARKER", marker)

	for _, args := range [][]string{
		{"parse", root, "--no-db", "--json"},
		{"show", root, "a", "--no-db", "--json"},
		{"search", root, "alpha", "--no-db", "--mode", "keyword", "--json"},
	} {
		code, _, stderr := invokeRaw(t, args...)
		if code != 0 {
			t.Fatalf("%v exit=%d stderr=%q", args, code, stderr)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("read-only commands must not invoke extensions")
	}

	db := filepath.Join(t.TempDir(), "index.db")
	code, _, stderr := invokeRaw(t, "ingest", root, "--db", db, "--strict", "--with-enricher", "echo", "--json")
	if code != 0 {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("expected enrichment invocation marker: %v", err)
	}
}

func TestExtensionsTimeoutRequiresEnricher(t *testing.T) {
	root := extensionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, _, _ := invokeRaw(t, "ingest", root, "--db", db, "--timeout", "5s", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2 for --timeout without extension, got %d", code)
	}

	installExtension(t, root, "echo", "echo.sh")
	code, _, _ = invokeRaw(t, "ingest", root, "--db", db, "--with-enricher", "echo", "--timeout", "soon", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2 for invalid timeout, got %d", code)
	}
}

func TestExtensionsRerankBlendsScores(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "rerank", "rerank.sh")

	response := extensionSearchKeyword(t, root, "alpha", "--rerank", "rerank")
	ids := keywordIDs(response)
	if strings.Join(ids, ",") != "c,b,a" {
		t.Fatalf("unexpected reranked order: %v", ids)
	}
	want := map[string]float64{"c": 1.0, "b": 0.75, "a": 0.5}
	for _, hit := range response.Keyword {
		if hit.RerankScore == nil {
			t.Fatalf("missing rerank_score for %s", hit.ID)
		}
		if hit.Score < want[hit.ID]-0.001 || hit.Score > want[hit.ID]+0.001 {
			t.Fatalf("score for %s = %v, want %v", hit.ID, hit.Score, want[hit.ID])
		}
	}
}

func TestExtensionsRerankDeterministic(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "rerank", "rerank.sh")

	code, first, stderr := invokeRaw(t, "search", root, "alpha", "--no-db", "--mode", "keyword", "--rerank", "rerank", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	code, second, stderr := invokeRaw(t, "search", root, "alpha", "--no-db", "--mode", "keyword", "--rerank", "rerank", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	if first != second {
		t.Fatal("rerank output must be deterministic")
	}
}

func TestExtensionsRerankFailureKeepsCoreOrder(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "fail", "fail.sh")

	response := extensionSearchKeyword(t, root, "alpha", "--rerank", "fail")
	ids := keywordIDs(response)
	if strings.Join(ids, ",") != "a,b,c" {
		t.Fatalf("core ordering must be preserved: %v", ids)
	}
	for _, hit := range response.Keyword {
		if hit.RerankScore != nil {
			t.Fatalf("fallback must not set rerank_score: %+v", hit)
		}
	}
}

func TestExtensionsRerankInvalidResponseKeepsCoreOrder(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "partial", "rerank-partial.sh")

	response := extensionSearchKeyword(t, root, "alpha", "--rerank", "partial")
	ids := keywordIDs(response)
	if strings.Join(ids, ",") != "a,b,c" {
		t.Fatalf("incomplete response must fall back: %v", ids)
	}
}

func TestExtensionsRerankTimeoutKeepsCoreOrder(t *testing.T) {
	root := extensionFixture(t)
	installExtension(t, root, "slow", "slow.sh")

	started := time.Now()
	response := extensionSearchKeyword(t, root, "alpha", "--rerank", "slow", "--timeout", "50ms")
	if elapsed := time.Since(started); elapsed > 1500*time.Millisecond {
		t.Fatalf("rerank timeout was not enforced: %s", elapsed)
	}
	if ids := keywordIDs(response); strings.Join(ids, ",") != "a,b,c" {
		t.Fatalf("timeout must preserve core order: %v", ids)
	}
}

func TestExtensionsRerankMissingFails(t *testing.T) {
	root := extensionFixture(t)
	code, stdout, _ := invokeRaw(t, "search", root, "alpha", "--no-db", "--mode", "keyword", "--rerank", "ghost", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "EXTENSION_NOT_FOUND" {
		t.Fatalf("expected EXTENSION_NOT_FOUND, got %+v", envelope.Error)
	}
}
