package privacy

import (
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// AllowsAI reports whether a document opts into sending unredacted content to
// AI extensions via privacy.ai.send: true.
func AllowsAI(frontmatter map[string]any) bool {
	block, ok := frontmatter["privacy"].(map[string]any)
	if !ok {
		return false
	}
	ai, ok := block["ai"].(map[string]any)
	if !ok {
		return false
	}
	send, _ := ai["send"].(bool)
	return send
}

// RedactForAI returns the body safe to send to an extension. Documents with
// declared privacy policies are redacted unless they explicitly opt in.
func RedactForAI(concept contractv1.OkfConcept) string {
	if AllowsAI(concept.Frontmatter) {
		return concept.Body
	}
	if concept.Privacy != nil && concept.Privacy.Policy != contractv1.PrivacyNone {
		return "[redacted]"
	}
	spans, _ := ScanInline(concept.Body)
	if len(spans) == 0 {
		return concept.Body
	}
	body := concept.Body
	for index := len(spans) - 1; index >= 0; index-- {
		span := spans[index]
		if open := strings.LastIndex(body[:span.Start], "<!--"); open >= 0 {
			if close := strings.Index(body[span.End:], "-->"); close >= 0 {
				body = body[:open] + "[redacted]" + body[span.End+close+3:]
				continue
			}
		}
		body = body[:span.Start] + "[redacted]" + body[span.End:]
	}
	return body
}
