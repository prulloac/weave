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

// Behavior-level tests for body storage. They drive the CLI and inspect the
// stored body columns directly, so the contract stays storage-internal.

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
	code, stdout, stderr := Invoke(t, "parse", root, "--no-db", "--json")
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
	return OpenDB(t, path)
}

func storedCodec(t *testing.T, raw *sql.DB, root, id string) string {
	t.Helper()
	var codec sql.NullString
	if err := raw.QueryRow(
		"SELECT body_codec FROM documents WHERE bundle_root = ? AND id = ?",
		compressionRoot(t, root), id,
	).Scan(&codec); err != nil {
		t.Fatalf("query codec: %v", err)
	}
	return codec.String
}

func storedBody(t *testing.T, raw *sql.DB, root, id string) []byte {
	t.Helper()
	var body []byte
	if err := raw.QueryRow(
		"SELECT COALESCE(body_raw, body_compressed) FROM documents WHERE bundle_root = ? AND id = ?",
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
	code, stdout, stderr := Invoke(t, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if envelope := decodeEnvelope(t, stdout); !envelope.OK {
		t.Fatalf("ingest failed: %+v", envelope.Error)
	}
}

func TestTextBodiesAreStoredRaw(t *testing.T) {
	t.Setenv(compressionKey, strings.Repeat("ab", 32))
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)
	raw := compressionRaw(t, db)
	bundle := compressionParse(t, root)

	for _, id := range []string{"small", "boundary", "large"} {
		if codec := storedCodec(t, raw, root, id); codec != "" {
			t.Fatalf("%s codec = %q, want NULL", id, codec)
		}
		if body := string(storedBody(t, raw, root, id)); body != parsedBody(bundle, id) {
			t.Fatalf("%s body did not round-trip", id)
		}
	}

	code, stdout, stderr := Invoke(t, "show", root, "large", "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	var view contractv1.ShowResult
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &view); err != nil {
		t.Fatalf("decode show: %v", err)
	}
	if view.Body != parsedBody(bundle, "large") {
		t.Fatal("raw body did not round-trip through show")
	}
}

func TestTextBodiesRecordSizes(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)
	raw := compressionRaw(t, db)
	bundle := compressionParse(t, root)

	for _, id := range []string{"small", "boundary", "large"} {
		var size, storedSize int
		if err := raw.QueryRow(
			"SELECT size, stored_size FROM documents WHERE bundle_root = ? AND id = ?",
			compressionRoot(t, root), id,
		).Scan(&size, &storedSize); err != nil {
			t.Fatal(err)
		}
		if size != len(parsedBody(bundle, id)) || storedSize != size {
			t.Fatalf("%s sizes = %d/%d, want %d", id, size, storedSize, len(parsedBody(bundle, id)))
		}
	}
}

func TestCodecFlagDoesNotCompressText(t *testing.T) {
	t.Setenv(compressionKey, strings.Repeat("ab", 32))
	root := compressionFixture(t)
	for _, codec := range []string{"lz4", "zstd", "none"} {
		db := filepath.Join(t.TempDir(), codec+".db")
		compressionIngest(t, root, db, "--codec", codec)
		raw := compressionRaw(t, db)
		for _, id := range []string{"large", "encrypted"} {
			if stored := storedCodec(t, raw, root, id); stored != "" {
				t.Fatalf("%s/%s codec = %q, want NULL", codec, id, stored)
			}
		}
	}
}

func TestTextCodecFlagWarns(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, stderr := Invoke(t, "ingest", root, "--db", db, "--codec", "zstd", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("ingest exit=%d stderr=%q", code, stderr)
	}
	var report contractv1.IngestReport
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &report); err != nil {
		t.Fatal(err)
	}
	if report.Warnings < 4 {
		t.Fatalf("warnings = %d, want at least one per text document", report.Warnings)
	}
}

func TestEncryptedBodyIsStoredRaw(t *testing.T) {
	t.Setenv(compressionKey, strings.Repeat("ab", 32))
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)
	raw := compressionRaw(t, db)

	if codec := storedCodec(t, raw, root, "encrypted"); codec != "" {
		t.Fatalf("encrypted codec = %q, want NULL", codec)
	}
	body := storedBody(t, raw, root, "encrypted")
	if !strings.HasPrefix(string(body), "enc:v1:") {
		t.Fatalf("expected ciphertext envelope, got %q", string(body))
	}

	bundle := compressionParse(t, root)
	code, stdout, stderr := Invoke(t, "show", root, "encrypted", "--db", db, "--decrypt", "--json")
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

func TestCompressedBlobDecodeFailureWarns(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)
	raw := compressionRaw(t, db)

	if _, err := raw.Exec(`
		INSERT INTO documents (
			bundle_root, id, file_path, content_hash, title, keywords_json,
			frontmatter_json, tags_json, links_json, content_kind, media_type,
			content_metadata, body_compressed, body_codec, size, stored_size, updated_at
		) VALUES (?, 'broken', 'broken.bin', 'hash', 'Broken', '[]', '{}', '[]', '[]',
		          'blob', 'application/octet-stream', '{"v":1}', x'00', 'zstd', 1, 1, 1)`,
		compressionRoot(t, root)); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := Invoke(t, "parse", root, "--db", db, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("parse exit=%d stderr=%q", code, stderr)
	}
	var bundle contractv1.Bundle
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &bundle); err != nil {
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
	if body := parsedBody(bundle, "broken"); body != "" {
		t.Fatalf("corrupt body must decode empty, got %q", body)
	}
}

func TestTextSearchUsesRawBodies(t *testing.T) {
	root := compressionFixture(t)
	db := filepath.Join(t.TempDir(), "index.db")
	compressionIngest(t, root, db)

	code, stdout, stderr := Invoke(t, "search", root, "compressible", "--db", db, "--mode", "fuzzy", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("search exit=%d stderr=%q", code, stderr)
	}
	var response contractv1.SearchResponse
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Fuzzy) == 0 {
		t.Fatal("expected fuzzy hits on raw bodies")
	}

	code, stdout, stderr = Invoke(t, "search", root, "compressible", "--db", db, "--mode", "bm25", "--json")
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
		t.Fatal("expected BM25 hits on raw bodies")
	}
}
