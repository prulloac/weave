package extensions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

// Check runs the manifest, requirement and role conformance checks for one extension.
//
// The canned requests mirror contract/v1/golden/extension-ipc; a test keeps them in sync. The check
// never reads bundle documents and never writes to a bundle or database.
func Check(ctx context.Context, root, name string, timeout time.Duration) (report contractv1.ExtensionCheckReport, err error) {
	report = contractv1.ExtensionCheckReport{
		V:         ProtocolVersion,
		Extension: name,
		Checks:    []contractv1.ExtensionCheckResult{},
	}
	defer finalizeReport(&report)
	extension, err := Resolve(root, name)
	if err != nil {
		return report, err
	}
	if err := CheckTrusted(root, extension); err != nil {
		report.Checks = append(report.Checks, failedCheck("requirements", err))
		return report, nil
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	manifest, err := DescribeWithTimeout(ctx, extension, timeout)
	if err != nil {
		report.Checks = append(report.Checks, failedCheck("manifest", err))
		return report, nil
	}
	roles, err := declaredRoles(manifest)
	if err != nil {
		report.Checks = append(report.Checks, failedCheck("manifest", err))
		return report, nil
	}
	report.Checks = append(report.Checks, passedCheck("manifest", ""))

	warnings, err := CheckRequirements(ctx, root, name, manifest.Requires, timeout)
	if err == nil && (manifestNeedsDescriptor(roles)) {
		warning, failure := CheckDescriptorVersion(extension.Name, manifest.Version)
		if warning != "" {
			warnings = append(warnings, warning)
		}
		err = failure
	}
	if err != nil {
		report.Checks = append(report.Checks, failedCheck("requirements", err))
		return report, nil
	}
	if len(manifest.Requires) > 0 || manifestNeedsDescriptor(roles) || len(warnings) > 0 {
		report.Checks = append(report.Checks, passedCheck("requirements", strings.Join(warnings, "; ")))
	}

	if result, ok := checkProtocols(manifest, roles); ok {
		report.Checks = append(report.Checks, result)
	}

	runner := Runner{Timeout: timeout}
	if manifest.SupportsCapability("batch") && containsRole(roles, "enrichment") {
		report.Checks = append(report.Checks, checkBatch(ctx, runner, extension))
	}
	for _, role := range roles {
		if err := runRoleCheck(ctx, runner, extension, role); err != nil {
			report.Checks = append(report.Checks, failedCheck(role, err))
			continue
		}
		report.Checks = append(report.Checks, passedCheck(role, ""))
	}

	return report, nil
}

// finalizeReport sorts the checks by name and sets the verdict. It runs on every return path so a
// failed check is reported in the same order and shape as a passing one.
func finalizeReport(report *contractv1.ExtensionCheckReport) {
	sort.Slice(report.Checks, func(i, j int) bool {
		return report.Checks[i].Check < report.Checks[j].Check
	})
	report.OK = len(report.Checks) > 0
	for _, check := range report.Checks {
		if check.Status != contractv1.ExtensionCheckPass {
			report.OK = false
			break
		}
	}
}

// declaredRoles reports the roles a manifest declares. A `roles` list wins over the older keys. When
// it is absent, enrichment has no explicit manifest key: an extension is an enricher unless it
// declares a source provider or opts out by declaring only non-document analysis modes.
func declaredRoles(manifest Manifest) ([]string, error) {
	if len(manifest.Roles) > 0 {
		declared := make(map[string]bool, len(manifest.Roles))
		for _, role := range manifest.Roles {
			if _, ok := Role(role); !ok {
				return nil, fmt.Errorf("%w: unknown role %q", ErrProtocol, role)
			}
			declared[role] = true
		}
		roles := make([]string, 0, len(declared))
		for _, role := range RoleNames() {
			if declared[role] {
				roles = append(roles, role)
			}
		}
		return roles, nil
	}
	roles := make([]string, 0, 3)
	source := len(manifest.SourceProviders) > 0
	if source {
		roles = append(roles, "source")
	}
	if manifest.SupportsProjectAnalysis() {
		roles = append(roles, "project")
	}
	if !source && supportsDocumentMode(manifest.AnalysisModes) {
		roles = append(roles, "enrichment")
	}
	return roles, nil
}

func supportsDocumentMode(modes []string) bool {
	if len(modes) == 0 {
		return true
	}
	for _, mode := range modes {
		if mode == "document" {
			return true
		}
	}
	return false
}

func manifestNeedsDescriptor(roles []string) bool {
	for _, role := range roles {
		if role == "source" || role == "project" {
			return true
		}
	}
	return false
}

func checkProtocols(manifest Manifest, roles []string) (contractv1.ExtensionCheckResult, bool) {
	if len(manifest.Protocols) == 0 {
		return contractv1.ExtensionCheckResult{}, false
	}
	wanted := make(map[string]bool, 2)
	var unknown []string
	for _, protocol := range manifest.Protocols {
		switch protocol {
		case ProtocolVersion, SourceProtocolVersion:
		default:
			unknown = append(unknown, protocol)
		}
	}
	for _, role := range roles {
		descriptor, ok := Role(role)
		if !ok {
			return failedCheck("protocols", fmt.Errorf("%w: unknown role %q", ErrProtocol, role)), true
		}
		wanted[descriptor.Protocol] = true
	}
	for protocol := range wanted {
		found := false
		for _, declared := range manifest.Protocols {
			if declared == protocol {
				found = true
				break
			}
		}
		if !found {
			return failedCheck("protocols", fmt.Errorf("%w: %s declares no %s", ErrProtocol, manifest.Name, protocol)), true
		}
	}
	message := ""
	if len(unknown) > 0 {
		message = "ignored protocol identifiers: " + strings.Join(unknown, ", ")
	}
	return passedCheck("protocols", message), true
}

func runRoleCheck(ctx context.Context, runner Runner, extension Extension, role string) error {
	return runner.RunRoleCheck(ctx, extension, role)
}

func cannedEnrichmentRequest() EnrichmentRequest {
	return EnrichmentRequest{
		DocID:      "docs/adr-004.md",
		DocPath:    "docs/adr-004.md",
		BundleRoot: "/home/user/notes",
		Workspace:  "/tmp/weave-extension-12345",
		Title:      "Database Storage Selection",
		Frontmatter: map[string]any{
			"type":     "adr",
			"keywords": []string{"database", "storage"},
		},
		Body: "We decided to adopt pure-Go SQLite...",
	}
}

func cannedProjectRequest() ProjectAnalysisRequest {
	return ProjectAnalysisRequest{
		BundleRoot: "/home/user/repo",
		Files: []ProjectFile{{
			ID:          "src/main.go",
			Path:        "src/main.go",
			MediaType:   "text/x-go",
			ContentHash: "sha256:4f2b1c",
		}},
		Options: ProjectOptions{
			SymbolDocuments: false,
			Relations:       []string{"imports", "declares"},
		},
	}
}

func cannedSourceRequest() SourceRequest {
	return SourceRequest{
		SourceID: "git-7f3a",
		URI:      "https://example.com/team/weave.git",
		Ref:      "main",
		CacheDir: "/home/user/.cache/weave",
		Options: SourceOptions{
			Worktree: true,
			Depth:    1,
		},
	}
}

func cannedRerankRequest() RerankRequest {
	return RerankRequest{
		Query: "zero cgo database",
		Candidates: []Candidate{
			{ID: "docs/adr-004.md", KeywordScore: 0.82, LexicalScore: 0.82, FuzzyScore: 0.45},
			{ID: "docs/adr-001.md", KeywordScore: 0.31, LexicalScore: 0.31, FuzzyScore: 0.1},
		},
	}
}

func cannedScorerRequest() ScorerRequest {
	return ScorerRequest{
		Query: "zero cgo database",
		Mode:  "hybrid",
		Candidates: []ScorerCandidate{
			{
				ID:       "docs/adr-004.md",
				Path:     "docs/adr-004.md",
				Title:    "Database Storage Selection",
				Type:     "adr",
				Keywords: []string{"sqlite", "storage"},
				Scores:   ScorerScores{Keyword: 0.82, Lexical: 0.82, Fuzzy: 0.45, Core: 0.64},
			},
			{
				ID:       "docs/adr-001.md",
				Path:     "docs/adr-001.md",
				Title:    "Keyword Search",
				Type:     "adr",
				Keywords: []string{"search"},
				Scores:   ScorerScores{Keyword: 0.31, Lexical: 0.31, Fuzzy: 0.1, Core: 0.21},
			},
		},
	}
}

func cannedEmbeddingRequest() EmbeddingRequest {
	return EmbeddingRequest{
		Text:      "zero cgo database",
		MediaType: "text/plain",
		Model:     "nomic-embed-text",
	}
}

func cannedTokenizerRequest() TokenizerRequest {
	return TokenizerRequest{
		Text:     "Tokenization normalizes text",
		Language: "en",
		Mode:     "index",
	}
}

func passedCheck(name, message string) contractv1.ExtensionCheckResult {
	return contractv1.ExtensionCheckResult{
		Check:   name,
		Status:  contractv1.ExtensionCheckPass,
		Message: message,
	}
}

func failedCheck(name string, err error) contractv1.ExtensionCheckResult {
	return contractv1.ExtensionCheckResult{
		Check:   name,
		Status:  contractv1.ExtensionCheckFail,
		Code:    checkErrorCode(err),
		Message: err.Error(),
	}
}

func checkBatch(ctx context.Context, runner Runner, extension Extension) contractv1.ExtensionCheckResult {
	_, _, err := runner.EnrichBatch(ctx, extension, []EnrichmentRequest{cannedEnrichmentRequest()})
	if err != nil {
		return failedCheck("batch", err)
	}
	return passedCheck("batch", "")
}

func containsRole(roles []string, role string) bool {
	for _, value := range roles {
		if value == role {
			return true
		}
	}
	return false
}

func checkErrorCode(err error) contractv1.ErrorCode {
	switch {
	case errors.Is(err, ErrUntrusted):
		return contractv1.ErrExtensionUntrusted
	case errors.Is(err, ErrRequirementUnmet):
		return contractv1.ErrExtensionRequirement
	case errors.Is(err, ErrTimeout):
		return contractv1.ErrExtensionTimeout
	case errors.Is(err, ErrNotFound):
		return contractv1.ErrExtensionMissing
	case errors.Is(err, ErrFailed):
		return contractv1.ErrExtensionFailed
	case errors.Is(err, ErrProtocol):
		return contractv1.ErrExtensionProto
	default:
		return contractv1.ErrExtensionProto
	}
}
