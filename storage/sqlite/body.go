package sqlite

import (
	"context"
	"errors"
	"strings"

	"github.com/prulloac/weave/engine/codecs"
	"github.com/prulloac/weave/engine/compression"
)

func encodeStoredBody(ctx context.Context, root, codec string, data []byte) ([]byte, string, error) {
	if codec == "" {
		codec = string(compression.CodecZstd)
	}
	switch codec {
	case string(compression.CodecNone):
		return data, codec, nil
	case string(compression.CodecZstd):
		encoded, stored, err := compression.EncodeBlob(compression.CodecZstd, data)
		return encoded, string(stored), err
	default:
		encoded, err := codecs.New(root, codecs.DefaultTimeout).Compress(ctx, codec, data)
		return encoded, codec, err
	}
}

func decodeStoredBody(ctx context.Context, root, codec string, data []byte) ([]byte, error) {
	codec = strings.TrimSpace(codec)
	if codec == "" {
		codec = string(compression.CodecNone)
	}
	switch codec {
	case string(compression.CodecNone):
		return data, nil
	case string(compression.CodecZstd):
		return compression.Decode(compression.CodecZstd, data)
	default:
		decoded, err := codecs.New(root, codecs.DefaultTimeout).Decompress(ctx, codec, data)
		var codecErr *codecs.Error
		if errors.As(err, &codecErr) && codecErr.Code == "CODEC_NOT_FOUND" {
			return nil, &codecs.Error{Code: "CODEC_UNAVAILABLE", Message: "stored codec unavailable: " + codec}
		}
		return decoded, err
	}
}
