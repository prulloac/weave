package privacy

import (
	"strings"
	"unicode"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// KeyRef identifies one key used by a stored bundle.
type KeyRef struct {
	Policy   contractv1.PrivacyPolicy
	Provider string
	KeyID    string
}

// Resolver resolves a key id for one bundle root.
type Resolver func(keyID string) ([]byte, error)

// ExpandQuery returns deterministic aliases for a query under the given key
// references. References whose key cannot be resolved are skipped.
func ExpandQuery(query string, refs []KeyRef, resolve Resolver) []string {
	aliases := make([]string, 0, len(refs))
	seen := make(map[string]bool)
	for _, ref := range refs {
		if ref.KeyID == "" || resolve == nil {
			continue
		}
		key, err := resolve(ref.KeyID)
		if err != nil {
			continue
		}
		switch ref.Policy {
		case contractv1.PrivacyTokenize:
			for _, token := range queryTokens(query) {
				alias := "tok" + hexMAC(key, token)
				if !seen[alias] {
					seen[alias] = true
					aliases = append(aliases, alias)
				}
			}
		case contractv1.PrivacyHash:
			alias := "hash" + hexMAC(key, normalizeText(query))
			if !seen[alias] {
				seen[alias] = true
				aliases = append(aliases, alias)
			}
		}
	}
	return aliases
}

func queryTokens(query string) []string {
	tokens := make([]string, 0)
	var builder strings.Builder
	flush := func() {
		if builder.Len() > 0 {
			tokens = append(tokens, strings.ToLower(builder.String()))
			builder.Reset()
		}
	}
	for _, char := range query {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
			continue
		}
		flush()
	}
	flush()
	return tokens
}
