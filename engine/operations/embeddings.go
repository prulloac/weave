package operations

import (
	"context"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/extensions"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func (e *Executor) newEmbedder(request contractv1.OperationRequest) (sqlitestore.Embedder, error) {
	if request.Embedder == "" {
		return nil, nil
	}
	extension, err := extensions.Resolve(request.Path, request.Embedder)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(request.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = extensions.DefaultTimeout
	}
	runner := extensions.Runner{Timeout: timeout}
	return func(ctx context.Context, text, mediaType string) (sqlitestore.Embedding, error) {
		reply, err := runner.Embed(ctx, extension, extensions.EmbeddingRequest{
			Text:      text,
			MediaType: mediaType,
		})
		if err != nil {
			return sqlitestore.Embedding{}, err
		}
		return sqlitestore.Embedding{
			Model:  reply.Model,
			Dim:    reply.Dim,
			Vector: reply.Vector,
		}, nil
	}, nil
}
