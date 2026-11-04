package main

import (
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestEveryContractErrorCodeHasCLIPolicy(t *testing.T) {
	code, stdout, stderr := Invoke(t, "contract", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("contract exit=%d stderr=%q", code, stderr)
	}
	info := Decode[contractv1.ContractInfo](t, decodeEnvelope(t, stdout).Result)
	if len(info.Errors) != 19 {
		t.Fatalf("expected 19 contract errors, got %d", len(info.Errors))
	}
	if len(errorPolicies) != 19 {
		t.Fatalf("expected 19 CLI error policies, got %d", len(errorPolicies))
	}
	for _, errorCode := range info.Errors {
		policy, ok := errorPolicies[errorCode]
		if !ok {
			t.Errorf("missing CLI policy for %s", errorCode)
			continue
		}
		if policy.exit < 0 || policy.exit > 3 {
			t.Errorf("%s: unexpected exit %d", errorCode, policy.exit)
		}
		if policy.hint == "" {
			t.Errorf("%s: missing remediation hint", errorCode)
		}
		if got := exitCodeFor(errorCode); got != policy.exit {
			t.Errorf("%s: exitCodeFor = %d, want %d", errorCode, got, policy.exit)
		}
		if got := hintFor(errorCode); got != policy.hint {
			t.Errorf("%s: hintFor = %q, want %q", errorCode, got, policy.hint)
		}
	}
}
