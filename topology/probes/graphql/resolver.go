package graphql

import (
	"github.com/skydive-project/skydive/graffiti/graph"
)

// Resolver graphql object defined by the graphql probe to create the GraphQL API
type Resolver struct {
	Graph *graph.Graph
}
