package main

import (
	"context"
	"fmt"
	"io"

	"github.com/prulloac/weave/engine/extensions"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func runEmbeddings(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return usageError(stdout, stderr, false, "usage: weave embeddings list|rebuild|drop <path> [flags]")
	}
	action := args[0]
	flags, err := parseFlags(args[1:])
	if err != nil {
		return usageError(stdout, stderr, false, err.Error())
	}
	if len(flags.positionals) != 1 {
		return usageError(stdout, stderr, flags.json, "embeddings "+action+" requires one bundle path")
	}
	root := flags.positionals[0]
	cfg, _, err := loadAppConfig(flags.configPath, flags.noConfig, flags.strictConfig)
	if err != nil {
		return usageError(stdout, stderr, flags.json, err.Error())
	}
	dbPath := flags.db
	if dbPath == "" {
		dbPath = cfg.DB
	}
	dbPath, err = sqlitestore.ExpandPath(dbPath)
	if err != nil {
		return usageError(stdout, stderr, flags.json, err.Error())
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "weave: %v\n", err)
		return 1
	}
	defer store.Close()

	switch action {
	case "list":
		spaces, err := store.EmbeddingSpaces(context.Background(), root)
		if err != nil {
			fmt.Fprintf(stderr, "weave: %v\n", err)
			return 1
		}
		if flags.json {
			result := make([]map[string]any, 0, len(spaces))
			for _, space := range spaces {
				result = append(result, map[string]any{
					"model": space.Model, "dim": space.Dim, "metric": space.Metric,
					"element_type": space.ElementType, "table": space.Table,
					"active": space.Active, "rows": space.Rows,
				})
			}
			if err := writeJSON(stdout, successEnvelope(map[string]any{"spaces": result})); err != nil {
				fmt.Fprintf(stderr, "weave: %v\n", err)
				return 1
			}
			return 0
		}
		if len(spaces) == 0 {
			fmt.Fprintln(stdout, "no embedding spaces")
			return 0
		}
		for _, space := range spaces {
			fmt.Fprintf(stdout, "%s dim=%d metric=%s rows=%d active=%t table=%s\n",
				space.Model, space.Dim, space.Metric, space.Rows, space.Active, space.Table)
		}
		return 0
	case "rebuild", "drop":
		if flags.model == "" {
			return usageError(stdout, stderr, flags.json, "embeddings "+action+" requires --model")
		}
		dim := flags.dim
		reembed := action == "rebuild" && flags.withEmbedder != ""
		if dim == 0 && !reembed {
			dim, err = store.EmbeddingDim(context.Background(), root, flags.model)
			if err != nil {
				fmt.Fprintf(stderr, "weave: %v\n", err)
				return 1
			}
		}
		documents := 0
		if reembed {
			extension, err := extensions.Resolve(root, flags.withEmbedder)
			if err != nil {
				fmt.Fprintf(stderr, "weave: %v\n", err)
				return 1
			}
			timeout := flags.timeout
			if timeout <= 0 {
				timeout = extensions.DefaultTimeout
			}
			runner := extensions.Runner{Timeout: timeout}
			embedder := func(ctx context.Context, text, mediaType string) (sqlitestore.Embedding, error) {
				reply, err := runner.Embed(ctx, extension, extensions.EmbeddingRequest{
					Text: text, MediaType: mediaType, Model: flags.model,
				})
				if err != nil {
					return sqlitestore.Embedding{}, err
				}
				return sqlitestore.Embedding{Model: reply.Model, Dim: reply.Dim, Vector: reply.Vector}, nil
			}
			documents, err = store.ReembedBundle(context.Background(), root, flags.model, embedder)
			if err == nil && dim == 0 {
				dim, _ = store.EmbeddingDim(context.Background(), root, flags.model)
			}
		} else if action == "rebuild" {
			err = store.RebuildEmbeddingSpace(context.Background(), root, flags.model, dim)
		} else {
			err = store.DropEmbeddingSpace(context.Background(), root, flags.model, dim, flags.keepSource)
		}
		if err != nil {
			fmt.Fprintf(stderr, "weave: %v\n", err)
			return 1
		}
		if flags.json {
			if err := writeJSON(stdout, successEnvelope(map[string]any{
				"action": action, "model": flags.model, "dim": dim, "documents": documents,
			})); err != nil {
				fmt.Fprintf(stderr, "weave: %v\n", err)
				return 1
			}
			return 0
		}
		fmt.Fprintf(stdout, "%s: %s dim=%d\n", action, flags.model, dim)
		return 0
	default:
		return usageError(stdout, stderr, flags.json, "unknown embeddings action: "+action)
	}
}
