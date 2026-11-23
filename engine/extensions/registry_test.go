package extensions

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRoleRegistryIsComplete(t *testing.T) {
	want := []string{"enrichment", "project", "source", "rerank", "scorer", "embedder", "tokenizer"}
	if got := RoleNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("roles = %v, want %v", got, want)
	}
	for _, role := range Roles() {
		if role.Protocol != ProtocolVersion && role.Protocol != SourceProtocolVersion {
			t.Fatalf("%s has an unknown protocol %q", role.Name, role.Protocol)
		}
		if role.Timeout <= 0 || role.Timeout > 10*time.Second {
			t.Fatalf("%s has an invalid timeout %s", role.Name, role.Timeout)
		}
		switch role.Failure {
		case FailureFallback, FailureOperation, FailureRefuse:
		default:
			t.Fatalf("%s has an unknown failure policy %q", role.Name, role.Failure)
		}
		if role.CLI == "" || role.conformance == nil {
			t.Fatalf("%s is incomplete: %+v", role.Name, role)
		}
	}
}

func TestRoleLookup(t *testing.T) {
	if _, ok := Role("scorer"); !ok {
		t.Fatal("scorer is not registered")
	}
	if _, ok := Role("telepathy"); ok {
		t.Fatal("unknown role resolved")
	}
}

func TestRunRoleCheckUnknownRole(t *testing.T) {
	err := (Runner{Timeout: time.Second}).RunRoleCheck(context.Background(), Extension{Name: "x"}, "telepathy")
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected ErrProtocol, got %v", err)
	}
}
