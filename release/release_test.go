package release_test

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prulloac/weave/release"
	"gopkg.in/yaml.v3"
)

func TestNormalizeVersion(t *testing.T) {
	for input, want := range map[string]string{
		"v1.2.3":       "1.2.3",
		"v1.2.3-rc.1":  "1.2.3-rc.1",
		"1.2.3":        "1.2.3",
		"v0.1.0+build": "0.1.0+build",
	} {
		got, err := release.NormalizeVersion(input)
		if err != nil || got != want {
			t.Fatalf("NormalizeVersion(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"", "v", "release", "v1", "v1.2", "v1.2.x", "1.2", "v1.2.3.4"} {
		if _, err := release.NormalizeVersion(input); err == nil {
			t.Fatalf("NormalizeVersion(%q) must fail", input)
		}
	}
}

func TestArtifactName(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
	}{
		{"linux", "amd64", "weave_1.2.3_linux_amd64.tar.gz"},
		{"darwin", "arm64", "weave_1.2.3_darwin_arm64.tar.gz"},
		{"windows", "amd64", "weave_1.2.3_windows_amd64.zip"},
	}
	for _, testCase := range cases {
		if got := release.ArtifactName("1.2.3", testCase.goos, testCase.goarch); got != testCase.want {
			t.Fatalf("ArtifactName(%s/%s) = %q, want %q", testCase.goos, testCase.goarch, got, testCase.want)
		}
	}
}

func packageFixture(t *testing.T) (root, out string) {
	t.Helper()
	root = t.TempDir()
	out = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Weave\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "LICENSE"), []byte("MIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, out
}

func fakeBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "weave")
	if err := os.WriteFile(path, []byte("fake-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPackageTarGz(t *testing.T) {
	root, out := packageFixture(t)
	binary := fakeBinary(t)

	path, err := release.Package(release.PackageOptions{
		Version: "1.2.3",
		GOOS:    "linux",
		GOARCH:  "amd64",
		Binary:  binary,
		Root:    root,
		Out:     out,
	})
	if err != nil {
		t.Fatalf("package: %v", err)
	}
	if filepath.Base(path) != "weave_1.2.3_linux_amd64.tar.gz" {
		t.Fatalf("unexpected archive: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(gzipReader)
	contents := map[string]struct {
		mode int64
		body string
	}{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		contents[header.Name] = struct {
			mode int64
			body string
		}{header.Mode, string(body)}
	}
	if contents["weave"].body != "fake-binary" || contents["weave"].mode&0o111 == 0 {
		t.Fatalf("unexpected weave entry: %+v", contents["weave"])
	}
	if contents["README.md"].body != "# Weave\n" || contents["LICENSE"].body != "MIT\n" {
		t.Fatalf("missing docs in archive: %+v", contents)
	}
}

func TestPackageZip(t *testing.T) {
	root, out := packageFixture(t)
	binary := fakeBinary(t)
	path, err := release.Package(release.PackageOptions{
		Version: "1.2.3",
		GOOS:    "windows",
		GOARCH:  "arm64",
		Binary:  binary,
		Root:    root,
		Out:     out,
	})
	if err != nil {
		t.Fatalf("package: %v", err)
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	names := make(map[string]bool)
	for _, file := range reader.File {
		names[file.Name] = true
		if file.Name == "weave.exe" {
			opened, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(opened)
			opened.Close()
			if string(body) != "fake-binary" {
				t.Fatalf("unexpected binary body: %q", body)
			}
		}
	}
	for _, name := range []string{"weave.exe", "README.md", "LICENSE"} {
		if !names[name] {
			t.Fatalf("missing %s in zip: %v", name, names)
		}
	}
}

func TestWriteChecksums(t *testing.T) {
	out := t.TempDir()
	files := map[string]string{
		"zeta.tar.gz":  "zeta",
		"alpha.tar.gz": "alpha",
		"beta.zip":     "beta",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(out, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path, err := release.WriteChecksums(out, "1.2.3")
	if err != nil {
		t.Fatalf("checksums: %v", err)
	}
	if filepath.Base(path) != "weave_1.2.3_checksums.txt" {
		t.Fatalf("unexpected checksum file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 checksum lines, got %d", len(lines))
	}
	names := []string{"alpha.tar.gz", "beta.zip", "zeta.tar.gz"}
	for index, line := range lines {
		parts := strings.Fields(line)
		if len(parts) != 2 || parts[1] != names[index] {
			t.Fatalf("line %d is not sorted: %q", index, line)
		}
		sum := sha256.Sum256([]byte(files[parts[1]]))
		if parts[0] != hex.EncodeToString(sum[:]) {
			t.Fatalf("checksum mismatch for %s", parts[1])
		}
	}
}

func TestBuildInfo(t *testing.T) {
	info := release.BuildInfo("v1.2.3", "abc123", "go1.27.3")
	for _, want := range []string{"version=v1.2.3", "commit=abc123", "go=go1.27.3", "built_at="} {
		if !strings.Contains(info, want) {
			t.Fatalf("build info missing %q:\n%s", want, info)
		}
	}
}

type workflow struct {
	On struct {
		Push struct {
			Tags []string `yaml:"tags"`
		} `yaml:"push"`
		Dispatch map[string]any `yaml:"workflow_dispatch"`
	} `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]struct {
		Needs    any    `yaml:"needs"`
		If       string `yaml:"if"`
		Strategy struct {
			Matrix struct {
				Include []map[string]string `yaml:"include"`
			} `yaml:"matrix"`
		} `yaml:"strategy"`
		Steps []map[string]any `yaml:"steps"`
	} `yaml:"jobs"`
}

func loadWorkflow(t *testing.T) (workflow, string) {
	t.Helper()
	path := filepath.Join("..", ".github", "workflows", "release.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	var parsed workflow
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	return parsed, string(data)
}

func TestWorkflowTriggersAndJobs(t *testing.T) {
	parsed, raw := loadWorkflow(t)
	foundTag := false
	for _, tag := range parsed.On.Push.Tags {
		if tag == "v*" {
			foundTag = true
		}
	}
	if !foundTag {
		t.Fatalf("workflow must trigger on v* tags: %s", raw)
	}
	if parsed.On.Dispatch == nil {
		t.Fatal("workflow must support workflow_dispatch")
	}

	for _, job := range []string{"verify", "build", "release"} {
		if _, ok := parsed.Jobs[job]; !ok {
			t.Fatalf("missing job %q", job)
		}
	}
	if needs(parsed.Jobs["build"].Needs) != "verify" {
		t.Fatalf("build must need verify, got %v", parsed.Jobs["build"].Needs)
	}
	if !needsContains(parsed.Jobs["release"].Needs, "build") {
		t.Fatalf("release must need build, got %v", parsed.Jobs["release"].Needs)
	}
	if !strings.Contains(parsed.Jobs["release"].If, "refs/tags/v") {
		t.Fatalf("release must be tag-gated, got %q", parsed.Jobs["release"].If)
	}
	if !strings.Contains(raw, "./scripts/verify.sh") {
		t.Fatal("workflow must run the verification script")
	}
}

func TestWorkflowMatrix(t *testing.T) {
	parsed, _ := loadWorkflow(t)
	include := parsed.Jobs["build"].Strategy.Matrix.Include
	seen := make(map[string]bool)
	for _, entry := range include {
		seen[entry["os"]+"/"+entry["arch"]] = true
	}
	for _, target := range []string{
		"linux/amd64", "linux/arm64",
		"darwin/amd64", "darwin/arm64",
		"windows/amd64", "windows/arm64",
	} {
		if !seen[target] {
			t.Fatalf("matrix missing %s: %+v", target, include)
		}
	}
}

func TestWorkflowBuildAndReleaseFlags(t *testing.T) {
	_, raw := loadWorkflow(t)
	for _, want := range []string{
		"CGO_ENABLED=0",
		"-trimpath",
		"-X main.version",
		"release-package",
		"contents: write",
		"draft",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("workflow missing %q", want)
		}
	}
	if !strings.Contains(raw, "prerelease") {
		t.Fatal("workflow must mark prerelease tags")
	}
}

func needs(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		if len(typed) == 1 {
			return typed[0].(string)
		}
	}
	return ""
}

func needsContains(value any, target string) bool {
	switch typed := value.(type) {
	case string:
		return typed == target
	case []any:
		for _, entry := range typed {
			if entry == target {
				return true
			}
		}
	}
	return false
}
