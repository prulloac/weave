package codecs_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/codecs"
	"github.com/prulloac/weave/engine/extensions"
)

const checkHelperSource = `package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	name := strings.TrimPrefix(filepath.Base(os.Args[0]), "weave-codec-")
	if len(os.Args) > 1 && os.Args[1] == "--describe" {
		if os.Getenv("CODEC_CHECK_BAD_MANIFEST") != "" {
			os.Stdout.WriteString("not json")
			return
		}
		directions := []string{"compress", "decompress"}
		if os.Getenv("CODEC_CHECK_BAD_DIRECTION") != "" {
			directions = append(directions, "rewrite")
		}
		json.NewEncoder(os.Stdout).Encode(map[string]any{
			"v": "weave.extension/v1", "name": name, "kind": "codec",
			"version": "1.0.0", "directions": directions,
		})
		return
	}
	if os.Getenv("CODEC_CHECK_SLEEP") != "" {
		time.Sleep(250 * time.Millisecond)
	}
	if os.Getenv("CODEC_CHECK_FAIL") != "" {
		os.Exit(3)
	}
	data, _ := io.ReadAll(os.Stdin)
	switch os.Args[1] {
	case "compress":
		os.Stdout.WriteString(strings.ToUpper(string(data)))
	case "decompress":
		if os.Getenv("CODEC_CHECK_LOSSY") != "" {
			return
		}
		os.Stdout.WriteString(strings.ToLower(string(data)))
	}
}
`

func buildCheckCodec(t *testing.T, dir, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("codec helper requires a Go toolchain on POSIX")
	}
	source := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(source, []byte(checkHelperSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "weave-codec-"+name)
	command := exec.Command("go", "build", "-o", path, source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build codec: %v\n%s", err, output)
	}
	return path
}

func codecCheck(t *testing.T, report contractv1.ExtensionCheckReport, name string) contractv1.ExtensionCheckResult {
	t.Helper()
	for _, check := range report.Checks {
		if check.Check == name {
			return check
		}
	}
	t.Fatalf("check %q missing from %+v", name, report.Checks)
	return contractv1.ExtensionCheckResult{}
}

func TestCheckPassesCodec(t *testing.T) {
	root := t.TempDir()
	buildCheckCodec(t, filepath.Join(root, ".weave", "bin"), "upper")
	registry := codecs.New(root, time.Second)

	report, err := registry.Check(context.Background(), "upper")
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Extension != "codec-upper" {
		t.Fatalf("unexpected report: %+v", report)
	}
	for _, name := range []string{"manifest", "compress", "decompress", "determinism"} {
		if got := codecCheck(t, report, name); got.Status != contractv1.ExtensionCheckPass {
			t.Fatalf("%s = %+v", name, got)
		}
	}
}

func TestCheckFailsBadCodecManifest(t *testing.T) {
	root := t.TempDir()
	buildCheckCodec(t, filepath.Join(root, ".weave", "bin"), "upper")
	t.Setenv("CODEC_CHECK_BAD_MANIFEST", "1")
	registry := codecs.New(root, time.Second)

	report, err := registry.Check(context.Background(), "upper")
	if err != nil {
		t.Fatal(err)
	}
	check := codecCheck(t, report, "manifest")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrCodecProtocol {
		t.Fatalf("unexpected manifest check: %+v", check)
	}
}

func TestCheckFailsBadDirection(t *testing.T) {
	root := t.TempDir()
	buildCheckCodec(t, filepath.Join(root, ".weave", "bin"), "upper")
	t.Setenv("CODEC_CHECK_BAD_DIRECTION", "1")
	registry := codecs.New(root, time.Second)

	report, err := registry.Check(context.Background(), "upper")
	if err != nil {
		t.Fatal(err)
	}
	check := codecCheck(t, report, "manifest")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrCodecProtocol {
		t.Fatalf("unexpected manifest check: %+v", check)
	}
}

func TestCheckFailsCodecRoundTrip(t *testing.T) {
	root := t.TempDir()
	buildCheckCodec(t, filepath.Join(root, ".weave", "bin"), "upper")
	t.Setenv("CODEC_CHECK_LOSSY", "1")
	registry := codecs.New(root, time.Second)

	report, err := registry.Check(context.Background(), "upper")
	if err != nil {
		t.Fatal(err)
	}
	check := codecCheck(t, report, "decompress")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrCodecProtocol {
		t.Fatalf("unexpected decompress check: %+v", check)
	}
}

func TestCheckFailsCodecExit(t *testing.T) {
	root := t.TempDir()
	buildCheckCodec(t, filepath.Join(root, ".weave", "bin"), "upper")
	t.Setenv("CODEC_CHECK_FAIL", "1")
	registry := codecs.New(root, time.Second)

	report, err := registry.Check(context.Background(), "upper")
	if err != nil {
		t.Fatal(err)
	}
	check := codecCheck(t, report, "compress")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrCodecFailed {
		t.Fatalf("unexpected compress check: %+v", check)
	}
}

func TestCheckFailsCodecTimeout(t *testing.T) {
	root := t.TempDir()
	buildCheckCodec(t, filepath.Join(root, ".weave", "bin"), "upper")
	t.Setenv("CODEC_CHECK_SLEEP", "1")
	registry := codecs.New(root, 50*time.Millisecond)

	report, err := registry.Check(context.Background(), "upper")
	if err != nil {
		t.Fatal(err)
	}
	check := codecCheck(t, report, "compress")
	if check.Status != contractv1.ExtensionCheckFail || check.Code != contractv1.ErrCodecTimeout {
		t.Fatalf("unexpected compress check: %+v", check)
	}
}

func TestCheckUnknownCodec(t *testing.T) {
	registry := codecs.New(t.TempDir(), time.Second)
	_, err := registry.Check(context.Background(), "missing")
	if !errors.Is(err, extensions.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
