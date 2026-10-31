package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/prulloac/weave/benchmarks/benchutil"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestReportSchema(t *testing.T) {
	value := report{
		Schema:     schemaVersion,
		Commit:     "abc",
		Go:         "go1.27",
		Runner:     "linux/amd64",
		GOMAXPROCS: 4,
		Corpus: corpusInfo{
			Name: "10k", SHA256: "hash", Docs: 10000, BodyBytes: 1000, Edges: 200,
		},
		Results: []result{{
			ID: "WEAVE-B-CLI-VERSION-COLD", Unit: "ms", Aggregate: "p95",
			Value: 4.2, Samples: []float64{4.0, 4.2}, Status: "pass",
		}},
	}
	validateReport(t, value)
}

func validateReport(t *testing.T, value report) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	schemaData, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "schema", "bench-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaData))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://weave.dev/bench/v1.json", schemaDoc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("https://weave.dev/bench/v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("report validation failed: %v", err)
	}
}

func TestCommittedBaseline(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "baseline", "10k.json"))
	if os.IsNotExist(err) {
		t.Skip("baseline not generated")
	}
	if err != nil {
		t.Fatal(err)
	}
	var value report
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	validateReport(t, value)
	if len(value.Results) == 0 {
		t.Fatal("baseline has no results")
	}
}

func TestCompareBaseline(t *testing.T) {
	registry := map[string]benchutil.RegistryEntry{
		"WEAVE-B-TEST": {ID: "WEAVE-B-TEST", Blocking: true},
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	baseline := report{Schema: schemaVersion, Results: []result{{ID: "WEAVE-B-TEST", Value: 100, Status: "pass"}}}
	data, err := json.Marshal(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	current := report{Results: []result{{ID: "WEAVE-B-TEST", Value: 105, Status: "pass"}}}
	if err := compareBaseline(&current, path, registry); err != nil {
		t.Fatalf("5%% change should pass: %v", err)
	}
	if current.Results[0].Status != "pass" {
		t.Fatalf("unexpected status: %s", current.Results[0].Status)
	}

	current = report{Results: []result{{ID: "WEAVE-B-TEST", Value: 115, Status: "pass"}}}
	if err := compareBaseline(&current, path, registry); err == nil {
		t.Fatal("15% change should fail")
	}
	if current.Results[0].Status != "fail" {
		t.Fatalf("unexpected status: %s", current.Results[0].Status)
	}

	current = report{}
	if err := compareBaseline(&current, path, registry); err == nil {
		t.Fatal("missing blocking result should fail")
	}
}

func TestAggregateSamples(t *testing.T) {
	samples := []float64{4, 1, 3, 2, 5}
	if got := aggregateSamples("p50", samples); got != 3 {
		t.Fatalf("p50 = %v", got)
	}
	if got := aggregateSamples("p95", samples); got != 5 {
		t.Fatalf("p95 = %v", got)
	}
}
