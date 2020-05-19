package procpeering

import (
	"os"
	"testing"

	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/proccon"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Remove comment to change logging level to debug
	//logging.InitLogging("id", true, []*logging.LoggerConfig{logging.NewLoggerConfig(logging.NewStdioBackend(os.Stdout), "5", "UTF-8")})
	os.Exit(m.Run())
}

func newGraph(t *testing.T) *graph.Graph {
	b, err := graph.NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	return graph.NewGraph("testhost", b, "analyzer.testhost")
}

// TestMatchConnectionListener check if given a Software with a determined listener, if a new Server appears with a connection matching the listener, an edge should be created
func TestMatchConnectionListener(t *testing.T) {
	// GIVEN
	g := newGraph(t)
	p := NewProbe(g)
	p.Start()

	softwareServer, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		proccon.MetadataNameKey:    "swServer",
		proccon.MetadataTypeKey:    proccon.MetadataTypeSoftware,
		proccon.MetadataTCPConnKey: map[string]proccon.ProcInfo{},
		proccon.MetadataListenEndpointKey: map[string]proccon.ProcInfo{
			"1.1.1.1:80": {
				CreatedAt: 0,
				UpdatedAt: 0,
				Revision:  1,
			},
		},
	})
	if err != nil {
		t.Errorf("Unable to create software server: %v", err)
	}

	// WHEN
	softwareClient, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		proccon.MetadataNameKey: "swClient",
		proccon.MetadataTypeKey: proccon.MetadataTypeSoftware,
		proccon.MetadataTCPConnKey: map[string]proccon.ProcInfo{
			"1.1.1.1:80": {
				CreatedAt: 0,
				UpdatedAt: 0,
				Revision:  1,
			},
		},
	})
	if err != nil {
		t.Errorf("Unable to create software client: %v", err)
	}

	// THEN
	hasSoftwareEdgeFilter := graph.NewElementFilter(filters.NewTermStringFilter(proccon.MetadataRelationTypeKey, RelationTypeConnection))
	hasSoftwareEdges := g.GetEdges(hasSoftwareEdgeFilter)
	if len(hasSoftwareEdges) == 0 {
		t.Fatalf("Edge %s not created", proccon.RelationTypeHasSoftware)
	} else if len(hasSoftwareEdges) > 1 {
		t.Errorf("Too many edges %s created", proccon.RelationTypeHasSoftware)
	}

	hasSoftwareEdge := hasSoftwareEdges[0]

	assert.Equal(t, hasSoftwareEdge.Parent, softwareClient.ID)
	assert.Equal(t, hasSoftwareEdge.Child, softwareServer.ID)
}

// TestMatchConnectionListenerUpdatedNode given a Software node without network info, it is updated to add a determined listener, if a new Server appears with a connection matching the listener, an edge should be created
func TestMatchConnectionListenerUpdatedNode(t *testing.T) {
	// GIVEN
	g := newGraph(t)
	p := NewProbe(g)
	p.Start()

	softwareServer, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		proccon.MetadataNameKey: "swServer",
		proccon.MetadataTypeKey: proccon.MetadataTypeSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create software server: %v", err)
	}

	p.graph.AddMetadata(softwareServer, proccon.MetadataListenEndpointKey, map[string]proccon.ProcInfo{
		"1.1.1.1:80": {
			CreatedAt: 0,
			UpdatedAt: 0,
			Revision:  1,
		},
	})

	// WHEN
	softwareClient, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		proccon.MetadataNameKey: "swClient",
		proccon.MetadataTypeKey: proccon.MetadataTypeSoftware,
		proccon.MetadataTCPConnKey: map[string]proccon.ProcInfo{
			"1.1.1.1:80": {
				CreatedAt: 0,
				UpdatedAt: 0,
				Revision:  1,
			},
		},
	})
	if err != nil {
		t.Errorf("Unable to create software client: %v", err)
	}

	// THEN
	hasSoftwareEdgeFilter := graph.NewElementFilter(filters.NewTermStringFilter(proccon.MetadataRelationTypeKey, RelationTypeConnection))
	hasSoftwareEdges := g.GetEdges(hasSoftwareEdgeFilter)
	if len(hasSoftwareEdges) == 0 {
		t.Fatalf("Edge %s not created", proccon.RelationTypeHasSoftware)
	} else if len(hasSoftwareEdges) > 1 {
		t.Errorf("Too many edges %s created", proccon.RelationTypeHasSoftware)
	}

	hasSoftwareEdge := hasSoftwareEdges[0]

	// Conex from parent (client) to child (server)
	assert.Equal(t, hasSoftwareEdge.Parent, softwareClient.ID)
	assert.Equal(t, hasSoftwareEdge.Child, softwareServer.ID)
}

