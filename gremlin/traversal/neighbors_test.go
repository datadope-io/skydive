package traversal

import (
	"testing"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/stretchr/testify/assert"
)

func TestGetNeighbors(t *testing.T) {
	testCases := []struct {
		desc          string
		graphNodes    []*graph.Node
		graphEdges    []*graph.Edge
		originNodes   []*graph.Node
		maxDepth      int64
		edgeFilter    graph.ElementMatcher
		expectedNodes []*graph.Node
	}{
		{
			desc: "one graph node",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("A"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			graphEdges: []*graph.Edge{},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("A"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			maxDepth:   0,
			edgeFilter: nil,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("A"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		},
		{
			desc: "interface connected to host and to other interface",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			graphEdges: []*graph.Edge{
				graph.CreateEdge(
					graph.Identifier("HostA-IntA"),
					graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("IntA-IntB"),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ConnectsTo"},
					graph.Unix(0, 0),
					"",
					"",
				),
			},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			maxDepth:   1,
			edgeFilter: nil,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		},
		{
			desc: "host connected to interface and that to other interface, depth 2",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			graphEdges: []*graph.Edge{
				graph.CreateEdge(
					graph.Identifier("HostA-IntA"),
					graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("IntA-IntB"),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ConnectsTo"},
					graph.Unix(0, 0),
					"",
					"",
				),
			},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			maxDepth:   2,
			edgeFilter: nil,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		},
		{
			desc: "two hosts connected through interfaces, depth 3",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			graphEdges: []*graph.Edge{
				graph.CreateEdge(
					graph.Identifier("HostA-IntA"),
					graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("HostB-IntB"),
					graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("IntA-IntB"),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ConnectsTo"},
					graph.Unix(0, 0),
					"",
					"",
				),
			},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			maxDepth:   3,
			edgeFilter: nil,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		}, {
			desc: "two hosts connected through interfaces, reverse connection, depth 3",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			graphEdges: []*graph.Edge{
				graph.CreateEdge(
					graph.Identifier("HostA-IntA"),
					graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("HostB-IntB"),
					graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("IntB-IntA"),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ConnectsTo"},
					graph.Unix(0, 0),
					"",
					"",
				),
			},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			maxDepth:   3,
			edgeFilter: nil,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			b, err := graph.NewMemoryBackend()
			if err != nil {
				t.Error(err.Error())
			}
			g := graph.NewGraph("testhost", b, "analyzer.testhost")

			for _, n := range tC.graphNodes {
				err := g.AddNode(n)
				if err != nil {
					t.Error(err.Error())
				}
			}

			for _, e := range tC.graphEdges {
				err := g.AddEdge(e)
				if err != nil {
					t.Error(err.Error())
				}
			}

			neighbors := getNeighbors(g, tC.originNodes, tC.maxDepth, tC.edgeFilter)

			assert.ElementsMatch(t, neighbors, tC.expectedNodes)

		})
	}
}
