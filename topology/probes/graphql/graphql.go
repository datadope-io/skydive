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
	"fmt"
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
	graph.DefaultGraphListener
	graph *graph.Graph
	// resolver links the resolver from the probe to be able to call from node/edge events to the event/net
	resolver *Resolver
	// GCdone channel to stop event garbageCollector ticker
	EventGCdone       chan bool
	pseudoWireIndexer *graph.Indexer
	linker            *graph.ResourceLinker
}

// Start initilizates the graphql probe, starting a web server serving a GraphQL API
func (p *Probe) Start() error {
	listenEndpoint := config.GetString("analyzer.topology.graphql.listen")

	// Listen to graph events
	p.graph.AddEventListener(p)

	p.resolver = &Resolver{
		Graph: p.graph,
	}

	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{
		Resolvers: p.resolver,
	}))

	err := p.pseudoWireIndexer.Start()
	if err != nil {
		return fmt.Errorf("starting pseudoWireIndexer: %v", err)
	}

	err = p.linker.Start()
	if err != nil {
		return fmt.Errorf("starting linker: %v", err)
	}

	mux := http.NewServeMux()

	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))
	mux.Handle("/query", srv)

	go http.ListenAndServe(listenEndpoint, mux)
	logging.GetLogger().Infof("GraphQL listening on http://%v", listenEndpoint)

	p.StartEvents()

	return nil
}

// OnEdgeDeleted detect when an edge is lost and remove propagated events and pseudo wires
func (p *Probe) OnEdgeDeleted(e *graph.Edge) {
	relationType, err := e.Metadata.GetFieldString(MetaKeyRelationType)
	if err != nil {
		logging.GetLogger().Errorf("Error getting relation type from edge %v: %v", e.ID, err)
	}

	// If is an ownership relation, and the child have events defined, handle them (delete those propagated)
	if relationType == RelationTypeOwnership {
		childNode := p.graph.GetNode(e.Child)

		// If node has events defined, handle them (delete those propagated)
		if _, err := childNode.GetField(MetaKeyEvents); err == nil {
			if err = p.handleDeletedEdgeWithChildEvents(childNode, e); err != nil {
				logging.GetLogger().Errorf("Error handling node with events %v deletion: %v", childNode.ID, err)
			}
		}
	} else if relationType == RelationTypePseudoWire {
		// Gestinamos el borrado de edge-pseudowire generados entre nodos padres (building, province, region)
		// cuando se está borrando un edge-pw entre dos interfaces.
		if p.graph.GetNode(e.Parent).Metadata[MetaKeyType] == MetadataTypeNetworkInterface &&
			p.graph.GetNode(e.Child).Metadata[MetaKeyType] == MetadataTypeNetworkInterface {

			err := p.handleDeletedPseudowire(e)
			if err != nil {
				logging.GetLogger().Errorf("Error handling pseudo wire parent deletion: %v", err)
			}
		}
	}
}

// Stop the probe
func (p *Probe) Stop() {
	p.StopEvents()
	p.pseudoWireIndexer.Stop()
	p.linker.Stop()
}

// OnError implements the LinkerEventListener interface
func (p *Probe) OnError(err error) {
	if err.Error() == "Edge ID conflict" {
		logging.GetLogger().Info(err)
		return
	}
	logging.GetLogger().Error(err)
}

// NewProbe initialize the probe with the parameters from the config file
func NewProbe(g *graph.Graph) (*Probe, error) {
	probe := &Probe{
		graph:             g,
		pseudoWireIndexer: graph.NewIndexer(g, g, pseudoWireHasher, false),
	}

	// Only generate events with connections, not with listeners
	probe.linker = graph.NewResourceLinker(
		g,
		[]graph.ListenerHandler{probe.pseudoWireIndexer},
		[]graph.ListenerHandler{probe.pseudoWireIndexer},
		&simpleLinker{probe: probe},
		nil,
	)

	probe.linker.AddEventListener(probe)

	// Subscribirnos para obtener eventos de node updated
	g.AddEventListener(probe)

	return probe, nil
}

// Register called at initialization to register metadata decoders
func Register() {
	graph.NodeMetadataDecoders[MetaKeyEvents] = EventsMetadataDecoder
	graph.NodeMetadataDecoders[MetadataKeyNetworkInfo] = NetworkInfoMetadataDecoder
}
