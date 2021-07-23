package graphql

import (
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
	"github.com/stretchr/testify/assert"
)

func TestDeleteExpiredEvents(t *testing.T) {
	tests := []struct {
		desc                 string
		nodeID               graph.Identifier
		thresholdTime        time.Time
		nodes                []*graph.Node
		expectedNodeMetadata graph.Metadata
		expectedError        string
	}{
		{
			desc:          "error if Metadata.Events have the wrong format",
			nodeID:        graph.Identifier("a"),
			thresholdTime: time.Unix(0, 0),
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": map[string]string{},
			})},
			expectedError: "unable to delete expired events: invalid Metadata.Events format",
		},
		{
			desc:          "events with TTL 0 should not be deleted",
			nodeID:        graph.Identifier("a"),
			thresholdTime: time.Unix(0, 0),
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       0,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(100, 0),
						UpdatedAt: graph.Unix(100, 0),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       0,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(100, 0),
						UpdatedAt: graph.Unix(100, 0),
					},
				},
			},
		},
		{
			desc:          "event older than threshold but less than event+ttl should not be deleted",
			nodeID:        graph.Identifier("a"),
			thresholdTime: time.Unix(100, 0),
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       50,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(80, 0),
						UpdatedAt: graph.Unix(80, 0),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       50,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(80, 0),
						UpdatedAt: graph.Unix(80, 0),
					},
				},
			},
		},
		{
			desc:          "event + ttl older than threshold should be deleted",
			nodeID:        graph.Identifier("a"),
			thresholdTime: time.Unix(100, 0),
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"should_be_deleted": Event{
						TTL:       50,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(30, 0),
						UpdatedAt: graph.Unix(30, 0),
					},
					"should_not_be_deleted": Event{
						TTL:       100,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(30, 0),
						UpdatedAt: graph.Unix(30, 0),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"should_not_be_deleted": Event{
						TTL:       100,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(30, 0),
						UpdatedAt: graph.Unix(30, 0),
					},
				},
			},
		},
	}

	for _, tC := range tests {
		t.Run(tC.desc, func(t *testing.T) {
			g := newGraph(t)
			for _, n := range tC.nodes {
				g.AddNode(n)
			}

			p := Probe{graph: g}
			err := p.deleteExpiredEvents(tC.nodeID, tC.thresholdTime)

			if tC.expectedError != "" {
				assert.EqualError(t, err, tC.expectedError)
			} else {
				// Check the node has been modified
				modNode := g.GetNode(tC.nodeID)
				assert.Equal(t, tC.expectedNodeMetadata, modNode.Metadata)
			}

		})
	}
}

// TestCleanNodesEvents delete old events, do not delete recent events, do nothing with
// node without Events
func TestCleanNodesEvents(t *testing.T) {
	g := newGraph(t)
	p := Probe{graph: g}

	g.AddNode(createNode("a", graph.Metadata{
		"Name": "a",
		"Type": "host",
		"Events": &Events{
			"should_be_deleted": Event{
				TTL:       100,
				Level:     "INFO",
				Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
				CreatedAt: graph.Unix(0, 0),
				UpdatedAt: graph.Unix(0, 0),
			},
			"should_not_be_deleted": Event{
				TTL:       200,
				Level:     "INFO",
				Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
				CreatedAt: graph.Unix(0, 0),
				UpdatedAt: graph.Unix(0, 0),
			},
		},
	}))
	g.AddNode(createNode("b", graph.Metadata{
		"Name": "b",
		"Type": "host",
	}))

	p.cleanNodesEvents(time.Unix(150, 0))

	assert.Equal(t, graph.Metadata{
		"Name": "a",
		"Type": "host",
		"Events": &Events{
			"should_not_be_deleted": Event{
				TTL:       200,
				Level:     "INFO",
				Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
				CreatedAt: graph.Unix(0, 0),
				UpdatedAt: graph.Unix(0, 0),
			},
		},
	}, g.GetNode("a").Metadata)
	assert.Equal(t, graph.Metadata{"Name": "b", "Type": "host"}, g.GetNode("b").Metadata)
}

