// Package privacy implements declared privacy directives: encryption,
// tokenization and hashing of concept content plus deterministic aliases for
// search.
package privacy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// Directive codes reported by the parser and audit.
const (
	CodeDirectiveInvalid  = "PRIVACY_DIRECTIVE_INVALID"
	CodeDirectiveConflict = "PRIVACY_DIRECTIVE_CONFLICT"
	CodeKeyUnavailable    = "PRIVACY_KEY_UNAVAILABLE"
	CodeKeyFileInsecure   = "PRIVACY_KEY_FILE_INSECURE"
)

// Errors returned by transformations.
var (
	// ErrKeyUnavailable reports a missing key without exposing key material.
	ErrKeyUnavailable = errors.New("privacy key unavailable")
	// ErrDecrypt reports a failed authenticated decryption.
	ErrDecrypt = errors.New("privacy decrypt failed")
	// ErrProvider reports an unsupported policy/provider pairing.
	ErrProvider = errors.New("privacy provider unsupported")
)

const (
	providerAES  = "aes-256-gcm"
	providerHMAC = "hmac-sha256"
	envelope     = "enc:v1:"
)

// Span is one inline directive and the offsets of its content.
type Span struct {
	Policy   contractv1.PrivacyPolicy `json:"policy"`
	Provider string                   `json:"provider"`
	KeyID    string                   `json:"key_id"`
	Start    int                      `json:"start"`
	End      int                      `json:"end"`
}

// Result is one transformed concept body.
type Result struct {
	Body      string
	Directive *contractv1.PrivacyDirective
	Redacted  bool
	Spans     []Span
}

var (
	openPattern  = regexp.MustCompile(`<!--\s*weave:(encrypt|tokenize|hash)([^>]*)-->`)
	closePattern = regexp.MustCompile(`<!--\s*/weave:(encrypt|tokenize|hash)\s*-->`)
	attrPattern  = regexp.MustCompile(`^\s*(provider|key_id)\s*=\s*"([^"]*)"\s*`)
)

// ParseFrontmatter reads the file-level directive.
func ParseFrontmatter(frontmatter map[string]any) (*contractv1.PrivacyDirective, error) {
	if frontmatter == nil {
		return nil, nil
	}
	raw, ok := frontmatter["privacy"]
	if !ok || raw == nil {
		return nil, nil
	}
	block, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: privacy must be a mapping", CodeDirectiveInvalid)
	}
	policyValue, _ := block["policy"].(string)
	policy := contractv1.PrivacyPolicy(strings.ToLower(strings.TrimSpace(policyValue)))
	keyID, _ := block["key_id"].(string)
	provider, _ := block["provider"].(string)
	directive, err := NormalizeDirective(policy, provider, keyID)
	if err != nil {
		return nil, err
	}
	return directive, nil
}

// NormalizeDirective validates a policy/provider/key triple.
func NormalizeDirective(
	policy contractv1.PrivacyPolicy,
	provider string,
	keyID string,
) (*contractv1.PrivacyDirective, error) {
	policy = contractv1.PrivacyPolicy(strings.ToLower(strings.TrimSpace(string(policy))))
	keyID = strings.TrimSpace(keyID)
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch policy {
	case contractv1.PrivacyEncrypt:
		if provider == "" {
			provider = providerAES
		}
		if provider != providerAES {
			return nil, fmt.Errorf("%s: %s cannot %s", CodeDirectiveInvalid, provider, policy)
		}
	case contractv1.PrivacyTokenize, contractv1.PrivacyHash:
		if provider == "" {
			provider = providerHMAC
		}
		if provider != providerHMAC {
			return nil, fmt.Errorf("%s: %s cannot %s", CodeDirectiveInvalid, provider, policy)
		}
	case contractv1.PrivacyMixed:
		// mixed is a stored-only policy and is never declared.
		return nil, fmt.Errorf("%s: unsupported policy mixed", CodeDirectiveInvalid)
	default:
		return nil, fmt.Errorf("%s: unsupported policy %q", CodeDirectiveInvalid, policy)
	}
	if keyID == "" {
		return nil, fmt.Errorf("%s: key_id is required", CodeDirectiveInvalid)
	}
	return &contractv1.PrivacyDirective{Policy: policy, Provider: provider, KeyID: keyID}, nil
}

