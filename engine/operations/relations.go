package operations

import (
	"context"
	"fmt"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

func (e *Executor) traversalOptions(ctx context.Context, request contractv1.OperationRequest) (graph.TraversalOptions, error) {
	if len(request.Relations) > 0 {
		for _, relation := range request.Relations {
			if !contractv1.ValidRelationKind(relation) {
				return graph.TraversalOptions{}, fmt.Errorf("unknown relation kind %q", relation)
			}
		}
		return graph.TraversalOptions{Relations: request.Relations}, nil
	}
	relations := append([]contractv1.RelationKind(nil), graph.DefaultRelations...)
	root := request.Path
	if request.Source != nil && request.Source.SnapshotPath != "" {
		root = request.Source.SnapshotPath
	}
	if request.DB != "" {
		if dbPath, err := resolveDBPath(request.DB); err == nil {
			if store, err := sqlitestore.OpenExisting(dbPath); err == nil {
				if defaults, err := store.ListRelationDefaults(ctx, root); err == nil {
					for _, kind := range defaults {
						relations = append(relations, contractv1.RelationKind(kind))
					}
				}
				store.Close()
			}
		}
	}
	seen := make(map[contractv1.RelationKind]bool, len(relations))
	result := make([]contractv1.RelationKind, 0, len(relations))
	for _, relation := range relations {
		if relation == "" || seen[relation] {
			continue
		}
		seen[relation] = true
		result = append(result, relation)
	}
	return graph.TraversalOptions{Relations: result}, nil
}
