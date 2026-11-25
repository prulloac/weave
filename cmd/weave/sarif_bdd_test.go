package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Behavior-level tests for audit SARIF output. See docs/bdd/sarif-output.md.

type sarifDocument struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []struct {
		Tool struct {
			Driver struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				Rules   []struct {
					ID string `json:"id"`
				} `json:"rules"`
			} `json:"driver"`
		} `json:"tool"`
		Results []struct {
			RuleID  string `json:"ruleId"`
			Level   string `json:"level"`
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
			Locations []struct {
				PhysicalLocation struct {
					ArtifactLocation struct {
						URI string `json:"uri"`
					} `json:"artifactLocation"`
				} `json:"physicalLocation"`
			} `json:"locations"`
		} `json:"results"`
	} `json:"runs"`
}

func decodeSARIF(t *testing.T, output string) sarifDocument {
	t.Helper()
	var document sarifDocument
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		t.Fatalf("decode SARIF: %v\n%s", err, output)
	}
	return document
}

func TestSARIFShape(t *testing.T) {
	code, stdout, stderr := InvokeRead(t, "audit", auditFixture("invalid-docs"), "--format", "sarif")
	if code == 0 {
		t.Fatalf("expected the gate to fail, got 0 stderr=%s", stderr)
	}
	document := decodeSARIF(t, stdout)
	if document.Version != "2.1.0" || !strings.Contains(document.Schema, "sarif") {
		t.Fatalf("unexpected header: %+v", document)
	}
	if len(document.Runs) != 1 || document.Runs[0].Tool.Driver.Name != "weave" {
		t.Fatalf("unexpected run: %+v", document.Runs)
	}
	if len(document.Runs[0].Results) == 0 {
		t.Fatal("expected results")
	}
}

func TestSARIFMapping(t *testing.T) {
	_, stdout, _ := InvokeRead(t, "audit", auditFixture("invalid-docs"), "--format", "sarif")
	document := decodeSARIF(t, stdout)
	rules := map[string]bool{}
	for _, rule := range document.Runs[0].Tool.Driver.Rules {
		rules[rule.ID] = true
	}
	for _, result := range document.Runs[0].Results {
		if !rules[result.RuleID] {
			t.Fatalf("result rule %q is not declared", result.RuleID)
		}
		if result.Level != "error" && result.Level != "warning" && result.Level != "note" {
			t.Fatalf("unexpected level %q", result.Level)
		}
	}
}

func TestSARIFLocations(t *testing.T) {
	root := t.TempDir()
	writeTypeSchema(t, root, "adr.schema.json", `{"type":"object","required":["status"]}`)
	writeTypeDoc(t, root, "a.md", "type: adr\nkeywords:\n  - adr\n")
	_, stdout, _ := InvokeRead(t, "audit", root, "--format", "sarif")
	document := decodeSARIF(t, stdout)
	found := false
	for _, result := range document.Runs[0].Results {
		if result.RuleID != "TYPE_SCHEMA_INVALID" || len(result.Locations) == 0 {
			continue
		}
		if result.Locations[0].PhysicalLocation.ArtifactLocation.URI == "a.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("location missing: %+v", document.Runs[0].Results)
	}
}

func TestSARIFDeterministic(t *testing.T) {
	_, first, _ := InvokeRead(t, "audit", auditFixture("invalid-docs"), "--format", "sarif")
	_, second, _ := InvokeRead(t, "audit", auditFixture("invalid-docs"), "--format", "sarif")
	if first != second {
		t.Fatal("SARIF output is not deterministic")
	}
}

func TestSARIFGate(t *testing.T) {
	code, _, _ := InvokeRead(t, "audit", auditFixture("invalid-docs"), "--format", "sarif")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	code, stdout, _ := InvokeRead(t, "audit", auditFixture("invalid-docs"), "--format", "sarif", "--fail-on", "none")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	document := decodeSARIF(t, stdout)
	if len(document.Runs[0].Results) == 0 {
		t.Fatal("fail-on none dropped findings")
	}
}

func TestSARIFWithJSON(t *testing.T) {
	code, _, stderr := InvokeRead(t, "audit", auditFixture("invalid-docs"), "--format", "sarif", "--json")
	if code != 2 || !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected a usage error, got %d %q", code, stderr)
	}
}

func TestSARIFOutFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "audit.sarif")
	code, stdout, stderr := InvokeRead(t, "audit", auditFixture("invalid-docs"), "--format", "sarif", "--out", out)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout must be empty with --out, got %q", stdout)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	document := decodeSARIF(t, string(data))
	if document.Version != "2.1.0" {
		t.Fatalf("unexpected file: %+v", document)
	}
}

func TestSARIFNoPartialFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "audit.sarif")
	code, _, _ := InvokeRead(t, "audit", auditFixture("invalid-docs"), "--format", "bogus", "--out", out)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("no file may be written on failure, stat err = %v", err)
	}
}
