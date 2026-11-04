package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

const upperCodecSource = `package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	name := strings.TrimPrefix(filepath.Base(os.Args[0]), "weave-codec-")
	if len(os.Args) > 1 && os.Args[1] == "--describe" {
		json.NewEncoder(os.Stdout).Encode(map[string]any{
			"v": "weave.extension/v1", "name": name, "kind": "codec",
			"version": "1.0.0", "directions": []string{"compress", "decompress"},
		})
		return
	}
	data, _ := io.ReadAll(os.Stdin)
	if os.Args[1] == "compress" {
		os.Stdout.Write(append([]byte{'X'}, data...))
	} else if len(data) > 0 {
		os.Stdout.Write(data[1:])
	}
}
`

func buildUpperCodec(t *testing.T, root string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(source, []byte(upperCodecSource), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".weave", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "-o", filepath.Join(dir, "weave-codec-upper"), source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build codec: %v\n%s", err, output)
	}
}

func TestCodecsListBuiltins(t *testing.T) {
	root := t.TempDir()
	buildUpperCodec(t, root)
	code, stdout, stderr := Invoke(t, "codecs", "list", root, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("codecs list exit=%d stderr=%q", code, stderr)
	}
	var list contractv1.CodecList
	if err := json.Unmarshal(decodeEnvelope(t, stdout).Result, &list); err != nil {
		t.Fatal(err)
	}
	found := map[string]contractv1.CodecInfo{}
	for _, info := range list.Codecs {
		found[info.Name] = info
	}
	if !found["none"].CanDecompress || found["none"].CanCompress {
		t.Fatalf("none = %+v", found["none"])
	}
	if !found["zstd"].CanCompress || !found["zstd"].CanDecompress {
		t.Fatalf("zstd = %+v", found["zstd"])
	}
	if !found["upper"].CanCompress || found["upper"].Source != "bundle" {
		t.Fatalf("upper = %+v", found["upper"])
	}
}

func TestCustomCodecRoundTrip(t *testing.T) {
	root := blobBundle(t)
	buildUpperCodec(t, root)
	db := filepath.Join(t.TempDir(), "index.db")
	blobIngest(t, root, db, "--assets", "--asset-types", "image/png", "--codec", "upper")

	var codec string
	raw := compressionRaw(t, db)
	if err := raw.QueryRow("SELECT body_codec FROM documents WHERE id = 'photo.png'").Scan(&codec); err != nil {
		t.Fatal(err)
	}
	if codec != "upper" {
		t.Fatalf("stored codec = %q", codec)
	}

	out := filepath.Join(t.TempDir(), "photo.png")
	code, _, stderr := Invoke(t, "show", root, "photo.png", "--db", db, "--out", out, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("show exit=%d stderr=%q", code, stderr)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if !bytes.Equal(got, want) {
		t.Fatal("custom codec did not round-trip")
	}
}

func TestMissingCodecFailsBeforeWrite(t *testing.T) {
	root := blobBundle(t)
	db := filepath.Join(t.TempDir(), "index.db")
	code, stdout, _ := Invoke(t, "ingest", root, "--db", db,
		"--assets", "--asset-types", "image/png", "--codec", "lz4", "--json")
	if code != 2 {
		t.Fatalf("exit=%d, want 2", code)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil || envelope.Error.Code != "CODEC_NOT_FOUND" {
		t.Fatalf("unexpected error: %+v", envelope.Error)
	}
}
