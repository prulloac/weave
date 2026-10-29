package privacy

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type securityFile struct {
	Keys map[string]any `yaml:"keys"`
}

// ResolveKey resolves one key id from the environment or the bundle's
// .weave/security.yaml. The returned material is never logged.
func ResolveKey(root, keyID string) ([]byte, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, fmt.Errorf("%w: missing key id", ErrKeyUnavailable)
	}
	if value, ok := os.LookupEnv("WEAVE_KEY_" + normalizeKeyID(keyID)); ok && strings.TrimSpace(value) != "" {
		return deriveKey(value), nil
	}
	keys, err := loadSecurityFile(root)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{keyID, normalizeKeyID(keyID)} {
		if value, ok := keys[name]; ok {
			material := keyValue(value)
			if material != "" {
				return deriveKey(material), nil
			}
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrKeyUnavailable, keyID)
}

// HasKey reports whether a key id resolves without returning material.
func HasKey(root, keyID string) bool {
	_, err := ResolveKey(root, keyID)
	return err == nil
}

// SecurityFileWarning reports an overly permissive security file.
func SecurityFileWarning(root string) string {
	path := filepath.Join(root, ".weave", "security.yaml")
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if info.Mode().Perm()&0o077 != 0 {
		return CodeKeyFileInsecure
	}
	return ""
}

func loadSecurityFile(root string) (map[string]any, error) {
	path := filepath.Join(root, ".weave", "security.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrKeyUnavailable, path)
	}
	var file securityFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrKeyUnavailable, path, err)
	}
	if file.Keys == nil {
		file.Keys = map[string]any{}
	}
	return file.Keys, nil
}

func keyValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		if nested, ok := typed["value"].(string); ok {
			return nested
		}
	}
	return ""
}

func normalizeKeyID(keyID string) string {
	upper := strings.ToUpper(keyID)
	var builder strings.Builder
	for _, char := range upper {
		switch {
		case char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
			builder.WriteRune(char)
		default:
			builder.WriteByte('_')
		}
	}
	return builder.String()
}

func deriveKey(value string) []byte {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 64 {
		if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) == 32 {
			return decoded
		}
	}
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) == 32 {
		return decoded
	}
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}
