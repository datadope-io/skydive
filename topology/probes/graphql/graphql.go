// Package graphql provides a GraphQL API to create and modify nodes and edges with a
// custom data schema.
//
// Implemented modules: event and net.
//
// net module is for adding network devices.
// The graph model used is:
//
//                                       ┌────────────────┐
//                                       │logicalInterface│
//                                       │ Type:interface │
//                                       │ SubType:logical│
//                                       └───────┬────────┘
//                                               │
//                                             (use)
//                                               │
//                                       ┌───────▼────────────┐
//                                       │aggregationInterface│
//                     ┌──(ownership)───►│ Type:interface     ├─────┐
//                     │                 │ SubType:aggregation│     │
//                     │                 └────────────────────┘     │
//                     │                                            │
//                     │                                            │
//                     │                                            │
//                     │                                       (aggregate)
//                     │                                            │
//                     │                                            │
//                     │                                            │
//          ┌──────────┴──┐                                    ┌────▼──────────────┐
//          │networkDevice│                                    │ physicalInterface │
//          │ Type:device ├───────────(ownership)─────────────►│  Type:interface   │
//          └─────────────┘                                    │  SubType:physical │
//                                                             └────────▲──────────┘
//                                                                      │
//                                                                    (use)
//                                                                      │
//                                                              ┌───────┴────────┐
//                                                              │logicalInterface│
//                                                              │ Type:interface │
//                                                              │ SubType:logical│
//                                                              └────────────────┘
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
	// GCdone channel to stop event garbageCollector ticker
	EventGCdone chan bool
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
	logging.GetLogger().Infof("GraphQL listening on http://%v", listenEndpoint)

	p.StartEvents()

	return nil
}

// Stop the probe
func (p *Probe) Stop() {
	p.StopEvents()
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
	graph.NodeMetadataDecoders[MetaKeyEvents] = EventsMetadataDecoder
	graph.NodeMetadataDecoders[MetadataKeyNetworkInfo] = NetworkInfoMetadataDecoder
}
