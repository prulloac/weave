package operations

import (
	"fmt"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
)

func traversalOptions(relations []contractv1.RelationKind) (graph.TraversalOptions, error) {
	for _, relation := range relations {
		if !contractv1.ValidRelationKind(relation) {
			return graph.TraversalOptions{}, fmt.Errorf("unknown relation kind %q", relation)
		}
	}
	return graph.TraversalOptions{Relations: relations}, nil
}
