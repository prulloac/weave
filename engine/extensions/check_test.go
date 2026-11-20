package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func installCheckExtension(t *testing.T, root, name, manifest, response, prelude string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script extensions are POSIX only")
	}
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then\nprintf '%s' '" + manifest + "'\nexit 0\nfi\n" +
		prelude + "cat >/dev/null\nprintf '%s' '" + response + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "weave-"+name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func useCheckCoreVersion(t *testing.T, release string) {
	t.Helper()
	previous := CoreVersion
	CoreVersion = release
	t.Cleanup(func() { CoreVersion = previous })
}

func checkNames(report contractv1.ExtensionCheckReport) []string {
	names := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		names = append(names, check.Check)
	}
	return names
}

func checkResult(t *testing.T, report contractv1.ExtensionCheckReport, name string) contractv1.ExtensionCheckResult {
	t.Helper()
	for _, check := range report.Checks {
		if check.Check == name {
			return check
		}
	}
	t.Fatalf("check %q is missing from %v", name, checkNames(report))
	return contractv1.ExtensionCheckResult{}
}

func TestCheckCannedRequestsMatchGoldenCorpus(t *testing.T) {
	cases := []struct {
		file    string
		request any
	}{
		{"enrichment-request.json", func() EnrichmentRequest {
			request := cannedEnrichmentRequest()
			request.V = ProtocolVersion
			request.Type = "enrichment_request"
			return request
		}()},
		{"project-analysis-request.json", func() ProjectAnalysisRequest {
			request := cannedProjectRequest()
			request.V = ProtocolVersion
			request.Type = "project_analysis_request"
			return request
		}()},
		{"source-request.json", func() SourceRequest {
			request := cannedSourceRequest()
			request.V = SourceProtocolVersion
			request.Type = "source_request"
			return request
		}()},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			path := filepath.Join("..", "..", "contract", "v1", "golden", "extension-ipc", c.file)
			golden, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			data, err := json.Marshal(c.request)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			if err := json.Unmarshal(golden, &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("canned request drifted from %s:\nwant %s\ngot  %s", c.file, golden, data)
			}
		})
	}
}

func TestCheckPassesDeclaredRoles(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	installCheckExtension(t, root, "ok",
		`{"v":"weave.extension/v1","name":"ok","version":"0.11.0","protocols":["weave.cli/v1"]}`,
		`{"v":"weave.cli/v1","inferred_keywords":["ok"]}`,
		"")

	report, err := Check(context.Background(), root, "ok", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected a passing report: %+v", report.Checks)
	}
	want := []string{"enrichment", "manifest", "protocols"}
	if got := checkNames(report); !reflect.DeepEqual(got, want) {
		t.Fatalf("checks = %v, want %v", got, want)
	}
}

func TestCheckFailsMalformedRoleResponse(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	installCheckExtension(t, root, "bad",
		`{"v":"weave.extension/v1","name":"bad","version":"0.11.0"}`,
		`{"v":"weave.cli/v1","inferred_keywords":"not-an-array"}`,
		"")

	report, err := Check(context.Background(), root, "bad", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatal("expected a failing report")
	}
	check := checkResult(t, report, "enrichment")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionProto {
		t.Fatalf("unexpected role check: %+v", check)
	}
	if got := checkResult(t, report, "manifest"); got.Status != contractv1.ExtensionCheckPass {
		t.Fatalf("manifest should pass: %+v", got)
	}
}

func TestCheckFailsTimeout(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	installCheckExtension(t, root, "slow",
		`{"v":"weave.extension/v1","name":"slow","version":"0.11.0"}`,
		`{"v":"weave.cli/v1","inferred_keywords":["slow"]}`,
		"sleep 2\n")

	report, err := Check(context.Background(), root, "slow", 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	check := checkResult(t, report, "enrichment")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionTimeout {
		t.Fatalf("unexpected role check: %+v", check)
	}
}

func TestCheckRequirementsFailureSkipsRoles(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	marker := filepath.Join(t.TempDir(), "invoked")
	t.Setenv("TEST_CHECK_MARKER", marker)
	installCheckExtension(t, root, "newer",
		`{"v":"weave.extension/v1","name":"newer","version":"0.1.0","requires":[{"name":"weave","min_version":"0.12.0"}]}`,
		`{"v":"weave.cli/v1","inferred_keywords":["newer"]}`,
		"echo invoked >> \"$TEST_CHECK_MARKER\"\n")

	report, err := Check(context.Background(), root, "newer", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	check := checkResult(t, report, "requirements")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionRequirement {
		t.Fatalf("unexpected requirements check: %+v", check)
	}
	if _, ok := findCheck(report, "enrichment"); ok {
		t.Fatal("role checks must not run when requirements are unmet")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("extension must not be invoked, stat err = %v", err)
	}
}

func TestCheckDescriptorMinimumBlocksUnversionedProvider(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	installCheckExtension(t, root, "git",
		`{"v":"weave.extension/v1","name":"git","source_providers":["git"]}`,
		`{"v":"weave.source/v1","type":"source_response"}`,
		"")

	report, err := Check(context.Background(), root, "git", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	check := checkResult(t, report, "requirements")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionRequirement {
		t.Fatalf("unexpected requirements check: %+v", check)
	}
	if _, ok := findCheck(report, "source"); ok {
		t.Fatal("source check must not run when the descriptor minimum is unmet")
	}
}

func TestCheckUnknownExtension(t *testing.T) {
	_, err := Check(context.Background(), t.TempDir(), "missing", time.Second)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func findCheck(report contractv1.ExtensionCheckReport, name string) (contractv1.ExtensionCheckResult, bool) {
	for _, check := range report.Checks {
		if check.Check == name {
			return check, true
		}
	}
	return contractv1.ExtensionCheckResult{}, false
}
