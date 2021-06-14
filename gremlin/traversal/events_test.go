package traversal

import (
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/stretchr/testify/assert"
)

func TestMergeEventsNilNodeEvents(t *testing.T) {
	key := "Events"

	metadataNode1 := graph.Metadata{key: map[string]interface{}{
		"abc": map[string]string{"descr": "foo"},
	}}
	node := CreateNode("nodeA", metadataNode1, graph.TimeUTC(), 1)

	nodeEventsAgg := mergeEvents(node, key, nil)

	expected := map[string][]interface{}{
		"abc": {
			map[string]string{"descr": "foo"},
		},
	}

	assert.Equal(t, expected, nodeEventsAgg)
}

func TestMergeEvents(t *testing.T) {
	tests := []struct {
		name        string
		nodesEvents []interface{}
		expected    map[string][]interface{}
	}{
		{
			name:     "no nodes",
			expected: map[string][]interface{}{},
		},
		{
			name: "one node",
			nodesEvents: []interface{}{
				map[string]interface{}{
					"abc": map[string]string{"descr": "foo"},
				}},
			expected: map[string][]interface{}{
				"abc": {
					map[string]string{"descr": "foo"},
				},
			},
		},
		{
			name: "two nodes, different keys",
			nodesEvents: []interface{}{
				map[string]interface{}{
					"abc": map[string]string{"descr": "foo"},
				},
				map[string]interface{}{
					"xyz": map[string]string{"descr": "bar"},
				}},
			expected: map[string][]interface{}{
				"abc": {
					map[string]string{"descr": "foo"},
				},
				"xyz": {
					map[string]string{"descr": "bar"},
				},
			},
		},
		{
			name: "two nodes, same keys",
			nodesEvents: []interface{}{
				map[string]interface{}{
					"abc": map[string]string{"descr": "foo"},
				},
				map[string]interface{}{
					"abc": map[string]string{"descr": "bar"},
				}},
			expected: map[string][]interface{}{
				"abc": {
					map[string]string{"descr": "foo"},
					map[string]string{"descr": "bar"},
				},
			},
		},
		{
			name: "two nodes, repeating one event, should be removed",
			nodesEvents: []interface{}{
				map[string]interface{}{
					"abc": map[string]string{"descr": "foo"},
				},
				map[string]interface{}{
					"abc": map[string]string{"descr": "foo"},
					"xxx": map[string]string{"descr": "bar"},
				}},
			expected: map[string][]interface{}{
				"abc": {
					map[string]string{"descr": "foo"},
				},
				"xxx": {
					map[string]string{"descr": "bar"},
				},
			},
		},
	}

	key := "Events"

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodeEventsAgg := map[string][]interface{}{}

			for _, nodeEvents := range test.nodesEvents {
				metadataNode1 := graph.Metadata{key: nodeEvents}
				node := CreateNode("nodeA", metadataNode1, graph.TimeUTC(), 1)

				nodeEventsAgg = mergeEvents(node, key, nodeEventsAgg)
			}

			assert.Equal(t, test.expected, nodeEventsAgg)
		})
	}
}

func TestInterfaceEvents(t *testing.T) {
	tests := []struct {
		name    string
		InNodes []*graph.Node
		key     string
		aggKey  string
		// Expected nodes
		OutNodes []*graph.Node
	}{
		{
			name: "no input nodes",
		},
		{
			// Node passes the step without being modified
			name:   "one input node without key defined",
			key:    "Events",
			aggKey: "EventsAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{}, graph.Time(time.Unix(0, 0)), 1),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"EventsAgg": map[string][]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
		},
		{
			name:   "one input node with key defined but empty",
			key:    "Events",
			aggKey: "EventsAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events":    map[string]interface{}{},
					"EventsAgg": map[string][]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
		},
		{
			name:   "one input node with key defined and one alarm",
			key:    "Events",
			aggKey: "EventsAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
					"EventsAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
					}},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
		},
		{
			name:   "two different input nodes with key defined and one alarm each one",
			key:    "Events",
			aggKey: "EventsAxx",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("B", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
					"EventsAxx": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
					}},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("B", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
					"EventsAxx": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
					}},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
		},
		{
			name:   "one node, with a previous version, both without key defined",
			key:    "Events",
			aggKey: "EventsAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{}, graph.Time(time.Unix(0, 0)), 2),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"EventsAgg": map[string][]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
		},
		{
			name:   "one node, with a previous version, both with key defined but empty",
			key:    "Events",
			aggKey: "EventsAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events":    map[string]interface{}{},
					"EventsAgg": map[string][]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
		},
		{
			name:   "one node, with a previous version, both with key defined, same event different content",
			key:    "Events",
			aggKey: "EventsAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "b"}},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "b"}},
					"EventsAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
						map[string]interface{}{"desc": "b"},
					}},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
		},
		{
			name:   "one node, with a previous version, first one without event, second one with event",
			key:    "Events",
			aggKey: "EventsAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "b"}},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "b"}},
					"EventsAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "b"},
					}},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
		},
		{
			name:   "one node, with a previous version, first one with event, second one without event",
			key:    "Events",
			aggKey: "EventsAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{},
					"EventsAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
					}},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
		},
		{
			name:   "one node, with two previous versions, first with, second without, third with",
			key:    "Events",
			aggKey: "EventsAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 2),
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "c"}},
				}, graph.Time(time.Unix(0, 0)), 3),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Events": map[string]interface{}{"e1": map[string]interface{}{"desc": "c"}},
					"EventsAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
						map[string]interface{}{"desc": "c"},
					}},
				}, graph.Time(time.Unix(0, 0)), 3),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := graph.NewMemoryBackend()
			if err != nil {
				t.Error(err.Error())
			}
			g := graph.NewGraph("testhost", b, "analyzer.testhost")

			gt := traversal.NewGraphTraversal(g, false)
			ctx := traversal.StepContext{}
			tvIn := traversal.NewGraphTraversalV(gt, test.InNodes)

			ts := InterfaceEvents(ctx, tvIn, test.key, test.aggKey)

			tvOut, ok := ts.(*traversal.GraphTraversalV)
			if !ok {
				t.Errorf("Invalid GraphTraversal type")
			}

			assert.ElementsMatch(t, test.OutNodes, tvOut.GetNodes())
		})
	}
}

// CreateNode func to create nodes with a specific node revision
func CreateNode(id string, m graph.Metadata, t graph.Time, revision int64) *graph.Node {
	n := graph.CreateNode(graph.Identifier(id), m, t, "host", "orig")
	n.Revision = revision
	return n
}
