package extensions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrUntrusted marks a bundle-local extension the user has not trusted.
var ErrUntrusted = errors.New("extension is untrusted")

// TrustRecord is one user grant for a bundle-local executable.
type TrustRecord struct {
	Path   string `yaml:"path"`
	SHA256 string `yaml:"sha256"`
}

type trustFile struct {
	Version    int                    `yaml:"version"`
	Extensions map[string]TrustRecord `yaml:"extensions"`
}

// TrustState reports one stored grant and whether the executable still matches it.
type TrustState struct {
	Name    string
	Path    string
	SHA256  string
	Matches bool
}

// TrustPath returns the per-user trust file under the home directory. WEAVE_TRUST_FILE overrides it
// for isolated environments and tests.
func TrustPath() string {
	if path := strings.TrimSpace(os.Getenv("WEAVE_TRUST_FILE")); path != "" {
		return path
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".weave", "trust.yaml")
	}
	return filepath.Join(".weave", "trust.yaml")
}

func loadTrust(path string) (trustFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return trustFile{Version: 1, Extensions: map[string]TrustRecord{}}, nil
	}
	if err != nil {
		return trustFile{}, err
	}
	file := trustFile{}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return trustFile{}, fmt.Errorf("invalid trust file: %w", err)
	}
	if file.Extensions == nil {
		file.Extensions = map[string]TrustRecord{}
	}
	return file, nil
}

func saveTrust(path string, file trustFile) error {
	file.Version = 1
	if file.Extensions == nil {
		file.Extensions = map[string]TrustRecord{}
	}
	data, err := yaml.Marshal(file)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".trust-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(name)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// FileHash returns the SHA-256 of one executable.
func FileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// CheckTrusted refuses a bundle-local executable that is not trusted for its current bytes. PATH
// extensions return nil because installing them is already a user action.
func CheckTrusted(root string, extension Extension) error {
	if extension.Source != "bundle" {
		return nil
	}
	file, err := loadTrust(TrustPath())
	if err != nil {
		return fmt.Errorf("%w: cannot read trust file", ErrUntrusted)
	}
	record, ok := file.Extensions[extension.Name]
	if !ok {
		return fmt.Errorf("%w: %s; run: weave extensions trust %s %s", ErrUntrusted, extension.Path, extension.Name, root)
	}
	path, err := filepath.Abs(extension.Path)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrUntrusted, extension.Name)
	}
	recorded, err := filepath.Abs(record.Path)
	if err != nil || recorded != path {
		return fmt.Errorf("%w: %s changed path; run: weave extensions trust %s %s", ErrUntrusted, extension.Name, extension.Name, root)
	}
	hash, err := FileHash(extension.Path)
	if err != nil || hash != record.SHA256 {
		return fmt.Errorf("%w: %s changed; run: weave extensions trust %s %s", ErrUntrusted, extension.Name, extension.Name, root)
	}
	return nil
}

// TrustExtension records an absolute path and hash for one bundle-local extension.
func TrustExtension(root, name string) (TrustRecord, error) {
	extension, err := Resolve(root, name)
	if err != nil {
		return TrustRecord{}, err
	}
	if extension.Source != "bundle" {
		return TrustRecord{}, fmt.Errorf("only bundle-local extensions need trust: %s is on PATH", name)
	}
	path, err := filepath.Abs(extension.Path)
	if err != nil {
		return TrustRecord{}, err
	}
	hash, err := FileHash(extension.Path)
	if err != nil {
		return TrustRecord{}, err
	}
	file, err := loadTrust(TrustPath())
	if err != nil {
		return TrustRecord{}, err
	}
	record := TrustRecord{Path: path, SHA256: hash}
	file.Extensions[name] = record
	if err := saveTrust(TrustPath(), file); err != nil {
		return TrustRecord{}, err
	}
	return record, nil
}

// UntrustExtension removes one grant.
func UntrustExtension(name string) error {
	file, err := loadTrust(TrustPath())
	if err != nil {
		return err
	}
	delete(file.Extensions, name)
	return saveTrust(TrustPath(), file)
}

// TrustStates lists stored grants with a fresh hash comparison.
func TrustStates() ([]TrustState, error) {
	file, err := loadTrust(TrustPath())
	if err != nil {
		return nil, err
	}
	states := make([]TrustState, 0, len(file.Extensions))
	for name, record := range file.Extensions {
		hash, hashErr := FileHash(record.Path)
		states = append(states, TrustState{
			Name:    name,
			Path:    record.Path,
			SHA256:  record.SHA256,
			Matches: hashErr == nil && hash == record.SHA256,
		})
	}
	sort.Slice(states, func(i, j int) bool { return states[i].Name < states[j].Name })
	return states, nil
}

// BundleTrustFileWarning reports a bundle file that attempts to grant trust. It is never read as
// authority; the warning makes the attempt visible.
func BundleTrustFileWarning(root string) string {
	path := filepath.Join(root, ".weave", "trust.yaml")
	data, err := os.ReadFile(path)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return ""
	}
	return "ignored " + path + ": bundle files cannot grant extension trust"
}
