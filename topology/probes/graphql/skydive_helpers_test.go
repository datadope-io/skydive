package graphql

import (
	"math"
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/stretchr/testify/assert"
)

const GraphOrigin = "analyzer.testhost"

func TestNewNodeWithTimeDefined(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	id := graph.Identifier("a")
	metadata := graph.Metadata{"Name": "a", "Type": "type"}
	tt := time.Unix(100, 0)

	node, err := r.newNode(id, metadata, &tt)
	assert.Nil(t, err)

	nodeGraph := g.GetNode(id)
	assert.Equalf(t, node, nodeGraph, "different node in graph and returned by func")

	assert.Equalf(t, PrefixOriginName+GraphOrigin, node.Origin, "incorrect origin")
	assert.Equalf(t, graph.Time(tt), node.CreatedAt, "incorrect CreatedAt")
	assert.Equalf(t, graph.Time(tt), node.UpdatedAt, "incorrect UpdatetAt")
}

func TestNewNodeWithTimeUndefined(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	id := graph.Identifier("a")
	metadata := graph.Metadata{"Name": "a", "Type": "type"}
	tt := graph.TimeUTC()

	node, err := r.newNode(id, metadata, nil)
	assert.Nil(t, err)

	nodeGraph := g.GetNode(id)
	assert.Equal(t, node, nodeGraph)

	assert.Equal(t, PrefixOriginName+GraphOrigin, node.Origin)
	// Time should be "now". We check that is close to that value (less than 1s).
	assert.Condition(t, func() bool {
		diffCreatedAt := math.Abs(float64(tt.UnixMilli() - node.CreatedAt.UnixMilli()))
		diffUpdatedAt := math.Abs(float64(tt.UnixMilli() - node.UpdatedAt.UnixMilli()))
		return diffCreatedAt < 1000 && diffUpdatedAt < 1000
	})
}

func TestNewEdgeWithTimeDefined(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	id := graph.Identifier("a")
	metadata := graph.Metadata{"Name": "a", "Type": "type"}
	aNode, err := r.newNode(id, metadata, nil)
	assert.Nil(t, err)

	id = graph.Identifier("b")
	metadata = graph.Metadata{"Name": "b", "Type": "type"}
	bNode, err := r.newNode(id, metadata, nil)
	assert.Nil(t, err)

	tt := time.Unix(100, 0)
	edgeID := graph.Identifier("a-b")
	edgeMetadata := graph.Metadata{"Name": "a-b", "RelationType": "type"}
	edge, err := r.newEdge(edgeID, aNode, bNode, edgeMetadata, &tt)
	assert.Nil(t, err)

	edgeGraph := g.GetEdge(edgeID)
	assert.Equal(t, edge, edgeGraph)

	assert.Equal(t, PrefixOriginName+GraphOrigin, edge.Origin)
	assert.Equal(t, graph.Time(tt), edge.CreatedAt)
	assert.Equal(t, graph.Time(tt), edge.UpdatedAt)
}

func TestNewEdgeWithTimeUndefined(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	id := graph.Identifier("a")
	metadata := graph.Metadata{"Name": "a", "Type": "type"}
	aNode, err := r.newNode(id, metadata, nil)
	assert.Nil(t, err)

	id = graph.Identifier("b")
	metadata = graph.Metadata{"Name": "b", "Type": "type"}
	bNode, err := r.newNode(id, metadata, nil)
	assert.Nil(t, err)

	tt := graph.TimeUTC()
	edgeID := graph.Identifier("a-b")
	edgeMetadata := graph.Metadata{"Name": "a-b", "RelationType": "type"}
	edge, err := r.newEdge(edgeID, aNode, bNode, edgeMetadata, nil)
	assert.Nil(t, err)

	edgeGraph := g.GetEdge(edgeID)
	assert.Equal(t, edge, edgeGraph)

	assert.Equal(t, PrefixOriginName+GraphOrigin, edge.Origin)
	// Time should be "now". We check that is close to that value (less than 1s).
	assert.Condition(t, func() bool {
		diffCreatedAt := math.Abs(float64(tt.UnixMilli() - edge.CreatedAt.UnixMilli()))
		diffUpdatedAt := math.Abs(float64(tt.UnixMilli() - edge.UpdatedAt.UnixMilli()))
		return diffCreatedAt < 1000 && diffUpdatedAt < 1000
	})
}

