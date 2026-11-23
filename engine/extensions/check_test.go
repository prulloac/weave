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
	trustFixture(t, root, name)
}

func trustFixture(t *testing.T, root, name string) {
	t.Helper()
	if os.Getenv("WEAVE_TRUST_FILE") == "" {
		t.Setenv("WEAVE_TRUST_FILE", filepath.Join(t.TempDir(), "trust.yaml"))
	}
	if _, err := TrustExtension(root, name); err != nil {
		t.Fatalf("trust %s: %v", name, err)
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
		{"rerank-request.json", func() RerankRequest {
			request := cannedRerankRequest()
			request.V = ProtocolVersion
			request.Type = "rerank_request"
			return request
		}()},
		{"scorer-request.json", func() ScorerRequest {
			request := cannedScorerRequest()
			request.V = ProtocolVersion
			request.Type = "scorer_request"
			request.CandidateK = len(request.Candidates)
			return request
		}()},
		{"embedding-request.json", func() EmbeddingRequest {
			request := cannedEmbeddingRequest()
			request.V = ProtocolVersion
			request.Type = "embedding_request"
			return request
		}()},
		{"tokenizer-request.json", func() TokenizerRequest {
			request := cannedTokenizerRequest()
			request.V = ProtocolVersion
			request.Type = "tokenizer_request"
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

// installCheckDispatch installs a fixture that answers each request type with its own response.
func installCheckDispatch(t *testing.T, root, name, manifest string, responses map[string]string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script extensions are POSIX only")
	}
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--describe\" ]; then\nprintf '%s' '" + manifest + "'\nexit 0\nfi\n" +
		"req=$(cat)\ncase \"$req\" in\n"
	for marker, response := range responses {
		body += "  *" + marker + "*) printf '%s' '" + response + "';;\n"
	}
	body += "  *) printf '%s' '{}';;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "weave-"+name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	trustFixture(t, root, name)
}

func TestCheckUsesDeclaredRoles(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	installCheckDispatch(t, root, "multi",
		`{"v":"weave.extension/v1","name":"multi","version":"0.11.0","protocols":["weave.cli/v1"],"roles":["enrichment","rerank"]}`,
		map[string]string{
			"enrichment_request": `{"v":"weave.cli/v1","inferred_keywords":["ok"]}`,
			"rerank_request":     `{"v":"weave.cli/v1","ranked_candidates":[{"id":"docs/adr-004.md","rerank_score":0.9},{"id":"docs/adr-001.md","rerank_score":0.1}]}`,
		})

	report, err := Check(context.Background(), root, "multi", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected a passing report: %+v", report.Checks)
	}
	want := []string{"enrichment", "manifest", "protocols", "rerank"}
	if got := checkNames(report); !reflect.DeepEqual(got, want) {
		t.Fatalf("checks = %v, want %v", got, want)
	}
}

func TestCheckLegacyKeysStillDeclareRoles(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		role     string
		response string
	}{
		{"document", `{"v":"weave.extension/v1","name":"x","version":"0.11.0","protocols":["weave.cli/v1"],"analysis_modes":["document"]}`, "enrichment", `{"v":"weave.cli/v1","inferred_keywords":["ok"]}`},
		{"project", `{"v":"weave.extension/v1","name":"x","version":"0.11.0","protocols":["weave.cli/v1"],"analysis_modes":["project"]}`, "project", `{"v":"weave.cli/v1","documents":[{"id":"src/main.go","title":"main"}],"relations":[]}`},
		{"source", `{"v":"weave.extension/v1","name":"x","version":"0.11.0","protocols":["weave.source/v1"],"source_providers":["git"]}`, "source", `{"v":"weave.source/v1","type":"source_response","snapshot":{"path":"/home/user/.cache/weave/sources/git-7f3a/worktree","source_id":"git-7f3a","revision":"8f3c2a1d4b5e6f708192a3b4c5d6e7f8091a2b3c"},"documents":[{"id":"_repo"}]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			useCheckCoreVersion(t, "0.11.0")
			root := t.TempDir()
			installCheckExtension(t, root, "x", c.manifest, c.response, "")
			report, err := Check(context.Background(), root, "x", time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := findCheck(report, c.role); !ok {
				t.Fatalf("role %q missing from %v", c.role, checkNames(report))
			}
		})
	}
}

func TestCheckRolesTakePrecedence(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	installCheckExtension(t, root, "score",
		`{"v":"weave.extension/v1","name":"score","version":"0.11.0","protocols":["weave.cli/v1"],"roles":["scorer"],"analysis_modes":["document"]}`,
		`{"v":"weave.cli/v1","ranked_candidates":[{"id":"docs/adr-004.md","score":0.9},{"id":"docs/adr-001.md","score":0.1}]}`,
		"")

	report, err := Check(context.Background(), root, "score", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findCheck(report, "enrichment"); ok {
		t.Fatalf("roles must take precedence: %v", checkNames(report))
	}
	if _, ok := findCheck(report, "scorer"); !ok {
		t.Fatalf("scorer must be checked: %v", checkNames(report))
	}
}

func TestCheckRejectsUnknownRole(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	installCheckExtension(t, root, "odd",
		`{"v":"weave.extension/v1","name":"odd","version":"0.11.0","protocols":["weave.cli/v1"],"roles":["enrichment","telepathy"]}`,
		`{"v":"weave.cli/v1","inferred_keywords":["ok"]}`,
		"")

	report, err := Check(context.Background(), root, "odd", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatal("expected a failing report")
	}
	check := checkResult(t, report, "manifest")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionProto {
		t.Fatalf("unexpected manifest check: %+v", check)
	}
	if _, ok := findCheck(report, "enrichment"); ok {
		t.Fatalf("no role check may run: %v", checkNames(report))
	}
}

func TestCheckRerankRole(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	installCheckExtension(t, root, "rerank",
		`{"v":"weave.extension/v1","name":"rerank","version":"0.11.0","protocols":["weave.cli/v1"],"roles":["rerank"]}`,
		`{"v":"weave.cli/v1","ranked_candidates":[{"id":"docs/adr-004.md","rerank_score":0.9},{"id":"docs/adr-001.md","rerank_score":0.1}]}`,
		"")

	report, err := Check(context.Background(), root, "rerank", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected a passing report: %+v", report.Checks)
	}
}

func TestCheckRerankMalformed(t *testing.T) {
	useCheckCoreVersion(t, "0.11.0")
	root := t.TempDir()
	installCheckExtension(t, root, "rerank",
		`{"v":"weave.extension/v1","name":"rerank","version":"0.11.0","protocols":["weave.cli/v1"],"roles":["rerank"]}`,
		`{"v":"weave.cli/v1","ranked_candidates":[{"id":"docs/adr-004.md","rerank_score":"high"}]}`,
		"")

	report, err := Check(context.Background(), root, "rerank", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	check := checkResult(t, report, "rerank")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionProto {
		t.Fatalf("unexpected rerank check: %+v", check)
	}
	if got := checkResult(t, report, "manifest"); got.Status != contractv1.ExtensionCheckPass {
		t.Fatalf("manifest should pass: %+v", got)
	}
}

func TestCheckScorerEmbedderTokenizerRoles(t *testing.T) {
	cases := []struct {
		role     string
		response string
	}{
		{"scorer", `{"v":"weave.cli/v1","ranked_candidates":[{"id":"docs/adr-004.md","score":0.9},{"id":"docs/adr-001.md","score":0.1}]}`},
		{"embedder", `{"v":"weave.cli/v1","model":"nomic-embed-text","dim":4,"vector":"AACAPwAAAAAAAAAAAAAAAA=="}`},
		{"tokenizer", `{"v":"weave.cli/v1","type":"tokenizer_response","terms":["token","normal","text"]}`},
	}
	for _, c := range cases {
		t.Run(c.role, func(t *testing.T) {
			useCheckCoreVersion(t, "0.11.0")
			root := t.TempDir()
			installCheckExtension(t, root, c.role,
				`{"v":"weave.extension/v1","name":"`+c.role+`","version":"0.11.0","protocols":["weave.cli/v1"],"roles":["`+c.role+`"]}`,
				c.response, "")
			report, err := Check(context.Background(), root, c.role, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if !report.OK {
				t.Fatalf("expected a passing report: %+v", report.Checks)
			}
			if got := checkResult(t, report, c.role); got.Status != contractv1.ExtensionCheckPass {
				t.Fatalf("%s = %+v", c.role, got)
			}
		})
	}
}

func TestCheckRoleValidators(t *testing.T) {
	cases := []struct {
		name     string
		role     string
		response string
	}{
		{"scorer missing candidate", "scorer", `{"v":"weave.cli/v1","ranked_candidates":[{"id":"docs/adr-004.md","score":0.9}]}`},
		{"embedder bad vector", "embedder", `{"v":"weave.cli/v1","model":"nomic-embed-text","dim":4,"vector":"AAAA"}`},
		{"tokenizer index expansions", "tokenizer", `{"v":"weave.cli/v1","type":"tokenizer_response","terms":["token"],"expansions":{"token":["tokenize"]}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			useCheckCoreVersion(t, "0.11.0")
			root := t.TempDir()
			installCheckExtension(t, root, c.role,
				`{"v":"weave.extension/v1","name":"`+c.role+`","version":"0.11.0","protocols":["weave.cli/v1"],"roles":["`+c.role+`"]}`,
				c.response, "")
			report, err := Check(context.Background(), root, c.role, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			check := checkResult(t, report, c.role)
			if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrExtensionProto {
				t.Fatalf("%s check = %+v", c.role, check)
			}
		})
	}
}
