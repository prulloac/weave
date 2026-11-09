package main

import (
	"fmt"
	"io"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

type errorPolicy struct {
	exit int
	hint string
}

var errorPolicies = map[contractv1.ErrorCode]errorPolicy{
	contractv1.ErrInvalidRequest:        {2, "run `weave --help` or `weave help <command>`"},
	contractv1.ErrInvalidRoot:           {2, "check the bundle path and permissions"},
	contractv1.ErrPermissionDenied:      {3, "check file and directory permissions"},
	contractv1.ErrParseError:            {1, "run `weave parse <path>` for validation details"},
	contractv1.ErrConceptNotFound:       {1, "run `weave list <path>` to see available ids"},
	contractv1.ErrPathNotFound:          {1, "verify both ids and the links between them"},
	contractv1.ErrBundleNotIndexed:      {1, "run `weave ingest <path>` first"},
	contractv1.ErrPrivacyDirective:      {2, "run `weave parse <path>` for directive warnings"},
	contractv1.ErrPrivacyKey:            {3, "set WEAVE_KEY_<ID> or add the key to .weave/security.yaml"},
	contractv1.ErrPrivacyProvider:       {3, "check the provider algorithm against `weave-crypto-<name> --capabilities`"},
	contractv1.ErrPrivacyDecrypt:        {3, "check the key version and ciphertext"},
	contractv1.ErrExtensionMissing:      {2, "run `weave extensions list <path>`"},
	contractv1.ErrExtensionTimeout:      {1, "increase `--timeout` or check the extension"},
	contractv1.ErrExtensionFailed:       {1, "run the extension manually to see stderr"},
	contractv1.ErrExtensionProto:        {1, "check the extension weave.cli/v1 payload"},
	contractv1.ErrExtensionDataInvalid:  {1, "check the extension data manifest, key and metadata"},
	contractv1.ErrExtensionDataTooLarge: {1, "reduce the file, manifest or workspace size"},
	contractv1.ErrExtensionDataQuota:    {1, "remove unused extension data or reduce the artifact set"},
	contractv1.ErrExtensionDataNotFound: {1, "run `weave extensions data list <path>`"},
	contractv1.ErrAssetTypesRequired:    {2, "pass --asset-types or use an extension that advertises asset types"},
	contractv1.ErrBlobsRequireDB:        {2, "remove --no-db or disable --assets"},
	contractv1.ErrCodecNotFound:         {2, "install weave-codec-<name> or choose a built-in codec"},
	contractv1.ErrCodecUnavailable:      {1, "install the stored codec extension or rebuild the bundle"},
	contractv1.ErrCodecFailed:           {1, "run the codec manually to see stderr"},
	contractv1.ErrCodecTimeout:          {1, "increase --timeout or check the codec"},
	contractv1.ErrCodecProtocol:         {1, "check the codec --describe --json manifest"},
	contractv1.ErrBodyDecode:            {1, "rebuild the bundle from its source files"},
	contractv1.ErrConfirmation:          {2, "pass `--yes` after reviewing the warning"},
	contractv1.ErrInternal:              {1, "rerun with `--json` for details"},
}

func exitCodeFor(code contractv1.ErrorCode) int {
	if policy, ok := errorPolicies[code]; ok {
		return policy.exit
	}
	return 1
}

func hintFor(code contractv1.ErrorCode) string {
	return errorPolicies[code].hint
}

func printOperationError(writer io.Writer, code contractv1.ErrorCode, message string) {
	fmt.Fprintf(writer, "weave: %s: %s\n", code, message)
	if hint := hintFor(code); hint != "" {
		fmt.Fprintf(writer, "hint: %s\n", hint)
	}
}
