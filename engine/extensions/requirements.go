package extensions

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CoreVersion is the release version of the running weave binary. "dev" means the version is unknown.
var CoreVersion = "dev"

// DescriptorMinVersion is the oldest manifest version core accepts for a role that depends on the
// generic descriptor contract: source providers and project analyzers. It is the 0.11.0 line.
const DescriptorMinVersion = "0.11.0"

// ErrRequirementUnmet reports that an extension's requires entries are not satisfied.
var ErrRequirementUnmet = errors.New("extension requirement unmet")

// CheckDescriptorVersion reports whether an extension manifest version satisfies the minimum for a
// descriptor role. Core enforces it only when the running core is at or above the minimum line; a
// development build cannot compare versions and returns one warning instead of refusing.
func CheckDescriptorVersion(owner, version string) (string, error) {
	coreComparison, ok := CompareVersions(CoreVersion, DescriptorMinVersion)
	if !ok {
		return fmt.Sprintf("cannot compare descriptor minimum %s: core version %q is unknown", DescriptorMinVersion, CoreVersion), nil
	}
	if coreComparison < 0 {
		return "", nil
	}
	if strings.TrimSpace(version) == "" {
		return "", fmt.Errorf("%w: %s declares no version; the descriptor role requires >= %s", ErrRequirementUnmet, owner, DescriptorMinVersion)
	}
	comparison, ok := CompareVersions(version, DescriptorMinVersion)
	if !ok {
		return "", fmt.Errorf("%w: %s has invalid version %q; the descriptor role requires >= %s", ErrRequirementUnmet, owner, version, DescriptorMinVersion)
	}
	if comparison < 0 {
		return "", fmt.Errorf("%w: %s is %s; the descriptor role requires >= %s", ErrRequirementUnmet, owner, version, DescriptorMinVersion)
	}
	return "", nil
}

// Require resolves an extension, reads its manifest and enforces its requires entries.
// It returns warnings for requirements that cannot be compared, such as on a development build.
// An extension whose manifest cannot be read declares no requirements and is returned unchanged.
func Require(ctx context.Context, root, name string, timeout time.Duration) (Extension, []string, error) {
	extension, err := Resolve(root, name)
	if err != nil {
		return Extension{}, nil, err
	}
	if err := CheckTrusted(root, extension); err != nil {
		return Extension{}, nil, err
	}
	manifest, err := DescribeWithTimeout(ctx, extension, timeout)
	if err != nil {
		return extension, nil, nil
	}
	warnings, err := CheckRequirements(ctx, root, extension.Name, manifest.Requires, timeout)
	if err != nil {
		return Extension{}, warnings, err
	}
	return extension, warnings, nil
}

// CheckRequirements evaluates requires entries declared by owner. It is shared by every selection path,
// including codecs and crypto providers, which do not use Require because their manifests differ.
func CheckRequirements(ctx context.Context, root, owner string, requires []Requirement, timeout time.Duration) ([]string, error) {
	var warnings, unmet []string
	for _, requirement := range requires {
		warning, failure := checkRequirement(ctx, root, requirement, timeout)
		if warning != "" {
			warnings = append(warnings, warning)
		}
		if failure != "" {
			unmet = append(unmet, failure)
		}
	}
	if len(unmet) > 0 {
		return warnings, fmt.Errorf("%w: %s: %s", ErrRequirementUnmet, owner, strings.Join(unmet, "; "))
	}
	return warnings, nil
}

func checkRequirement(ctx context.Context, root string, requirement Requirement, timeout time.Duration) (string, string) {
	if _, ok := parseVersion(requirement.MinVersion); !ok {
		return "", fmt.Sprintf("requires %s with invalid min_version %q", requirement.Name, requirement.MinVersion)
	}
	if requirement.Name == "weave" {
		return checkCoreRequirement(requirement)
	}
	companion, err := Resolve(root, requirement.Name)
	if err != nil {
		return "", fmt.Sprintf("requires %s >= %s; %s is not installed", requirement.Name, requirement.MinVersion, requirement.Name)
	}
	manifest, err := DescribeWithTimeout(ctx, companion, timeout)
	if err != nil {
		return "", fmt.Sprintf("requires %s >= %s; %s has no readable manifest", requirement.Name, requirement.MinVersion, requirement.Name)
	}
	if manifest.Version == "" {
		return "", fmt.Sprintf("requires %s >= %s; %s declares no version", requirement.Name, requirement.MinVersion, requirement.Name)
	}
	comparison, ok := CompareVersions(manifest.Version, requirement.MinVersion)
	if !ok {
		return "", fmt.Sprintf("requires %s >= %s; installed %s has invalid version %q", requirement.Name, requirement.MinVersion, requirement.Name, manifest.Version)
	}
	if comparison < 0 {
		return "", fmt.Sprintf("requires %s >= %s; installed %s is %s", requirement.Name, requirement.MinVersion, requirement.Name, manifest.Version)
	}
	return "", ""
}

func checkCoreRequirement(requirement Requirement) (string, string) {
	comparison, ok := CompareVersions(CoreVersion, requirement.MinVersion)
	if !ok {
		return fmt.Sprintf("cannot compare requires weave >= %s: core version %q is unknown", requirement.MinVersion, CoreVersion), ""
	}
	if comparison < 0 {
		return "", fmt.Sprintf("requires weave >= %s; core is %s", requirement.MinVersion, CoreVersion)
	}
	return "", ""
}

// CompareVersions compares two semantic versions, ignoring a leading "v" and build metadata.
// It reports false when either value is not a MAJOR.MINOR.PATCH version.
func CompareVersions(left, right string) (int, bool) {
	a, ok := parseVersion(left)
	if !ok {
		return 0, false
	}
	b, ok := parseVersion(right)
	if !ok {
		return 0, false
	}
	for index := range a.core {
		switch {
		case a.core[index] < b.core[index]:
			return -1, true
		case a.core[index] > b.core[index]:
			return 1, true
		}
	}
	switch {
	case a.pre == b.pre:
		return 0, true
	case a.pre == "":
		return 1, true
	case b.pre == "":
		return -1, true
	case a.pre < b.pre:
		return -1, true
	default:
		return 1, true
	}
}

type parsedVersion struct {
	core [3]int
	pre  string
}

func parseVersion(value string) (parsedVersion, bool) {
	value = strings.TrimPrefix(value, "v")
	if index := strings.IndexByte(value, '+'); index >= 0 {
		value = value[:index]
	}
	var out parsedVersion
	if index := strings.IndexByte(value, '-'); index >= 0 {
		out.pre = value[index+1:]
		value = value[:index]
		if out.pre == "" {
			return parsedVersion{}, false
		}
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return parsedVersion{}, false
	}
	for index, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return parsedVersion{}, false
		}
		number, err := strconv.Atoi(part)
		if err != nil {
			return parsedVersion{}, false
		}
		out.core[index] = number
	}
	return out, true
}
