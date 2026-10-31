package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/privacy"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

type exportManifest struct {
	Contract    string `json:"contract"`
	CLI         string `json:"cli"`
	Format      string `json:"format"`
	Source      string `json:"source"`
	Documents   int    `json:"documents"`
	Encrypted   int    `json:"encrypted"`
	Tokenized   int    `json:"tokenized"`
	Hashed      int    `json:"hashed"`
	GeneratedAt string `json:"generated_at"`
}

func (e *Executor) export(ctx context.Context, request contractv1.OperationRequest) any {
	format := strings.ToLower(strings.TrimSpace(request.Format))
	if request.Out == "" {
		return failure(contractv1.ErrInvalidRequest, "export requires --out", nil)
	}
	if format != "raw" && format != "encrypted" {
		return failure(contractv1.ErrInvalidRequest, "format must be raw or encrypted", map[string]any{"format": request.Format})
	}
	if format == "raw" && !request.Confirmed {
		return failure(contractv1.ErrConfirmation, "raw export writes plaintext; pass --yes after confirming", nil)
	}
	if format == "encrypted" && request.KeyID == "" {
		return failure(contractv1.ErrInvalidRequest, "encrypted export requires --key-id", nil)
	}

	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), nil)
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"db": dbPath})
	}
	defer store.Close()

	root, err := filepath.Abs(request.Path)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"path": request.Path})
	}
	root = filepath.Clean(root)
	has, err := store.HasBundle(ctx, root)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	if !has {
		return failure(contractv1.ErrBundleNotIndexed, "bundle is not indexed; run: weave ingest "+request.Path, map[string]any{
			"path": request.Path,
		})
	}
	bundle, err := store.LoadBundle(ctx, root)
	if err != nil {
		code, message := mapStorageError(err)
		return failure(code, message, map[string]any{"path": request.Path})
	}
	provider, err := e.resolveProvider(root, request.Provider)
	if err != nil {
		return failure(contractv1.ErrPrivacyKey, err.Error(), map[string]any{"provider": request.Provider})
	}
	if provider != nil {
		defer provider.Close()
	}

	out, err := filepath.Abs(request.Out)
	if err != nil {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"out": request.Out})
	}
	out = filepath.Clean(out)
	if overlaps(root, out) {
		return failure(contractv1.ErrInvalidRequest, "export target must not overlap the source bundle", map[string]any{
			"path": root,
			"out":  out,
		})
	}

	if err := preflightExportKeys(root, bundle, format, request.KeyID, provider); err != nil {
		if errors.Is(err, privacy.ErrKeyUnavailable) {
			return failure(contractv1.ErrPrivacyKey, err.Error(), map[string]any{"path": request.Path})
		}
		return failure(contractv1.ErrInvalidRequest, err.Error(), nil)
	}

	target := out
	temporary := ""
	if info, err := os.Stat(out); err == nil {
		if !info.IsDir() {
			return failure(contractv1.ErrInvalidRequest, "export target is not a directory", map[string]any{"out": out})
		}
		entries, err := os.ReadDir(out)
		if err != nil {
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"out": out})
		}
		if len(entries) > 0 && !request.Force {
			return failure(contractv1.ErrInvalidRequest, "export target is not empty; pass --force", map[string]any{"out": out})
		}
	} else if os.IsNotExist(err) {
		temporary = fmt.Sprintf("%s.tmp-%d", out, os.Getpid())
		target = temporary
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"out": out})
		}
	} else {
		return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"out": out})
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return failure(contractv1.ErrInternal, err.Error(), map[string]any{"out": out})
	}
	cleanup := func() {
		if temporary != "" {
			_ = os.RemoveAll(temporary)
		}
	}

	report := contractv1.ExportReport{
		Source:    root,
		Out:       out,
		Format:    format,
		Documents: len(bundle.Concepts),
	}
	for _, concept := range bundle.Concepts {
		body := concept.Body
		switch conceptPolicy(concept) {
		case contractv1.PrivacyEncrypt:
			report.Encrypted++
		case contractv1.PrivacyTokenize:
			report.Tokenized++
		case contractv1.PrivacyHash:
			report.Hashed++
		}

		if format == "raw" {
			if conceptPolicy(concept) == contractv1.PrivacyEncrypt {
				plain, err := privacy.DecryptBodyWith(ctx, root, body, concept.Privacy, nil, provider)
				if err != nil {
					cleanup()
					return failure(exportPrivacyCode(err), err.Error(), map[string]any{"concept": concept.ID})
				}
				body = plain
			}
		} else {
			if conceptPolicy(concept) == contractv1.PrivacyEncrypt {
				plain, err := privacy.DecryptBodyWith(ctx, root, body, concept.Privacy, nil, provider)
				if err != nil {
					cleanup()
					return failure(exportPrivacyCode(err), err.Error(), map[string]any{"concept": concept.ID})
				}
				body = plain
			}
			encrypted, err := privacy.Apply(root, contractv1.OkfConcept{
				Body: body,
				Privacy: &contractv1.PrivacyDirective{
					Policy:   contractv1.PrivacyEncrypt,
					Provider: "aes-256-gcm",
					KeyID:    request.KeyID,
				},
			})
			if err != nil {
				cleanup()
				return failure(exportPrivacyCode(err), err.Error(), map[string]any{"concept": concept.ID})
			}
			body = encrypted.Body
		}

		frontmatter := make(map[string]any, len(concept.Frontmatter)+2)
		for key, value := range concept.Frontmatter {
			frontmatter[key] = value
		}
		frontmatter["keywords"] = concept.Keywords
		if format == "encrypted" {
			frontmatter["privacy"] = map[string]any{
				"policy":   "encrypt",
				"provider": "aes-256-gcm",
				"key_id":   request.KeyID,
				"encoded":  true,
			}
		}

		path, err := exportPath(target, concept.Path)
		if err != nil {
			cleanup()
			return failure(contractv1.ErrInvalidRequest, err.Error(), map[string]any{"concept": concept.ID})
		}
		content := renderExportDocument(frontmatter, body)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			cleanup()
			return failure(contractv1.ErrInternal, err.Error(), nil)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			cleanup()
			return failure(contractv1.ErrInternal, err.Error(), nil)
		}
	}

	if len(bundle.Index) > 0 {
		if err := os.WriteFile(filepath.Join(target, "index.md"), []byte(renderExportIndex(bundle.Index)), 0o644); err != nil {
			cleanup()
			return failure(contractv1.ErrInternal, err.Error(), nil)
		}
	}

	manifest := exportManifest{
		Contract:    ContractVersion,
		CLI:         CLIVersion,
		Format:      format,
		Source:      root,
		Documents:   report.Documents,
		Encrypted:   report.Encrypted,
		Tokenized:   report.Tokenized,
		Hashed:      report.Hashed,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		cleanup()
		return failure(contractv1.ErrInternal, err.Error(), nil)
	}
	if err := os.WriteFile(filepath.Join(target, "weave-export.json"), append(manifestJSON, '\n'), 0o644); err != nil {
		cleanup()
		return failure(contractv1.ErrInternal, err.Error(), nil)
	}

	if temporary != "" {
		if err := os.Rename(temporary, out); err != nil {
			cleanup()
			return failure(contractv1.ErrInternal, err.Error(), nil)
		}
	}
	return success(report)
}