func TestCloseEvent(t *testing.T) {
	createEventTime := time.Unix(100, 100)
	testCases := []struct {
		desc                 string
		event                model.EventCloseInput
		nodes                []*graph.Node
		expectedNodeMetadata graph.Metadata
		expectedError        string
	}{
		{
			desc: "trying to delete an event from a node without Metadata.Event produces an error",
			event: model.EventCloseInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "unable to delete event: Field not found",
		},
		{
			desc: "trying to delete an event from a node with Metadata.Event empty produces an error",
			event: model.EventCloseInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedError: "unable to delete event: event ID 'Prefix_someId' does not exists in node a",
		},
		{
			desc: "error if Metadata.Event does not have the proper type",
			event: model.EventCloseInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_otherId",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": map[string]string{},
			})},
			expectedError: "unable to delete event: invalid Metadata.Events format",
		},
		{
			desc: "trying to delete an event with empty ID produces an error",
			event: model.EventCloseInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedError: "ID should not be empty",
		},
		{
			desc: "trying to delete an event with empty query produces an error",
			event: model.EventCloseInput{
				GremlinQuery: "",
				ID:           "Prefix_someId",
			},
			expectedError: "parsing gremlin query: found \"\", expected `G`",
		},
		{
			desc: "deleting an event",
			event: model.EventCloseInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			},
		},
		{
			desc: "deleting an event from a node with two events",
			event: model.EventCloseInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
					"Prefix_otherId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_otherId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			g := newGraph(t)
			for _, n := range tC.nodes {
				g.AddNode(n)
			}

			r := Resolver{Graph: g}

			n, err := r.closeEvent(tC.event)
			if tC.expectedError != "" {
				assert.EqualError(t, err, tC.expectedError)
			} else {
				assert.Equal(t, graph.Identifier("a"), n.ID)
				// Check the node has been modified
				modNode := g.GetNode(graph.Identifier("a"))
				assert.Equal(t, tC.expectedNodeMetadata, modNode.Metadata)
			}
		})
	}
}

func TestCreateEvent(t *testing.T) {
	ttl200 := 200
	ttl400 := 400
	createEventTime := time.Unix(100, 100)

	testCases := []struct {
		desc                 string
		event                model.EventAddInput
		nodes                []*graph.Node
		expectedNodeMetadata graph.Metadata
		expectedError        string
	}{
		{
			desc: "adding an event with empty query produces an error",
			event: model.EventAddInput{
				GremlinQuery: "",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.EventKeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "parsing gremlin query: found \"\", expected `G`",
		},
		{
			desc: "adding an event with empty ID produces an error",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "",
				Level:        model.EventLevelInfo,
				Payload:      []*model.EventKeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "ID should not be empty",
		},
		{
			desc: "adding an event to a non existant node produces an error",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','z')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.EventKeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "expecting only one node, gremlin query returned 0",
		},
		{
			desc: "adding an event to a node without Metadata.Event",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.EventKeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "adding an event to a node with Metadata.Event empty",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.EventKeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "adding an event with nil Payload",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				TTL:          &ttl200,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "adding an event with nil TTL",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelCritical,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       0,
						Level:     "CRITICAL",
						Payload:   []PayloadValues{},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "updating an event, new event define all values except CreatedAt",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelWarning,
				Payload:      []*model.EventKeyValue{{Key: "new", Value: "value"}},
				TTL:          &ttl400,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "original", Value: "value"}},
						CreatedAt: graph.Time(time.Unix(0, 0)),
						UpdatedAt: graph.Time(time.Unix(0, 0)),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       400,
						Level:     "WARNING",
						Payload:   []PayloadValues{{Key: "new", Value: "value"}},
						CreatedAt: graph.Time(time.Unix(0, 0)),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "add a second event",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_otherId",
				Level:        model.EventLevelWarning,
				Payload:      []*model.EventKeyValue{{Key: "new", Value: "value"}},
				TTL:          &ttl400,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "original", Value: "value"}},
						CreatedAt: graph.Time(time.Unix(0, 0)),
						UpdatedAt: graph.Time(time.Unix(0, 0)),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "original", Value: "value"}},
						CreatedAt: graph.Time(time.Unix(0, 0)),
						UpdatedAt: graph.Time(time.Unix(0, 0)),
					},
					"Prefix_otherId": Event{
						TTL:       400,
						Level:     "WARNING",
						Payload:   []PayloadValues{{Key: "new", Value: "value"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "error if Metadata.Event does not have the proper type",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_otherId",
				Level:        model.EventLevelWarning,
				Payload:      []*model.EventKeyValue{{Key: "new", Value: "value"}},
				TTL:          &ttl400,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": map[string]string{},
			})},
			expectedError: "unable to add event: invalid Metadata.Events format",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			g := newGraph(t)
			for _, n := range tC.nodes {
				g.AddNode(n)
			}

			r := Resolver{Graph: g}

			_, err := r.createEvent(
				tC.event.ID,
				tC.event.GremlinQuery,
				tC.event.Level,
				tC.event.TTL,
				tC.event.Payload,
				createEventTime,
			)
			if tC.expectedError != "" {
				assert.EqualError(t, err, tC.expectedError)
			} else {
				// Check the node has been modified
				modNode := g.GetNode(graph.Identifier("a"))
				assert.Equal(t, tC.expectedNodeMetadata, modNode.Metadata)
			}
		})
	}
}
