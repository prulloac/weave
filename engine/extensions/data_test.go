package extensions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dataWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "files"), 0o755); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func writeDataFile(t *testing.T, workspace, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(workspace, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeDataManifest(t *testing.T, workspace, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(workspace, "manifest.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hashHex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func TestValidateWorkspaceAcceptsOpaqueFiles(t *testing.T) {
	workspace := dataWorkspace(t)
	payload := []byte("hello extension data")
	writeDataFile(t, workspace, "files/notes/hello.txt", payload)
	writeDataManifest(t, workspace, `{
		"v": "weave.extension-data/v1",
		"items": [{
			"key": "notes/hello.txt",
			"path": "files/notes/hello.txt",
			"media_type": "text/plain",
			"metadata": {"language": "text"},
			"sha256": "`+hashHex(payload)+`"
		}]
	}`)

	files, err := ValidateWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one file, got %d", len(files))
	}
	file := files[0]
	if file.Key != "notes/hello.txt" || file.MediaType != "text/plain" {
		t.Fatalf("unexpected file: %+v", file)
	}
	if file.Size != int64(len(payload)) || file.SHA256 != hashHex(payload) {
		t.Fatalf("unexpected hash or size: %+v", file)
	}
	if !strings.Contains(string(file.Metadata), "language") {
		t.Fatalf("metadata not preserved: %s", file.Metadata)
	}
}

func TestValidateWorkspaceAcceptsEmptyManifest(t *testing.T) {
	workspace := dataWorkspace(t)
	writeDataManifest(t, workspace, `{"v":"weave.extension-data/v1","items":[]}`)

	files, err := ValidateWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no files, got %d", len(files))
	}
}

func TestValidateWorkspaceWithoutManifest(t *testing.T) {
	files, err := ValidateWorkspace(dataWorkspace(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no files, got %d", len(files))
	}
}

func TestValidateWorkspaceRejectsInvalidInputs(t *testing.T) {
	valid := `{"key":"a.txt","path":"files/a.txt"}`
	tests := []struct {
		name     string
		manifest string
		setup    func(t *testing.T, workspace string)
		want     error
	}{
		{
			name:     "wrong version",
			manifest: `{"v":"weave.extension-data/v2","items":[` + valid + `]}`,
			setup: func(t *testing.T, workspace string) {
				writeDataFile(t, workspace, "files/a.txt", []byte("a"))
			},
			want: ErrDataInvalid,
		},
		{
			name:     "duplicate key",
			manifest: `{"v":"weave.extension-data/v1","items":[` + valid + `,` + valid + `]}`,
			setup: func(t *testing.T, workspace string) {
				writeDataFile(t, workspace, "files/a.txt", []byte("a"))
			},
			want: ErrDataInvalid,
		},
		{
			name:     "path traversal",
			manifest: `{"v":"weave.extension-data/v1","items":[{"key":"a","path":"files/../outside"}]}`,
			setup: func(t *testing.T, workspace string) {
				writeDataFile(t, workspace, "files/a.txt", []byte("a"))
			},
			want: ErrDataInvalid,
		},
		{
			name:     "absolute path",
			manifest: `{"v":"weave.extension-data/v1","items":[{"key":"a","path":"/etc/passwd"}]}`,
			want:     ErrDataInvalid,
		},
		{
			name:     "missing file",
			manifest: `{"v":"weave.extension-data/v1","items":[` + valid + `]}`,
			want:     ErrDataInvalid,
		},
		{
			name:     "bad hash",
			manifest: `{"v":"weave.extension-data/v1","items":[{"key":"a.txt","path":"files/a.txt","sha256":"deadbeef"}]}`,
			setup: func(t *testing.T, workspace string) {
				writeDataFile(t, workspace, "files/a.txt", []byte("a"))
			},
			want: ErrDataInvalid,
		},
		{
			name:     "invalid key",
			manifest: `{"v":"weave.extension-data/v1","items":[{"key":"../a","path":"files/a.txt"}]}`,
			setup: func(t *testing.T, workspace string) {
				writeDataFile(t, workspace, "files/a.txt", []byte("a"))
			},
			want: ErrDataInvalid,
		},
		{
			name:     "media type too long",
			manifest: `{"v":"weave.extension-data/v1","items":[{"key":"a","path":"files/a.txt","media_type":"` + strings.Repeat("x", MaxMediaType+1) + `"}]}`,
			setup: func(t *testing.T, workspace string) {
				writeDataFile(t, workspace, "files/a.txt", []byte("a"))
			},
			want: ErrDataInvalid,
		},
		{
			name:     "metadata not object",
			manifest: `{"v":"weave.extension-data/v1","items":[{"key":"a","path":"files/a.txt","metadata":[1,2]}]}`,
			setup: func(t *testing.T, workspace string) {
				writeDataFile(t, workspace, "files/a.txt", []byte("a"))
			},
			want: ErrDataInvalid,
		},
		{
			name:     "metadata too large",
			manifest: `{"v":"weave.extension-data/v1","items":[{"key":"a","path":"files/a.txt","metadata":{"x":"` + strings.Repeat("x", MaxMetadata+1) + `"}}]}`,
			setup: func(t *testing.T, workspace string) {
				writeDataFile(t, workspace, "files/a.txt", []byte("a"))
			},
			want: ErrDataTooLarge,
		},
		{
			name:     "too many items",
			manifest: tooManyItemsManifest(),
			want:     ErrDataInvalid,
		},
		{
			name:     "oversized file",
			manifest: `{"v":"weave.extension-data/v1","items":[` + valid + `]}`,
			setup: func(t *testing.T, workspace string) {
				path := filepath.Join(workspace, "files", "a.txt")
				file, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if err := file.Truncate(MaxFileSize + 1); err != nil {
					t.Fatal(err)
				}
			},
			want: ErrDataTooLarge,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := dataWorkspace(t)
			if testCase.setup != nil {
				testCase.setup(t, workspace)
			}
			writeDataManifest(t, workspace, testCase.manifest)
			_, err := ValidateWorkspace(workspace)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestValidateWorkspaceRejectsSymlink(t *testing.T) {
	workspace := dataWorkspace(t)
	target := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(workspace, "files", "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	writeDataManifest(t, workspace, `{"v":"weave.extension-data/v1","items":[{"key":"link","path":"files/link.txt"}]}`)
	if _, err := ValidateWorkspace(workspace); !errors.Is(err, ErrDataInvalid) {
		t.Fatalf("error = %v, want %v", err, ErrDataInvalid)
	}
}

func TestValidateWorkspaceRejectsOversizedManifest(t *testing.T) {
	workspace := dataWorkspace(t)
	writeDataManifest(t, workspace, `{"v":"weave.extension-data/v1","items":[],"padding":"`+strings.Repeat("x", MaxManifestSize)+`"}`)
	if _, err := ValidateWorkspace(workspace); !errors.Is(err, ErrDataTooLarge) {
		t.Fatalf("error = %v, want %v", err, ErrDataTooLarge)
	}
}

func TestValidateWorkspaceRejectsTotalQuota(t *testing.T) {
	workspace := dataWorkspace(t)
	items := make([]string, 0, 9)
	for index := 0; index < 9; index++ {
		name := "files/f" + string(rune('a'+index)) + ".bin"
		path := filepath.Join(workspace, filepath.FromSlash(name))
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(MaxFileSize); err != nil {
			file.Close()
			t.Fatal(err)
		}
		file.Close()
		items = append(items, `{"key":"f`+string(rune('a'+index))+`","path":"`+name+`"}`)
	}
	writeDataManifest(t, workspace, `{"v":"weave.extension-data/v1","items":[`+strings.Join(items, ",")+`]}`)
	if _, err := ValidateWorkspace(workspace); !errors.Is(err, ErrDataQuota) {
		t.Fatalf("error = %v, want %v", err, ErrDataQuota)
	}
}

func tooManyItemsManifest() string {
	items := make([]string, MaxItems+1)
	for index := range items {
		items[index] = fmt.Sprintf(`{"key":"a%d","path":"files/a.txt"}`, index)
	}
	return `{"v":"weave.extension-data/v1","items":[` + strings.Join(items, ",") + `]}`
}

func TestDataManifestJSONShape(t *testing.T) {
	manifest := DataManifest{
		V: ExtensionDataVersion,
		Items: []DataItem{{
			Key:      "a",
			Path:     "files/a",
			Metadata: json.RawMessage(`{"x":1}`),
		}},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"weave.extension-data/v1"`) {
		t.Fatalf("unexpected JSON: %s", data)
	}
}
