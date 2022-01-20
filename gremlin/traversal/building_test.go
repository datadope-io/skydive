package traversal

import (
	"testing"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/stretchr/testify/assert"
)

type NodeCmp struct {
	ID   graph.Identifier
	Type string
}

func TestGetBuilding(t *testing.T) {
	testCases := []struct {
		desc          string
		graphNodes    []*graph.Node
		graphEdges    []*graph.Edge
		building      string
		expectedNodes []*graph.Node
	}{
		{
			desc: "only building node, building should not be returned",
			graphNodes: []*graph.Node{
				createTestNode("B", "building"),
			},
			graphEdges:    []*graph.Edge{},
			building:      "B",
			expectedNodes: []*graph.Node{},
		},
		{
			desc: "building with one device",
			graphNodes: []*graph.Node{
				createTestNode("B", "building"),
				createTestNode("D", "device"),
			},
			graphEdges: []*graph.Edge{
				createTestEdge("B", "building", "D", "device", "ownership"),
			},
			building: "B",
			expectedNodes: []*graph.Node{
				createTestNode("D", "device"),
			},
		},
		{
			desc: "building with one device and one interface",
			graphNodes: []*graph.Node{
				createTestNode("B", "building"),
				createTestNode("D", "device"),
				createTestNode("I", "interface"),
			},
			graphEdges: []*graph.Edge{
				createTestEdge("B", "building", "D", "device", "ownership"),
				createTestEdge("D", "device", "I", "interface", "ownership"),
			},
			building: "B",
			expectedNodes: []*graph.Node{
				createTestNode("D", "device"),
				createTestNode("I", "interface"),
			},
		},
		{
			desc: "building with one device, one interface and one logic interface",
			graphNodes: []*graph.Node{
				createTestNode("B", "building"),
				createTestNode("D", "device"),
				createTestNode("I", "interface"),
				createTestNode("L", "interface"),
			},
			graphEdges: []*graph.Edge{
				createTestEdge("B", "building", "D", "device", "ownership"),
				createTestEdge("D", "device", "I", "interface", "ownership"),
				createTestEdge("I", "interface", "L", "interface", "ownership"),
			},
			building: "B",
			expectedNodes: []*graph.Node{
				createTestNode("D", "device"),
				createTestNode("I", "interface"),
				createTestNode("L", "interface"),
			},
		},
		{
			desc: "building with one device, connected via LLDP to another device, via physical interfaces",
			graphNodes: []*graph.Node{
				createTestNode("B1", "building"),
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("B2", "building"),
				createTestNode("D2", "device"),
				createTestNode("I2", "interface"),
			},
			graphEdges: []*graph.Edge{
				createTestEdge("B1", "building", "D1", "device", "ownership"),
				createTestEdge("D1", "device", "I1", "interface", "ownership"),
				createTestEdge("B2", "building", "D2", "device", "ownership"),
				createTestEdge("D2", "device", "I2", "interface", "ownership"),
				createTestEdge("I1", "interface", "I2", "interface", "connect"),
			},
			building: "B1",
			expectedNodes: []*graph.Node{
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("D2", "device"),
				createTestNode("I2", "interface"),
			},
		},
		{
			desc: "building with one device, connected via pseudowire to another device, via physical interfaces",
			graphNodes: []*graph.Node{
				createTestNode("B1", "building"),
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("B2", "building"),
				createTestNode("D2", "device"),
				createTestNode("I2", "interface"),
			},
			graphEdges: []*graph.Edge{
				createTestEdge("B1", "building", "D1", "device", "ownership"),
				createTestEdge("D1", "device", "I1", "interface", "ownership"),
				createTestEdge("B2", "building", "D2", "device", "ownership"),
				createTestEdge("D2", "device", "I2", "interface", "ownership"),
				createTestEdge("I1", "interface", "I2", "interface", "pseudowire"),
			},
			building: "B1",
			expectedNodes: []*graph.Node{
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("D2", "device"),
				createTestNode("I2", "interface"),
			},
		},
		{
			desc: "building with one device, connected via pseudowire to another device, via logical interfaces",
			graphNodes: []*graph.Node{
				createTestNode("B1", "building"),
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("L1", "interface"),
				createTestNode("B2", "building"),
				createTestNode("D2", "device"),
				createTestNode("I2", "interface"),
				createTestNode("L2", "interface"),
			},
			graphEdges: []*graph.Edge{
				createTestEdge("B1", "building", "D1", "device", "ownership"),
				createTestEdge("D1", "device", "I1", "interface", "ownership"),
				createTestEdge("I1", "device", "L1", "interface", "ownership"),
				createTestEdge("B2", "building", "D2", "device", "ownership"),
				createTestEdge("D2", "device", "I2", "interface", "ownership"),
				createTestEdge("I2", "device", "L2", "interface", "ownership"),
				createTestEdge("L1", "interface", "L2", "interface", "pseudowire"),
			},
			building: "B1",
			expectedNodes: []*graph.Node{
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("L1", "interface"),
				createTestNode("D2", "device"),
				createTestNode("I2", "interface"),
				createTestNode("L2", "interface"),
			},
		},
		{
			desc: "building with one device, connected via pseudowire to another device, with logical interface connected to physical interface",
			graphNodes: []*graph.Node{
				createTestNode("B1", "building"),
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("L1", "interface"),
				createTestNode("B2", "building"),
				createTestNode("D2", "device"),
				createTestNode("I2", "interface"),
			},
			graphEdges: []*graph.Edge{
				createTestEdge("B1", "building", "D1", "device", "ownership"),
				createTestEdge("D1", "device", "I1", "interface", "ownership"),
				createTestEdge("I1", "device", "L1", "interface", "ownership"),
				createTestEdge("B2", "building", "D2", "device", "ownership"),
				createTestEdge("D2", "device", "I2", "interface", "ownership"),
				createTestEdge("L1", "interface", "I2", "interface", "pseudowire"),
			},
			building: "B1",
			expectedNodes: []*graph.Node{
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("L1", "interface"),
				createTestNode("D2", "device"),
				createTestNode("I2", "interface"),
			},
		},
		{
			desc: "building with one device, connected via pseudowire to another device, with logical interface connected to physical interface, using the opposite building",
			graphNodes: []*graph.Node{
				createTestNode("B1", "building"),
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("L1", "interface"),
				createTestNode("B2", "building"),
				createTestNode("D2", "device"),
				createTestNode("I2", "interface"),
			},
			graphEdges: []*graph.Edge{
				createTestEdge("B1", "building", "D1", "device", "ownership"),
				createTestEdge("D1", "device", "I1", "interface", "ownership"),
				createTestEdge("I1", "device", "L1", "interface", "ownership"),
				createTestEdge("B2", "building", "D2", "device", "ownership"),
				createTestEdge("D2", "device", "I2", "interface", "ownership"),
				createTestEdge("L1", "interface", "I2", "interface", "pseudowire"),
			},
			building: "B2",
			expectedNodes: []*graph.Node{
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("L1", "interface"),
				createTestNode("D2", "device"),
				createTestNode("I2", "interface"),
			},
		},
		{
			desc: "extra interfaces of the remote device are not included",
			graphNodes: []*graph.Node{
				createTestNode("B1", "building"),
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("B2", "building"),
				createTestNode("D2", "device"),
				createTestNode("I2.1", "interface"),
				createTestNode("I2.2", "interface"),
			},
			graphEdges: []*graph.Edge{
				createTestEdge("B1", "building", "D1", "device", "ownership"),
				createTestEdge("D1", "device", "I1", "interface", "ownership"),
				createTestEdge("B2", "building", "D2", "device", "ownership"),
				createTestEdge("D2", "device", "I2.1", "interface", "ownership"),
				createTestEdge("D2", "device", "I2.2", "interface", "ownership"),
				createTestEdge("I1", "interface", "I2.1", "interface", "pseudowire"),
			},
			building: "B1",
			expectedNodes: []*graph.Node{
				createTestNode("D1", "device"),
				createTestNode("I1", "interface"),
				createTestNode("D2", "device"),
				createTestNode("I2.1", "interface"),
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

			d := BuildingGremlinTraversalStep{
				building: tC.building,
			}
			nodes, err := d.getBuilding(g)
			assert.NoError(t, err)

			// Simplify the comparasion. Take only into account the node ID and the type.
			nodesCmp := make([]NodeCmp, len(nodes))
			for i, n := range nodes {
				nodesCmp[i] = NodeCmp{
					ID:   n.ID,
					Type: n.Metadata["Type"].(string),
				}
			}

			expectedNodesCmp := make([]NodeCmp, len(tC.expectedNodes))
			for i, n := range tC.expectedNodes {
				expectedNodesCmp[i] = NodeCmp{
					ID:   n.ID,
					Type: n.Metadata["Type"].(string),
				}
			}

			assert.ElementsMatch(t, nodesCmp, expectedNodesCmp)
		})
	}
}

func createTestNode(id, nodeType string) *graph.Node {
	return graph.CreateNode(graph.Identifier(id), graph.Metadata{"Type": nodeType, "Name": id}, graph.Unix(0, 0), "", "")
}

func createTestEdge(parentID, parentType, childID, childType, relationType string) *graph.Edge {
	return graph.CreateEdge(
		graph.Identifier(parentID+"-"+childID+"-"+relationType),
		createTestNode(parentID, parentType),
		createTestNode(childID, childType),
		graph.Metadata{"RelationType": relationType},
		graph.Unix(0, 0),
		"",
		"",
	)
}
