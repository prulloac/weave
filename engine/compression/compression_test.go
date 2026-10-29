package compression_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/prulloac/weave/engine/compression"
)

func TestRoundTrip(t *testing.T) {
	bodies := map[string][]byte{
		"repetitive": []byte(strings.Repeat("compressible phrase. ", 200)),
		"mixed":      []byte("# Title\n\nSome prose with Unicode: héllo 世界.\n"),
	}
	for _, codec := range []compression.Codec{compression.CodecZstd, compression.CodecLZ4} {
		for name, body := range bodies {
			encoded, stored, err := compression.Encode(codec, body, false)
			if err != nil {
				t.Fatalf("%s/%s encode: %v", codec, name, err)
			}
			decoded, err := compression.Decode(stored, encoded)
			if err != nil {
				t.Fatalf("%s/%s decode: %v", codec, name, err)
			}
			if string(decoded) != string(body) {
				t.Fatalf("%s/%s round trip mismatch", codec, name)
			}
		}
	}
}

func TestDeterminism(t *testing.T) {
	body := []byte(strings.Repeat("deterministic body. ", 300))
	for _, codec := range []compression.Codec{compression.CodecZstd, compression.CodecLZ4} {
		first, _, err := compression.Encode(codec, body, false)
		if err != nil {
			t.Fatal(err)
		}
		second, _, err := compression.Encode(codec, body, false)
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Fatalf("%s encoding is not deterministic", codec)
		}
	}
}

func TestThreshold(t *testing.T) {
	short := []byte(strings.Repeat("s", compression.Threshold-1))
	if encoded, codec, err := compression.Encode(compression.CodecZstd, short, false); err != nil {
		t.Fatal(err)
	} else if codec != compression.CodecNone || string(encoded) != string(short) {
		t.Fatalf("short body codec = %q", codec)
	}

	boundary := []byte(strings.Repeat("b", compression.Threshold))
	if _, codec, err := compression.Encode(compression.CodecZstd, boundary, false); err != nil {
		t.Fatal(err)
	} else if codec != compression.CodecZstd {
		t.Fatalf("boundary codec = %q", codec)
	}
}

func TestEncryptedBodiesStayVerbatim(t *testing.T) {
	body := []byte(strings.Repeat("ciphertext ", 300))
	encoded, codec, err := compression.Encode(compression.CodecZstd, body, true)
	if err != nil {
		t.Fatal(err)
	}
	if codec != compression.CodecNone || string(encoded) != string(body) {
		t.Fatalf("encrypted body codec = %q", codec)
	}
}

func TestDecodeFailures(t *testing.T) {
	if _, err := compression.Decode(compression.Codec("bogus"), []byte("x")); !errors.Is(err, compression.ErrDecode) {
		t.Fatalf("unknown codec: %v", err)
	}
	if _, err := compression.Decode(compression.CodecZstd, []byte{0x00}); !errors.Is(err, compression.ErrDecode) {
		t.Fatalf("corrupt zstd: %v", err)
	}
	if _, err := compression.Decode(compression.CodecLZ4, []byte{0x00}); !errors.Is(err, compression.ErrDecode) {
		t.Fatalf("corrupt lz4: %v", err)
	}
}
