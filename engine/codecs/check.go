package codecs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
)

const (
	codecConformanceText  = "weave codec conformance\n"
	codecConformanceBytes = 4096
)

// Check runs the manifest and raw-byte conformance checks for one codec extension. The name is the
// short codec name, so `codec-<name>` resolves to `weave-codec-<name>`.
func (r *Registry) Check(ctx context.Context, name string) (report contractv1.ExtensionCheckReport, err error) {
	report = contractv1.ExtensionCheckReport{
		V:         extensions.ProtocolVersion,
		Extension: "codec-" + name,
		Checks:    []contractv1.ExtensionCheckResult{},
	}
	defer finalizeCodecReport(&report)
	candidate, ok := r.candidate(name)
	if !ok {
		return report, fmt.Errorf("%w: codec-%s", extensions.ErrNotFound, name)
	}
	if err := extensions.CheckTrusted(r.BundleRoot, extensions.Extension{
		Name: "codec-" + name, Path: candidate.path, Source: "bundle",
	}); err != nil {
		report.Checks = append(report.Checks, failedCodecCheck("requirements", err))
		return report, nil
	}

	manifest, err := r.describe(ctx, candidate.path)
	if err != nil {
		report.Checks = append(report.Checks, failedCodecCheck("manifest", err))
		return report, nil
	}
	if manifest.Name != name || manifest.Kind != "codec" {
		report.Checks = append(report.Checks, failedCodecCheck("manifest", newError("CODEC_PROTOCOL", "codec manifest name does not match "+name)))
		return report, nil
	}
	for _, direction := range manifest.Directions {
		if direction != "compress" && direction != "decompress" {
			report.Checks = append(report.Checks, failedCodecCheck("manifest", newError("CODEC_PROTOCOL", "unknown codec direction: "+direction)))
			return report, nil
		}
	}
	report.Checks = append(report.Checks, passedCodecCheck("manifest", ""))

	if len(manifest.Requires) > 0 {
		warnings, err := extensions.CheckRequirements(ctx, r.BundleRoot, "codec "+name, manifest.Requires, r.Timeout)
		for _, warning := range warnings {
			if r.Warn != nil {
				r.Warn(warning)
			}
		}
		if err != nil {
			report.Checks = append(report.Checks, failedCodecCheck("requirements", err))
			return report, nil
		}
		report.Checks = append(report.Checks, passedCodecCheck("requirements", strings.Join(warnings, "; ")))
	}

	input := bytes.Repeat([]byte(codecConformanceText), codecConformanceBytes/len(codecConformanceText)+1)
	input = input[:codecConformanceBytes]

	compressed, err := r.execute(ctx, candidate.path, []string{"compress"}, input)
	if err != nil {
		report.Checks = append(report.Checks, failedCodecCheck("compress", err))
		return report, nil
	}
	report.Checks = append(report.Checks, passedCodecCheck("compress", ""))

	second, err := r.execute(ctx, candidate.path, []string{"compress"}, input)
	if err != nil {
		report.Checks = append(report.Checks, failedCodecCheck("determinism", err))
		return report, nil
	}
	if !bytes.Equal(compressed, second) {
		report.Checks = append(report.Checks, failedCodecCheck("determinism", newError("CODEC_PROTOCOL", "two compress runs produced different bytes")))
		return report, nil
	}
	report.Checks = append(report.Checks, passedCodecCheck("determinism", ""))

	decompressed, err := r.execute(ctx, candidate.path, []string{"decompress"}, compressed)
	if err != nil {
		report.Checks = append(report.Checks, failedCodecCheck("decompress", err))
		return report, nil
	}
	if !bytes.Equal(decompressed, input) {
		report.Checks = append(report.Checks, failedCodecCheck("decompress", newError("CODEC_PROTOCOL", "round trip changed the input bytes")))
		return report, nil
	}
	report.Checks = append(report.Checks, passedCodecCheck("decompress", ""))
	return report, nil
}

// finalizeCodecReport sorts the checks by name and sets the verdict. It runs on every return path so
// an unknown codec still yields an empty, well-formed report beside its error.
func finalizeCodecReport(report *contractv1.ExtensionCheckReport) {
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

func (r *Registry) candidate(name string) (candidate, bool) {
	for _, candidate := range r.candidates() {
		if candidate.name == name {
			return candidate, true
		}
	}
	return candidate{}, false
}

func passedCodecCheck(name, message string) contractv1.ExtensionCheckResult {
	return contractv1.ExtensionCheckResult{
		Check:   name,
		Status:  contractv1.ExtensionCheckPass,
		Message: message,
	}
}

func failedCodecCheck(name string, err error) contractv1.ExtensionCheckResult {
	return contractv1.ExtensionCheckResult{
		Check:   name,
		Status:  contractv1.ExtensionCheckFail,
		Code:    codecCheckErrorCode(err),
		Message: err.Error(),
	}
}

func codecCheckErrorCode(err error) contractv1.ErrorCode {
	var codecError *Error
	if errors.As(err, &codecError) {
		switch codecError.Code {
		case "CODEC_TIMEOUT":
			return contractv1.ErrCodecTimeout
		case "CODEC_PROTOCOL":
			return contractv1.ErrCodecProtocol
		case "CODEC_NOT_FOUND":
			return contractv1.ErrExtensionMissing
		case "EXTENSION_REQUIREMENT_UNMET":
			return contractv1.ErrExtensionRequirement
		default:
			return contractv1.ErrCodecFailed
		}
	}
	if errors.Is(err, extensions.ErrUntrusted) {
		return contractv1.ErrExtensionUntrusted
	}
	if errors.Is(err, extensions.ErrRequirementUnmet) {
		return contractv1.ErrExtensionRequirement
	}
	return contractv1.ErrCodecFailed
}
