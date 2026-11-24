package operations

import (
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func TestEnrichmentParallelism(t *testing.T) {
	cases := []struct {
		value int
		want  int
		fail  bool
	}{
		{0, 1, false},
		{1, 1, false},
		{8, 8, false},
		{9, 0, true},
		{-1, 0, true},
	}
	for _, c := range cases {
		got, err := enrichmentParallelism(contractv1.OperationRequest{Parallelism: c.value})
		if c.fail {
			if err == nil {
				t.Fatalf("parallelism %d should fail", c.value)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Fatalf("parallelism %d = %d, %v; want %d", c.value, got, err, c.want)
		}
	}
}

func TestMCPStaysSequential(t *testing.T) {
	got, err := enrichmentParallelism(contractv1.OperationRequest{})
	if err != nil || got != 1 {
		t.Fatalf("MCP default parallelism = %d, %v; want 1", got, err)
	}
}