func conceptPolicy(concept contractv1.OkfConcept) contractv1.PrivacyPolicy {
	if concept.Privacy == nil {
		return contractv1.PrivacyNone
	}
	return concept.Privacy.Policy
}

func preflightExportKeys(root string, bundle *contractv1.Bundle, format, keyID string, provider privacy.Provider) error {
	if format == "encrypted" {
		if _, err := privacy.ResolveKey(root, keyID); err != nil {
			return err
		}
	}
	for _, concept := range bundle.Concepts {
		policy := conceptPolicy(concept)
		if policy != contractv1.PrivacyEncrypt && policy != contractv1.PrivacyMixed {
			continue
		}
		if concept.Privacy == nil || concept.Privacy.KeyID == "" {
			continue
		}
		if concept.Privacy.ProviderName != "" {
			resolved := provider
			if resolved == nil || resolved.Name() != concept.Privacy.ProviderName {
				var err error
				resolved, err = privacy.ResolveProvider(root, concept.Privacy.ProviderName)
				if err != nil {
					return err
				}
				defer resolved.Close()
			}
			capabilities, err := resolved.Capabilities(context.Background())
			if err != nil {
				return err
			}
			if len(capabilities.KeyIDs) > 0 && !slices.Contains(capabilities.KeyIDs, concept.Privacy.KeyID) {
				return fmt.Errorf("%w: %s key %s", privacy.ErrKeyUnavailable, resolved.Name(), concept.Privacy.KeyID)
			}
			continue
		}
		if _, err := privacy.ResolveKey(root, concept.Privacy.KeyID); err != nil {
			return err
		}
	}
	return nil
}

