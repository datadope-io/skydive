// Package graphql provides an GraphQL API to handle different elements.
//
// This API is implemented with GraphQL to be able to define clearly the format that
// has to be used.
//
package graphql

import (
	"net/http"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"

	"github.com/skydive-project/skydive/topology/probes/graphql/generated"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
)

// Probe describes this probe
type Probe struct {
	graph *graph.Graph
}

// Start initilizates the graphql probe, starting a web server serving a GraphQL API
func (p *Probe) Start() error {
	listenEndpoint := config.GetString("analyzer.topology.graphql.listen")

	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{
		Resolvers: &Resolver{
			Graph: p.graph,
		},
	}))

	mux := http.NewServeMux()

	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))
	mux.Handle("/query", srv)

	go http.ListenAndServe(listenEndpoint, mux)

	logging.GetLogger().Infof("Listening for network elements on %v", listenEndpoint)

	return nil
}

// Stop the probe
func (p *Probe) Stop() {
}

// NewProbe initialize the probe with the parameters from the config file
func NewProbe(g *graph.Graph) (*Probe, error) {
	probe := &Probe{
		graph: g,
	}

	return probe, nil
}

// Register called at initialization to register metadata decoders
func Register() {
	graph.NodeMetadataDecoders[MetaKeyEvents] = MetadataDecoder
}