func TestCreateOrUpdateNode_create(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	id := graph.Identifier("a")
	metadata := graph.Metadata{"Name": "a", "Type": "type"}

	_, created, updated, err := r.createOrUpdateNode(id, metadata, nil)
	assert.Nil(t, err)
	assert.True(t, created)
	assert.False(t, updated)
}

func TestCreateOrUpdateNode_update(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	// Create new node
	id := graph.Identifier("a")
	metadata := graph.Metadata{"Name": "a", "Type": "type"}
	tt := time.Unix(100, 0)
	node, created, updated, err := r.createOrUpdateNode(id, metadata, &tt)
	assert.Nil(t, err)
	assert.True(t, created)
	assert.False(t, updated)
	assert.Equal(t, int64(1), node.Revision)

	// Trying to create the same node do nothing
	node, created, updated, err = r.createOrUpdateNode(id, metadata, &tt)
	assert.Nil(t, err)
	assert.False(t, created)
	assert.False(t, updated)
	assert.Equal(t, int64(1), node.Revision)

	// Update node metadata
	metadata = graph.Metadata{"Name": "a", "Type": "type2"}
	node, created, updated, err = r.createOrUpdateNode(id, metadata, nil)
	assert.Nil(t, err)
	assert.False(t, created)
	assert.True(t, updated)
	assert.Equal(t, int64(2), node.Revision)
	// UpdatedAt value updated
	assert.Greater(t, node.UpdatedAt.UnixMilli(), node.CreatedAt.UnixMilli())
	assert.Equal(t, metadata, node.Metadata)

	// Trying to update an existing node with different metadata and CreatedAt time
	// return an error and does not update the node
	tt2 := time.Unix(200, 0)
	metadata = graph.Metadata{"Name": "a", "Type": "type3"}
	_, _, updated, err = r.createOrUpdateNode(id, metadata, &tt2)
	assert.Falsef(t, updated, "updated trying to modify date")
	assert.EqualError(t, err, "forbidden to modify CreatedAt value of existing node")
}

func TestCreateOrUpdateEdgeCreate(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	// Edge metadata should have RelationType defined
	metadata := graph.Metadata{"Name": "a"}
	_, _, _, err := r.createOrUpdateEdge(nil, nil, metadata, nil)
	assert.EqualError(t, err, "missing RelationType: Field not found")

	// Define a valid metadata for the edge
	metadata = graph.Metadata{"Name": "a", "RelationType": "type"}

	// Create a valid source node, but it its not added to the graph
	srcNode := createNode("a", graph.Metadata{"Name": "a", "Type": "node"})
	_, _, _, err = r.createOrUpdateEdge(srcNode, nil, metadata, nil)
	assert.EqualError(t, err, "invalid destination node")

	// Create a valid destination node, but it its not added to the graph
	dstNode := createNode("b", graph.Metadata{"Name": "b", "Type": "node"})
	_, _, _, err = r.createOrUpdateEdge(nil, dstNode, metadata, nil)
	assert.EqualError(t, err, "invalid source node")

	_, _, _, err = r.createOrUpdateEdge(srcNode, dstNode, metadata, nil)
	assert.EqualError(t, err, "unable to create edge type type, between 'a' and 'b': Parent node not found")

	// Add source node to the graph
	err = g.AddNode(srcNode)
	assert.Nil(t, err)

	// source node is in the graph, but not child node
	_, _, _, err = r.createOrUpdateEdge(srcNode, dstNode, metadata, nil)
	assert.EqualError(t, err, "unable to create edge type type, between 'a' and 'b': Child node not found")

	// Add destination node to the graph
	err = g.AddNode(dstNode)
	assert.Nil(t, err)

	edge, created, updated, err := r.createOrUpdateEdge(srcNode, dstNode, metadata, nil)
	assert.Nil(t, err)
	assert.True(t, created)
	assert.False(t, updated)
	assert.Equal(t, srcNode.ID, edge.Parent)
	assert.Equal(t, dstNode.ID, edge.Child)

	// Check relation type is as expected
	edgeRelationType, err := edge.Metadata.GetFieldString(MetaKeyRelationType)
	assert.Nil(t, err)
	assert.Equal(t, metadata[MetaKeyRelationType], edgeRelationType)

	tt := time.Unix(100, 0)
	_, _, _, err = r.createOrUpdateEdge(srcNode, dstNode, metadata, &tt)
	assert.EqualError(t, err, "forbidden to modify CreatedAt value of existing edge")
}

