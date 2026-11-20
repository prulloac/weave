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
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	manifest, err := DescribeWithTimeout(ctx, extension, timeout)
	if err != nil {
		report.Checks = append(report.Checks, failedCheck("manifest", err))
		return report, nil
	}
	report.Checks = append(report.Checks, passedCheck("manifest", ""))

	roles := declaredRoles(manifest)
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

// declaredRoles reports the roles a manifest declares. Enrichment has no explicit manifest key: an
// extension is an enricher unless it declares a source provider or opts out by declaring only
// non-document analysis modes.
func declaredRoles(manifest Manifest) []string {
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
	return roles
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
		if role == "source" {
			wanted[SourceProtocolVersion] = true
		}
		if role == "enrichment" || role == "project" {
			wanted[ProtocolVersion] = true
		}
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
	switch role {
	case "enrichment":
		_, err := runner.Enrich(ctx, extension, cannedEnrichmentRequest())
		return err
	case "project":
		_, err := runner.ProjectAnalysis(ctx, extension, cannedProjectRequest())
		return err
	case "source":
		_, err := runner.MaterializeSource(ctx, extension, cannedSourceRequest())
		return err
	default:
		return fmt.Errorf("%w: unknown role %q", ErrProtocol, role)
	}
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

func checkErrorCode(err error) contractv1.ErrorCode {
	switch {
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
