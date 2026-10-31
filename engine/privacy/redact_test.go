package privacy_test

import (
	"strings"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/privacy"
)

func TestRedactForAI(t *testing.T) {
	cases := []struct {
		name    string
		concept contractv1.OkfConcept
		want    string
	}{
		{
			name: "encrypted body",
			concept: contractv1.OkfConcept{
				Body:    "zanzibar-secret",
				Privacy: &contractv1.PrivacyDirective{Policy: contractv1.PrivacyEncrypt, KeyID: "k"},
			},
			want: "[redacted]",
		},
		{
			name: "tokenized body",
			concept: contractv1.OkfConcept{
				Body:    "internal-client-alpha",
				Privacy: &contractv1.PrivacyDirective{Policy: contractv1.PrivacyTokenize, KeyID: "k"},
			},
			want: "[redacted]",
		},
		{
			name: "hashed body",
			concept: contractv1.OkfConcept{
				Body:    "CR-2026-0042",
				Privacy: &contractv1.PrivacyDirective{Policy: contractv1.PrivacyHash, KeyID: "k"},
			},
			want: "[redacted]",
		},
		{
			name: "mixed keeps prose",
			concept: contractv1.OkfConcept{
				Body: "Public: <!-- weave:tokenize key_id=\"k\" -->internal-client-alpha<!-- /weave:tokenize --> done",
			},
			want: "Public: [redacted] done",
		},
		{
			name: "no directive",
			concept: contractv1.OkfConcept{
				Body: "plain body",
			},
			want: "plain body",
		},
		{
			name: "ai send opt in",
			concept: contractv1.OkfConcept{
				Body: "zanzibar-secret",
				Privacy: &contractv1.PrivacyDirective{
					Policy: contractv1.PrivacyEncrypt,
					KeyID:  "k",
				},
				Frontmatter: map[string]any{
					"privacy": map[string]any{
						"policy": "encrypt",
						"key_id": "k",
						"ai":     map[string]any{"send": true},
					},
				},
			},
			want: "zanzibar-secret",
		},
	}
	for _, testCase := range cases {
		got := privacy.RedactForAI(testCase.concept)
		if got != testCase.want {
			t.Fatalf("%s: got %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestAllowsAI(t *testing.T) {
	if !privacy.AllowsAI(map[string]any{"privacy": map[string]any{"ai": map[string]any{"send": true}}}) {
		t.Fatal("expected opt-in")
	}
	if privacy.AllowsAI(map[string]any{"privacy": map[string]any{"ai": map[string]any{"send": false}}}) {
		t.Fatal("expected opt-out")
	}
	if privacy.AllowsAI(map[string]any{"privacy": map[string]any{"policy": "encrypt"}}) {
		t.Fatal("missing ai block must not opt in")
	}
	if privacy.AllowsAI(nil) {
		t.Fatal("nil frontmatter must not opt in")
	}
}

func TestRedactForAIBodyBounds(t *testing.T) {
	concept := contractv1.OkfConcept{Body: strings.Repeat("x", 100)}
	if got := privacy.RedactForAI(concept); got != concept.Body {
		t.Fatal("unprotected bodies must be unchanged")
	}
}