func TestCreateOrUpdateEdgeUpdate(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	// Create nodes
	srcNode := createNode("a", graph.Metadata{"Name": "a", "Type": "node"})
	err := g.AddNode(srcNode)
	assert.Nil(t, err)

	dstNode := createNode("b", graph.Metadata{"Name": "b", "Type": "node"})
	err = g.AddNode(dstNode)
	assert.Nil(t, err)

	metadata := graph.Metadata{"Name": "a", "RelationType": "type"}
	_, _, _, err = r.createOrUpdateEdge(srcNode, dstNode, metadata, nil)
	assert.Nil(t, err)

	metadata = graph.Metadata{"Name": "a", "RelationType": "type", "foo": "bar"}
	edge, created, updated, err := r.createOrUpdateEdge(srcNode, dstNode, metadata, nil)
	assert.Nil(t, err)
	assert.False(t, created)
	assert.True(t, updated)
	assert.Equal(t, edge.Revision, int64(2))
	assert.Equal(t, metadata, edge.Metadata)
}

func TestGetNodeFromGremlinQuery(t *testing.T) {
	testCases := []struct {
		desc          string
		query         string
		nodes         []*graph.Node
		expectedNode  *graph.Node
		expectedError string
	}{
		{
			desc:          "invalid gremlin query return an error",
			query:         "G.B.C",
			expectedError: "parsing gremlin query: Expected step function, got: B",
		},
		{
			desc:          "query should return nodes",
			query:         "G.E()",
			expectedError: "invalid graph output, expecting nodes",
		},
		{
			desc:          "expected one node",
			query:         "G.V()",
			expectedError: "expecting only one node, gremlin query returned 0",
		},
		{
			desc:  "only one node expected",
			query: "G.V()",
			nodes: []*graph.Node{
				createNode("a", graph.Metadata{"Name": "a", "Type": "host"}),
				createNode("b", graph.Metadata{"Name": "b", "Type": "host"}),
			},
			expectedError: "expecting only one node, gremlin query returned 2",
		},
		{
			desc:  "return the node selected by the query",
			query: "G.V().Has('Name','a')",
			nodes: []*graph.Node{
				createNode("a", graph.Metadata{"Name": "a", "Type": "host"}),
				createNode("b", graph.Metadata{"Name": "b", "Type": "host"}),
			},
			expectedNode: createNode("a", graph.Metadata{"Name": "a", "Type": "host"}),
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			g := newGraph(t)
			for _, n := range tC.nodes {
				g.AddNode(n)
			}

			r := Resolver{Graph: g}

			n, err := r.getNodeFromGremlinQuery(tC.query)
			if tC.expectedError != "" {
				assert.EqualError(t, err, tC.expectedError)
			}

			assert.Equal(t, n, tC.expectedNode)
		})
	}
}

func newGraph(t *testing.T) *graph.Graph {
	b, err := graph.NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	return graph.NewGraph("testhost", b, GraphOrigin)
}

// createNode func to create nodes for testing
func createNode(id string, m graph.Metadata) *graph.Node {
	return graph.CreateNode(graph.Identifier(id), m, graph.Time(time.Unix(100, 100)), "host", "origin")
}