func exportPrivacyCode(err error) contractv1.ErrorCode {
	switch {
	case errors.Is(err, privacy.ErrKeyUnavailable):
		return contractv1.ErrPrivacyKey
	case errors.Is(err, privacy.ErrDecrypt):
		return contractv1.ErrPrivacyDecrypt
	default:
		return contractv1.ErrInvalidRequest
	}
}

func overlaps(root, out string) bool {
	rootWithSeparator := root + string(os.PathSeparator)
	outWithSeparator := out + string(os.PathSeparator)
	return root == out ||
		strings.HasPrefix(outWithSeparator, rootWithSeparator) ||
		strings.HasPrefix(rootWithSeparator, outWithSeparator)
}

func exportPath(target, conceptPath string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(conceptPath))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid concept path %q", conceptPath)
	}
	return filepath.Join(target, clean), nil
}

func renderExportDocument(frontmatter map[string]any, body string) string {
	var builder strings.Builder
	builder.WriteString("---\n")
	keys := make([]string, 0, len(frontmatter))
	for key := range frontmatter {
		keys = append(keys, key)
	}
	sortStrings(keys)
	for _, key := range keys {
		writeYAMLValue(&builder, key, frontmatter[key])
	}
	builder.WriteString("---\n")
	if !strings.HasPrefix(body, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		builder.WriteString("\n")
	}
	return builder.String()
}

func writeYAMLValue(builder *strings.Builder, key string, value any) {
	switch typed := value.(type) {
	case string:
		fmt.Fprintf(builder, "%s: %q\n", key, typed)
	case []string:
		fmt.Fprintf(builder, "%s:\n", key)
		for _, entry := range typed {
			fmt.Fprintf(builder, "  - %q\n", entry)
		}
	case []any:
		fmt.Fprintf(builder, "%s:\n", key)
		for _, entry := range typed {
			if text, ok := entry.(string); ok {
				fmt.Fprintf(builder, "  - %q\n", text)
			}
		}
	case map[string]any:
		fmt.Fprintf(builder, "%s:\n", key)
		nested := make([]string, 0, len(typed))
		for nestedKey := range typed {
			nested = append(nested, nestedKey)
		}
		sortStrings(nested)
		for _, nestedKey := range nested {
			writeNestedYAML(builder, "  ", nestedKey, typed[nestedKey])
		}
	case bool:
		fmt.Fprintf(builder, "%s: %t\n", key, typed)
	case float64:
		fmt.Fprintf(builder, "%s: %v\n", key, typed)
	case int:
		fmt.Fprintf(builder, "%s: %d\n", key, typed)
	default:
		fmt.Fprintf(builder, "%s: %q\n", key, fmt.Sprint(typed))
	}
}

func writeNestedYAML(builder *strings.Builder, indent, key string, value any) {
	switch typed := value.(type) {
	case string:
		fmt.Fprintf(builder, "%s%s: %q\n", indent, key, typed)
	case bool:
		fmt.Fprintf(builder, "%s%s: %t\n", indent, key, typed)
	case []any:
		fmt.Fprintf(builder, "%s%s:\n", indent, key)
		for _, entry := range typed {
			if text, ok := entry.(string); ok {
				fmt.Fprintf(builder, "%s  - %q\n", indent, text)
			}
		}
	case map[string]any:
		fmt.Fprintf(builder, "%s%s:\n", indent, key)
		nested := make([]string, 0, len(typed))
		for nestedKey := range typed {
			nested = append(nested, nestedKey)
		}
		sortStrings(nested)
		for _, nestedKey := range nested {
			writeNestedYAML(builder, indent+"  ", nestedKey, typed[nestedKey])
		}
	default:
		fmt.Fprintf(builder, "%s%s: %q\n", indent, key, fmt.Sprint(typed))
	}
}

func renderExportIndex(entries []contractv1.IndexEntry) string {
	var builder strings.Builder
	builder.WriteString("# Index\n\n")
	for _, entry := range entries {
		fmt.Fprintf(&builder, "- [%s](%s)", entry.Title, entry.Target)
		if entry.Description != nil {
			fmt.Fprintf(&builder, " — %s", *entry.Description)
		}
		builder.WriteString("\n")
	}
	return builder.String()
}

func sortStrings(values []string) {
	for index := 1; index < len(values); index++ {
		for position := index; position > 0 && values[position] < values[position-1]; position-- {
			values[position], values[position-1] = values[position-1], values[position]
		}
	}
}
