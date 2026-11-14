package operations

import (
	"fmt"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/graph"
)

var allRelations = map[contractv1.RelationKind]bool{
	contractv1.RelationLink:       true,
	contractv1.RelationRelated:    true,
	contractv1.RelationImports:    true,
	contractv1.RelationDeclares:   true,
	contractv1.RelationReferences: true,
	contractv1.RelationCalls:      true,
	contractv1.RelationInherits:   true,
	contractv1.RelationImplements: true,
	contractv1.RelationContains:   true,
	contractv1.RelationDependsOn:  true,
	contractv1.RelationParentOf:   true,
}

func traversalOptions(relations []contractv1.RelationKind) (graph.TraversalOptions, error) {
	for _, relation := range relations {
		if !allRelations[relation] {
			return graph.TraversalOptions{}, fmt.Errorf("unknown relation kind %q", relation)
		}
	}
	return graph.TraversalOptions{Relations: relations}, nil
}
