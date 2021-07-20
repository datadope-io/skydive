package netexternal

import (
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/stretchr/testify/assert"
)

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

	return graph.NewGraph("testhost", b, "analyzer.testhost")
}

// createNode func to create nodes for testing
func createNode(id string, m graph.Metadata) *graph.Node {
	return graph.CreateNode(graph.Identifier(id), m, graph.Time(time.Unix(100, 100)), "host", "origin")
}