// ScanInline returns valid inline spans and directive warnings.
func ScanInline(body string) ([]Span, []string) {
	spans := make([]Span, 0)
	warnings := make([]string, 0)
	offset := 0
	inFence := false
	fence := ""
	for _, line := range strings.SplitAfter(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !inFence {
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				inFence = true
				fence = trimmed[:3]
				offset += len(line)
				continue
			}
		} else {
			if strings.HasPrefix(trimmed, fence) {
				inFence = false
			}
			offset += len(line)
			continue
		}

		lineSpans, lineWarnings := scanLine(line, offset)
		spans = append(spans, lineSpans...)
		warnings = append(warnings, lineWarnings...)
		offset += len(line)
	}
	return spans, warnings
}

func scanLine(line string, base int) ([]Span, []string) {
	spans := make([]Span, 0)
	warnings := make([]string, 0)
	cursor := 0
	for {
		location := openPattern.FindStringSubmatchIndex(line[cursor:])
		if location == nil {
			break
		}
		start := cursor + location[0]
		end := cursor + location[1]
		if inInlineCode(line, start) {
			cursor = end
			continue
		}
		name := line[cursor+location[2] : cursor+location[3]]
		attrs := line[cursor+location[4] : cursor+location[5]]
		directive, err := parseAttributes(name, attrs)
		if err != nil {
			warnings = append(warnings, err.Error())
			cursor = end
			continue
		}

		rest := line[end:]
		closing := closePattern.FindStringSubmatchIndex(rest)
		if closing == nil || rest[closing[2]:closing[3]] != name {
			warnings = append(warnings, CodeDirectiveInvalid+": unclosed weave:"+name)
			cursor = end
			continue
		}
		contentStart := end
		contentEnd := end + closing[0]
		inner := line[contentStart:contentEnd]
		if openPattern.MatchString(inner) || closePattern.MatchString(inner) {
			warnings = append(warnings, CodeDirectiveInvalid+": nested weave:"+name)
			cursor = end + closing[1]
			continue
		}
		spans = append(spans, Span{
			Policy:   directive.Policy,
			Provider: directive.Provider,
			KeyID:    directive.KeyID,
			Start:    base + contentStart,
			End:      base + contentEnd,
		})
		cursor = end + closing[1]
	}
	return spans, warnings
}

func parseAttributes(name, attrs string) (*contractv1.PrivacyDirective, error) {
	policy := contractv1.PrivacyPolicy(name)
	provider := ""
	keyID := ""
	rest := attrs
	for strings.TrimSpace(rest) != "" {
		match := attrPattern.FindStringSubmatch(rest)
		if match == nil {
			return nil, fmt.Errorf("%s: invalid attributes in weave:%s", CodeDirectiveInvalid, name)
		}
		switch match[1] {
		case "provider":
			provider = match[2]
		case "key_id":
			keyID = match[2]
		}
		rest = rest[len(match[0]):]
	}
	return NormalizeDirective(policy, provider, keyID)
}

func inInlineCode(line string, index int) bool {
	inCode := false
	for position, char := range line {
		if position >= index {
			break
		}
		if char == '`' {
			inCode = !inCode
		}
	}
	return inCode
}

// Apply transforms one concept according to its declared directives.
func Apply(root string, concept contractv1.OkfConcept) (Result, error) {
	if concept.Privacy != nil && concept.Privacy.Policy != contractv1.PrivacyNone {
		key, err := ResolveKey(root, concept.Privacy.KeyID)
		if err != nil {
			return Result{}, err
		}
		body, err := transformBody(concept.Body, *concept.Privacy, key)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Body:      body,
			Directive: concept.Privacy,
			Redacted:  concept.Privacy.Policy == contractv1.PrivacyEncrypt,
		}, nil
	}

	spans, _ := ScanInline(concept.Body)
	if len(spans) == 0 {
		return Result{Body: concept.Body}, nil
	}
	applied := make([]Span, len(spans))
	body := concept.Body
	for index := len(spans) - 1; index >= 0; index-- {
		span := spans[index]
		key, err := ResolveKey(root, span.KeyID)
		if err != nil {
			return Result{}, err
		}
		directive := contractv1.PrivacyDirective{
			Policy:   span.Policy,
			Provider: span.Provider,
			KeyID:    span.KeyID,
		}
		transformed, err := transformBody(body[span.Start:span.End], directive, key)
		if err != nil {
			return Result{}, err
		}
		body = body[:span.Start] + transformed + body[span.End:]
		span.Start = 0
		span.End = 0
		applied[index] = span
	}
	return Result{
		Body:      body,
		Directive: &contractv1.PrivacyDirective{Policy: contractv1.PrivacyMixed},
		Spans:     applied,
	}, nil
}

