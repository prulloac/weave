package privacy_test

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/privacy"
)

const testKeyID = "test-key"

func testKey(t *testing.T) {
	t.Helper()
	t.Setenv("WEAVE_KEY_TEST_KEY", strings.Repeat("ab", 32))
}

func TestParseFrontmatter(t *testing.T) {
	directive, err := privacy.ParseFrontmatter(map[string]any{
		"privacy": map[string]any{
			"policy": "encrypt",
			"key_id": "ops",
		},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if directive.Policy != contractv1.PrivacyEncrypt || directive.Provider != "aes-256-gcm" {
		t.Fatalf("unexpected directive: %+v", directive)
	}

	for name, frontmatter := range map[string]map[string]any{
		"missing policy": {"privacy": map[string]any{"key_id": "ops"}},
		"missing key":    {"privacy": map[string]any{"policy": "hash"}},
		"bad policy":     {"privacy": map[string]any{"policy": "scramble", "key_id": "ops"}},
		"bad provider":   {"privacy": map[string]any{"policy": "hash", "provider": "aes-256-gcm", "key_id": "ops"}},
		"not a mapping":  {"privacy": "encrypt"},
	} {
		if _, err := privacy.ParseFrontmatter(frontmatter); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func TestScanInline(t *testing.T) {
	body := "Owner: <!-- weave:tokenize key_id=\"tenant\" -->internal-client<!-- /weave:tokenize -->\n"
	spans, warnings := privacy.ScanInline(body)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(spans) != 1 || body[spans[0].Start:spans[0].End] != "internal-client" {
		t.Fatalf("unexpected spans: %+v", spans)
	}

	fenced := "```\n<!-- weave:hash key_id=\"x\" -->secret<!-- /weave:hash -->\n```\n"
	if spans, _ := privacy.ScanInline(fenced); len(spans) != 0 {
		t.Fatalf("directives in code fences must be ignored: %+v", spans)
	}

	inlineCode := "`<!-- weave:hash key_id=\"x\" -->secret<!-- /weave:hash -->`\n"
	if spans, _ := privacy.ScanInline(inlineCode); len(spans) != 0 {
		t.Fatalf("directives in inline code must be ignored: %+v", spans)
	}

	unclosed := "<!-- weave:hash key_id=\"x\" -->secret\n"
	if _, warnings := privacy.ScanInline(unclosed); len(warnings) == 0 {
		t.Fatal("expected unclosed directive warning")
	}

	nested := "<!-- weave:hash key_id=\"x\" -->a<!-- weave:hash key_id=\"x\" -->b<!-- /weave:hash -->\n"
	if _, warnings := privacy.ScanInline(nested); len(warnings) == 0 {
		t.Fatal("expected nested directive warning")
	}
}

func TestApplyTokenizeIsDeterministic(t *testing.T) {
	testKey(t)
	concept := contractv1.OkfConcept{
		Body:    "Account internal-client-alpha owns it.",
		Privacy: &contractv1.PrivacyDirective{Policy: contractv1.PrivacyTokenize, Provider: "hmac-sha256", KeyID: testKeyID},
	}
	first, err := privacy.Apply(t.TempDir(), concept)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	second, err := privacy.Apply(t.TempDir(), concept)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if first.Body != second.Body {
		t.Fatalf("tokenize must be deterministic:\n%s\n%s", first.Body, second.Body)
	}
	if strings.Contains(first.Body, "internal-client-alpha") || !strings.Contains(first.Body, "tok") {
		t.Fatalf("unexpected tokenized body: %q", first.Body)
	}
	if first.Redacted {
		t.Fatal("tokenized content is not redacted")
	}
}

func TestApplyHashNormalizesText(t *testing.T) {
	testKey(t)
	base := contractv1.OkfConcept{
		Body:    "Secret Value",
		Privacy: &contractv1.PrivacyDirective{Policy: contractv1.PrivacyHash, Provider: "hmac-sha256", KeyID: testKeyID},
	}
	upper, err := privacy.Apply(t.TempDir(), base)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	base.Body = "  secret   value \n"
	lower, err := privacy.Apply(t.TempDir(), base)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if upper.Body != lower.Body || !strings.HasPrefix(upper.Body, "hash") {
		t.Fatalf("hash must normalize case and whitespace: %q vs %q", upper.Body, lower.Body)
	}
}

func TestApplyEncryptRoundTrip(t *testing.T) {
	testKey(t)
	concept := contractv1.OkfConcept{
		Body:    "\n# Secret\n\nlaunch code\n",
		Privacy: &contractv1.PrivacyDirective{Policy: contractv1.PrivacyEncrypt, Provider: "aes-256-gcm", KeyID: testKeyID},
	}
	result, err := privacy.Apply(t.TempDir(), concept)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !strings.HasPrefix(result.Body, "enc:v1:") || !result.Redacted {
		t.Fatalf("unexpected encrypted result: %+v", result)
	}
	plain, err := privacy.DecryptBody(t.TempDir(), result.Body, concept.Privacy, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != concept.Body {
		t.Fatalf("round trip mismatch: %q", plain)
	}

	tampered := result.Body[:len(result.Body)-4] + "AAAA"
	if _, err := privacy.DecryptBody(t.TempDir(), tampered, concept.Privacy, nil); !errors.Is(err, privacy.ErrDecrypt) {
		t.Fatalf("expected decrypt error, got %v", err)
	}
}

func TestEncodedDirective(t *testing.T) {
	testKey(t)
	directive, err := privacy.ParseFrontmatter(map[string]any{
		"privacy": map[string]any{
			"policy":  "encrypt",
			"key_id":  testKeyID,
			"encoded": true,
		},
	})
	if err != nil || directive == nil || !directive.Encoded {
		t.Fatalf("expected encoded directive, got %+v %v", directive, err)
	}

	concept := contractv1.OkfConcept{
		Body:    "enc:v1:AAAA",
		Privacy: directive,
	}
	result, err := privacy.Apply(t.TempDir(), concept)
	if err != nil {
		t.Fatalf("apply encoded: %v", err)
	}
	if result.Body != concept.Body || !result.Redacted {
		t.Fatalf("encoded bodies must be stored verbatim: %+v", result)
	}

	concept.Body = "plaintext"
	if _, err := privacy.Apply(t.TempDir(), concept); err == nil {
		t.Fatal("encoded plaintext body must be rejected")
	}

	if _, err := privacy.ParseFrontmatter(map[string]any{
		"privacy": map[string]any{"policy": "tokenize", "key_id": "k", "encoded": true},
	}); err == nil {
		t.Fatal("encoded must require encrypt")
	}
}

func TestResolveKeyPrecedence(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".weave"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := "keys:\n  test-key: \"" + strings.Repeat("cd", 32) + "\"\n"
	if err := os.WriteFile(filepath.Join(root, ".weave", "security.yaml"), []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}

	fromFile, err := privacy.ResolveKey(root, testKeyID)
	if err != nil {
		t.Fatalf("resolve file key: %v", err)
	}
	t.Setenv("WEAVE_KEY_TEST_KEY", strings.Repeat("ab", 32))
	fromEnv, err := privacy.ResolveKey(root, testKeyID)
	if err != nil {
		t.Fatalf("resolve env key: %v", err)
	}
	if string(fromEnv) == string(fromFile) {
		t.Fatal("environment key must take precedence over security.yaml")
	}

	if !privacy.HasKey(root, "test-key") {
		t.Fatal("expected key to resolve")
	}
	if privacy.HasKey(root, "absent-key") {
		t.Fatal("unexpected key resolution")
	}
	if warning := privacy.SecurityFileWarning(root); warning != "" {
		t.Fatalf("secure file must not warn: %q", warning)
	}
}

func TestResolveKeyFormats(t *testing.T) {
	root := t.TempDir()
	raw := strings.Repeat("ab", 32)
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(decoded)

	t.Setenv("WEAVE_KEY_TEST_KEY", raw)
	hexKey, err := privacy.ResolveKey(root, testKeyID)
	if err != nil {
		t.Fatalf("hex key: %v", err)
	}
	t.Setenv("WEAVE_KEY_TEST_KEY", encoded)
	b64Key, err := privacy.ResolveKey(root, testKeyID)
	if err != nil {
		t.Fatalf("base64 key: %v", err)
	}
	if string(hexKey) != string(b64Key) {
		t.Fatal("hex and base64 encodings of the same bytes must resolve equally")
	}
}

func TestExpandQuery(t *testing.T) {
	testKey(t)
	root := t.TempDir()
	query := "internal-client-alpha"
	refs := []privacy.KeyRef{
		{Policy: contractv1.PrivacyTokenize, KeyID: testKeyID},
		{Policy: contractv1.PrivacyHash, KeyID: testKeyID},
		{Policy: contractv1.PrivacyTokenize, KeyID: "missing"},
	}
	aliases := privacy.ExpandQuery(query, refs, func(keyID string) ([]byte, error) {
		return privacy.ResolveKey(root, keyID)
	})
	if len(aliases) != 4 {
		t.Fatalf("expected three token aliases and one hash alias, got %d: %v", len(aliases), aliases)
	}
	for _, alias := range aliases {
		if !strings.HasPrefix(alias, "tok") && !strings.HasPrefix(alias, "hash") {
			t.Fatalf("unexpected alias: %q", alias)
		}
	}
}
