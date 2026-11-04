package codecs_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/prulloac/weave/engine/codecs"
)

const helperSource = `package main

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
		if os.Getenv("CODEC_FAKE_BAD_MANIFEST") != "" {
			os.Stdout.WriteString("not json")
			return
		}
		json.NewEncoder(os.Stdout).Encode(map[string]any{
			"v": "weave.extension/v1", "name": name, "kind": "codec",
			"version": "1.0.0", "directions": []string{"compress", "decompress"}, "frame": true,
		})
		return
	}
	if os.Getenv("CODEC_FAKE_SLEEP") != "" {
		time.Sleep(250 * time.Millisecond)
	}
	data, _ := io.ReadAll(os.Stdin)
	switch os.Args[1] {
	case "compress":
		os.Stdout.WriteString(strings.ToUpper(string(data)))
	case "decompress":
		os.Stdout.WriteString(strings.ToLower(string(data)))
	}
}
`

func buildCodec(t *testing.T, dir, name string) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(source, []byte(helperSource), 0o644); err != nil {
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

func TestBuiltinsAndDiscovery(t *testing.T) {
	root := t.TempDir()
	buildCodec(t, filepath.Join(root, ".weave", "bin"), "upper")
	registry := codecs.New(root, time.Second)
	list, err := registry.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]codecs.Info{}
	for _, info := range list {
		names[info.Name] = info
	}
	if !names["none"].CanDecompress || names["none"].CanCompress {
		t.Fatalf("none = %+v", names["none"])
	}
	if !names["zstd"].CanCompress || !names["zstd"].CanDecompress {
		t.Fatalf("zstd = %+v", names["zstd"])
	}
	if names["upper"].Source != "bundle" || names["upper"].Path == "" {
		t.Fatalf("upper = %+v", names["upper"])
	}
}

func TestRoundTripExtension(t *testing.T) {
	root := t.TempDir()
	buildCodec(t, filepath.Join(root, ".weave", "bin"), "upper")
	registry := codecs.New(root, time.Second)
	encoded, err := registry.Compress(context.Background(), "upper", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "HELLO" {
		t.Fatalf("encoded = %q", encoded)
	}
	decoded, err := registry.Decompress(context.Background(), "upper", encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "hello" {
		t.Fatalf("decoded = %q", decoded)
	}
}

func TestMissingCodec(t *testing.T) {
	registry := codecs.New(t.TempDir(), time.Second)
	_, err := registry.Compress(context.Background(), "missing", []byte("x"))
	var codecErr *codecs.Error
	if !errors.As(err, &codecErr) || codecErr.Code != "CODEC_NOT_FOUND" {
		t.Fatalf("err = %v", err)
	}
}

func TestInvalidManifest(t *testing.T) {
	root := t.TempDir()
	buildCodec(t, filepath.Join(root, ".weave", "bin"), "broken")
	t.Setenv("CODEC_FAKE_BAD_MANIFEST", "1")
	registry := codecs.New(root, time.Second)
	_, err := registry.Compress(context.Background(), "broken", []byte("x"))
	var codecErr *codecs.Error
	if !errors.As(err, &codecErr) || codecErr.Code != "CODEC_PROTOCOL" {
		t.Fatalf("err = %v", err)
	}
}

func TestTimeout(t *testing.T) {
	root := t.TempDir()
	buildCodec(t, filepath.Join(root, ".weave", "bin"), "slow")
	t.Setenv("CODEC_FAKE_SLEEP", "1")
	registry := codecs.New(root, 50*time.Millisecond)
	_, err := registry.Compress(context.Background(), "slow", []byte("x"))
	var codecErr *codecs.Error
	if !errors.As(err, &codecErr) || codecErr.Code != "CODEC_TIMEOUT" {
		t.Fatalf("err = %v", err)
	}
}

func TestBuiltinsCannotBeShadowed(t *testing.T) {
	root := t.TempDir()
	buildCodec(t, filepath.Join(root, ".weave", "bin"), "zstd")
	registry := codecs.New(root, time.Second)
	list, err := registry.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range list {
		if info.Name == "zstd" && info.Source != "builtin" {
			t.Fatalf("zstd was shadowed: %+v", info)
		}
	}
}