func transformBody(
	body string,
	directive contractv1.PrivacyDirective,
	key []byte,
) (string, error) {
	switch directive.Policy {
	case contractv1.PrivacyEncrypt:
		return encryptString(body, key)
	case contractv1.PrivacyTokenize:
		return tokenizeText(body, key), nil
	case contractv1.PrivacyHash:
		return hashText(body, key), nil
	default:
		return body, nil
	}
}

func encryptString(value string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(value), nil)
	return envelope + base64.StdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func decryptString(value string, key []byte) (string, error) {
	if !strings.HasPrefix(value, envelope) {
		return "", fmt.Errorf("%w: missing envelope", ErrDecrypt)
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, envelope))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("%w: short payload", ErrDecrypt)
	}
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	return string(plain), nil
}

func tokenizeText(value string, key []byte) string {
	return mapRuns(value, func(run string) string {
		return "tok" + hexMAC(key, strings.ToLower(run))
	})
}

func hashText(value string, key []byte) string {
	return "hash" + hexMAC(key, normalizeText(value))
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func hexMAC(key []byte, value string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

func mapRuns(value string, transform func(string) string) string {
	var builder strings.Builder
	start := -1
	for index, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			if start < 0 {
				start = index
			}
			continue
		}
		if start >= 0 {
			builder.WriteString(transform(value[start:index]))
			start = -1
		}
		builder.WriteRune(char)
	}
	if start >= 0 {
		builder.WriteString(transform(value[start:]))
	}
	return builder.String()
}

// DecryptBody restores encrypted content. Whole-body and inline envelopes are
// both supported.
func DecryptBody(
	root string,
	body string,
	directive *contractv1.PrivacyDirective,
	spans []Span,
) (string, error) {
	if directive != nil && directive.Policy == contractv1.PrivacyEncrypt &&
		strings.HasPrefix(strings.TrimSpace(body), envelope) {
		key, err := ResolveKey(root, directive.KeyID)
		if err != nil {
			return "", err
		}
		return decryptString(strings.TrimSpace(body), key)
	}

	result := body
	index := 0
	spanIndex := 0
	for {
		location := strings.Index(result[index:], envelope)
		if location < 0 {
			break
		}
		start := index + location
		end := start + len(envelope)
		for end < len(result) && isBase64(result[end]) {
			end++
		}
		keyID := ""
		if directive != nil {
			keyID = directive.KeyID
		}
		if spanIndex < len(spans) {
			keyID = spans[spanIndex].KeyID
		}
		spanIndex++
		if keyID == "" {
			return "", fmt.Errorf("%w: no key id for inline envelope", ErrDecrypt)
		}
		key, err := ResolveKey(root, keyID)
		if err != nil {
			return "", err
		}
		plain, err := decryptString(result[start:end], key)
		if err != nil {
			return "", err
		}
		result = result[:start] + plain + result[end:]
		index = start + len(plain)
	}
	return result, nil
}

func isBase64(char byte) bool {
	return char >= 'A' && char <= 'Z' ||
		char >= 'a' && char <= 'z' ||
		char >= '0' && char <= '9' ||
		char == '+' || char == '/' || char == '='
}

// StateFor returns the presentation state for one concept.
func StateFor(concept contractv1.OkfConcept) *contractv1.PrivacyState {
	if concept.Privacy == nil || concept.Privacy.Policy == contractv1.PrivacyNone {
		return nil
	}
	return &contractv1.PrivacyState{
		Policy:   concept.Privacy.Policy,
		Provider: concept.Privacy.Provider,
		KeyID:    concept.Privacy.KeyID,
		Redacted: concept.Privacy.Policy == contractv1.PrivacyEncrypt,
	}
}
