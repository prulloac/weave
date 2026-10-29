// Package compression encodes and decodes stored document bodies.
package compression

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

// Codec identifies one stored body encoding.
type Codec string

const (
	// CodecNone stores the body verbatim.
	CodecNone Codec = "none"
	// CodecZstd is the default codec.
	CodecZstd Codec = "zstd"
	// CodecLZ4 is the fast opt-in codec.
	CodecLZ4 Codec = "lz4"
)

// Threshold is the minimum body size that may be compressed.
const Threshold = 1024

// ErrDecode reports a corrupt, truncated or unknown body codec.
var ErrDecode = errors.New("body decode failed")

// Valid reports whether codec is supported for new writes.
func Valid(codec string) bool {
	switch Codec(codec) {
	case CodecNone, CodecZstd, CodecLZ4:
		return true
	default:
		return false
	}
}

// Encode compresses body with codec unless it is encrypted, empty or shorter
// than Threshold. Short and encrypted bodies report CodecNone.
func Encode(codec Codec, body []byte, encrypted bool) ([]byte, Codec, error) {
	if encrypted || len(body) == 0 || len(body) < Threshold || codec == CodecNone {
		return body, CodecNone, nil
	}
	switch codec {
	case CodecZstd:
		encoded, err := encodeZstd(body)
		if err != nil {
			return nil, "", err
		}
		return encoded, CodecZstd, nil
	case CodecLZ4:
		encoded, err := encodeLZ4(body)
		if err != nil {
			return nil, "", err
		}
		return encoded, CodecLZ4, nil
	default:
		return nil, "", fmt.Errorf("unsupported codec %q", codec)
	}
}

// Decode restores a stored body. Unknown codecs and corrupt blobs return
// ErrDecode without partial output.
func Decode(codec Codec, blob []byte) ([]byte, error) {
	switch codec {
	case CodecNone:
		return blob, nil
	case CodecZstd:
		decoder, err := zstd.NewReader(bytes.NewReader(blob))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecode, err)
		}
		defer decoder.Close()
		decoded, err := io.ReadAll(decoder)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecode, err)
		}
		return decoded, nil
	case CodecLZ4:
		decoded, err := io.ReadAll(lz4.NewReader(bytes.NewReader(blob)))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecode, err)
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("%w: unknown codec %q", ErrDecode, codec)
	}
}

func encodeZstd(body []byte) ([]byte, error) {
	var buffer bytes.Buffer
	encoder, err := zstd.NewWriter(
		&buffer,
		zstd.WithEncoderLevel(zstd.SpeedDefault),
		zstd.WithEncoderConcurrency(1),
	)
	if err != nil {
		return nil, err
	}
	if _, err := encoder.Write(body); err != nil {
		encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func encodeLZ4(body []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := lz4.NewWriter(&buffer)
	if err := writer.Apply(
		lz4.BlockSizeOption(lz4.Block64Kb),
		lz4.ChecksumOption(false),
	); err != nil {
		return nil, err
	}
	if _, err := writer.Write(body); err != nil {
		writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
