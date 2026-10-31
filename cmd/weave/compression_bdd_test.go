package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	_ "modernc.org/sqlite"
)

// Behavior-level tests for body compression. They drive the CLI and inspect
// the stored codec/blob directly, so the contract stays storage-internal.

const compressionKey = "WEAVE_KEY_OPS_PROD_KEY"

func compressionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	small := strings.Repeat("s", 1021)
	boundary := strings.Repeat("b", 1022)
	large := "# Large\n\n" + strings.Repeat("compressible phrase. ", 200)
	writeCompressionDoc(t, root, "small", "Small", small, "")
	writeCompressionDoc(t, root, "boundary", "Boundary", boundary, "")
	writeCompressionDoc(t, root, "large", "Large", large, "")
	writeCompressionDoc(t, root, "encrypted", "Encrypted Large", large, `privacy:
  policy: encrypt
  provider: aes-256-gcm
  key_id: ops-prod-key
`)
	return root
}

func writeCompressionDoc(t *testing.T, root, name, title, body, privacy string) {
	t.Helper()
	frontmatter := "---\ntype: Concept\ntitle: \"" + title + "\"\nkeywords:\n  - \"" + name + "\"\n" + privacy + "---\n"
	if privacy == "" {
		frontmatter = "---\ntype: Concept\ntitle: \"" + title + "\"\nkeywords:\n  - \"" + name + "\"\n---\n"
	}
	content := frontmatter + body + "\n"
	if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func compressionParse(t *testing.T, root string) contractv1.Bundle {
	t.Helper()
	code, stdout, stderr := invokeRaw(t, "parse", root, "--no-db", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("parse exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("parse failed: %+v", envelope.Error)
	}
	var bundle contractv1.Bundle
	if err := json.Unmarshal(envelope.Result, &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	return bundle
}

func parsedBody(bundle contractv1.Bundle, id string) string {
	for _, concept := range bundle.Concepts {
		if concept.ID == id {
			return concept.Body
		}
	}
	return ""
}

func compressionRaw(t *testing.T, path string) *sql.DB {
	t.Helper()
	return HarnessOpenDB(t, path)
}

func storedCodec(t *testing.T, raw *sql.DB, root, id string) string {
	t.Helper()
	var codec string
	if err := raw.QueryRow(
		"SELECT body_codec FROM documents WHERE bundle_root = ? AND id = ?",
		compressionRoot(t, root), id,
	).Scan(&codec); err != nil {
		t.Fatalf("query codec: %v", err)
	}
	return codec
}

func storedBody(t *testing.T, raw *sql.DB, root, id string) []byte {
	t.Helper()
	var body []byte
	if err := raw.QueryRow(
		"SELECT body_compressed FROM documents WHERE bundle_root = ? AND id = ?",
		compressionRoot(t, root), id,
	).Scan(&body); err != nil {
		t.Fatalf("query body: %v", err)
	}
	return body
}

func compressionRoot(t *testing.T, root string) string {
	t.Helper()
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(absolute)
}

func compressionIngest(t *testing.T, root, db string, extra ...string) {
	t.Helper()
	args := append([]string{"ingest", root, "--db", db}, extra...)
	args = append(args, "--json")
	code, stdout, stderr := invokeRaw(t, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if envelope := decodeEnvelope(t, stdout); !envelope.OK {
		t.Fatalf("ingest failed: %+v", envelope.Error)
	}
}

func TestCompressionDefaultZstd(t *testing.T) {
	t.Setenv(compressionKey, strings.Repeat("ab", 32))
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)
	raw := compressionRaw(t, db)
	bundle := compressionParse(t, root)

	if len(parsedBody(bundle, "small")) != 1023 {
		t.Fatalf("small fixture body is %d bytes, want 1023", len(parsedBody(bundle, "small")))
	}
	if len(parsedBody(bundle, "boundary")) != 1024 {
		t.Fatalf("boundary fixture body is %d bytes, want 1024", len(parsedBody(bundle, "boundary")))
	}
	if codec := storedCodec(t, raw, root, "small"); codec != "none" {
		t.Fatalf("small body codec = %q, want none", codec)
	}
	if codec := storedCodec(t, raw, root, "boundary"); codec != "zstd" {
		t.Fatalf("boundary body codec = %q, want zstd", codec)
	}
	if codec := storedCodec(t, raw, root, "large"); codec != "zstd" {
		t.Fatalf("large body codec = %q, want zstd", codec)
	}

	code, stdout, stderr := invokeRaw(t, "show", root, "large", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatalf("decode show: %v", err)
	}
	if view.Body != parsedBody(bundle, "large") {
		t.Fatal("compressed body did not round-trip through show")
	}
}

func TestCompressionLZ4AndNone(t *testing.T) {
	root := compressionFixture(t)
	bundle := compressionParse(t, root)

	lz4DB := filepath.Join(t.TempDir(), "lz4.db")
	compressionIngest(t, root, lz4DB, "--codec", "lz4")
	lz4Raw := compressionRaw(t, lz4DB)
	if codec := storedCodec(t, lz4Raw, root, "large"); codec != "lz4" {
		t.Fatalf("lz4 codec = %q", codec)
	}
	code, stdout, stderr := invokeRaw(t, "show", root, "large", "--db", lz4DB, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show lz4 exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatal(err)
	}
	if view.Body != parsedBody(bundle, "large") {
		t.Fatal("lz4 body did not round-trip")
	}

	noneDB := filepath.Join(t.TempDir(), "none.db")
	compressionIngest(t, root, noneDB, "--codec", "none")
	noneRaw := compressionRaw(t, noneDB)
	if codec := storedCodec(t, noneRaw, root, "large"); codec != "none" {
		t.Fatalf("none codec = %q", codec)
	}
	if stored := string(storedBody(t, noneRaw, root, "large")); stored != parsedBody(bundle, "large") {
		t.Fatal("none codec must store bodies verbatim")
	}
}

func TestCompressionRejectsUnknownCodec(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, _ := invokeRaw(t, "ingest", root, "--db", db, "--codec", "brotli", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %+v", envelope.Error)
	}
}

func TestCompressionEncryptedBodyStaysPlainCiphertext(t *testing.T) {
	t.Setenv(compressionKey, strings.Repeat("ab", 32))
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)
	raw := compressionRaw(t, db)

	if codec := storedCodec(t, raw, root, "encrypted"); codec != "none" {
		t.Fatalf("encrypted codec = %q, want none", codec)
	}
	body := storedBody(t, raw, root, "encrypted")
	if !strings.HasPrefix(string(body), "enc:v1:") {
		t.Fatalf("expected ciphertext envelope, got %q", string(body))
	}

	bundle := compressionParse(t, root)
	code, stdout, stderr := invokeRaw(t, "show", root, "encrypted", "--db", db, "--decrypt", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("decrypt exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatal(err)
	}
	if view.Body != parsedBody(bundle, "encrypted") {
		t.Fatal("encrypted body did not decrypt after storage")
	}
}

func TestCompressionRebuildMigration(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	bundle := compressionParse(t, root)
	compressionIngest(t, root, db, "--codec", "none")
	raw := compressionRaw(t, db)
	if codec := storedCodec(t, raw, root, "large"); codec != "none" {
		t.Fatalf("initial codec = %q", codec)
	}

	code, stdout, stderr := invokeRaw(t, "rebuild", root, "--db", db, "--codec", "zstd", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("rebuild exit=%d stderr=%q", code, stderr)
	}
	if envelope := decodeEnvelope(t, stdout); !envelope.OK {
		t.Fatalf("rebuild failed: %+v", envelope.Error)
	}
	if codec := storedCodec(t, raw, root, "large"); codec != "zstd" {
		t.Fatalf("rebuilt codec = %q", codec)
	}

	showCode, showOut, showErr := invokeRaw(t, "show", root, "large", "--db", db, "--json")
	if showCode != 0 || showErr != "" {
		t.Fatalf("show exit=%d stderr=%q", showCode, showErr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, showOut).Result, &view); err != nil {
		t.Fatal(err)
	}
	if view.Body != parsedBody(bundle, "large") {
		t.Fatal("rebuilt body did not round-trip")
	}

	code, stdout, stderr = invokeRaw(t, "rebuild", root, "--db", db, "--codec", "none", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("rebuild none exit=%d stderr=%q", code, stderr)
	}
	if codec := storedCodec(t, raw, root, "large"); codec != "none" {
		t.Fatalf("disabled codec = %q", codec)
	}
}

func TestCompressionSyncPreservesCodec(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db, "--codec", "lz4")
	raw := compressionRaw(t, db)

	writeCompressionDoc(t, root, "large", "Large", "# Large\n\nupdated "+strings.Repeat("compressible phrase. ", 200), "")
	writeCompressionDoc(t, root, "fresh", "Fresh", "# Fresh\n\n"+strings.Repeat("new phrase. ", 200), "")
	code, stdout, stderr := invokeRaw(t, "sync", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("sync exit=%d stderr=%q", code, stderr)
	}
	if envelope := decodeEnvelope(t, stdout); !envelope.OK {
		t.Fatalf("sync failed: %+v", envelope.Error)
	}
	if codec := storedCodec(t, raw, root, "large"); codec != "lz4" {
		t.Fatalf("updated codec = %q, want lz4", codec)
	}
	if codec := storedCodec(t, raw, root, "fresh"); codec != "zstd" {
		t.Fatalf("new codec = %q, want zstd", codec)
	}

	writeCompressionDoc(t, root, "large", "Large", "# Large\n\nupdated again "+strings.Repeat("compressible phrase. ", 200), "")
	code, stdout, stderr = invokeRaw(t, "sync", root, "--db", db, "--codec", "zstd", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("sync zstd exit=%d stderr=%q", code, stderr)
	}
	if codec := storedCodec(t, raw, root, "large"); codec != "zstd" {
		t.Fatalf("forced codec = %q, want zstd", codec)
	}
}

func TestCompressionCorruptBodyWarns(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)
	raw := compressionRaw(t, db)

	if _, err := raw.Exec(
		"UPDATE documents SET body_codec = 'bogus' WHERE bundle_root = ? AND id = ?",
		compressionRoot(t, root), "large",
	); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := invokeRaw(t, "parse", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("parse exit=%d stderr=%q", code, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	var bundle contractv1.Bundle
	if err := json.Unmarshal(envelope.Result, &bundle); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, warning := range bundle.Validation.Warnings {
		if warning == "BODY_DECODE_FAILED" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected BODY_DECODE_FAILED warning, got %v", bundle.Validation.Warnings)
	}
	if body := parsedBody(bundle, "large"); body != "" {
		t.Fatalf("corrupt body must decode empty, got %q", body)
	}

	showCode, _, showErr := invokeRaw(t, "show", root, "large", "--db", db, "--json")
	if showCode != 0 {
		t.Fatalf("show on corrupt body exit=%d stderr=%q", showCode, showErr)
	}
}

func TestCompressionTruncatedBlobWarns(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)
	raw := compressionRaw(t, db)

	if _, err := raw.Exec(
		"UPDATE documents SET body_compressed = x'00' WHERE bundle_root = ? AND id = ?",
		compressionRoot(t, root), "large",
	); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := invokeRaw(t, "parse", root, "--db", db, "--json")
	if code != 0 {
		t.Fatalf("parse exit=%d", code)
	}
	var bundle contractv1.Bundle
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &bundle); err != nil {
		t.Fatal(err)
	}
	if body := parsedBody(bundle, "large"); body != "" {
		t.Fatalf("truncated body must decode empty, got %q", body)
	}

	searchCode, searchOut, searchErr := invokeRaw(t, "search", root, "compressible", "--db", db, "--mode", "fuzzy", "--json")
	if searchCode == 2 || searchCode == 3 {
		t.Fatalf("search must not fail on corrupt bodies: exit=%d stderr=%q", searchCode, searchErr)
	}
	if strings.TrimSpace(searchOut) != "" {
		decodeEnvelope(t, searchOut)
	}
}

func TestCompressionReducesStoredSize(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)
	raw := compressionRaw(t, db)
	bundle := compressionParse(t, root)

	stored := len(storedBody(t, raw, root, "large"))
	parsed := len(parsedBody(bundle, "large"))
	if stored >= parsed {
		t.Fatalf("compressed size %d is not smaller than parsed size %d", stored, parsed)
	}
}

func TestCompressionSearchParity(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)

	code, stdout, stderr := invokeRaw(t, "search", root, "compressible", "--db", db, "--mode", "fuzzy", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Fuzzy) == 0 {
		t.Fatal("expected fuzzy hits on compressed bodies")
	}

	code, stdout, stderr = invokeRaw(t, "search", root, "compressible", "--db", db, "--mode", "bm25", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("bm25 exit=%d stderr=%q", code, stderr)
	}
	var lexical contractv1.SearchResponse
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &lexical); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, hit := range lexical.Lexical {
		if hit.ID == "large" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected BM25 hits on decompressed bodies")
	}

	code, stdout, stderr = invokeRaw(t, "explain", root, "compressible", "--db", db, "--mode", "fuzzy", "--id", "large", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("explain exit=%d stderr=%q", code, stderr)
	}
	if envelope := decodeEnvelope(t, stdout); !envelope.OK {
		t.Fatalf("explain failed: %+v", envelope.Error)
	}
}