// TestUpdatingNetworkingMetadataDoesNotCreateNewEdges check that updating network metadata does not duplicate edges
func TestUpdatingNetworkingMetadataDoesNotCreateNewEdges(t *testing.T) {
	// GIVEN
	g := newGraph(t)
	p := NewProbe(g)
	p.Start()

	softwareServer, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		proccon.MetadataNameKey:    "swServer",
		proccon.MetadataTypeKey:    proccon.MetadataTypeSoftware,
		proccon.MetadataTCPConnKey: map[string]proccon.ProcInfo{},
		proccon.MetadataListenEndpointKey: map[string]proccon.ProcInfo{
			"1.1.1.1:80": {
				CreatedAt: 0,
				UpdatedAt: 0,
				Revision:  1,
			},
		},
	})
	if err != nil {
		t.Errorf("Unable to create software server: %v", err)
	}

	softwareClient, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		proccon.MetadataNameKey: "swClient",
		proccon.MetadataTypeKey: proccon.MetadataTypeSoftware,
		proccon.MetadataTCPConnKey: map[string]proccon.ProcInfo{
			"1.1.1.1:80": {
				CreatedAt: 0,
				UpdatedAt: 0,
				Revision:  1,
			},
		},
	})
	if err != nil {
		t.Errorf("Unable to create software client: %v", err)
	}

	// WHEN
	// Once the edge tcp_conn is connected, we modify the network metadata for server and client
	p.graph.AddMetadata(softwareServer, proccon.MetadataListenEndpointKey, map[string]proccon.ProcInfo{
		"1.1.1.1:80": {
			CreatedAt: 0,
			UpdatedAt: 1,
			Revision:  2,
		},
	})

	p.graph.AddMetadata(softwareClient, proccon.MetadataTCPConnKey, map[string]proccon.ProcInfo{
		"1.1.1.1:80": {
			CreatedAt: 0,
			UpdatedAt: 1,
			Revision:  2,
		},
	})

	// THEN
	hasSoftwareEdgeFilter := graph.NewElementFilter(filters.NewTermStringFilter(proccon.MetadataRelationTypeKey, RelationTypeConnection))
	hasSoftwareEdges := g.GetEdges(hasSoftwareEdgeFilter)
	if len(hasSoftwareEdges) == 0 {
		t.Fatalf("Edge %s not created", proccon.RelationTypeHasSoftware)
	} else if len(hasSoftwareEdges) > 1 {
		t.Errorf("Too many edges %s created", proccon.RelationTypeHasSoftware)
	}

	hasSoftwareEdge := hasSoftwareEdges[0]

	assert.Equal(t, hasSoftwareEdge.Parent, softwareClient.ID)
	assert.Equal(t, hasSoftwareEdge.Child, softwareServer.ID)
}

