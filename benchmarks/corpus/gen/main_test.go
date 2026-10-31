package main

import (
	"reflect"
	"testing"
)

func TestGenerateDeterministic(t *testing.T) {
	first, err := generateAll(t.TempDir(), []string{"100"}, defaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generateAll(t.TempDir(), []string{"100"}, defaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("generated manifests differ")
	}

	summary := first.Profiles["100"]
	if summary.Documents != 100 || summary.BodyBytes == 0 || summary.Edges == 0 || summary.Keywords == 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.SHA256) != 64 {
		t.Fatalf("unexpected hash: %q", summary.SHA256)
	}
}

func TestProfileNames(t *testing.T) {
	if got := len(profileNames("")); got != len(profiles) {
		t.Fatalf("default profiles = %d, want %d", got, len(profiles))
	}
	names := profileNames("100, 1k")
	if !reflect.DeepEqual(names, []string{"100", "1k"}) {
		t.Fatalf("unexpected names: %v", names)
	}
}
