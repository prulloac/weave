package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
		if role.RequestType == "" || role.ResponseType == "" {
			t.Fatalf("%s is missing request/response types: %+v", role.Name, role)
		}
		if role.MaxRequestBytes <= 0 || role.MaxResponseBytes <= 0 {
			t.Fatalf("%s is missing size bounds: %+v", role.Name, role)
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

func TestSelectRole(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	installCheckExtension(t, root, "x",
		`{"v":"weave.extension/v1","name":"x","version":"0.11.0","roles":["rerank"]}`,
		`{"v":"weave.cli/v1","ranked_candidates":[]}`, "")

	extension, manifest, warnings, err := SelectRole(context.Background(), root, "x", "rerank", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if extension.Name != "x" || len(warnings) != 0 {
		t.Fatalf("selection = %+v warnings=%v", extension, warnings)
	}
	if len(manifest.Roles) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
	if _, _, _, err := SelectRole(context.Background(), root, "x", "enrichment", time.Second); !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected ErrProtocol for an undeclared role, got %v", err)
	}
}

func TestSelectRoleRejectsUntrusted(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".weave", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".weave", "bin", "weave-x")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("WEAVE_TRUST_FILE") == "" {
		t.Setenv("WEAVE_TRUST_FILE", filepath.Join(t.TempDir(), "trust.yaml"))
	}
	if _, _, _, err := SelectRole(context.Background(), root, "x", "rerank", time.Second); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("expected ErrUntrusted, got %v", err)
	}
}