// TestRemovedConnectionFromMetadataDeleteConnectionEdge if we have two nodes connected with a tcp_conn edge, if the connection from the client node is removed, edge should be deleted
func TestRemovedConnectionFromMetadataDeleteConnectionEdge(t *testing.T) {
	// GIVEN
	g := newGraph(t)
	p := NewProbe(g)
	p.Start()

	_, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		proccon.MetadataNameKey:    "swServer",
		proccon.MetadataTypeKey:    proccon.MetadataTypeSoftware,
		proccon.MetadataTCPConnKey: map[string]proccon.ProcInfo{},
		proccon.MetadataListenEndpointKey: map[string]proccon.ProcInfo{
			"1.1.1.1:80": {
				CreatedAt: 0,
				UpdatedAt: 0,
				Revision:  1,
			},
		},
	})
	if err != nil {
		t.Errorf("Unable to create software server: %v", err)
	}

	softwareClient, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		proccon.MetadataNameKey: "swClient",
		proccon.MetadataTypeKey: proccon.MetadataTypeSoftware,
		proccon.MetadataTCPConnKey: map[string]proccon.ProcInfo{
			"1.1.1.1:80": {
				CreatedAt: 0,
				UpdatedAt: 0,
				Revision:  1,
			},
		},
	})
	if err != nil {
		t.Errorf("Unable to create software client: %v", err)
	}

	// Check that we have the tcp_conn edge
	hasSoftwareEdgeFilter := graph.NewElementFilter(filters.NewTermStringFilter(proccon.MetadataRelationTypeKey, RelationTypeConnection))
	assert.Len(t, g.GetEdges(hasSoftwareEdgeFilter), 1)

	// WHEN
	// Once the edge tcp_conn is connected, we remove the connection from the client
	p.graph.AddMetadata(softwareClient, proccon.MetadataTCPConnKey, map[string]proccon.ProcInfo{})

	// THEN
	assert.Empty(t, g.GetEdges(hasSoftwareEdgeFilter))
}

// TestRemovedListenerFromMetadataDeleteEdgesFromClients if a server node updates its metadata to delete a listener, all edges from clients should be removed in the next update of clients
func TestRemovedListenerFromMetadataDeleteEdgesFromClients(t *testing.T) {
	// GIVEN
	g := newGraph(t)
	p := NewProbe(g)
	p.Start()

	softwareServer, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		proccon.MetadataNameKey:    "swServer",
		proccon.MetadataTypeKey:    proccon.MetadataTypeSoftware,
		proccon.MetadataTCPConnKey: map[string]proccon.ProcInfo{},
		proccon.MetadataListenEndpointKey: map[string]proccon.ProcInfo{
			"1.1.1.1:80": {
				CreatedAt: 0,
				UpdatedAt: 0,
				Revision:  1,
			},
		},
	})
	if err != nil {
		t.Errorf("Unable to create software server: %v", err)
	}

	// Create numberOfClients clients connected to the server
	numberOfClients := 3
	clients := make([]*graph.Node, 3)

	for i := 0; i < numberOfClients; i++ {
		c, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
			proccon.MetadataNameKey: "swClient",
			proccon.MetadataTypeKey: proccon.MetadataTypeSoftware,
			proccon.MetadataTCPConnKey: map[string]proccon.ProcInfo{
				"1.1.1.1:80": {
					CreatedAt: 0,
					UpdatedAt: 0,
					Revision:  1,
				},
			},
		})
		if err != nil {
			t.Errorf("Unable to create software client: %v", err)
		}
		clients[i] = c
	}
	// Check that we have the three tcp_conn edges
	connectionEdgeFilter := graph.NewElementFilter(filters.NewTermStringFilter(proccon.MetadataRelationTypeKey, RelationTypeConnection))
	assert.Len(t, g.GetEdges(connectionEdgeFilter), numberOfClients)

	// WHEN
	// Once we have all clients connected, delete the listener
	p.graph.AddMetadata(softwareServer, proccon.MetadataListenEndpointKey, map[string]proccon.ProcInfo{})

	// Edges will be still there, for simplicity they are only handled when modifications take place in clients
	assert.Len(t, g.GetEdges(connectionEdgeFilter), numberOfClients)

	// Simulate a new reception of network data in client
	for _, c := range clients {
		p.graph.UpdateMetadata(c, proccon.MetadataTCPConnKey, func(field interface{}) bool {
			info := field.(map[string]proccon.ProcInfo)
			for _, v := range info {
				v.UpdatedAt = graph.TimeNow().UnixMilli()
				v.Revision++
			}
			return true
		})
	}

	// THEN
	assert.Empty(t, g.GetEdges(connectionEdgeFilter))
}
